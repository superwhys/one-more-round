// Package group owns groups, their members, nickname players, join invitations
// and the account-to-player claims awaiting owner approval.
package group

import (
	"time"

	"github.com/superwhys/one-more-round/internal/domain/game"
)

// InviteTTL is how long a group invitation stays valid.
const InviteTTL = 7 * 24 * time.Hour

// Group is a private group of friends.
type Group struct {
	ID    string
	Name  string
	Owner string
}

// Member is an account that belongs to the group.
type Member struct {
	UserID string
	Email  string
}

// Player is a nickname profile that takes part in rounds; it is isolated per
// group and may be linked to at most one account.
type Player struct {
	ID      string
	Name    string
	Account *string
}

// Linked reports whether the player already belongs to an account.
func (p *Player) Linked() bool { return p.Account != nil }

// Invite is a group invitation granting membership, never the records.
type Invite struct {
	ID      string
	Expires time.Time
	Revoked bool
}

// Claim is a pending request to link the account to a player profile.
type Claim struct {
	UserID   string
	PlayerID string
}

// Snapshot is the read model of one group with its members, players, games and
// claims.
type Snapshot struct {
	Group   *Group
	Members []*Member
	Players []*Player
	Games   []*game.Game
	Claims  []*Claim
}

// IsOwner reports whether the account owns the group.
func (s *Snapshot) IsOwner(userID string) bool { return s.Group != nil && s.Group.Owner == userID }

// IsMember reports whether the account is a current member.
func (s *Snapshot) IsMember(userID string) bool {
	for _, m := range s.Members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}

// HasGame reports whether the game belongs to the group.
func (s *Snapshot) HasGame(gameID string) bool {
	for _, g := range s.Games {
		if g.ID == gameID {
			return true
		}
	}
	return false
}

// HasPlayer reports whether the player belongs to the group.
func (s *Snapshot) HasPlayer(playerID string) bool {
	for _, p := range s.Players {
		if p.ID == playerID {
			return true
		}
	}
	return false
}
