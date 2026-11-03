// Package handler maps HTTP requests to the auth service and renders the
// spec §4/§4.5 JSON envelopes.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// AuthService is the login service seam the handlers consume.
type AuthService interface {
	Login(ctx context.Context, username, password, ip string) (service.LoginResult, error)
	Logout(ctx context.Context, sid string, user store.User, ip string) error
	Me(ctx context.Context, sid string) (store.User, string, error)
}

// AuthHandler serves the /api/v1/auth routes.
type AuthHandler struct {
	auth AuthService
}

// NewAuthHandler builds an AuthHandler over the given service.
func NewAuthHandler(auth AuthService) *AuthHandler {
	return &AuthHandler{auth: auth}
}

// writeError renders the spec §4.5 envelope {code, message}.
func writeError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{"code": code, "message": message})
}

// writeFieldErrors renders a 400 VALIDATION_ERROR with field-level detail.
func writeFieldErrors(c *gin.Context, fields []gin.H) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code":    "VALIDATION_ERROR",
		"message": "请求参数校验失败",
		"errors":  fields,
	})
}

// writeInternal logs the cause through gin's error chain (surfaced by the
// accesslog/recovery middleware) and renders an opaque 500.
func writeInternal(c *gin.Context, err error) {
	_ = c.Error(err)
	writeError(c, http.StatusInternalServerError, "INTERNAL", "服务器内部错误")
}

// writeAuthError maps a service error to its §4.5 status and envelope.
func writeAuthError(c *gin.Context, err error) {
	var serr *service.Error
	if !errors.As(err, &serr) {
		writeInternal(c, err)
		return
	}
	switch serr.Kind {
	case service.KindInvalidCredentials:
		writeError(c, http.StatusUnauthorized, service.KindInvalidCredentials, serr.Message)
	case service.KindAccountPending:
		writeError(c, http.StatusForbidden, service.KindAccountPending, serr.Message)
	case service.KindAccountDisabled:
		writeError(c, http.StatusForbidden, service.KindAccountDisabled, serr.Message)
	case service.KindRateLimited:
		writeError(c, http.StatusTooManyRequests, service.KindRateLimited, serr.Message)
	case service.KindSessionExpired:
		writeError(c, http.StatusUnauthorized, service.KindSessionExpired, serr.Message)
	default:
		writeInternal(c, err)
	}
}
