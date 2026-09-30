// Package router registers the HTTP routes of every resource and adapts the
// protocol to the application services.
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

// SessionOptions configures how the session cookie is written.
type SessionOptions struct {
	Secure bool
	// MaxAge is the cookie lifetime in seconds.
	MaxAge int
}

type authRouterFn func(router gin.IRouter)

func (fn authRouterFn) Init(router gin.IRouter) { fn(router) }

// AuthRouter registers the unauthenticated entry points; the session cookie is
// issued here and the remaining endpoints rely on the session middleware.
func AuthRouter(authApp *services.AuthApp, opts SessionOptions) authRouterFn {
	return func(router gin.IRouter) {
		router.POST("/code", sendCodeHandler(authApp))
		router.POST("/login", loginHandler(authApp, opts))
		router.POST("/password/login", passwordLoginHandler(authApp, opts))
		router.POST("/password/register", passwordRegisterHandler(authApp, opts))
		router.POST("/wx-login", wechatLoginHandler(authApp))
		router.POST("/logout", logoutHandler(authApp, opts))
	}
}

// sendCodeHandler 发送邮箱验证码
// @Summary 发送邮箱验证码
// @Description 向目标邮箱发送验证码；不判断账号是否存在或校验邀请码
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.SendCodeReq true "发送验证码请求体"
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/auth/code [post]
func sendCodeHandler(authApp *services.AuthApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.SendCodeReq) {
		err := authApp.SendCode(ctx.Request.Context(), req, ctx.ClientIP())
		if common.HandleRouterError(ctx, err, "send code failed", errcode.ErrSendCode) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	})
}

// loginHandler 验证码登录
// @Summary 验证码登录
// @Description 已有账号验证邮箱即可登录，忽略邀请码；新账号需 invite 或 group_token，小组邀请注册时原子加入小组
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.LoginReq true "登录请求体"
// @Success 200 {object} ginutils.Ret[dto.LoginResp]
// @Router /v1/auth/login [post]
func loginHandler(authApp *services.AuthApp, opts SessionOptions) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.LoginReq) {
		user, token, err := authApp.Login(ctx.Request.Context(), req)
		if common.HandleRouterError(ctx, err, "login failed", errcode.ErrLogin) {
			return
		}
		common.SetSessionCookie(ctx, token, opts.MaxAge, opts.Secure)
		ctx.JSON(http.StatusOK, dto.ResponseWithData(user))
	})
}

// passwordLoginHandler verifies a password and issues the existing browser cookie.
// @Summary 账号密码登录
// @Description 使用用户名或已绑定邮箱登录已有密码账号；不会自动注册，也不需要邀请码
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.PasswordLoginReq true "账号密码登录请求"
// @Success 200 {object} ginutils.Ret[dto.LoginResp]
// @Router /v1/auth/password/login [post]
func passwordLoginHandler(authApp *services.AuthApp, opts SessionOptions) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.PasswordLoginReq) {
		user, token, err := authApp.LoginPassword(ctx.Request.Context(), req, ctx.ClientIP())
		if common.HandleRouterError(ctx, err, "password login failed", errcode.ErrLogin) {
			return
		}
		common.SetSessionCookie(ctx, token, opts.MaxAge, opts.Secure)
		ctx.JSON(http.StatusOK, dto.ResponseWithData(user))
	})
}

// passwordRegisterHandler creates an invited account before issuing its cookie.
// @Summary 邀请注册账号密码
// @Description 使用有效试用邀请或小组邀请创建用户名账号；注册、受邀入组和会话在同一事务提交
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.PasswordRegisterReq true "账号密码注册请求"
// @Success 200 {object} ginutils.Ret[dto.LoginResp]
// @Router /v1/auth/password/register [post]
func passwordRegisterHandler(authApp *services.AuthApp, opts SessionOptions) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.PasswordRegisterReq) {
		user, token, err := authApp.RegisterPassword(ctx.Request.Context(), req)
		if common.HandleRouterError(ctx, err, "password registration failed", errcode.ErrRegister) {
			return
		}
		common.SetSessionCookie(ctx, token, opts.MaxAge, opts.Secure)
		ctx.JSON(http.StatusOK, dto.ResponseWithData(user))
	})
}

