package handler_test

import (
	"context"
	"errors"
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

type stubUsers struct {
	createFn  func(ctx context.Context, actor store.User, in service.CreateUserInput, ip string) (service.CreatedUser, error)
	resetFn   func(ctx context.Context, actor store.User, userID int64, newPassword, ip string) error
	disableFn func(ctx context.Context, actor store.User, userID int64, ip string) error
}

func (s *stubUsers) Create(ctx context.Context, actor store.User, in service.CreateUserInput, ip string) (service.CreatedUser, error) {
	return s.createFn(ctx, actor, in, ip)
}

func (s *stubUsers) ResetPassword(ctx context.Context, actor store.User, userID int64, newPassword, ip string) error {
	return s.resetFn(ctx, actor, userID, newPassword, ip)
}

func (s *stubUsers) Disable(ctx context.Context, actor store.User, userID int64, ip string) error {
	return s.disableFn(ctx, actor, userID, ip)
}

func newUsersEngine(users handler.UsersService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(middleware.UserKey, store.User{ID: 1, Username: "bob", Role: store.RoleSysadmin, Status: store.StatusActive})
		c.Next()
	})
	h := handler.NewUsersHandler(users)
	engine.POST("/users", h.Create)
	engine.PATCH("/users/:id/password", h.ResetPassword)
	engine.POST("/users/:id/disable", h.Disable)
	return engine
}

