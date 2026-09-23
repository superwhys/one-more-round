// Package errcode defines the business error contract shared by the domain,
// application and API layers. Each error carries a stable business code, the
// HTTP status the API should use, and the message shown to the user.
package errcode

import "errors"

const (
	CodeBadRequest       = 100400
	CodeUnauthorized     = 100401
	CodeForbidden        = 100403
	CodeNotFound         = 100404
	CodeMethodNotAllowed = 100405
	CodeConflict         = 100409
	CodeTooManyRequests  = 100429
	CodeInternal         = 100500
	CodeBadGateway       = 100502
	CodeUnavailable      = 100503
)

// Error is a business error. It implements ginutils.ErrCoder through Code and
// ginutils.HTTPStatusCoder through HTTPStatus, so the response layer can emit
// the business code and the HTTP status independently.
type Error struct {
	ErrCode int
	status  int
	Message string
}

// New builds a business error with an explicit status and message.
func New(errCode, status int, message string) Error {
	return Error{ErrCode: errCode, status: status, Message: message}
}

// Error returns the user-facing message.
func (e Error) Error() string { return e.Message }

// Code returns the business code.
func (e Error) Code() int { return e.ErrCode }

// HTTPStatus returns the HTTP status the API should use.
func (e Error) HTTPStatus() int { return e.status }

// WithMessage returns a copy of the error carrying another message.
func (e Error) WithMessage(message string) Error {
	e.Message = message
	return e
}

// Is reports whether target is the same business error.
func (e Error) Is(target error) bool {
	switch t := target.(type) {
	case Error:
		return e.ErrCode != 0 && e.ErrCode == t.ErrCode
	case *Error:
		return t != nil && e.ErrCode != 0 && e.ErrCode == t.ErrCode
	default:
		return false
	}
}

var (
	// ErrSysInternal is the fallback for unexpected failures.
	ErrSysInternal = New(CodeInternal, 500, "服务暂时不可用，请重试")
	// ErrBadRequest is the fallback for rejected input.
	ErrBadRequest = New(CodeBadRequest, 400, "请求内容无效")
	// ErrNotFound reports a missing resource.
	ErrNotFound = New(CodeNotFound, 404, "内容不存在")
	// ErrUnauthorized reports a missing or expired session.
	ErrUnauthorized = New(CodeUnauthorized, 401, "请先登录")
	// ErrForbidden reports a denied action for the current member.
	ErrForbidden = New(CodeForbidden, 403, "没有访问或修改权限")
	// ErrConflict reports a state conflict such as a duplicate or stale write.
	ErrConflict = New(CodeConflict, 409, "内容已存在或已被关联，请刷新后重试")
	// ErrTooManyRequests reports a throttled operation.
	ErrTooManyRequests = New(CodeTooManyRequests, 429, "发送次数过多，请稍后重试")
)

// 身份与登录
var (
	ErrInvalidEmail      = ErrBadRequest.WithMessage("请填写有效邮箱")
	ErrResendTooSoon     = ErrTooManyRequests.WithMessage("请在 60 秒后重发验证码")
	ErrChallengeUpdated  = ErrConflict.WithMessage("验证码已更新，请使用最新邮件")
	ErrChallengeInvalid  = ErrBadRequest.WithMessage("验证码无效、已过期或尝试次数过多")
	ErrChallengeMismatch = ErrBadRequest.WithMessage("验证码不正确")
	ErrTrialInvalid      = ErrForbidden.WithMessage("试用邀请无效、已使用或已过期")
	ErrMailFailed        = New(CodeBadGateway, 502, "邮件发送失败，请稍后重试")
	ErrSendCode          = ErrSysInternal.WithMessage("发送验证码失败")
	ErrLogin             = ErrSysInternal.WithMessage("登录失败")
	ErrLogout            = ErrSysInternal.WithMessage("退出登录失败")
	ErrSession           = ErrSysInternal.WithMessage("读取登录状态失败")
)

// 小组、成员与玩家
var (
	ErrGroupNameInvalid  = ErrBadRequest.WithMessage("请填写小组名称（最多 255 字）")
	ErrGroupName         = ErrBadRequest.WithMessage("小组名称无效")
	ErrGroupSnapshot     = ErrSysInternal.WithMessage("读取小组失败")
	ErrGroupCreate       = ErrSysInternal.WithMessage("创建小组失败")
	ErrGroupList         = ErrSysInternal.WithMessage("获取小组列表失败")
	ErrGroupManage       = ErrSysInternal.WithMessage("小组操作失败")
	ErrPlayerName        = ErrBadRequest.WithMessage("请填写玩家昵称（最多 255 字）")
	ErrPlayerDuplicate   = ErrConflict.WithMessage("已有同名玩家，请选择已有档案或使用可区分昵称")
	ErrPlayerSave        = ErrSysInternal.WithMessage("保存玩家失败")
	ErrInviteInvalid     = ErrBadRequest.WithMessage("小组邀请无效，请向组主索取新链接")
	ErrInviteRevoked     = ErrBadRequest.WithMessage("小组邀请已撤销，请向组主索取新链接")
	ErrInviteExpired     = ErrBadRequest.WithMessage("小组邀请已过期，请向组主索取新链接")
	ErrInviteCreate      = ErrSysInternal.WithMessage("生成邀请失败")
	ErrInviteList        = ErrSysInternal.WithMessage("获取邀请列表失败")
	ErrJoin              = ErrSysInternal.WithMessage("加入小组失败")
	ErrOwnerTransfer     = ErrConflict.WithMessage("组主需先转让身份")
	ErrTransferTarget    = ErrBadRequest.WithMessage("只能转让给当前成员")
	ErrClaimLinked       = ErrConflict.WithMessage("该玩家已被关联或不存在")
	ErrClaimSelf         = ErrConflict.WithMessage("你已关联玩家档案")
	ErrClaimPending      = ErrConflict.WithMessage("已有待确认的玩家关联申请")
	ErrClaimAccount      = ErrConflict.WithMessage("该账号已有玩家档案")
	ErrClaimPlayer       = ErrConflict.WithMessage("该玩家已被关联")
	ErrActionUnsupported = ErrBadRequest.WithMessage("操作不支持")
)

