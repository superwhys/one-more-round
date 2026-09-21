package router

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

type groupRouterFn func(router gin.IRouter)

func (fn groupRouterFn) Init(router gin.IRouter) { fn(router) }

// GroupInvitationRouter registers the minimal public invitation preview.
func GroupInvitationRouter(groupApp *services.GroupApp) groupRouterFn {
	return func(router gin.IRouter) { router.POST("/group-invite", previewInviteHandler(groupApp)) }
}

// previewInviteHandler 查看小组邀请
// @Summary 查看小组邀请
// @Description 无需登录，凭有效邀请仅返回小组 ID 与名称，不展示成员或记录
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.JoinReq true "邀请令牌"
// @Success 200 {object} ginutils.Ret[dto.InvitePreview]
// @Router /v1/auth/group-invite [post]
func previewInviteHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.JoinReq) {
		preview, err := groupApp.PreviewInvite(ctx.Request.Context(), req.Token)
		if common.HandleRouterError(ctx, err, "preview invitation failed", errcode.ErrSysInternal) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(preview))
	})
}

// GroupRouter registers the group collection endpoints.
func GroupRouter(groupApp *services.GroupApp) groupRouterFn {
	return func(router gin.IRouter) {
		router.GET("/groups", listGroupsHandler(groupApp))
		router.POST("/groups", createGroupHandler(groupApp))
		router.POST("/join", joinGroupHandler(groupApp))
	}
}

// GroupDetailRouter registers the endpoints of one group.
func GroupDetailRouter(groupApp *services.GroupApp, origin string) groupRouterFn {
	return func(router gin.IRouter) {
		router.GET("", groupSnapshotHandler(groupApp))
		router.POST("/players", addPlayerHandler(groupApp))
		router.POST("/games", addGameHandler(groupApp))
		router.GET("/bgg/search", searchExternalGamesHandler(groupApp))
		router.GET("/invites", listInvitesHandler(groupApp))
		router.POST("/invites", createInviteHandler(groupApp, origin))
		router.POST("/manage", manageGroupHandler(groupApp))
		router.GET("/export", exportGroupHandler(groupApp))
	}
}

// exportGroupHandler 导出小组备份
// @Summary 导出小组备份
// @Description 仅组主可下载包含 JSON、CSV 与已关联照片的 ZIP
// @Tags Group
// @Produce application/zip
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {file} binary
// @Router /v1/groups/{group}/export [get]
func exportGroupHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		data, err := groupApp.Export(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "export group failed", errcode.ErrSysInternal) {
			return
		}
		ctx.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="one-more-round-%s.zip"`, time.Now().Format("20060102")))
		ctx.Data(http.StatusOK, "application/zip", data)
	})
}

// listGroupsHandler 获取我的小组
// @Summary 获取我的小组
// @Description 返回当前账号加入的小组
// @Tags Group
// @Produce json
// @Security SessionCookie
// @Success 200 {object} ginutils.Ret[[]dto.Group]
// @Router /v1/groups [get]
func listGroupsHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		groups, err := groupApp.List(ctx.Request.Context(), common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "list groups failed", errcode.ErrGroupList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(groups))
	}
}

// createGroupHandler 创建小组
// @Summary 创建小组
// @Description 创建小组与创建者自己的玩家档案，创建者成为组主与首位成员
// @Tags Group
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body dto.CreateGroupReq true "创建小组请求体"
// @Success 200 {object} ginutils.Ret[dto.Group]
// @Router /v1/groups [post]
func createGroupHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.CreateGroupReq) {
		group, err := groupApp.Create(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "create group failed", errcode.ErrGroupCreate) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(group))
	})
}

// joinGroupHandler 加入小组
// @Summary 加入小组
// @Description 使用小组邀请链接中的令牌加入小组
// @Tags Group
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body dto.JoinReq true "加入小组请求体"
// @Success 200 {object} ginutils.Ret[string]
// @Router /v1/join [post]
func joinGroupHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.JoinReq) {
		groupID, err := groupApp.Join(ctx.Request.Context(), common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "join group failed", errcode.ErrJoin) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(groupID))
	})
}

// groupSnapshotHandler 获取小组详情
// @Summary 获取小组详情
// @Description 返回小组成员、玩家、桌游与待确认关联
// @Tags Group
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[dto.Snapshot]
// @Router /v1/groups/{group} [get]
func groupSnapshotHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		snapshot, err := groupApp.Snapshot(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "read group failed", errcode.ErrGroupSnapshot) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(snapshot))
	})
}

// addPlayerHandler 添加玩家
// @Summary 添加玩家
// @Description 在小组内添加昵称玩家，重名会被拒绝
// @Tags Group
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param request body dto.AddPlayerReq true "添加玩家请求体"
// @Success 200 {object} ginutils.Ret[dto.Player]
// @Router /v1/groups/{group}/players [post]
func addPlayerHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.AddPlayerReq) {
		player, err := groupApp.AddPlayer(ctx.Request.Context(), req.GroupID, common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "add player failed", errcode.ErrPlayerSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(player))
	})
}

// addGameHandler 添加桌游
// @Summary 添加桌游
// @Description 在小组内添加桌游，同组同名复用已有条目
// @Tags Game
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param request body dto.AddGameReq true "添加桌游请求体"
// @Success 200 {object} ginutils.Ret[dto.Game]
// @Router /v1/groups/{group}/games [post]
func addGameHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.AddGameReq) {
		game, err := groupApp.AddGame(ctx.Request.Context(), req.GroupID, common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "add game failed", errcode.ErrGameSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(game))
	})
}

// searchExternalGamesHandler 搜索外部桌游
// @Summary 搜索外部桌游
// @Description 外部桌游检索；授权未配置时返回明确不可用提示
// @Tags Game
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/bgg/search [get]
func searchExternalGamesHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		err := groupApp.SearchExternalGames(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "search external games failed", errcode.ErrBGGUnavailable) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}

// listInvitesHandler 获取小组邀请
// @Summary 获取小组邀请
// @Description 组主查看本组邀请记录，不返回明文令牌
// @Tags Group
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[[]dto.Invite]
// @Router /v1/groups/{group}/invites [get]
func listInvitesHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		invites, err := groupApp.Invites(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "list invites failed", errcode.ErrInviteList) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(invites))
	})
}

// createInviteHandler 生成小组邀请
// @Summary 生成小组邀请
// @Description 组主生成七天有效的小组邀请，明文令牌仅返回一次
// @Tags Group
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Success 200 {object} ginutils.Ret[dto.InviteResp]
// @Router /v1/groups/{group}/invites [post]
func createInviteHandler(groupApp *services.GroupApp, origin string) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.GroupPathReq) {
		invite, token, err := groupApp.Invite(ctx.Request.Context(), req.GroupID, common.UserID(ctx))
		if common.HandleRouterError(ctx, err, "create invite failed", errcode.ErrInviteCreate) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(dto.InviteResp{
			ID:      invite.ID,
			URL:     origin + "/join#" + url.QueryEscape(token),
			Expires: invite.Expires,
		}))
	})
}

// manageGroupHandler 修改小组与成员
// @Summary 修改小组与成员
// @Description 组主或本人执行改名、转让、移除、撤销邀请、关联与审核玩家
// @Tags Group
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param request body dto.ManageReq true "小组操作请求体"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/groups/{group}/manage [post]
func manageGroupHandler(groupApp *services.GroupApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.ManageReq) {
		err := groupApp.Manage(ctx.Request.Context(), req.GroupID, common.UserID(ctx), req)
		if common.HandleRouterError(ctx, err, "manage group failed", errcode.ErrGroupManage) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}
