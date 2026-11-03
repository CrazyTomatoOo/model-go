package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

func testRoleEngine(t *testing.T, role string) (*gin.Engine, *store.SessionStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	_, rdb := newTestRedis(t)
	sessions := store.NewSessionStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := (&stubUserStore{}).found(store.User{ID: 7, Username: "alice", Role: role})
	engine := gin.New()
	engine.Use(middleware.Session(sessions, users, logger), middleware.RequireRole(store.RoleSysadmin))
	engine.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine, sessions
}

func TestRequireRoleAllowed(t *testing.T) {
	engine, sessions := testRoleEngine(t, store.RoleSysadmin)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: sess.SID})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRoleDenied(t *testing.T) {
	engine, sessions := testRoleEngine(t, store.RoleUser)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: sess.SID})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"code":"FORBIDDEN","message":"无权限执行此操作"}`, rec.Body.String())
}

func TestRequireRoleUnauthenticated(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.RequireRole(store.RoleSysadmin))
	engine.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/protected", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}

var _ = context.Background
