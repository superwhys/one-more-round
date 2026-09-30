package dto

// User is the logged-in account.
type User struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Username string `json:"username,omitempty"`
}

// SendCodeReq asks for verification proof without making a registration decision.
type SendCodeReq struct {
	Email string `json:"email" validate:"required"`
}

// LoginReq exchanges proof for a session; only new accounts need an invitation.
type LoginReq struct {
	Email      string `json:"email"       validate:"required"`
	Code       string `json:"code"        validate:"required"`
	Invite     string `json:"invite"      validate:"max=128"`
	GroupToken string `json:"group_token"`
}

// PasswordLoginReq verifies an existing account with its username or bound email.
type PasswordLoginReq struct {
	Identifier string `json:"identifier" validate:"required,max=254"`
	Password   string `json:"password" mod:"-" validate:"required,max=128"`
}

// PasswordRegisterReq creates an invited username account and its first session.
type PasswordRegisterReq struct {
	Username   string `json:"username" validate:"required"`
	Password   string `json:"password" mod:"-" validate:"required,max=128"`
	Invite     string `json:"invite" validate:"max=128"`
	GroupToken string `json:"group_token" validate:"max=128"`
}

// SetPasswordReq opens or changes password access for the current account.
type SetPasswordReq struct {
	Username string `json:"username"`
	Password string `json:"password" mod:"-" validate:"required,max=128"`
}

// LoginResp includes the group joined atomically during a new registration.
type LoginResp struct {
	User
	GroupID string `json:"group_id,omitempty"`
}

// WechatLoginReq exchanges a wx.login code and optionally verifies an email.
// Existing email accounts are reused only before this WeChat identity registers.
type WechatLoginReq struct {
	Code       string `json:"code"        validate:"required,max=512"`
	Invite     string `json:"invite"      validate:"max=128"`
	GroupToken string `json:"group_token" validate:"max=128"`
	Email      string `json:"email"       validate:"max=254"`
	EmailCode  string `json:"email_code"  validate:"max=16"`
}

// BindEmailReq verifies a new email for the authenticated WeChat account.
type BindEmailReq struct {
	Email string `json:"email" validate:"required,max=254"`
	Code  string `json:"code"  validate:"required,max=16"`
}

// WechatLoginResp returns an opaque application session for mini-program memory.
type WechatLoginResp struct {
	LoginResp
	Token string `json:"token"`
}
