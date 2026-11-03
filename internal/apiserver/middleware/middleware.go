// Package middleware provides the apiserver gin middleware chain
// (login design spec §8): requestid → recovery → securityheaders → accesslog.
package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestIDKey is the gin context key holding the current request ID.
const RequestIDKey = "request_id"

// RequestID stamps each response with a fresh random X-Request-ID.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := make([]byte, 16)
		if _, err := rand.Read(id); err != nil {
			id = []byte(fmt.Sprintf("%d", time.Now().UnixNano()))
		}
		requestID := hex.EncodeToString(id)
		c.Set(RequestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// Recovery converts a handler panic into a unified 500 response carrying the
// request ID (login design spec §8: no stack trace leaks to the response).
func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recovered",
					"request_id", c.GetString(RequestIDKey),
					"panic", r,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code":       "INTERNAL",
					"request_id": c.GetString(RequestIDKey),
				})
			}
		}()
		c.Next()
	}
}

// SecurityHeaders sets the spec §8 security headers on every response.
// HSTS is intentionally absent: the TLS-terminating gateway adds it.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		c.Next()
	}
}

// AccessLog logs one structured line per request after it completes.
func AccessLog(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info("http request",
			"request_id", c.GetString(RequestIDKey),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
