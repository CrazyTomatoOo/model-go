package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/middleware"
	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// Login handles POST /api/v1/auth/login (spec §4.1): validates the body,
// delegates to the service, sets the __Host-session cookie, and returns the
// user plus the session's CSRF token. All failure modes are rendered by
// writeAuthError from the service's typed error.
func (h *AuthHandler) Login(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		writeFieldErrors(c, []gin.H{{"field": "body", "message": "请求体必须是 JSON"}})
		return
	}
	var fields []gin.H
	if req.Username == "" {
		fields = append(fields, gin.H{"field": "username", "message": "用户名不能为空"})
	}
	if req.Password == "" {
		fields = append(fields, gin.H{"field": "password", "message": "密码不能为空"})
	}
	if len(fields) > 0 {
		writeFieldErrors(c, fields)
		return
	}

	result, err := h.auth.Login(c.Request.Context(), req.Username, req.Password, c.ClientIP())
	if err != nil {
		writeAuthError(c, err)
		return
	}
	http.SetCookie(c.Writer, sessionCookie(result.SID))
	c.JSON(http.StatusOK, gin.H{
		"user":       publicUser(result.User),
		"csrf_token": result.CSRF,
	})
}

// Logout handles POST /api/v1/auth/logout (spec §4.1): deletes the session
// through the service and clears the cookie. The session middleware has
// already authenticated the request, so the user is read from the context.
func (h *AuthHandler) Logout(c *gin.Context) {
	sid, err := c.Cookie(middleware.CookieName)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "会话已过期，请重新登录")
		return
	}
	user := c.MustGet(middleware.UserKey).(store.User)
	if err := h.auth.Logout(c.Request.Context(), sid, user, c.ClientIP()); err != nil {
		writeInternal(c, err)
		return
	}
	http.SetCookie(c.Writer, clearSessionCookie())
	c.Status(http.StatusNoContent)
}

// Me handles GET /api/v1/auth/me (spec §4.1): re-resolves the session's
// user and returns it with the CSRF token.
func (h *AuthHandler) Me(c *gin.Context) {
	sid, err := c.Cookie(middleware.CookieName)
	if err != nil {
		writeError(c, http.StatusUnauthorized, "SESSION_EXPIRED", "会话已过期，请重新登录")
		return
	}
	user, csrf, err := h.auth.Me(c.Request.Context(), sid)
	if err != nil {
		writeAuthError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"user":       publicUser(user),
		"csrf_token": csrf,
	})
}

// publicUser renders the user without the password hash (spec §4.1 forbids
// returning it on any endpoint).
func publicUser(u store.User) gin.H {
	return gin.H{
		"id":                   u.ID,
		"username":             u.Username,
		"role":                 u.Role,
		"status":               u.Status,
		"must_change_password": u.MustChangePassword,
	}
}

// sessionCookie builds the __Host- session cookie (spec §4.1: HttpOnly,
// Secure, SameSite=Lax, Path=/).
func sessionCookie(sid string) *http.Cookie {
	return &http.Cookie{
		Name:     middleware.CookieName,
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// clearSessionCookie expires the session cookie on logout.
func clearSessionCookie() *http.Cookie {
	cookie := sessionCookie("")
	cookie.MaxAge = -1
	return cookie
}
