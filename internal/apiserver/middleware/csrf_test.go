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

func testCSRFEngine(t *testing.T) (*gin.Engine, *store.SessionStore) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	_, rdb := newTestRedis(t)
	sessions := store.NewSessionStore(rdb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	users := (&stubUserStore{}).found(store.User{ID: 7, Username: "alice", Role: store.RoleUser})
	engine := gin.New()
	engine.Use(middleware.Session(sessions, users, logger), middleware.CSRF())
	engine.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine, sessions
}

func csrfRequest(engine *gin.Engine, method, path, sid, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	if sid != "" {
		req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: sid})
	}
	if token != "" {
		req.Header.Set(middleware.CSRFHeader, token)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestCSRFPostWithoutToken(t *testing.T) {
	engine, sessions := testCSRFEngine(t)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := csrfRequest(engine, http.MethodPost, "/protected", sess.SID, "")
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"code":"CSRF_TOKEN_INVALID","message":"CSRF token 无效"}`, rec.Body.String())
}

func TestCSRFWrongToken(t *testing.T) {
	engine, sessions := testCSRFEngine(t)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := csrfRequest(engine, http.MethodPost, "/protected", sess.SID, "wrong-token")
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCSRFValidToken(t *testing.T) {
	engine, sessions := testCSRFEngine(t)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := csrfRequest(engine, http.MethodPost, "/protected", sess.SID, sess.CSRF)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestCSRFGetExempt(t *testing.T) {
	engine, sessions := testCSRFEngine(t)
	sess, err := sessions.Create(context.Background(), 7)
	require.NoError(t, err)

	rec := csrfRequest(engine, http.MethodGet, "/protected", sess.SID, "")
	require.Equal(t, http.StatusOK, rec.Code)
}
