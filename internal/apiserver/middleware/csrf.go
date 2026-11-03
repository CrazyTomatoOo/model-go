package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/CrazyTomatoOo/model-go/internal/apiserver/store"
)

// CSRFHeader is the request header carrying the session's CSRF token.
const CSRFHeader = "X-CSRF-Token"

// CSRF guards every state-changing request on an authenticated route
// (spec §4): the X-CSRF-Token header must equal the session's stored csrf
// field. GET/HEAD/OPTIONS are exempt. Login is never mounted behind this
// middleware, so no path special-casing is needed.
func CSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
			return
		}
		sess, ok := c.Get(SessionKey)
		if !ok {
			abortSessionExpired(c)
			return
		}
		if c.GetHeader(CSRFHeader) != sess.(store.Session).CSRF {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code":    "CSRF_TOKEN_INVALID",
				"message": "CSRF token 无效",
			})
			return
		}
		c.Next()
	}
}
