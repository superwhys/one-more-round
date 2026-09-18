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