func usersPostJSON(engine *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:1234"
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func TestUsersCreateSuccess(t *testing.T) {
	var gotActor store.User
	var gotIn service.CreateUserInput
	var gotIP string
	users := &stubUsers{createFn: func(_ context.Context, actor store.User, in service.CreateUserInput, ip string) (service.CreatedUser, error) {
		gotActor, gotIn, gotIP = actor, in, ip
		return service.CreatedUser{ID: 42, Status: store.StatusPending}, nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users",
		`{"username":"carol","password":"s3cret-pass-123!","role":"user"}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.JSONEq(t, `{"id":42,"status":"pending"}`, rec.Body.String())
	require.Equal(t, int64(1), gotActor.ID)
	require.Equal(t, store.RoleSysadmin, gotActor.Role)
	require.Equal(t, service.CreateUserInput{Username: "carol", Password: "s3cret-pass-123!", Role: store.RoleUser}, gotIn)
	require.Equal(t, "203.0.113.9", gotIP)
}

func TestUsersCreateIgnoresServerControlledFields(t *testing.T) {
	var gotIn service.CreateUserInput
	users := &stubUsers{createFn: func(_ context.Context, actor store.User, in service.CreateUserInput, ip string) (service.CreatedUser, error) {
		gotIn = in
		return service.CreatedUser{ID: 42, Status: store.StatusPending}, nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users",
		`{"username":"carol","password":"s3cret-pass-123!","role":"user","status":"active","must_change_password":false,"created_by":99}`)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, service.CreateUserInput{Username: "carol", Password: "s3cret-pass-123!", Role: store.RoleUser}, gotIn,
		"server-controlled fields in the body must be ignored")
}

func TestUsersCreateMalformedBody(t *testing.T) {
	users := &stubUsers{createFn: func(context.Context, store.User, service.CreateUserInput, string) (service.CreatedUser, error) {
		t.Fatal("service must not be called on a malformed body")
		return service.CreatedUser{}, nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users", "not-json")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"body","message":"请求体必须是 JSON"}]}`, rec.Body.String())
}

func TestUsersCreateValidationError(t *testing.T) {
	users := &stubUsers{createFn: func(context.Context, store.User, service.CreateUserInput, string) (service.CreatedUser, error) {
		return service.CreatedUser{}, &service.Error{Kind: service.KindValidation, Field: "role", Message: "角色无效"}
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users",
		`{"username":"carol","password":"s3cret-pass-123!","role":"superuser"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"role","message":"角色无效"}]}`, rec.Body.String())
}

func TestUsersCreateConflict(t *testing.T) {
	users := &stubUsers{createFn: func(context.Context, store.User, service.CreateUserInput, string) (service.CreatedUser, error) {
		return service.CreatedUser{}, service.ErrUsernameTaken
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users",
		`{"username":"carol","password":"s3cret-pass-123!","role":"user"}`)

	require.Equal(t, http.StatusConflict, rec.Code)
	require.JSONEq(t, `{"code":"CONFLICT","message":"用户名已存在"}`, rec.Body.String())
}

func TestUsersCreateInternalErrorIsOpaque(t *testing.T) {
	users := &stubUsers{createFn: func(context.Context, store.User, service.CreateUserInput, string) (service.CreatedUser, error) {
		return service.CreatedUser{}, errors.New("boom")
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users",
		`{"username":"carol","password":"s3cret-pass-123!","role":"user"}`)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.JSONEq(t, `{"code":"INTERNAL","message":"服务器内部错误"}`, rec.Body.String())
}

func TestUsersResetPasswordSuccess(t *testing.T) {
	var gotActor store.User
	var gotUserID int64
	var gotPassword, gotIP string
	users := &stubUsers{resetFn: func(_ context.Context, actor store.User, userID int64, newPassword, ip string) error {
		gotActor, gotUserID, gotPassword, gotIP = actor, userID, newPassword, ip
		return nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPatch, "/users/7/password", `{"password":"new-pass-2026!"}`)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, int64(1), gotActor.ID)
	require.Equal(t, int64(7), gotUserID)
	require.Equal(t, "new-pass-2026!", gotPassword)
	require.Equal(t, "203.0.113.9", gotIP)
}

func TestUsersResetPasswordInvalidID(t *testing.T) {
	for _, id := range []string{"abc", "0", "-3"} {
		t.Run(id, func(t *testing.T) {
			users := &stubUsers{resetFn: func(context.Context, store.User, int64, string, string) error {
				t.Fatal("service must not be called with an invalid user id")
				return nil
			}}
			rec := usersPostJSON(newUsersEngine(users), http.MethodPatch, "/users/"+id+"/password", `{"password":"new-pass-2026!"}`)

			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"id","message":"用户 ID 无效"}]}`, rec.Body.String())
		})
	}
}

func TestUsersResetPasswordNotFound(t *testing.T) {
	users := &stubUsers{resetFn: func(context.Context, store.User, int64, string, string) error {
		return service.ErrNotFound
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPatch, "/users/999/password", `{"password":"new-pass-2026!"}`)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"用户不存在"}`, rec.Body.String())
}

func TestUsersResetPasswordValidationError(t *testing.T) {
	users := &stubUsers{resetFn: func(context.Context, store.User, int64, string, string) error {
		return &service.Error{Kind: service.KindValidation, Field: "password", Message: "新密码不能与当前或最近使用的密码相同"}
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPatch, "/users/7/password", `{"password":"old-pass-2025!"}`)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"password","message":"新密码不能与当前或最近使用的密码相同"}]}`, rec.Body.String())
}

func TestUsersResetPasswordMalformedBody(t *testing.T) {
	users := &stubUsers{resetFn: func(context.Context, store.User, int64, string, string) error {
		t.Fatal("service must not be called on a malformed body")
		return nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPatch, "/users/7/password", "not-json")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"body","message":"请求体必须是 JSON"}]}`, rec.Body.String())
}

func TestUsersDisableSuccess(t *testing.T) {
	var gotActor store.User
	var gotUserID int64
	var gotIP string
	users := &stubUsers{disableFn: func(_ context.Context, actor store.User, userID int64, ip string) error {
		gotActor, gotUserID, gotIP = actor, userID, ip
		return nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users/7/disable", "")

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, int64(1), gotActor.ID)
	require.Equal(t, int64(7), gotUserID)
	require.Equal(t, "203.0.113.9", gotIP)
}

func TestUsersDisableInvalidID(t *testing.T) {
	users := &stubUsers{disableFn: func(context.Context, store.User, int64, string) error {
		t.Fatal("service must not be called with an invalid user id")
		return nil
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users/abc/disable", "")

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.JSONEq(t, `{"code":"VALIDATION_ERROR","message":"请求参数校验失败","errors":[{"field":"id","message":"用户 ID 无效"}]}`, rec.Body.String())
}

func TestUsersDisableNotFound(t *testing.T) {
	users := &stubUsers{disableFn: func(context.Context, store.User, int64, string) error {
		return service.ErrNotFound
	}}
	rec := usersPostJSON(newUsersEngine(users), http.MethodPost, "/users/999/disable", "")

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.JSONEq(t, `{"code":"NOT_FOUND","message":"用户不存在"}`, rec.Body.String())
}
