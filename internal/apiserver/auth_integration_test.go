package apiserver_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	apistore "github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
	"github.com/CrazyTomatoOo/model-go/internal/password"
	"github.com/CrazyTomatoOo/model-go/internal/testutil"
	"github.com/CrazyTomatoOo/model-go/pkg/store"
)

const testPassword = "Str0ng-Passw0rd!"

type authStack struct {
	pool   *pgxpool.Pool
	rdb    *redis.Client
	engine *gin.Engine
	ctx    context.Context
}

func newAuthStack(t *testing.T) *authStack {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	t.Cleanup(cancel)
	pool, rdb := testutil.StartStack(ctx, t)
	require.NoError(t, store.Migrate(ctx, pool))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &authStack{pool: pool, rdb: rdb, engine: apiserver.NewEngine(pool, rdb, logger), ctx: ctx}
}

func (s *authStack) seedUser(t *testing.T, username, role, plaintext string) {
	t.Helper()
	hash, err := password.Hash(plaintext)
	require.NoError(t, err)
	_, err = s.pool.Exec(s.ctx,
		`INSERT INTO users (username, password_hash, role, status) VALUES ($1, $2, $3, 'active')`,
		username, hash, role)
	require.NoError(t, err)
}

func loginRequest(engine *gin.Engine, username, password, ip string) *httptest.ResponseRecorder {
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func sidOf(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies, "login response must set the session cookie")
	require.Equal(t, middleware.CookieName, cookies[0].Name)
	return cookies[0].Value
}

func authedRequest(engine *gin.Engine, method, path, sid, csrf, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: sid})
	if csrf != "" {
		req.Header.Set(middleware.CSRFHeader, csrf)
	}
	req.RemoteAddr = ip + ":1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestIntegration_LoginMeLogout(t *testing.T) {
	s := newAuthStack(t)
	s.seedUser(t, "alice", "user", testPassword)

	// When — alice logs in with the right password
	login := loginRequest(s.engine, "alice", testPassword, "203.0.113.10")
	require.Equal(t, http.StatusOK, login.Code)
	require.JSONEq(t, `{"user":{"id":1,"username":"alice","role":"user","status":"active","must_change_password":true},"csrf_token":"`+csrfToken(login.Body.String())+`"}`, login.Body.String())
	require.NotContains(t, login.Body.String(), "password_hash", "password hash must never be returned")
	require.True(t, login.Result().Cookies()[0].HttpOnly)
	require.True(t, login.Result().Cookies()[0].Secure)
	require.Equal(t, http.SameSiteLaxMode, login.Result().Cookies()[0].SameSite)

	sid := sidOf(t, login)
	csrf := csrfToken(login.Body.String())

	// Then — me returns the user and the same CSRF token
	me := authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", sid, "", "203.0.113.10")
	require.Equal(t, http.StatusOK, me.Code)
	require.JSONEq(t, `{"user":{"id":1,"username":"alice","role":"user","status":"active","must_change_password":true},"csrf_token":"`+csrf+`"}`, me.Body.String())

	// When — alice logs out with the CSRF token
	logoutResp := authedRequest(s.engine, http.MethodPost, "/api/v1/auth/logout", sid, csrf, "203.0.113.10")
	require.Equal(t, http.StatusNoContent, logoutResp.Code)
	require.Equal(t, -1, logoutResp.Result().Cookies()[0].MaxAge)

	// Then — the session is gone
	meAgain := authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", sid, "", "203.0.113.10")
	require.Equal(t, http.StatusUnauthorized, meAgain.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, meAgain.Body.String())

	// Then — audit rows were written (spec §7)
	var bootstrap, loginSuccess, logout int
	err := s.pool.QueryRow(s.ctx,
		`SELECT count(*) FILTER (WHERE action = 'admin.bootstrap'),
		        count(*) FILTER (WHERE action = 'auth.login_success'),
		        count(*) FILTER (WHERE action = 'auth.logout')
		   FROM audit_logs`).Scan(&bootstrap, &loginSuccess, &logout)
	require.NoError(t, err)
	require.Zero(t, bootstrap, "direct INSERT seeds no admin.bootstrap audit row")
	require.Equal(t, 1, loginSuccess)
	require.Equal(t, 1, logout)
}

