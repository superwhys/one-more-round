package dto

// User is the logged-in account.
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

// SendCodeReq asks for a login verification code; invite carries the trial
// invitation of a first registration.
type SendCodeReq struct {
	Email      string `json:"email" validate:"required"`
	Invite     string `json:"invite"`
	GroupToken string `json:"group_token"`
}

// LoginReq exchanges a verification code for a session.
type LoginReq struct {
	Email      string `json:"email" validate:"required"`
	Code       string `json:"code" validate:"required"`
	GroupToken string `json:"group_token"`
}

// LoginResp includes the group joined atomically during an invitation login.
type LoginResp struct {
	User
	GroupID string `json:"group_id,omitempty"`
}
