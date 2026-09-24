package common

import (
	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"

	"github.com/superwhys/one-more-round/internal/errcode"
)

// RespondError writes a business error response.
func RespondError(c *gin.Context, e errcode.Error) {
	ginutils.ReturnError(c, e)
}

// ConfigureRequestFailures makes a failed request binding or validation answer
// with the shared business error, so a rejected input keeps a business code and
// an HTTP status instead of ginutils' default success status and bare parser
// message. The detailed reason is logged by ginutils and never sent to clients.
// It is called once while the HTTP layer is assembled.
func ConfigureRequestFailures() {
	ginutils.SetBindFailureHandler(func(*gin.Context, ginutils.FailureStage, error) any {
		return errcode.ErrBadRequest
	})
}
