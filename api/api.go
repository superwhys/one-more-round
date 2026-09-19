package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/api/middleware"
	"github.com/superwhys/one-more-round/api/router"
	_ "github.com/superwhys/one-more-round/cmd/swagger/docs"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// Status describes the running build.
type Status struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Stage   string `json:"stage"`
}

// stage marks the product phase the deployed build serves.
const stage = "invite-trial"

// API assembles the HTTP routes on top of the application services.
type API struct {
	version  string
	config   *config.Runtime
	authApp  *services.AuthApp
	groupApp *services.GroupApp
	roundApp *services.RoundApp
	photoApp *services.PhotoApp
}

// NewAPI wires the application services into the HTTP layer.
func NewAPI(version string, conf *config.Runtime, authApp *services.AuthApp, groupApp *services.GroupApp, roundApp *services.RoundApp, photoApp *services.PhotoApp) *API {
	return &API{version: version, config: conf, authApp: authApp, groupApp: groupApp, roundApp: roundApp, photoApp: photoApp}
}

// SetupRouter godoc
// @title 又一局 API
// @version 1.0
// @description 熟人小组对局日记的接口文档；路径相对 /api 挂载点。
// @description 除图片读取外，所有接口返回 ginutils.Ret 包络：{code, data, message}；code 为稳定业务码，失败时 HTTP 状态同时表达语义。
// @BasePath /api
// @securityDefinitions.apikey SessionCookie
// @in cookie
// @name omr_session
func (api *API) SetupRouter() http.Handler {
	handler := ginutils.NewServerHandler(
		ginutils.WithMiddleware(
			middleware.RecoveryMiddleware(),
			middleware.NoCacheMiddleware(),
			middleware.OriginMiddleware(api.config.Origin),
			observeOperations(),
		),
		ginutils.WithHandler(http.MethodGet, "/v1/status", statusHandler(api.version)),
		// The authentication entry points are the only routes without a session.
		ginutils.WithGroupHandlers(
			ginutils.WithPrefix("/v1/auth"),
			ginutils.WithRouterHandler(router.AuthRouter(api.authApp, api.sessionOptions()), router.GroupInvitationRouter(api.groupApp)),
		),
		// Everything else requires a current session and group membership.
		ginutils.WithGroupHandlers(
			ginutils.WithPrefix("/v1"),
			ginutils.WithMiddleware(
				middleware.TokenVerifyMiddleware(api.authApp),
				middleware.ContextInjectMiddleware(),
			),
			ginutils.WithRouterHandler(router.MeRouter(), router.GroupRouter(api.groupApp)),
			ginutils.WithGroupHandlers(
				ginutils.WithPrefix("/groups/:group"),
				ginutils.WithRouterHandler(
					router.GroupDetailRouter(api.groupApp, api.config.Origin),
					router.RoundRouter(api.roundApp),
					router.PhotoRouter(api.photoApp),
				),
			),
		),
	)

	handler.HandleMethodNotAllowed = true
	_ = handler.SetTrustedProxies(nil)
	handler.NoRoute(func(c *gin.Context) { common.RespondError(c, errcode.ErrRouteNotFound) })
	handler.NoMethod(func(c *gin.Context) { common.RespondError(c, errcode.ErrMethodNotAllowed) })
	return handler
}

// SwaggerRouter serves the generated API documentation; production builds
// expose it only when explicitly allowed.
func (api *API) SwaggerRouter(isProd bool) http.Handler {
	if isProd {
		return http.NotFoundHandler()
	}
	docs := httpSwagger.Handler()
	// http-swagger derives its paths from the unstripped request URI, so the
	// mount root would redirect with a doubled slash; serve the index instead.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			r.RequestURI = strings.TrimSuffix(r.RequestURI, "/") + "/index.html"
		}
		docs(w, r)
	})
}

// sessionOptions configures the session cookie: HTTPS deployments require the
// Secure attribute, and the cookie lifetime matches the server-side session.
func (api *API) sessionOptions() router.SessionOptions {
	return router.SessionOptions{
		Secure: strings.HasPrefix(api.config.Origin, "https://"),
		MaxAge: 30 * 24 * 3600,
	}
}

// statusHandler 服务健康与版本
// @Summary 服务健康与版本
// @Description 返回服务名、构建版本与产品阶段
// @Tags Status
// @Produce json
// @Success 200 {object} ginutils.Ret[api.Status]
// @Router /v1/status [get]
func statusHandler(version string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, dto.ResponseWithData(Status{Name: "one-more-round", Version: version, Stage: stage}))
	}
}
