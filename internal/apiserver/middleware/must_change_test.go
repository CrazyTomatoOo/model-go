package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

func testGateEngine(t *testing.T, mustChange bool) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(middleware.UserKey, store.User{ID: 7, Username: "alice", Role: store.RoleUser, Status: store.StatusActive, MustChangePassword: mustChange})
		c.Next()
	}, middleware.MustChangeGate())
	engine.GET("/api/v1/auth/me", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/api/v1/auth/me", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/api/v1/auth/password", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/api/v1/auth/logout", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/api/v1/users", func(c *gin.Context) { c.Status(http.StatusOK) })
	return engine
}

func TestMustChangeGateAllowsOnlyExemptPaths(t *testing.T) {
	engine := testGateEngine(t, true)
	cases := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"me is exempt", http.MethodGet, "/api/v1/auth/me", http.StatusOK},
		{"password change is exempt", http.MethodPost, "/api/v1/auth/password", http.StatusOK},
		{"logout is gated", http.MethodPost, "/api/v1/auth/logout", http.StatusForbidden},
		{"protected get is gated", http.MethodGet, "/api/v1/users", http.StatusForbidden},
		{"non-exempt method on exempt path is gated", http.MethodPost, "/api/v1/auth/me", http.StatusForbidden},
		{"non-exempt path on exempt method is gated", http.MethodGet, "/api/v1/auth/password", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.want, rec.Code)
		})
	}
}

func TestMustChangeGatePassesCleanUsers(t *testing.T) {
	engine := testGateEngine(t, false)
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodGet, "/api/v1/users"},
		{http.MethodPost, "/api/v1/auth/me"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
		require.Equal(t, http.StatusOK, rec.Code, "%s %s", tc.method, tc.path)
	}
}

func TestMustChangeGateRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(middleware.MustChangeGate())
	engine.GET("/api/v1/users", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/users", nil))
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}

func TestMustChangeGateDeniedBody(t *testing.T) {
	engine := testGateEngine(t, true)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"code":"PASSWORD_CHANGE_REQUIRED","message":"需要修改密码后才能继续操作"}`, rec.Body.String())
}
