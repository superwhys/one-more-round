package mapper

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/diary"
)

// RoundDTOToDomain converts a submitted round body into the domain aggregate
// bound to group groupID.
func RoundDTOToDomain(r *dto.Round, groupID string) *diary.Round {
	if r == nil {
		return nil
	}
	teams := make([]diary.Team, 0, len(r.Teams))
	for _, t := range r.Teams {
		teams = append(teams, diary.Team{ID: t.ID, Name: t.Name, Players: t.Players, Score: t.Score, Winner: t.Winner})
	}
	return &diary.Round{ID: r.ID, GroupID: groupID, GameID: r.GameID, Date: r.Date, Mode: r.Mode, Outcome: r.Outcome, Players: r.Players, Winners: r.Winners, Scores: r.Scores, Teams: teams, TeamScore: r.TeamScore, Memory: r.Memory, Minutes: r.Minutes, Photos: r.Photos, Author: r.Author, UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt, Version: r.Version}
}

// RoundDomainToDTO converts the round aggregate into the API DTO.
func RoundDomainToDTO(r *diary.Round) dto.Round {
	if r == nil {
		return dto.Round{}
	}
	teams := make([]dto.Team, 0, len(r.Teams))
	for _, t := range r.Teams {
		teams = append(teams, dto.Team{ID: t.ID, Name: t.Name, Players: t.Players, Score: t.Score, Winner: t.Winner})
	}
	return dto.Round{ID: r.ID, GameID: r.GameID, Date: r.Date, Mode: r.Mode, Outcome: r.Outcome, Players: r.Players, Winners: r.Winners, Scores: r.Scores, Teams: teams, TeamScore: r.TeamScore, Memory: r.Memory, Minutes: r.Minutes, Photos: r.Photos, Author: r.Author, UpdatedBy: r.UpdatedBy, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt, Version: r.Version}
}

// RecapDomainToDTO converts period highlights into the API contract.
func RecapDomainToDTO(period string, r *diary.Recap) dto.Recap {
	return dto.Recap{Period: period, From: r.From, To: r.To, Rounds: r.Rounds, Games: r.Games, Players: r.Players, Minutes: r.Minutes, TopGame: r.TopGame, TopGameRounds: r.TopGameRounds, TopPlayer: r.TopPlayer, TopPlays: r.TopPlays, Photos: r.Photos}
}

// RoundDomainListToDTOList converts round aggregates into API DTOs.
func RoundDomainListToDTOList(rounds []*diary.Round) []dto.Round {
	items := make([]dto.Round, 0, len(rounds))
	for _, r := range rounds {
		items = append(items, RoundDomainToDTO(r))
	}
	return items
}

// PageDomainToDTO converts the filtered timeline with its statistics.
func PageDomainToDTO(p *diary.Page) dto.Page {
	page := dto.Page{Activity: map[string]dto.GameActivity{}, Items: []dto.Round{}, Stats: []dto.Stat{}}
	if p == nil {
		return page
	}
	for gameID, activity := range p.Activity {
		page.Activity[gameID] = dto.GameActivity{Count: activity.Count, LastDate: activity.LastDate}
	}
	page.Items = RoundDomainListToDTOList(p.Items)
	page.Total = p.Total
	page.Games = p.Games
	page.Players = p.Players
	for _, s := range p.Stats {
		page.Stats = append(page.Stats, dto.Stat{Game: s.Game, Player: s.Player, Mode: s.Mode, Played: s.Played, Wins: s.Wins, Samples: s.Samples})
	}
	return page
}
