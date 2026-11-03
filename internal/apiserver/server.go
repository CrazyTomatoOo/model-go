package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/handler"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	apistore "github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/pkg/store"
)

// Config holds the env-based apiserver settings (MODELGO_* variables).
type Config struct {
	HTTPAddr    string
	PostgresDSN string
	RedisAddr   string
}

// ConfigFromEnv builds Config from the environment. MODELGO_POSTGRES_DSN and
// MODELGO_REDIS_ADDR are required (fail-fast on absence); MODELGO_HTTP_ADDR
// defaults to ":8080".
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		HTTPAddr:    envOr("MODELGO_HTTP_ADDR", ":8080"),
		PostgresDSN: os.Getenv("MODELGO_POSTGRES_DSN"),
		RedisAddr:   os.Getenv("MODELGO_REDIS_ADDR"),
	}
	if cfg.PostgresDSN == "" {
		return Config{}, errors.New("MODELGO_POSTGRES_DSN is required")
	}
	if cfg.RedisAddr == "" {
		return Config{}, errors.New("MODELGO_REDIS_ADDR is required")
	}
	return cfg, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// NewEngine builds the gin engine: the spec §8 middleware chain (requestid
// → recovery → securityheaders → accesslog), the GET /healthz route, and the
// /api/v1/auth routes behind the session and CSRF middleware (§4.1).
func NewEngine(pool *pgxpool.Pool, rdb *redis.Client, logger *slog.Logger) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(
		middleware.RequestID(),
		middleware.Recovery(logger),
		middleware.SecurityHeaders(),
		middleware.AccessLog(logger),
	)
	engine.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	users := apistore.NewUserStore(pool)
	sessions := apistore.NewSessionStore(rdb)
	locks := apistore.NewLockStore(rdb)
	rate := apistore.NewRateLimiter(rdb)
	audits := apistore.NewAuditStore(pool)
	authService := service.NewAuthService(users, sessions, locks, rate, audits, logger)
	auth := handler.NewAuthHandler(authService)
	audit := handler.NewAuditHandler(audits)
	usersService := service.NewUsersService(users, sessions, audits, logger)
	usersHandler := handler.NewUsersHandler(usersService)
	secadminService := service.NewSecadminService(users, sessions, locks, audits, logger)
	sec := handler.NewSecadminHandler(secadminService)

	v1 := engine.Group("/api/v1")
	v1.POST("/auth/login", auth.Login)
	authed := v1.Group("", middleware.Session(sessions, users, logger), middleware.CSRF(), middleware.MustChangeGate())
	authed.POST("/auth/logout", auth.Logout)
	authed.GET("/auth/me", auth.Me)
	// GET /audit-logs is the auditadmin-only read API (spec §4.4/§4.6);
	// the role check runs per-route behind session+CSRF.
	authed.GET("/audit-logs", middleware.RequireRole(apistore.RoleAuditadmin), audit.List)
	sysadmin := authed.Group("", middleware.RequireRole(apistore.RoleSysadmin))
	sysadmin.POST("/users", usersHandler.Create)
	sysadmin.PATCH("/users/:id/password", usersHandler.ResetPassword)
	sysadmin.POST("/users/:id/disable", usersHandler.Disable)
	authed.POST("/users/:id/approve", middleware.RequireRole(apistore.RoleSecadmin), sec.Approve)
	authed.POST("/users/:id/unlock", middleware.RequireRole(apistore.RoleSecadmin), sec.Unlock)
	authed.GET("/sessions", middleware.RequireRole(apistore.RoleSecadmin), sec.ListSessions)
	authed.DELETE("/sessions/:sid", middleware.RequireRole(apistore.RoleSecadmin), sec.ForceLogout)
	authed.POST("/auth/password", auth.Password)
	return engine
}

// Run connects Postgres and Redis, applies migrations (fail-fast), serves
// HTTP, and shuts down gracefully when ctx is cancelled.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	pool, err := pgxpool.New(ctx, cfg.PostgresDSN)
	if err != nil {
		return fmt.Errorf("connect postgres: %w", err)
	}
	defer pool.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	if err := store.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	if err := rdb.Ping(pingCtx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	engine := NewEngine(pool, rdb, logger)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           engine,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
	}()
	logger.Info("apiserver serving", "addr", cfg.HTTPAddr)

	select {
	case err := <-serveErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	}
}
