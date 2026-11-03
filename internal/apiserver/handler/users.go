package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// UsersService is the user-management service seam the handlers consume.
type UsersService interface {
	Create(ctx context.Context, actor store.User, in service.CreateUserInput, ip string) (service.CreatedUser, error)
	ResetPassword(ctx context.Context, actor store.User, userID int64, newPassword, ip string) error
	Disable(ctx context.Context, actor store.User, userID int64, ip string) error
}

// UsersHandler serves the /api/v1/users routes (spec §4.2, sysadmin only).
type UsersHandler struct {
	users UsersService
}

// NewUsersHandler builds a UsersHandler over the given service.
func NewUsersHandler(users UsersService) *UsersHandler {
	return &UsersHandler{users: users}
}

// Create handles POST /api/v1/users (spec §4.2): only username/password/
// role are bound from the body; the created account is always pending with
// must_change_password=true and created_by set to the acting admin.
func (h *UsersHandler) Create(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFieldErrors(c, []gin.H{{"field": "body", "message": "请求体必须是 JSON"}})
		return
	}
	actor := c.MustGet(middleware.UserKey).(store.User)
	result, err := h.users.Create(c.Request.Context(), actor, service.CreateUserInput{
		Username: req.Username,
		Password: req.Password,
		Role:     req.Role,
	}, c.ClientIP())
	if err != nil {
		writeUsersError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"id": result.ID, "status": result.Status})
}

// ResetPassword handles PATCH /api/v1/users/:id/password (spec §4.2).
func (h *UsersHandler) ResetPassword(c *gin.Context) {
	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFieldErrors(c, []gin.H{{"field": "body", "message": "请求体必须是 JSON"}})
		return
	}
	userID, ok := userIDParam(c)
	if !ok {
		writeFieldErrors(c, []gin.H{{"field": "id", "message": "用户 ID 无效"}})
		return
	}
	actor := c.MustGet(middleware.UserKey).(store.User)
	if err := h.users.ResetPassword(c.Request.Context(), actor, userID, req.Password, c.ClientIP()); err != nil {
		writeUsersError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Disable handles POST /api/v1/users/:id/disable (spec §4.2).
func (h *UsersHandler) Disable(c *gin.Context) {
	userID, ok := userIDParam(c)
	if !ok {
		writeFieldErrors(c, []gin.H{{"field": "id", "message": "用户 ID 无效"}})
		return
	}
	actor := c.MustGet(middleware.UserKey).(store.User)
	if err := h.users.Disable(c.Request.Context(), actor, userID, c.ClientIP()); err != nil {
		writeUsersError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// userIDParam parses and validates the :id route parameter.
func userIDParam(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// writeUsersError maps a user-management service error to its §4.5 status
// and envelope.
func writeUsersError(c *gin.Context, err error) {
	var serr *service.Error
	if !errors.As(err, &serr) {
		writeInternal(c, err)
		return
	}
	switch serr.Kind {
	case service.KindValidation:
		writeFieldErrors(c, []gin.H{{"field": serr.Field, "message": serr.Message}})
	case service.KindNotFound:
		writeError(c, http.StatusNotFound, service.KindNotFound, serr.Message)
	case service.KindConflict:
		writeError(c, http.StatusConflict, service.KindConflict, serr.Message)
	default:
		writeInternal(c, err)
	}
}
