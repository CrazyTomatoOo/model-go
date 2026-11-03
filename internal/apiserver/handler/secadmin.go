package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/service"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// SecadminHandler serves the §4.3 security operations behind
// middleware.RequireRole("secadmin").
type SecadminHandler struct {
	secadmins *service.SecadminService
}

func NewSecadminHandler(secadmins *service.SecadminService) *SecadminHandler {
	return &SecadminHandler{secadmins: secadmins}
}

// Approve handles POST /api/v1/users/:id/approve: pending→active and
// disabled→active (204), 404 for an unknown user, 409 for an already-active
// account.
func (h *SecadminHandler) Approve(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeFieldErrors(c, []gin.H{{"field": "id", "message": "必须是整数"}})
		return
	}
	user := c.MustGet(middleware.UserKey).(store.User)
	if err := h.secadmins.Approve(c.Request.Context(), id, user, c.ClientIP()); err != nil {
		writeSecadminError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// Unlock handles POST /api/v1/users/:id/unlock: clears the login lock
// counter (204), 404 for an unknown user.
func (h *SecadminHandler) Unlock(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		writeFieldErrors(c, []gin.H{{"field": "id", "message": "必须是整数"}})
		return
	}
	user := c.MustGet(middleware.UserKey).(store.User)
	if err := h.secadmins.Unlock(c.Request.Context(), id, user, c.ClientIP()); err != nil {
		writeSecadminError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListSessions handles GET /api/v1/sessions: session enumeration with
// optional user_id filter and fixed 50-per-page pagination, ordered by
// created_at descending. SIDs are digests; CSRF values are never returned.
func (h *SecadminHandler) ListSessions(c *gin.Context) {
	var (
		userID *int64
		page   = 1
	)
	if raw := c.Query("user_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			writeFieldErrors(c, []gin.H{{"field": "user_id", "message": "必须是整数"}})
			return
		}
		userID = &id
	}
	if raw := c.Query("page"); raw != "" {
		p, err := strconv.Atoi(raw)
		if err != nil || p < 1 {
			writeFieldErrors(c, []gin.H{{"field": "page", "message": "必须是正整数"}})
			return
		}
		page = p
	}
	pageResult, err := h.secadmins.ListSessions(c.Request.Context(), userID, page)
	if err != nil {
		writeSecadminError(c, err)
		return
	}
	items := make([]gin.H, 0, len(pageResult.Items))
	for _, it := range pageResult.Items {
		items = append(items, gin.H{
			"sid":         it.SID,
			"user_id":     it.UserID,
			"created_at":  it.CreatedAt,
			"abs_expires": it.AbsExpires,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"page":      pageResult.Page,
		"page_size": pageResult.PageSize,
		"total":     pageResult.Total,
	})
}

// ForceLogout handles DELETE /api/v1/sessions/:sid: deletes the target
// session (204), 404 for an unknown sid.
func (h *SecadminHandler) ForceLogout(c *gin.Context) {
	sid := c.Param("sid")
	user := c.MustGet(middleware.UserKey).(store.User)
	if err := h.secadmins.ForceLogout(c.Request.Context(), sid, user, c.ClientIP()); err != nil {
		writeSecadminError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// writeSecadminError maps the secadmin service outcomes to their §4.5
// statuses and envelopes.
func writeSecadminError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSecadminNotFound):
		writeError(c, http.StatusNotFound, service.KindNotFound, service.ErrSecadminNotFound.Message)
	case errors.Is(err, service.ErrAccountAlreadyActive):
		writeError(c, http.StatusConflict, service.KindConflict, service.ErrAccountAlreadyActive.Message)
	default:
		writeInternal(c, err)
	}
}
