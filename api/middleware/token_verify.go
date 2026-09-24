// Package middleware holds the HTTP middlewares of the API layer.
package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// TokenVerifyMiddleware resolves the session cookie and rejects unauthenticated
// requests. Handlers behind it read the account through common.CurrentUser.
func TokenVerifyMiddleware(authApp *services.AuthApp) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, err := authApp.Authenticate(c.Request.Context(), common.SessionToken(c))
		if common.HandleRouterError(
			c,
			err,
			"authenticate request failed",
			errcode.ErrUnauthorized,
		) {
			c.Abort()
			return
		}
		common.SetUser(c, user)
		c.Next()
	}
}
