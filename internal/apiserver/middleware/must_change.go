package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// MustChangeGate aborts with 403 PASSWORD_CHANGE_REQUIRED while the
// authenticated user still has must_change_password set (spec §6): only
// GET/HEAD/OPTIONS /api/v1/auth/me and POST /api/v1/auth/password pass
// until the password is changed. A missing user context means the route
// was reached unauthenticated.
func MustChangeGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(UserKey)
		if !ok {
			abortSessionExpired(c)
			return
		}
		if !user.(store.User).MustChangePassword {
			c.Next()
			return
		}
		if c.FullPath() == "/api/v1/auth/password" && c.Request.Method == http.MethodPost {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			if c.FullPath() == "/api/v1/auth/me" {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "PASSWORD_CHANGE_REQUIRED",
			"message": "需要修改密码后才能继续操作",
		})
	}
}
