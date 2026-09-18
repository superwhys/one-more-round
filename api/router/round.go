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
		router.GET("/rounds/:id", getRoundHandler(roundApp))
		router.POST("/rounds", saveRoundHandler(roundApp))
		router.PUT("/rounds/:id", saveRoundHandler(roundApp))
		router.DELETE("/rounds/:id", deleteRoundHandler(roundApp))
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
		req := &dto.ListRoundsReq{
			From: ctx.Query("from"), To: ctx.Query("to"),
			Game: ctx.Query("game"), Player: ctx.Query("player"),
			Offset: offset, Limit: limit,
		}
		page, err := roundApp.List(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "list rounds failed", errcode.ErrRoundList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(page))
	}
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
// @Description 按版本号删除对局，版本不一致时提示刷新
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
