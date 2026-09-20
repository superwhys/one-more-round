package dto

import "time"

// Team is one team of a team-competition round; the result and the optional
// score belong to the team.
type Team struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Players []string `json:"players"`
	Score   *string  `json:"score"`
	Winner  bool     `json:"winner"`
}

// Round is one recorded game session.
type Round struct {
	ID        string             `json:"id"`
	GameID    string             `json:"game_id"`
	Date      string             `json:"date"`
	Mode      string             `json:"mode"`
	Outcome   string             `json:"outcome"`
	Players   []string           `json:"players"`
	Winners   []string           `json:"winners"`
	Scores    map[string]*string `json:"scores"`
	Teams     []Team             `json:"teams"`
	TeamScore *string            `json:"team_score"`
	Memory    string             `json:"memory"`
	Location  string             `json:"location"`
	Minutes   *int               `json:"minutes"`
	Photos    []string           `json:"photos"`
	Author    string             `json:"author"`
	UpdatedBy string             `json:"updated_by"`
	UpdatedAt time.Time          `json:"updated_at"`
	DeletedAt *time.Time         `json:"deleted_at"`
	Version   int                `json:"version"`
}

// SaveRoundReq is the round body of a create or edit request. The round fields
// are inlined in the JSON body; the path and the submission key come from the
// request metadata.
type SaveRoundReq struct {
	Round
	GroupID        string `uri:"group"`
	RoundID        string `uri:"id"`
	IdempotencyKey string `header:"Idempotency-Key"`
}

// DeleteRoundReq deletes a round at the version the client last saw.
type DeleteRoundReq struct {
	GroupID string `uri:"group"`
	RoundID string `uri:"id"`
	Version int    `json:"version"`
}

// ListRoundsReq filters the timeline; every field is optional and an empty
// value means "no restriction".
type ListRoundsReq struct {
	GroupID   string `uri:"group"`
	From      string `form:"from"`
	To        string `form:"to"`
	Game      string `form:"game"`
	Player    string `form:"player"`
	Query     string `form:"q"`
	Location  string `form:"location"`
	Mode      string `form:"mode"`
	Outcome   string `form:"outcome"`
	HasPhotos *bool  `form:"has_photos"`
	Offset    int    `form:"offset"`
	Limit     int    `form:"limit"`
}

// RecapReq selects one calendar month (YYYY-MM) or year (YYYY).
type RecapReq struct {
	GroupID string `uri:"group"`
	Period  string `form:"period" validate:"required"`
}

// Recap contains highlights derived from all rounds in the selected period.
type Recap struct {
	Period        string   `json:"period"`
	From          string   `json:"from"`
	To            string   `json:"to"`
	Rounds        int      `json:"rounds"`
	Games         int      `json:"games"`
	Players       int      `json:"players"`
	Minutes       int      `json:"minutes"`
	TopGame       string   `json:"top_game"`
	TopGameRounds int      `json:"top_game_rounds"`
	TopPlayer     string   `json:"top_player"`
	TopPlays      int      `json:"top_plays"`
	Photos        []string `json:"photos"`
}

// RestoreRoundReq restores a soft-deleted round at the shown version.
type RestoreRoundReq struct {
	GroupID string `uri:"group"`
	RoundID string `uri:"id"`
	Version int    `json:"version"`
}

// Stat is the record of one player in one game and mode.
type Stat struct {
	Game    string `json:"game"`
	Player  string `json:"player"`
	Mode    string `json:"mode"`
	Played  int    `json:"played"`
	Wins    int    `json:"wins"`
	Samples int    `json:"samples"`
}

// GameActivity is the recent activity of one game.
type GameActivity struct {
	Count    int    `json:"count"`
	LastDate string `json:"last_date"`
}

// Page is the filtered timeline with its statistics. Play counts include
// rounds without a recorded result; rates exclude them.
type Page struct {
	Activity map[string]GameActivity `json:"activity"`
	Items    []Round                 `json:"items"`
	Total    int                     `json:"total"`
	Games    int                     `json:"games"`
	Players  int                     `json:"players"`
	Stats    []Stat                  `json:"stats"`
}
