package dto

// Game is one game of the group catalogue; original keeps the external name.
type Game struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Original string `json:"original"`
	BGGID    *int   `json:"bgg_id"`
	Cover    string `json:"cover,omitempty"`
}

// AddGameReq adds a game by its local name.
type AddGameReq struct {
	GroupID string `uri:"group"`
	Name    string `json:"name" validate:"required"`
}

// ExternalGame is one BoardGameGeek search hit. Year is omitted when unknown.
type ExternalGame struct {
	BGGID     int    `json:"bgg_id"`
	Name      string `json:"name"`
	Year      *int   `json:"year"`
	Thumbnail string `json:"thumbnail,omitempty"`
}

// ExternalSearch is a page of external games plus the required source credit.
type ExternalSearch struct {
	Items     []ExternalGame `json:"items"`
	Source    string         `json:"source"`
	SourceURL string         `json:"source_url"`
}

// SearchExternalGamesReq searches the external catalogue for one group.
type SearchExternalGamesReq struct {
	GroupID string `uri:"group"`
	Query   string `form:"q"`
}

// SyncCoverReq attaches an external cover to a game already on the shelf.
type SyncCoverReq struct {
	GroupID string `uri:"group"`
	GameID  string `uri:"game"`
	BGGID   int    `json:"bgg_id"`
}

// ImportGameReq copies one external game into the group catalogue.
type ImportGameReq struct {
	GroupID string `uri:"group"`
	BGGID   int    `json:"bgg_id" validate:"required,min=1"`
	Name    string `json:"name"`
}
