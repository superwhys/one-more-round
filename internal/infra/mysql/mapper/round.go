package mapper

import (
	"encoding/json"
	"time"

	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// roundDocument is the JSON payload of the omr_rounds.body column. The indexed
// group, game, date and version columns stay outside this document.
type roundDocument struct {
	ID        string             `json:"id"`
	GameID    string             `json:"game_id"`
	Date      string             `json:"date"`
	Mode      string             `json:"mode"`
	Outcome   string             `json:"outcome"`
	Players   []string           `json:"players"`
	Winners   []string           `json:"winners"`
	Scores    map[string]*string `json:"scores"`
	Teams     []teamDocument     `json:"teams"`
	TeamScore *string            `json:"team_score"`
	Memory    string             `json:"memory"`
	Minutes   *int               `json:"minutes"`
	Photos    []string           `json:"photos"`
	Author    string             `json:"author"`
	UpdatedBy string             `json:"updated_by"`
	UpdatedAt time.Time          `json:"updated_at"`
	Version   int                `json:"version"`
}

// teamDocument is the stored form of a team inside the round document.
type teamDocument struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Players []string `json:"players"`
	Score   *string  `json:"score"`
	Winner  bool     `json:"winner"`
}

// RoundModelToDomain decodes a stored round row into the domain aggregate.
func RoundModelToDomain(m *models.Round) (*diary.Round, error) {
	if m == nil {
		return nil, nil
	}
	var document roundDocument
	if err := json.Unmarshal(m.Body, &document); err != nil {
		return nil, err
	}
	teams := make([]diary.Team, 0, len(document.Teams))
	for _, t := range document.Teams {
		teams = append(
			teams,
			diary.Team{
				ID:      t.ID,
				Name:    t.Name,
				Players: t.Players,
				Score:   t.Score,
				Winner:  t.Winner,
			},
		)
	}
	return &diary.Round{
		ID:        document.ID,
		GroupID:   m.GroupID,
		GameID:    document.GameID,
		Date:      document.Date,
		Mode:      document.Mode,
		Outcome:   document.Outcome,
		Players:   document.Players,
		Winners:   document.Winners,
		Scores:    document.Scores,
		Teams:     teams,
		TeamScore: document.TeamScore,
		Memory:    document.Memory,
		Minutes:   document.Minutes,
		Photos:    document.Photos,
		Author:    document.Author,
		UpdatedBy: document.UpdatedBy,
		UpdatedAt: document.UpdatedAt,
		DeletedAt: m.DeletedAt,
		Version:   document.Version,
	}, nil
}

// RoundDomainToModel encodes the round aggregate into its stored row.
func RoundDomainToModel(r *diary.Round) (*models.Round, error) {
	if r == nil {
		return nil, nil
	}
	teams := make([]teamDocument, 0, len(r.Teams))
	for _, t := range r.Teams {
		teams = append(
			teams,
			teamDocument{
				ID:      t.ID,
				Name:    t.Name,
				Players: t.Players,
				Score:   t.Score,
				Winner:  t.Winner,
			},
		)
	}
	body, err := json.Marshal(
		roundDocument{
			ID:        r.ID,
			GameID:    r.GameID,
			Date:      r.Date,
			Mode:      r.Mode,
			Outcome:   r.Outcome,
			Players:   r.Players,
			Winners:   r.Winners,
			Scores:    r.Scores,
			Teams:     teams,
			TeamScore: r.TeamScore,
			Memory:    r.Memory,
			Minutes:   r.Minutes,
			Photos:    r.Photos,
			Author:    r.Author,
			UpdatedBy: r.UpdatedBy,
			UpdatedAt: r.UpdatedAt,
			Version:   r.Version,
		},
	)
	if err != nil {
		return nil, err
	}
	return &models.Round{
		ID:        r.ID,
		GroupID:   r.GroupID,
		GameID:    r.GameID,
		Played:    r.Date,
		Version:   r.Version,
		Body:      body,
		DeletedAt: r.DeletedAt,
	}, nil
}
