package dto

// Wishlist lists the games the group wants to play.
type Wishlist struct {
	GameIDs []string `json:"game_ids"`
}

// GameWishPathReq identifies one game in a group.
type GameWishPathReq struct {
	GroupID string `uri:"group" json:"-" form:"-" header:"-" validate:"required"`
	GameID  string `uri:"game" json:"-" form:"-" header:"-" validate:"required"`
}
