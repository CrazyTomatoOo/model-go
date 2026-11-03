package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/handler"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

type stubAuth struct {
	loginFn  func(ctx context.Context, username, password, ip string) (service.LoginResult, error)
	logoutFn func(ctx context.Context, sid string, user store.User, ip string) error
	meFn     func(ctx context.Context, sid string) (store.User, string, error)
}

func (s *stubAuth) Login(ctx context.Context, username, password, ip string) (service.LoginResult, error) {
	return s.loginFn(ctx, username, password, ip)
}

func (s *stubAuth) Logout(ctx context.Context, sid string, user store.User, ip string) error {
	return s.logoutFn(ctx, sid, user, ip)
}

func (s *stubAuth) Me(ctx context.Context, sid string) (store.User, string, error) {
	return s.meFn(ctx, sid)
}

const testUserJSON = `{"id":7,"username":"alice","role":"user","status":"active","must_change_password":false}`

func newAuthEngine(auth handler.AuthService, withUser bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	if withUser {
		engine.Use(func(c *gin.Context) {
			c.Set(middleware.UserKey, store.User{ID: 7, Username: "alice", Role: store.RoleUser, Status: store.StatusActive})
			c.Next()
		})
	}
	h := handler.NewAuthHandler(auth)
	engine.POST("/login", h.Login)
	engine.POST("/logout", h.Logout)
	engine.GET("/me", h.Me)
	return engine
}

func postJSON(engine *gin.Engine, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestLoginSuccessSetsSessionCookie(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{
			User: store.User{ID: 7, Username: "alice", Role: store.RoleUser, Status: store.StatusActive, PasswordHash: "secret-hash"},
			SID:  "sid-123",
			CSRF: "csrf-abc",
		}, nil
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"s3cret"}`)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"user":`+testUserJSON+`,"csrf_token":"csrf-abc"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "password_hash", "password hash must never be returned")

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	require.Equal(t, middleware.CookieName, cookie.Name)
	require.Equal(t, "sid-123", cookie.Value)
	require.True(t, cookie.HttpOnly)
	require.True(t, cookie.Secure)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	require.Equal(t, "/", cookie.Path)
}

func TestLoginMissingUsername(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		t.Fatal("service must not be called on validation failure")
		return service.LoginResult{}, nil
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"","password":"s3cret"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"username","message":"用户名不能为空"}]}`, rec.Body.String())
}

func TestLoginMalformedBody(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		t.Fatal("service must not be called on malformed body")
		return service.LoginResult{}, nil
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `not-json`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"body","message":"请求体必须是 JSON"}]}`, rec.Body.String())
}

func TestLoginWrongCredentials(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{}, service.ErrInvalidCredentials
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"wrong"}`)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"INVALID_CREDENTIALS","message":"用户名或密码错误"}`, rec.Body.String())
}

func TestLoginAccountPending(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{}, service.ErrAccountPending
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"s3cret"}`)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_PENDING","message":"账号待安全管理员审核启用"}`, rec.Body.String())
}

func TestLoginAccountDisabled(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{}, service.ErrAccountDisabled
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"s3cret"}`)

	require.Equal(t, http.StatusForbidden, rec.Code)
	require.JSONEq(t, `{"code":"ACCOUNT_DISABLED","message":"账号已停用，请联系管理员"}`, rec.Body.String())
}

func TestLoginRateLimited(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{}, service.ErrRateLimited
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"s3cret"}`)

	require.Equal(t, http.StatusTooManyRequests, rec.Code)
	require.JSONEq(t, `{"code":"RATE_LIMITED","message":"登录尝试过于频繁，请稍后再试"}`, rec.Body.String())
}

func TestLoginInternalErrorIsOpaque(t *testing.T) {
	auth := &stubAuth{loginFn: func(context.Context, string, string, string) (service.LoginResult, error) {
		return service.LoginResult{}, context.DeadlineExceeded
	}}
	rec := postJSON(newAuthEngine(auth, false), "/login", `{"username":"alice","password":"s3cret"}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"code":"INTERNAL","message":"服务器内部错误"}`, rec.Body.String())
}

func TestLogoutDeletesSessionAndClearsCookie(t *testing.T) {
	var gotSid, gotIP string
	auth := &stubAuth{logoutFn: func(_ context.Context, sid string, user store.User, ip string) error {
		gotSid, gotIP = sid, ip
		require.Equal(t, int64(7), user.ID)
		return nil
	}}
	engine := newAuthEngine(auth, true)
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: "sid-123"})
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "sid-123", gotSid)
	require.Equal(t, "203.0.113.9", gotIP)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, middleware.CookieName, cookies[0].Name)
	require.Equal(t, -1, cookies[0].MaxAge)
}

func TestLogoutWithoutCookie(t *testing.T) {
	auth := &stubAuth{logoutFn: func(context.Context, string, store.User, string) error {
		t.Fatal("service must not be called without a session cookie")
		return nil
	}}
	rec := postJSON(newAuthEngine(auth, true), "/logout", "")

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}

func TestMeReturnsUserAndCSRF(t *testing.T) {
	auth := &stubAuth{meFn: func(_ context.Context, sid string) (store.User, string, error) {
		require.Equal(t, "sid-123", sid)
		return store.User{ID: 7, Username: "alice", Role: store.RoleUser, Status: store.StatusActive, PasswordHash: "secret-hash"}, "csrf-abc", nil
	}}
	engine := newAuthEngine(auth, false)
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: "sid-123"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"user":`+testUserJSON+`,"csrf_token":"csrf-abc"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "password_hash", "password hash must never be returned")
}

func TestMeSessionExpired(t *testing.T) {
	auth := &stubAuth{meFn: func(context.Context, string) (store.User, string, error) {
		return store.User{}, "", service.ErrSessionExpired
	}}
	engine := newAuthEngine(auth, false)
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: "sid-123"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}
