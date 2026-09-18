package diary

import (
	"sort"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

// Filter narrows the rounds of a timeline. Empty fields mean "no restriction".
type Filter struct {
	From, To, Game, Player string
	Offset, Limit          int
}

// Stat is the record of one player in one game and mode.
type Stat struct {
	Game    string
	Player  string
	Mode    string
	Played  int
	Wins    int
	Samples int
}

// Activity is the recent activity of one game.
type Activity struct {
	Count    int
	LastDate string
}

// Page is the filtered timeline with its statistics.
type Page struct {
	Activity map[string]Activity
	Items    []*Round
	Total    int
	Games    int
	Players  int
	Stats    []*Stat
}

// ValidateFilter rejects an unusable timeline filter before any round is read.
func ValidateFilter(f Filter) error {
	for _, d := range []string{f.From, f.To} {
		if d == "" {
			continue
		}
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return errcode.ErrFilterDate
		}
	}
	if f.From != "" && f.To != "" && f.From > f.To {
		return errcode.ErrFilterRange
	}
	if f.Limit < 1 || f.Limit > 100 || f.Offset < 0 {
		return errcode.ErrPageRange
	}
	return nil
}

// List validates the filter and returns the matching page. Rounds without a
// recorded result stay in the play count but are excluded from the rate.
func List(rounds []*Round, f Filter) (*Page, error) {
	if err := ValidateFilter(f); err != nil {
		return nil, err
	}
	page := &Page{Activity: map[string]Activity{}, Items: []*Round{}, Stats: []*Stat{}}
	selected := []*Round{}
	games := map[string]bool{}
	players := map[string]bool{}
	stats := map[string]*Stat{}
	for _, r := range rounds {
		if f.Game != "" && r.GameID != f.Game || f.Player != "" && !contains(r.Players, f.Player) || f.From != "" && r.Date < f.From || f.To != "" && r.Date > f.To {
			continue
		}
		selected = append(selected, r)
		activity := page.Activity[r.GameID]
		activity.Count++
		if r.Date > activity.LastDate {
			activity.LastDate = r.Date
		}
		page.Activity[r.GameID] = activity
		games[r.GameID] = true
		for _, p := range r.Players {
			players[p] = true
			key := r.GameID + ":" + p + ":" + r.Mode
			if stats[key] == nil {
				stats[key] = &Stat{Game: r.GameID, Player: p, Mode: r.Mode}
			}
			s := stats[key]
			s.Played++
			if r.Outcome != "unknown" {
				s.Samples++
				if r.Won(p) {
					s.Wins++
				}
			}
		}
	}
	page.Total = len(selected)
	page.Games = len(games)
	page.Players = len(players)
	for _, s := range stats {
		page.Stats = append(page.Stats, s)
	}
	sort.Slice(page.Stats, func(i, j int) bool {
		a, b := page.Stats[i], page.Stats[j]
		return a.Game+a.Player+a.Mode < b.Game+b.Player+b.Mode
	})
	if f.Offset < len(selected) {
		end := min(f.Offset+f.Limit, len(selected))
		page.Items = selected[f.Offset:end]
	}
	return page, nil
}

// contains reports whether the slice holds the value.
func contains(values []string, v string) bool {
	for _, item := range values {
		if item == v {
			return true
		}
	}
	return false
}
