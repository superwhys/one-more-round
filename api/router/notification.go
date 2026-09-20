package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

type notificationRouterFn func(gin.IRouter)

func (fn notificationRouterFn) Init(router gin.IRouter) { fn(router) }

func NotificationRouter(app *services.NotificationApp) notificationRouterFn {
	return func(router gin.IRouter) {
		router.GET("/notifications", listNotificationsHandler(app))
		router.POST("/notifications/:id/read", readNotificationHandler(app))
	}
}

// listNotificationsHandler 获取当前账号通知
// @Summary 获取站内通知
// @Tags Notification
// @Produce json
// @Security SessionCookie
// @Success 200 {object} ginutils.Ret[dto.NotificationPage]
// @Router /v1/notifications [get]
func listNotificationsHandler(app *services.NotificationApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		page, err := app.List(ctx.Request.Context(), common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "list notifications failed", errcode.ErrSysInternal) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(page))
	}
}

// readNotificationHandler 标记一条通知已读
// @Summary 标记通知已读
// @Tags Notification
// @Produce json
// @Security SessionCookie
// @Param id path string true "通知 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/notifications/{id}/read [post]
func readNotificationHandler(app *services.NotificationApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.ReadNotificationReq) {
		if common.HandleRouterError(ctx, app.Read(ctx.Request.Context(), common.UserID(ctx), req.ID), "read notification failed", errcode.ErrSysInternal) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}
