package dto

// User is the logged-in account.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// SendCodeReq asks for a login verification code; invite carries the trial
// invitation of a first registration.
type SendCodeReq struct {
	Email      string `json:"email"       validate:"required"`
	Invite     string `json:"invite"`
	GroupToken string `json:"group_token"`
}

// LoginReq exchanges a verification code for a session.
type LoginReq struct {
	Email      string `json:"email"       validate:"required"`
	Code       string `json:"code"        validate:"required"`
	GroupToken string `json:"group_token"`
}

// LoginResp includes the group joined atomically during an invitation login.
type LoginResp struct {
	User
	GroupID string `json:"group_id,omitempty"`
}

// WechatLoginReq exchanges a wx.login code and optionally verifies an email.
// Existing email accounts are reused only before this WeChat identity registers.
type WechatLoginReq struct {
	Code       string `json:"code" validate:"required,max=512"`
	Invite     string `json:"invite" validate:"max=128"`
	GroupToken string `json:"group_token" validate:"max=128"`
	Email      string `json:"email" validate:"max=254"`
	EmailCode  string `json:"email_code" validate:"max=16"`
}

// BindEmailReq verifies a new email for the authenticated WeChat account.
type BindEmailReq struct {
	Email string `json:"email" validate:"required,max=254"`
	Code  string `json:"code" validate:"required,max=16"`
}

// WechatLoginResp returns an opaque application session for mini-program memory.
type WechatLoginResp struct {
	LoginResp
	Token string `json:"token"`
}
