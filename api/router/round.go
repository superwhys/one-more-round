package router

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

type roundRouterFn func(router gin.IRouter)

func (fn roundRouterFn) Init(router gin.IRouter) { fn(router) }

// RoundRouter registers the round endpoints of one group.
func RoundRouter(roundApp *services.RoundApp) roundRouterFn {
	return func(router gin.IRouter) {
		router.GET("/rounds", listRoundsHandler(roundApp))
		router.GET("/rounds/recap", recapHandler(roundApp))
		router.GET("/rounds/recycle-bin", recycleBinHandler(roundApp))
		router.GET("/rounds/:id", getRoundHandler(roundApp))
		router.POST("/rounds", saveRoundHandler(roundApp))
		router.PUT("/rounds/:id", saveRoundHandler(roundApp))
		router.POST("/rounds/:id/restore", restoreRoundHandler(roundApp))
		router.GET("/rounds/:id/share", roundShareStatusHandler(roundApp))
		router.POST("/rounds/:id/share", createRoundShareHandler(roundApp))
		router.DELETE("/rounds/:id/share", revokeRoundShareHandler(roundApp))
		router.DELETE("/rounds/:id", deleteRoundHandler(roundApp))
	}
}

// PublicRoundRouter registers bearer-link endpoints without session middleware.
func PublicRoundRouter(roundApp *services.RoundApp) roundRouterFn {
	return func(router gin.IRouter) {
		router.GET("/shared-rounds", publicRoundHandler(roundApp))
		router.GET("/shared-rounds/photos/:photo", publicRoundPhotoHandler(roundApp))
	}
}

// roundShareStatusHandler 查询对局分享状态
// @Summary 查询对局分享状态
// @Tags Round Share
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Success 200 {object} ginutils.Ret[dto.RoundShareStatus]
// @Router /v1/groups/{group}/rounds/{id}/share [get]
func roundShareStatusHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		status, err := roundApp.ShareStatus(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Param("id"))
		if common.HandleRouterError(ctx, err, "get round share status failed", errcode.ErrRoundShareRead) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(status))
	}
}

// createRoundShareHandler 生成或轮换对局分享链接
// @Summary 生成或轮换对局分享链接
// @Tags Round Share
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Success 200 {object} ginutils.Ret[dto.RoundShareToken]
// @Router /v1/groups/{group}/rounds/{id}/share [post]
func createRoundShareHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		share, err := roundApp.CreateShare(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Param("id"))
		if common.HandleRouterError(ctx, err, "create round share failed", errcode.ErrRoundShare) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(share))
	}
}

// revokeRoundShareHandler 撤销对局分享链接
// @Summary 撤销对局分享链接
// @Tags Round Share
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/rounds/{id}/share [delete]
func revokeRoundShareHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if err := roundApp.RevokeShare(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Param("id")); common.HandleRouterError(ctx, err, "revoke round share failed", errcode.ErrRoundShare) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	}
}

// publicRoundHandler 查看公开分享的对局
// @Summary 查看公开分享的对局
// @Tags Round Share
// @Produce json
// @Param X-Round-Share header string true "分享令牌"
// @Success 200 {object} ginutils.Ret[dto.PublicRound]
// @Router /v1/shared-rounds [get]
func publicRoundHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		round, err := roundApp.PublicRound(ctx.Request.Context(), ctx.GetHeader("X-Round-Share"))
		if common.HandleRouterError(ctx, err, "read public round failed", errcode.ErrRoundShareRead) {
			return
		}
		ctx.Header("Cache-Control", "private, no-store")
		ctx.JSON(http.StatusOK, dto.ResponseWithData(round))
	}
}

// publicRoundPhotoHandler 查看公开分享的对局照片
// @Summary 查看公开分享的对局照片
// @Tags Round Share
// @Produce image/jpeg
// @Param X-Round-Share header string true "分享令牌"
// @Param photo path string true "照片 ID"
// @Success 200 {file} file
// @Router /v1/shared-rounds/photos/{photo} [get]
func publicRoundPhotoHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		content, err := roundApp.PublicPhoto(ctx.Request.Context(), ctx.GetHeader("X-Round-Share"), ctx.Param("photo"))
		if common.HandleRouterError(ctx, err, "read public round photo failed", errcode.ErrPhotoRead) {
			return
		}
		defer content.Body.Close()
		ctx.Header("Cache-Control", "private, no-store")
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.DataFromReader(http.StatusOK, content.Size, "image/jpeg", content.Body, nil)
	}
}

