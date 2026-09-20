// Package common holds the session cookie handling and the error response
// shared by the HTTP middlewares and handlers.
package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/miebyte/goutils/logging"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// SessionCookie is the name of the session cookie; the credential itself lives
// in the database and only its digest is stored server side.
const SessionCookie = "omr_session"

const userKey = "authenticated_user"

// SetSessionCookie writes the session cookie; a negative maxAge clears it.
func SetSessionCookie(c *gin.Context, token string, maxAge int, secure bool) {
	http.SetCookie(c.Writer, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", MaxAge: maxAge, HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode})
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(c *gin.Context, secure bool) {
	SetSessionCookie(c, "", -1, secure)
}

// SessionToken returns the raw session token of the request cookie.
func SessionToken(c *gin.Context) string {
	token, _ := c.Cookie(SessionCookie)
	return token
}

// SetUser stores the authenticated account on the request.
func SetUser(c *gin.Context, u *dto.User) { c.Set(userKey, u) }

// CurrentUser returns the authenticated account.
func CurrentUser(c *gin.Context) *dto.User {
	if value, ok := c.Get(userKey); ok {
		if u, ok := value.(*dto.User); ok {
			return u
		}
	}
	return nil
}

// UserID returns the identifier of the authenticated account.
func UserID(c *gin.Context) string {
	if u := CurrentUser(c); u != nil {
		return u.ID
	}
	return ""
}

// HandleRouterError writes the response of a use-case error and reports whether
// it handled one. A business error keeps its own code, HTTP status and message;
// anything else is logged and reported as the fallback of the handler.
func HandleRouterError(c *gin.Context, err error, logMsg string, fallback errcode.Error) bool {
	if err == nil {
		return false
	}
	if ec, ok := errcode.AsErrcode(err); ok {
		ginutils.ReturnError(c, ec)
		return true
	}
	if logMsg != "" {
		logging.Errorc(c.Request.Context(), "%s: %v", logMsg, err)
	}
	ginutils.ReturnError(c, fallback)
	return true
}
