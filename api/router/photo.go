package router

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// maxPhotoRequest bounds the whole multipart request, leaving room for the
// multipart envelope around the ten megabyte image limit.
const maxPhotoRequest = 10*1024*1024 + 64*1024

type photoRouterFn func(router gin.IRouter)

func (fn photoRouterFn) Init(router gin.IRouter) { fn(router) }

// PhotoRouter registers the photo upload and read endpoints of one group.
func PhotoRouter(photoApp *services.PhotoApp) photoRouterFn {
	return func(router gin.IRouter) {
		router.POST("/photos", uploadPhotoHandler(photoApp))
		router.GET("/photos/:id", readPhotoHandler(photoApp))
	}
}

// uploadPhotoHandler 上传照片
// @Summary 上传照片
// @Description 上传单张 JPEG/PNG/WebP 图片，服务端重编码后保存
// @Tags Photo
// @Accept multipart/form-data
// @Produce json
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param photo formData file true "图片文件"
// @Success 200 {object} dto.UploadPhotoResp
// @Router /v1/groups/{group}/photos [post]
func uploadPhotoHandler(photoApp *services.PhotoApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxPhotoRequest)
		if err := ctx.Request.ParseMultipartForm(1024 * 1024); err != nil {
			common.RespondError(ctx, errcode.ErrPhotoTooLarge)
			return
		}
		defer ctx.Request.MultipartForm.RemoveAll()
		file, _, err := ctx.Request.FormFile("photo")
		if err != nil {
			common.RespondError(ctx, errcode.ErrPhotoMissing)
			return
		}
		defer file.Close()
		id, err := photoApp.Upload(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), file)
		if common.HandleRouterError(ctx, err, "upload photo failed", errcode.ErrPhotoSave) {
			return
		}
		ctx.JSON(http.StatusOK, dto.ResponseWithData(dto.UploadPhotoResp{ID: id}))
	}
}

// readPhotoHandler 读取照片
// @Summary 读取照片
// @Description 校验成员权限后返回图片，size=thumb 时返回缩略图
// @Tags Photo
// @Produce image/jpeg
// @Security SessionCookie
// @Param group path string true "小组 ID"
// @Param id path string true "照片 ID"
// @Param size query string false "缩略图传 thumb"
// @Success 200 {file} file
// @Router /v1/groups/{group}/photos/{id} [get]
func readPhotoHandler(photoApp *services.PhotoApp) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		thumb := strings.EqualFold(ctx.Query("size"), "thumb")
		data, err := photoApp.Read(ctx.Request.Context(), ctx.Param("group"), common.UserID(ctx), ctx.Param("id"), thumb)
		if common.HandleRouterError(ctx, err, "read photo failed", errcode.ErrPhotoRead) {
			return
		}
		ctx.Header("Cache-Control", "private, no-store")
		ctx.Header("X-Content-Type-Options", "nosniff")
		ctx.Data(http.StatusOK, "image/jpeg", data)
	}
}