// listRoundsHandler 获取对局列表
// @Summary 获取对局列表
// @Description 按日期、桌游、玩家筛选对局，返回分页结果与战绩统计
// @Tags Round
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param from query string false "开始日期"
// @Param to query string false "结束日期"
// @Param game query string false "桌游 ID"
// @Param player query string false "玩家 ID"
// @Param q query string false "搜索回忆"
// @Param mode query string false "individual/team/coop"
// @Param outcome query string false "win/loss/draw/unknown"
// @Param has_photos query bool false "是否有照片"
// @Param offset query int false "偏移量"
// @Param limit query int false "每页条数，最大 100"
// @Success 200 {object} ginutils.Ret[dto.Page]
// @Router /v1/groups/{group}/rounds [get]
func listRoundsHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		offset, e1 := strconv.Atoi(ctx.DefaultQuery("offset", "0"))
		limit, e2 := strconv.Atoi(ctx.DefaultQuery("limit", "30"))
		if e1 != nil || e2 != nil {
			common.RespondError(ctx, errcode.ErrPageRange)
			return
		}
		var hasPhotos *bool
		if raw := ctx.Query("has_photos"); raw != "" {
			value, err := strconv.ParseBool(raw)
			if err != nil {
				common.RespondError(ctx, errcode.ErrBadRequest)
				return
			}
			hasPhotos = &value
		}
		req := &dto.ListRoundsReq{
			From: ctx.Query("from"), To: ctx.Query("to"),
			Game: ctx.Query("game"), Player: ctx.Query("player"),
			Query: ctx.Query("q"), Mode: ctx.Query("mode"), Outcome: ctx.Query("outcome"), HasPhotos: hasPhotos,
			Offset: offset, Limit: limit,
		}
		page, err := roundApp.List(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "list rounds failed", errcode.ErrRoundList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(page))
	}
}

// recapHandler 获取月度或年度回顾
// @Summary 获取月度或年度回顾
// @Tags Round
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param period query string true "YYYY-MM 或 YYYY"
// @Success 200 {object} ginutils.Ret[dto.Recap]
// @Router /v1/groups/{group}/rounds/recap [get]
func recapHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		result, err := roundApp.Recap(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Query("period"))
		if common.HandleRouterError(ctx, err, "recap failed", errcode.ErrRoundList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(result))
	}
}

// recycleBinHandler 获取七天内可恢复的对局
// @Summary 获取回收站
// @Tags Round
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[[]dto.Round]
// @Router /v1/groups/{group}/rounds/recycle-bin [get]
func recycleBinHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		items, err := roundApp.RecycleBin(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "recycle bin failed", errcode.ErrRoundList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(items))
	}
}

// restoreRoundHandler 恢复已删除对局
// @Summary 恢复已删除对局
// @Tags Round
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Param request body dto.RestoreRoundReq true "恢复请求"
// @Success 200 {object} ginutils.Ret[dto.Round]
// @Router /v1/groups/{group}/rounds/{id}/restore [post]
func restoreRoundHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.RestoreRoundReq) {
		restored, err := roundApp.Restore(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "restore round failed", errcode.ErrRoundSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(restored))
	})
}

// getRoundHandler 获取对局详情
// @Summary 获取对局详情
// @Description 返回一条对局的完整内容，含版本号
// @Tags Round
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Success 200 {object} ginutils.Ret[dto.Round]
// @Router /v1/groups/{group}/rounds/{id} [get]
func getRoundHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		round, err := roundApp.Get(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Param("id"))
		if common.HandleRouterError(ctx, err, "get round failed", errcode.ErrRoundGet) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(round))
	}
}

// saveRoundHandler 记一局或修改对局
// @Summary 记一局或修改对局
// @Description 创建对局需携带 Idempotency-Key；修改对局需携带版本号
// @Tags Round
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string false "对局 ID，修改时必填"
// @Param Idempotency-Key header string false "提交标识，创建时必填"
// @Param request body dto.Round true "对局内容"
// @Success 200 {object} ginutils.Ret[dto.Round]
// @Router /v1/groups/{group}/rounds [post]
func saveRoundHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.SaveRoundReq) {
		saved, err := roundApp.Save(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "save round failed", errcode.ErrRoundSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(saved))
	})
}

// deleteRoundHandler 删除对局
// @Summary 删除对局
// @Description 按版本号移入七天回收站，版本不一致时提示刷新
// @Tags Round
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Param request body dto.DeleteRoundReq true "删除对局请求体"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/rounds/{id} [delete]
func deleteRoundHandler(roundApp *services.RoundApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.DeleteRoundReq) {
		err := roundApp.Delete(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "delete round failed", errcode.ErrRoundDelete) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}
