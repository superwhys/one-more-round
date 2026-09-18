package dto

import "time"

// Group is one group of friends.
type Group struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Owner string `json:"owner"`
}

// Member is an account belonging to the group.
type Member struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// Player is a nickname profile of the group.
type Player struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Account *string `json:"account"`
}

// Claim is a pending account-to-player link awaiting owner approval.
type Claim struct {
	UserID   string `json:"user_id"`
	PlayerID string `json:"player_id"`
}

// Invite is a group invitation; the plaintext token is returned only once at
// creation and never stored.
type Invite struct {
	ID      string    `json:"id"`
	Expires time.Time `json:"expires"`
	Revoked bool      `json:"revoked"`
}

// Snapshot is the group with everything the member may see.
type Snapshot struct {
	Locations []string `json:"locations"`
	Group     Group    `json:"group"`
	Members   []Member `json:"members"`
	Players   []Player `json:"players"`
	Games     []Game   `json:"games"`
	Claims    []Claim  `json:"claims"`
}

// CreateGroupReq creates a group.
type CreateGroupReq struct {
	Name string `json:"name" validate:"required"`
}

// JoinReq joins a group with an invitation token.
type JoinReq struct {
	Token string `json:"token" validate:"required"`
}

// AddPlayerReq creates a nickname profile.
type AddPlayerReq struct {
	Name string `json:"name" validate:"required"`
}

// ManageReq applies an action on the group.
type ManageReq struct {
	Action string `json:"action" validate:"required"`
	Target string `json:"target"`
	Value  string `json:"value"`
}

// InviteResp carries the one-time invitation link.
type InviteResp struct {
	ID      string    `json:"id"`
	URL     string    `json:"url"`
	Expires time.Time `json:"expires"`
}