// logoutHandler 退出登录
// @Summary 退出登录
// @Description 撤销当前会话并清除会话 Cookie
// @Tags Auth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} ginutils.Ret[any]
// @Router /v1/auth/logout [post]
func logoutHandler(authApp *services.AuthApp, opts SessionOptions) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		err := authApp.Logout(ctx.Request.Context(), common.SessionToken(ctx))
		if common.HandleRouterError(ctx, err, "logout failed", errcode.ErrLogout) {
			return
		}
		common.ClearSessionCookie(ctx, opts.Secure)
		ctx.JSON(http.StatusOK, dto.ResponseSuccess())
	}
}

// MeRouter registers the account endpoints behind the session middleware.
func MeRouter() authRouterFn {
	return func(router gin.IRouter) {
		router.GET("/me", meHandler())
	}
}

// PasswordAccountRouter registers password changes behind session authentication.
func PasswordAccountRouter(authApp *services.AuthApp, opts SessionOptions) authRouterFn {
	return func(router gin.IRouter) {
		router.POST("/auth/password/set", setPasswordHandler(authApp, opts))
	}
}

// setPasswordHandler updates the current account and replaces its browser cookie.
// @Summary 设置账号密码
// @Description 首次开通需用户名；已有用户名不可更改。设置成功撤销原有会话，并续签当前浏览器
// @Tags Auth
// @Accept json
// @Produce json
// @Security SessionCookie
// @Param request body dto.SetPasswordReq true "设置密码请求"
// @Success 200 {object} ginutils.Ret[dto.User]
// @Router /v1/auth/password/set [post]
func setPasswordHandler(authApp *services.AuthApp, opts SessionOptions) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.SetPasswordReq) {
		user, token, err := authApp.SetPassword(
			ctx.Request.Context(), common.UserID(ctx), common.SessionToken(ctx), req,
		)
		if common.HandleRouterError(ctx, err, "set password failed", errcode.ErrSetPassword) {
			return
		}
		common.SetSessionCookie(ctx, token, opts.MaxAge, opts.Secure)
		ctx.JSON(http.StatusOK, dto.ResponseWithData(user))
	})
}

// meHandler 获取当前账号
// @Summary 获取当前账号
// @Description 返回当前会话对应的账号
// @Tags Auth
// @Produce json
// @Security SessionCookie
// @Success 200 {object} ginutils.Ret[dto.User]
// @Router /v1/me [get]
func meHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, dto.ResponseWithData(common.CurrentUser(ctx)))
	}
}

// wechatLoginHandler 微信小程序登录
// @Summary 微信小程序登录并可首次绑定已有邮箱
// @Description wx.login code 由后端兑换，返回不透明 Bearer 会话；首次注册需有效邀请，已有邮箱须验证验证码
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body dto.WechatLoginReq true "微信登录请求"
// @Success 200 {object} ginutils.Ret[dto.WechatLoginResp]
// @Router /v1/auth/wx-login [post]
func wechatLoginHandler(authApp *services.AuthApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.WechatLoginReq) {
		result, err := authApp.WechatLogin(ctx.Request.Context(), req)
		if common.HandleRouterError(ctx, err, "wechat login failed", errcode.ErrLogin) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(result))
	})
}

// WechatAccountRouter registers binding behind the authenticated API group.
func WechatAccountRouter(authApp *services.AuthApp) authRouterFn {
	return func(router gin.IRouter) {
		router.POST("/auth/wx-bind-email", bindWechatEmailHandler(authApp))
	}
}

// bindWechatEmailHandler 微信账号绑定邮箱
// @Summary 给微信账号绑定经过验证码验证的邮箱
// @Description 返回新 Bearer 会话并撤销当前会话；不同账号不合并，已有邮箱不替换
// @Tags Auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body dto.BindEmailReq true "邮箱绑定请求"
// @Success 200 {object} ginutils.Ret[dto.WechatLoginResp]
// @Router /v1/auth/wx-bind-email [post]
func bindWechatEmailHandler(authApp *services.AuthApp) gin.HandlerFunc {
	return ginutils.RequestHandler(func(ctx *gin.Context, req *dto.BindEmailReq) {
		result, err := authApp.BindWechatEmail(
			ctx.Request.Context(),
			common.UserID(ctx),
			common.SessionToken(ctx),
			req,
		)
		if common.HandleRouterError(ctx, err, "wechat email binding failed", errcode.ErrLogin) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(result))
	})
}
