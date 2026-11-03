package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

const (
	// CookieName is the session cookie (login design spec §4: __Host-
	// prefix, HttpOnly, Secure, SameSite=Lax, Path=/).
	CookieName = "__Host-session"

	// SessionKey is the gin context key holding the authenticated
	// store.Session; UserKey holds the store.User loaded from it.
	SessionKey = "session"
	UserKey    = "user"
)

type sessionStore interface {
	Get(ctx context.Context, sid string) (store.Session, error)
	Touch(ctx context.Context, sid string) error
}

type userStore interface {
	ByID(ctx context.Context, id int64) (store.User, error)
}

// Session authenticates the request cookie: parse → Redis lookup → absolute
// expiry check → sliding TTL touch → load user. Redis errors, unknown
// sessions, expired sessions, and vanished users all abort with a unified
// 401 SESSION_EXPIRED (fail-closed, spec §8).
func Session(sessions sessionStore, users userStore, logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(CookieName)
		if err != nil {
			abortSessionExpired(c)
			return
		}
		sess, err := sessions.Get(c.Request.Context(), cookie)
		if err != nil {
			logger.Warn("session lookup failed", "request_id", c.GetString(RequestIDKey), "error", err)
			abortSessionExpired(c)
			return
		}
		if !time.Now().Before(sess.AbsExpires) {
			abortSessionExpired(c)
			return
		}
		if err := sessions.Touch(c.Request.Context(), cookie); err != nil {
			logger.Warn("session touch failed", "request_id", c.GetString(RequestIDKey), "error", err)
			abortSessionExpired(c)
			return
		}
		user, err := users.ByID(c.Request.Context(), sess.UserID)
		if err != nil {
			abortSessionExpired(c)
			return
		}
		c.Set(SessionKey, sess)
		c.Set(UserKey, user)
		c.Next()
	}
}

func abortSessionExpired(c *gin.Context) {
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
		"code":    "SESSION_EXPIRED",
		"message": "会话已过期，请重新登录",
	})
}
