package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// RequireRole aborts with 403 FORBIDDEN when the authenticated user's role
// is not in the allowed list. Runs after the session middleware, so a
// missing user context means the route was reached unauthenticated.
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := c.Get(UserKey)
		if !ok {
			abortSessionExpired(c)
			return
		}
		role := user.(store.User).Role
		for _, allowed := range roles {
			if role == allowed {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"code":    "FORBIDDEN",
			"message": "无权限执行此操作",
		})
	}
}
