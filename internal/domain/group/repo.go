package group

import (
	"context"
	"time"
)

// IGroupRepository reads and writes groups, their members and their claims.
type IGroupRepository interface {
	// ListByUser returns the groups the account belongs to.
	ListByUser(ctx context.Context, userID string) ([]*Group, error)
	// GetByID returns the group under a write lock.
	GetByID(ctx context.Context, id string) (*Group, error)
	// Create stores the group and its first member.
	Create(ctx context.Context, g *Group, ownerID string) error
	// Save updates the group name and owner.
	Save(ctx context.Context, g *Group) error
	// Snapshot reads the group with its members, players, games and claims.
	Snapshot(ctx context.Context, id string) (*Snapshot, error)
	// AddMember adds the account to the group, ignoring an existing membership.
	AddMember(ctx context.Context, groupID, userID string) error
	// RemoveMember removes the membership and the account's pending claim.
	RemoveMember(ctx context.Context, groupID, userID string) error
}

// IPlayerRepository reads and writes nickname profiles of a group.
type IPlayerRepository interface {
	// Save inserts the player or updates an existing row of the same group.
	Save(ctx context.Context, groupID string, p *Player) error
}

// IClaimRepository reads and writes pending account claims of a group.
type IClaimRepository interface {
	// Save stores the account's claim, replacing an earlier one.
	Save(ctx context.Context, groupID string, c *Claim) error
	// Delete removes the account's pending claim.
	Delete(ctx context.Context, groupID, userID string) error
}

// IInviteRepository reads and writes group invitations by token digest.
type IInviteRepository interface {
	// Create stores an invitation holding the token digest.
	Create(ctx context.Context, groupID string, inv *Invite, hash string) error
	// ListByGroup returns the group's invitations.
	ListByGroup(ctx context.Context, groupID string) ([]*Invite, error)
	// Revoke disables an unused invitation.
	Revoke(ctx context.Context, groupID, id string) error
	// Resolve returns the group of a live, unrevoked invitation digest.
	Resolve(ctx context.Context, hash string, now time.Time) (string, error)
}
