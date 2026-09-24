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

// commentRouterFn registers the comment routes of one group.
type commentRouterFn func(router gin.IRouter)

func (fn commentRouterFn) Init(router gin.IRouter) { fn(router) }

// CommentRouter registers the comment endpoints of one group's rounds.
func CommentRouter(commentApp *services.CommentApp) commentRouterFn {
	return func(router gin.IRouter) {
		router.GET("/rounds/:id/comments", listRoundCommentsHandler(commentApp))
		router.POST("/rounds/:id/comments", createRoundCommentHandler(commentApp))
		router.DELETE("/rounds/:id/comments/:comment", deleteRoundCommentHandler(commentApp))
	}
}

// listRoundCommentsHandler 获取对局评论
// @Summary 获取对局评论
// @Description 按时间正序返回当前对局的组内评论，公开分享页不包含这些内容
// @Tags Round Comment
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Param offset query int false "偏移量"
// @Param limit query int false "每页条数，最大 100"
// @Success 200 {object} ginutils.Ret[dto.Paginated[dto.RoundComment]]
// @Router /v1/groups/{group}/rounds/{id}/comments [get]
func listRoundCommentsHandler(commentApp *services.CommentApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.ListRoundCommentsReq) {
		page, err := commentApp.List(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(
			ctx,
			err,
			"list round comments failed",
			errcode.ErrCommentList,
		) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(page))
	})
}

// createRoundCommentHandler 发表对局评论或一层回复
// @Summary 发表对局评论或一层回复
// @Description 创建需携带 Idempotency-Key；parent_id 为空时评论对局，非空时只能回复根评论
// @Tags Round Comment
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Param Idempotency-Key header string true "提交标识"
// @Param request body dto.RoundCommentBody true "评论内容"
// @Success 200 {object} ginutils.Ret[dto.RoundComment]
// @Router /v1/groups/{group}/rounds/{id}/comments [post]
func createRoundCommentHandler(commentApp *services.CommentApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.CreateRoundCommentReq) {
		saved, err := commentApp.Create(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "save round comment failed", errcode.ErrCommentSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(saved))
	})
}

// deleteRoundCommentHandler 删除对局评论
// @Summary 删除对局评论
// @Description 作者可删自己的评论，组主可删任何条；删除根评论时一并删除回复
// @Tags Round Comment
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "对局 ID"
// @Param comment path string true "评论 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/rounds/{id}/comments/{comment} [delete]
func deleteRoundCommentHandler(commentApp *services.CommentApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.DeleteRoundCommentReq) {
		if err := commentApp.Delete(
			ctx.Request.Context(),
			common.UserID(ctx),
			req,
		); common.HandleRouterError(
			ctx,
			err,
			"delete round comment failed",
			errcode.ErrCommentDelete,
		) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}
