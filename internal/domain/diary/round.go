package diary

import (
	"errors"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("请检查填写的内容")
var scorePattern = regexp.MustCompile(`^-?(0|[1-9][0-9]{0,11})(\.[0-9]{1,4})?$`)

type Team struct {
	ID, Name string
	Players  []string
	Score    *string
	Winner   bool
}
type Round struct {
	ID, GroupID, GameID, Date, Mode, Outcome string
	Players, Winners                         []string
	Scores                                   map[string]*string
	Teams                                    []Team
	TeamScore                                *string
	Memory, Location                         string
	Minutes                                  *int
	Photos                                   []string
	Author, UpdatedBy                        string
	UpdatedAt                                time.Time
	DeletedAt                                *time.Time
	Version                                  int
}

// Share is the revocable public access grant for one round. Only the digest of
// its bearer token is persisted.
type Share struct {
	RoundID, GroupID, TokenHash, CreatedBy string
	CreatedAt                              time.Time
	RevokedAt                              *time.Time
}

// Active reports whether the public link can still be used.
func (s Share) Active() bool { return s.RevokedAt == nil }

func ValidScore(s *string) bool { return s == nil || scorePattern.MatchString(*s) }
func (r Round) Validate() error {
	if _, err := time.Parse("2006-01-02", r.Date); err != nil {
		return errors.New("请填写有效的对局日期")
	}
	if r.GameID == "" || len(r.Players) == 0 {
		return errors.New("请选择游戏和玩家")
	}
	seen := map[string]bool{}
	for _, p := range r.Players {
		if p == "" || seen[p] {
			return errors.New("玩家不能重复")
		}
		seen[p] = true
	}
	if utf8.RuneCountInString(r.Memory) > 500 {
		return errors.New("回忆最多 500 字")
	}
	if len(r.Photos) > 3 {
		return errors.New("照片最多 3 张")
	}
	photos := map[string]bool{}
	for _, id := range r.Photos {
		if id == "" || photos[id] {
			return ErrInvalid
		}
		photos[id] = true
	}
	if r.Minutes != nil && *r.Minutes <= 0 {
		return errors.New("时长必须为正整数分钟")
	}
	if !ValidScore(r.TeamScore) {
		return errors.New("分数最多 12 位整数和 4 位小数")
	}
	for p, s := range r.Scores {
		if !seen[p] || !ValidScore(s) {
			return errors.New("玩家分数不合法")
		}
	}
	wins := map[string]bool{}
	for _, p := range r.Winners {
		if !seen[p] || wins[p] {
			return errors.New("获胜玩家不合法")
		}
		wins[p] = true
	}
	switch r.Mode {
	case "individual":
		if len(r.Players) < 2 {
			return errors.New("个人竞技至少两位玩家")
		}
		if len(r.Teams) > 0 || r.TeamScore != nil {
			return ErrInvalid
		}
		if r.Outcome != "win" && r.Outcome != "draw" && r.Outcome != "unknown" {
			return ErrInvalid
		}
		if (r.Outcome == "win") != (len(r.Winners) > 0) {
			return errors.New("请选择获胜玩家，或清空不适用的结果")
		}
	case "coop":
		if r.Outcome != "win" && r.Outcome != "loss" && r.Outcome != "unknown" {
			return ErrInvalid
		}
		if len(r.Teams) > 0 || len(r.Winners) > 0 || len(r.Scores) > 0 {
			return ErrInvalid
		}
	case "team":
		if len(r.Teams) < 2 || len(r.Scores) > 0 || r.TeamScore != nil || len(r.Winners) > 0 {
			return errors.New("组队至少两队，结果和分数需填写在队伍上")
		}
		if r.Outcome != "win" && r.Outcome != "draw" && r.Outcome != "unknown" {
			return ErrInvalid
		}
		assigned := map[string]bool{}
		ids := map[string]bool{}
		winning := 0
		for _, t := range r.Teams {
			if t.ID == "" || ids[t.ID] || t.Name == "" || len(t.Players) == 0 || !ValidScore(t.Score) {
				return errors.New("每队需要名称、独立标识和至少一位玩家")
			}
			ids[t.ID] = true
			if t.Winner {
				winning++
			}
			for _, p := range t.Players {
				if !seen[p] || assigned[p] {
					return errors.New("每位玩家必须恰好属于一队")
				}
				assigned[p] = true
			}
		}
		if len(assigned) != len(seen) || (r.Outcome == "win") != (winning > 0) {
			return errors.New("请完成分队与队伍结果")
		}
	default:
		return ErrInvalid
	}
	return nil
}
func (r Round) Won(player string) bool {
	if !slices.Contains(r.Players, player) || r.Outcome != "win" {
		return false
	}
	if r.Mode == "coop" {
		return true
	}
	if r.Mode == "individual" {
		return slices.Contains(r.Winners, player)
	}
	for _, t := range r.Teams {
		if t.Winner && slices.Contains(t.Players, player) {
			return true
		}
	}
	return false
}
