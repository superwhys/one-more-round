package middleware

import (
	"mime"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"

	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// OriginMiddleware rejects state-changing requests that do not come from the
// configured origin served by this process.
func OriginMiddleware(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead &&
			c.GetHeader("Origin") != origin && !miniProgramRequest(c) {
			common.RespondError(c, errcode.ErrOrigin)
			c.Abort()
			return
		}
		c.Next()
	}
}

// miniProgramRequest permits explicit non-cookie clients without weakening
// browser CSRF checks or granting CORS access. Browser-readable login responses
// require a custom header and JSON, which cross-origin forms cannot send.
func miniProgramRequest(c *gin.Context) bool {
	if c.GetHeader("Origin") != "" || c.GetHeader("Cookie") != "" {
		return false
	}
	if common.BearerToken(c) != "" {
		return true
	}
	if c.GetHeader("Authorization") != "" || c.GetHeader("X-OMR-Client") != "wechat-mini" || c.Request.Method != http.MethodPost {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return false
	}
	switch c.Request.URL.Path {
	case "/v1/auth/wx-login", "/v1/auth/code", "/v1/auth/group-invite":
		return true
	default:
		return false
	}
}

// RecoveryMiddleware converts a panic into an explicit error response; the
// fallback recovery of the framework would report it as a business success.
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				logging.Errorc(
					c.Request.Context(),
					"panic while handling %s: %v",
					c.Request.URL.Path,
					r,
				)
				common.RespondError(c, errcode.ErrSysInternal)
				c.Abort()
			}
		}()
		c.Next()
	}
}

// NoCacheMiddleware keeps every API response out of shared caches.
func NoCacheMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Next()
	}
}
