package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/logging"

	"github.com/superwhys/one-more-round/api/common"
)

// ContextInjectMiddleware annotates the request context with the authenticated
// account so every later log line carries the request identity.
func ContextInjectMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := common.CurrentUser(c)
		if user == nil {
			c.Next()
			return
		}
		c.Request = c.Request.WithContext(logging.With(c.Request.Context(), "UserID", user.ID))
		c.Next()
	}
}