func TestIntegration_LoginFailuresLockoutRateLimitCSRFAndRole(t *testing.T) {
	s := newAuthStack(t)
	s.seedUser(t, "alice", "user", testPassword)
	s.seedUser(t, "bob", "sysadmin", testPassword)
	s.seedUser(t, "carol", "user", testPassword)
	s.seedUser(t, "dave", "user", testPassword)

	// Unified 401: wrong password and unknown user are indistinguishable
	wrong := loginRequest(s.engine, "alice", "wrong-password", "203.0.113.20")
	require.Equal(t, http.StatusUnauthorized, wrong.Code)
	ghost := loginRequest(s.engine, "ghost", testPassword, "203.0.113.21")
	require.Equal(t, http.StatusUnauthorized, ghost.Code)
	require.JSONEq(t, wrong.Body.String(), ghost.Body.String(), "unknown user must not be distinguishable from a wrong password")
	require.JSONEq(t, `{"code":"INVALID_CREDENTIALS","message":"用户名或密码错误"}`, wrong.Body.String())

	// Lockout: five failures lock the account; the right password still 401s
	for i := 0; i < 4; i++ {
		require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "alice", "wrong-password", "203.0.113.22").Code)
	}
	require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "alice", testPassword, "203.0.113.23").Code, "locked account must 401 even with the right password")

	// Rate limit: the 21st attempt from one IP in a minute is rejected
	for i := 0; i < 20; i++ {
		require.Equal(t, http.StatusUnauthorized, loginRequest(s.engine, "ghost", testPassword, "203.0.113.30").Code)
	}
	limited := loginRequest(s.engine, "ghost", testPassword, "203.0.113.30")
	require.Equal(t, http.StatusTooManyRequests, limited.Code)
	require.JSONEq(t, `{"code":"RATE_LIMITED","message":"登录尝试过于频繁，请稍后再试"}`, limited.Body.String())

	// CSRF: GET is exempt; state-changing requests need the session token
	login := loginRequest(s.engine, "carol", testPassword, "203.0.113.40")
	require.Equal(t, http.StatusOK, login.Code)
	sid := sidOf(t, login)
	csrf := csrfToken(login.Body.String())

	require.Equal(t, http.StatusOK, authedRequest(s.engine, http.MethodGet, "/api/v1/auth/me", sid, "", "203.0.113.40").Code)
	noToken := authedRequest(s.engine, http.MethodPost, "/api/v1/auth/logout", sid, "", "203.0.113.40")
	require.Equal(t, http.StatusForbidden, noToken.Code)
	require.JSONEq(t, `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token 无效"}`, noToken.Body.String())
	require.Equal(t, http.StatusForbidden, authedRequest(s.engine, http.MethodPost, "/api/v1/auth/logout", sid, "wrong-token", "203.0.113.40").Code)
	require.Equal(t, http.StatusNoContent, authedRequest(s.engine, http.MethodPost, "/api/v1/auth/logout", sid, csrf, "203.0.113.40").Code)

	// Role: a sysadmin-gated route denies the plain user and admits the admin
	gin.SetMode(gin.ReleaseMode)
	users := apistore.NewUserStore(s.pool)
	sessions := apistore.NewSessionStore(s.rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	roleEngine := gin.New()
	roleEngine.Use(
		middleware.RequestID(),
		middleware.Recovery(logger),
		middleware.SecurityHeaders(),
		middleware.AccessLog(logger),
		middleware.Session(sessions, users, logger),
		middleware.CSRF(),
		middleware.RequireRole(apistore.RoleSysadmin),
	)
	roleEngine.POST("/api/v1/admin/gate", func(c *gin.Context) { c.Status(http.StatusOK) })

	daveLogin := loginRequest(s.engine, "dave", testPassword, "203.0.113.45")
	daveDenied := authedRequest(roleEngine, http.MethodPost, "/api/v1/admin/gate", sidOf(t, daveLogin), csrfToken(daveLogin.Body.String()), "203.0.113.45")
	require.Equal(t, http.StatusForbidden, daveDenied.Code)
	require.JSONEq(t, `{"code":"FORBIDDEN","message":"无权限执行此操作"}`, daveDenied.Body.String())

	bobLogin := loginRequest(s.engine, "bob", testPassword, "203.0.113.46")
	bobAllowed := authedRequest(roleEngine, http.MethodPost, "/api/v1/admin/gate", sidOf(t, bobLogin), csrfToken(bobLogin.Body.String()), "203.0.113.46")
	require.Equal(t, http.StatusOK, bobAllowed.Code)

	// Then — failure audit rows carry the reason
	var reasons []string
	rows, err := s.pool.Query(s.ctx, `SELECT detail->>'reason' FROM audit_logs WHERE action = 'auth.login_failure'`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var reason string
		require.NoError(t, rows.Scan(&reason))
		reasons = append(reasons, reason)
	}
	require.Contains(t, reasons, "invalid_password")
	require.Contains(t, reasons, "user_not_found")
	require.Contains(t, reasons, "locked")
}

func csrfToken(body string) string {
	var parsed struct {
		CSRF string `json:"csrf_token"`
	}
	_ = json.Unmarshal([]byte(body), &parsed)
	return parsed.CSRF
}
