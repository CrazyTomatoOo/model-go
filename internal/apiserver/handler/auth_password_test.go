package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

func passwordRequest(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	return postJSON(engine, "/password", body)
}

// passwordPost sends a body with a valid-looking session cookie so the
// handler reaches the service layer.
func passwordPost(engine *gin.Engine, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/password", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: "sid-123"})
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestPasswordChangeSuccess(t *testing.T) {
	var gotSid, gotOld, gotNew, gotIP string
	var gotUser store.User
	auth := &stubAuth{changeFn: func(_ context.Context, sid string, user store.User, oldPassword, newPassword, ip string) error {
		gotSid, gotOld, gotNew, gotIP = sid, oldPassword, newPassword, ip
		gotUser = user
		return nil
	}}
	engine := newAuthEngine(auth, true)
	req := httptest.NewRequest(http.MethodPost, "/password", strings.NewReader(`{"old_password":"Old-Strong-Passw0rd!","new_password":"New-Strong-Passw0rd!"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: middleware.CookieName, Value: "sid-123"})
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "sid-123", gotSid)
	require.Equal(t, "Old-Strong-Passw0rd!", gotOld)
	require.Equal(t, "New-Strong-Passw0rd!", gotNew)
	require.Equal(t, "203.0.113.9", gotIP)
	require.Equal(t, int64(7), gotUser.ID)
	require.Equal(t, "alice", gotUser.Username)
}

func TestPasswordChangeMissingFields(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		t.Fatal("service must not be called on validation failure")
		return nil
	}}
	rec := passwordRequest(newAuthEngine(auth, true), `{"old_password":"","new_password":""}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"old_password","message":"旧密码不能为空"},{"field":"new_password","message":"新密码不能为空"}]}`, rec.Body.String())
}

func TestPasswordChangeMalformedBody(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		t.Fatal("service must not be called on malformed body")
		return nil
	}}
	rec := passwordRequest(newAuthEngine(auth, true), `not-json`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"body","message":"请求体必须是 JSON"}]}`, rec.Body.String())
}

func TestPasswordChangeWrongOld(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		return service.ErrWrongOldPassword
	}}
	rec := passwordPost(newAuthEngine(auth, true), `{"old_password":"wrong","new_password":"New-Strong-Passw0rd!"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"old_password","message":"旧密码不正确"}]}`, rec.Body.String())
}

func TestPasswordChangePolicyRejected(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		return &service.Error{Kind: service.KindValidationError, Field: "new_password", Message: "密码长度不足，至少需要 16 位"}
	}}
	rec := passwordPost(newAuthEngine(auth, true), `{"old_password":"Old-Strong-Passw0rd!","new_password":"short"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"new_password","message":"密码长度不足，至少需要 16 位"}]}`, rec.Body.String())
}

func TestPasswordChangeReuseRejected(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		return &service.Error{Kind: service.KindValidationError, Field: "new_password", Message: "新密码不能与旧密码相同"}
	}}
	rec := passwordPost(newAuthEngine(auth, true), `{"old_password":"Old-Strong-Passw0rd!","new_password":"Old-Strong-Passw0rd!"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"new_password","message":"新密码不能与旧密码相同"}]}`, rec.Body.String())
}

func TestPasswordChangeWithoutCookie(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		t.Fatal("service must not be called without a session cookie")
		return nil
	}}
	rec := passwordRequest(newAuthEngine(auth, true), `{"old_password":"Old-Strong-Passw0rd!","new_password":"New-Strong-Passw0rd!"}`)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.JSONEq(t, `{"code":"SESSION_EXPIRED","message":"会话已过期，请重新登录"}`, rec.Body.String())
}

func TestPasswordChangeInternalErrorOpaque(t *testing.T) {
	auth := &stubAuth{changeFn: func(context.Context, string, store.User, string, string, string) error {
		return context.DeadlineExceeded
	}}
	rec := passwordPost(newAuthEngine(auth, true), `{"old_password":"Old-Strong-Passw0rd!","new_password":"New-Strong-Passw0rd!"}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"code":"INTERNAL","message":"服务器内部错误"}`, rec.Body.String())
}
