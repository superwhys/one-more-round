package dto

// Game is one game of the group catalogue; original keeps the external name.
type Game struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Original string `json:"original"`
	BGGID    *int   `json:"bgg_id"`
}

// AddGameReq adds a game by its local name.
type AddGameReq struct {
	Name string `json:"name" validate:"required"`
}
