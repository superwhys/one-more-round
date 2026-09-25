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

// wishlistRouterFn adapts the wishlist routes to ginutils.Router.
type wishlistRouterFn func(gin.IRouter)

// Init registers the wishlist routes on a group router.
func (fn wishlistRouterFn) Init(router gin.IRouter) { fn(router) }

// WishlistRouter registers the group's want-to-play endpoints.
func WishlistRouter(groupApp *services.GroupApp) wishlistRouterFn {
	return func(router gin.IRouter) {
		router.GET("/wishlist", listWishlistHandler(groupApp))
		router.POST("/wishlist/:game", addWishlistHandler(groupApp))
		router.DELETE("/wishlist/:game", removeWishlistHandler(groupApp))
	}
}

// listWishlistHandler lists the games a group wants to play.
// @Summary 获取小组想玩清单
// @Tags Game
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[dto.Wishlist]
// @Router /v1/groups/{group}/wishlist [get]
func listWishlistHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		wishlist, err := groupApp.ListWishlist(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "list wishlist failed", errcode.ErrSysInternal) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(wishlist))
	})
}

// addWishlistHandler marks a game as wanted by the group.
// @Summary 加入小组想玩清单
// @Tags Game
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param game path string true "桌游 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/wishlist/{game} [post]
func addWishlistHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return setWishlistHandler(groupApp, true)
}

// removeWishlistHandler clears a game from the group's want-to-play list.
// @Summary 移出小组想玩清单
// @Tags Game
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param game path string true "桌游 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/wishlist/{game} [delete]
func removeWishlistHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return setWishlistHandler(groupApp, false)
}

// setWishlistHandler updates one group's wanted game.
func setWishlistHandler(groupApp *services.GroupApp, wanted bool) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GameWishPathReq) {
		err := groupApp.SetGameWanted(ctx.Request.Context(), req.GroupID, common.UserID(ctx), req.GameID, wanted)
		if common.HandleRouterError(ctx, err, "update wishlist failed", errcode.ErrSysInternal) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}