// 游戏目录
var (
	ErrGameName  = ErrBadRequest.WithMessage("请填写游戏名称（最多 255 字）")
	ErrGameAlias = ErrBadRequest.WithMessage("游戏名称无效")
	ErrGameSave  = ErrSysInternal.WithMessage("保存游戏失败")
)

// 对局
var (
	ErrRoundSave        = ErrSysInternal.WithMessage("保存对局失败")
	ErrRoundList        = ErrSysInternal.WithMessage("读取对局失败")
	ErrRoundGet         = ErrSysInternal.WithMessage("读取对局失败")
	ErrRoundDelete      = ErrSysInternal.WithMessage("删除对局失败")
	ErrIdempotencyKey   = ErrBadRequest.WithMessage("缺少有效提交标识，请重新打开记局表单")
	ErrIdempotencyBody  = ErrConflict.WithMessage("同一提交标识的内容发生变化，请核对上次保存结果")
	ErrIdempotencyGone  = ErrConflict.WithMessage("该次提交的记录已经删除")
	ErrGameNotInGroup   = ErrBadRequest.WithMessage("游戏不属于当前小组")
	ErrPlayerNotInGroup = ErrBadRequest.WithMessage("玩家不属于当前小组")
	ErrRoundStale       = ErrConflict.WithMessage("这条记录刚刚被修改，请刷新后再试")
	ErrRoundDeleteStale = ErrConflict.WithMessage("记录已修改，请刷新后重试")
	ErrRoundShare       = ErrSysInternal.WithMessage("生成分享链接失败")
	ErrRoundShareRead   = ErrSysInternal.WithMessage("读取分享内容失败")
	ErrFilterDate       = ErrBadRequest.WithMessage("筛选日期无效")
	ErrFilterRange      = ErrBadRequest.WithMessage("开始日期不能晚于结束日期")
	ErrPageRange        = ErrBadRequest.WithMessage("分页范围无效")
)

// 对局评论
var (
	ErrCommentParent = ErrBadRequest.WithMessage("只能回复对局下的评论")
	ErrCommentSave   = ErrSysInternal.WithMessage("保存评论失败")
	ErrCommentList   = ErrSysInternal.WithMessage("读取评论失败")
	ErrCommentDelete = ErrSysInternal.WithMessage("删除评论失败")
)

// 照片
var (
	ErrPhotoUpload     = ErrBadRequest.WithMessage("图片上传失败，请检查格式、大小和尺寸后重试")
	ErrPhotoNotFound   = ErrBadRequest.WithMessage("照片不存在或不属于当前小组")
	ErrPhotoLinked     = ErrConflict.WithMessage("照片已关联其他对局")
	ErrPhotoSave       = ErrSysInternal.WithMessage("保存照片失败")
	ErrPhotoRead       = ErrSysInternal.WithMessage("读取照片失败")
	ErrPhotoStorage    = New(CodeUnavailable, 503, "图片存储服务暂时不可用，请稍后重试")
	ErrPhotoMissing    = ErrBadRequest.WithMessage("缺少图片")
	ErrPhotoTooLarge   = ErrBadRequest.WithMessage("请选择不超过 2 MB 的图片")
	ErrPhotoDimensions = ErrBadRequest.WithMessage("图片长边不能超过 1600 像素，请刷新页面后重新选择照片")
	ErrPhotoBusy       = New(CodeUnavailable, 503, "正在处理其他照片，请稍后重试")
)

// 外部依赖
var (
	ErrBGGUnavailable = New(CodeUnavailable, 503, "BGG 授权尚未配置，请使用本组桌游或手动添加")
)

// HTTP 协议层
var (
	ErrRouteNotFound    = New(CodeNotFound, 404, "接口不存在")
	ErrMethodNotAllowed = New(CodeMethodNotAllowed, 405, "请求方法不支持")
	ErrOrigin           = ErrForbidden.WithMessage("请求来源校验失败，请从本站重试")
	ErrPhotoForm        = ErrBadRequest.WithMessage("请选择不超过 2 MB 的图片")
)

// AsErrcode extracts the business error from err.
func AsErrcode(err error) (Error, bool) {
	if err == nil {
		return Error{}, false
	}
	var e Error
	if errors.As(err, &e) {
		return e, true
	}
	return Error{}, false
}
