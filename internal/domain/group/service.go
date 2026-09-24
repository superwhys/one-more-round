package group

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// IService defines the group use cases over the group repositories.
type IService interface {
	// List returns the groups the account belongs to.
	List(ctx context.Context, userID string) ([]*Group, error)
	// Create creates a group owned by the account, which becomes its first member.
	Create(ctx context.Context, ownerID, name string) (*Group, error)
	// RequireMember returns the group snapshot and fails unless the account is
	// a current member.
	RequireMember(ctx context.Context, groupID, userID string) (*Snapshot, error)
	// AddPlayer creates a nickname profile, rejecting a duplicate name.
	AddPlayer(ctx context.Context, groupID, userID, name string) (*Player, error)
	// AddOwnerPlayer creates and links the owner's own player profile.
	AddOwnerPlayer(ctx context.Context, groupID, ownerID, name string) (*Player, error)
	// Invite creates a seven-day group invitation and returns the plaintext
	// token exactly once.
	Invite(ctx context.Context, groupID, userID string, now time.Time) (*Invite, string, error)
	// Invites lists the group's invitations for the owner.
	Invites(ctx context.Context, groupID, userID string) ([]*Invite, error)
	// InvitedGroup validates an invitation and locks its group until commit.
	InvitedGroup(ctx context.Context, token string, now time.Time) (*Group, error)
	// Join accepts a group invitation and grants membership.
	Join(ctx context.Context, userID, token string, now time.Time) (string, error)
	// Manage applies an owner or member action on the group; the game alias
	// action belongs to the game context and is handled by the caller.
	Manage(ctx context.Context, groupID, userID, action, target, value string) error
}

var _ IService = (*service)(nil)

type service struct {
	groups  IGroupRepository
	players IPlayerRepository
	claims  IClaimRepository
	invites IInviteRepository
}

// NewService builds the group service from its repositories.
func NewService(
	groups IGroupRepository,
	players IPlayerRepository,
	claims IClaimRepository,
	invites IInviteRepository,
) IService {
	return &service{groups: groups, players: players, claims: claims, invites: invites}
}

// ValidName reports whether a user-provided name fits the column limit.
func ValidName(
	v string,
) bool {
	return strings.TrimSpace(v) != "" && utf8.RuneCountInString(v) <= 255
}

// List returns the groups the account belongs to.
func (s *service) List(ctx context.Context, userID string) ([]*Group, error) {
	return s.groups.ListByUser(ctx, userID)
}

// Create creates a group owned by the account, which becomes its first member.
func (s *service) Create(ctx context.Context, ownerID, name string) (*Group, error) {
	name = strings.TrimSpace(name)
	if !ValidName(name) {
		return nil, errcode.ErrGroupNameInvalid
	}
	g := &Group{ID: secure.NewID(), Name: name, Owner: ownerID}
	if err := s.groups.Create(ctx, g, ownerID); err != nil {
		return nil, err
	}
	return g, nil
}

// RequireMember returns the group snapshot and fails unless the account is a
// current member.
func (s *service) RequireMember(ctx context.Context, groupID, userID string) (*Snapshot, error) {
	v, err := s.groups.Snapshot(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !v.IsMember(userID) {
		return nil, errcode.ErrForbidden
	}
	return v, nil
}

// AddPlayer creates a nickname profile, rejecting a duplicate name.
func (s *service) AddPlayer(ctx context.Context, groupID, userID, name string) (*Player, error) {
	v, err := s.RequireMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	return s.addPlayer(ctx, groupID, v, name, nil)
}

// AddOwnerPlayer creates the group owner's own profile and links it immediately.
func (s *service) AddOwnerPlayer(
	ctx context.Context,
	groupID, ownerID, name string,
) (*Player, error) {
	v, err := s.RequireMember(ctx, groupID, ownerID)
	if err != nil {
		return nil, err
	}
	if !v.IsOwner(ownerID) {
		return nil, errcode.ErrForbidden
	}
	if playerLinkedTo(v, ownerID) {
		return nil, errcode.ErrClaimSelf
	}
	return s.addPlayer(ctx, groupID, v, name, &ownerID)
}

func (s *service) addPlayer(
	ctx context.Context,
	groupID string,
	v *Snapshot,
	name string,
	account *string,
) (*Player, error) {
	name = strings.TrimSpace(name)
	if !ValidName(name) {
		return nil, errcode.ErrPlayerName
	}
	for _, old := range v.Players {
		if strings.EqualFold(old.Name, name) {
			return nil, errcode.ErrPlayerDuplicate
		}
	}
	p := &Player{ID: secure.NewID(), Name: name, Account: account}
	if err := s.players.Save(ctx, groupID, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Invite creates a seven-day group invitation and returns the plaintext token
// exactly once.
func (s *service) Invite(
	ctx context.Context,
	groupID, userID string,
	now time.Time,
) (*Invite, string, error) {
	v, err := s.RequireMember(ctx, groupID, userID)
	if err != nil {
		return nil, "", err
	}
	if !v.IsOwner(userID) {
		return nil, "", errcode.ErrForbidden
	}
	inv := &Invite{ID: secure.NewID(), Expires: now.Add(InviteTTL)}
	token := secure.NewID()
	if err = s.invites.Create(ctx, groupID, inv, secure.Hash(token)); err != nil {
		return nil, "", err
	}
	return inv, token, nil
}

// Invites lists the group's invitations for the owner.
func (s *service) Invites(ctx context.Context, groupID, userID string) ([]*Invite, error) {
	v, err := s.RequireMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	if !v.IsOwner(userID) {
		return nil, errcode.ErrForbidden
	}
	return s.invites.ListByGroup(ctx, groupID)
}

// InvitedGroup validates the invitation under the group's membership lock.
func (s *service) InvitedGroup(ctx context.Context, token string, now time.Time) (*Group, error) {
	hash := secure.Hash(token)
	groupID, err := s.invites.Resolve(ctx, hash, now)
	if err != nil {
		return nil, err
	}
	g, err := s.groups.GetByID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	// The invitation is re-read so a concurrent revoke still blocks the join.
	if _, err = s.invites.Resolve(ctx, hash, now); err != nil {
		return nil, err
	}
	return g, nil
}

// Join accepts a group invitation and grants membership idempotently.
func (s *service) Join(ctx context.Context, userID, token string, now time.Time) (string, error) {
	g, err := s.InvitedGroup(ctx, token, now)
	if err != nil {
		return "", err
	}
	if err = s.groups.AddMember(ctx, g.ID, userID); err != nil {
		return "", err
	}
	return g.ID, nil
}

// Manage applies an owner or member action on the group; the game alias action
// belongs to the game context and is handled by the caller.
func (s *service) Manage(ctx context.Context, groupID, userID, action, target, value string) error {
	v, err := s.RequireMember(ctx, groupID, userID)
	if err != nil {
		return err
	}
	owner := v.IsOwner(userID)
	switch action {
	case "rename":
		if !owner {
			return errcode.ErrForbidden
		}
		if !ValidName(value) {
			return errcode.ErrGroupName
		}
		v.Group.Name = strings.TrimSpace(value)
		return s.groups.Save(ctx, v.Group)
	case "revoke":
		if !owner {
			return errcode.ErrForbidden
		}
		return s.invites.Revoke(ctx, groupID, target)
	case "remove":
		if !owner && target != userID {
			return errcode.ErrForbidden
		}
		if target == v.Group.Owner {
			return errcode.ErrOwnerTransfer
		}
		return s.groups.RemoveMember(ctx, groupID, target)
	case "transfer":
		if !owner {
			return errcode.ErrForbidden
		}
		if !slices.ContainsFunc(v.Members, func(m *Member) bool { return m.UserID == target }) {
			return errcode.ErrTransferTarget
		}
		v.Group.Owner = target
		return s.groups.Save(ctx, v.Group)
	case "claim":
		if playerLinkedTo(v, userID) {
			return errcode.ErrClaimSelf
		}
		if claimPendingFor(v, userID) {
			return errcode.ErrClaimPending
		}
		if !slices.ContainsFunc(
			v.Players,
			func(p *Player) bool { return p.ID == target && !p.Linked() },
		) {
			return errcode.ErrClaimLinked
		}
		return s.claims.Save(ctx, groupID, &Claim{UserID: userID, PlayerID: target})
	case "claim-new":
		if playerLinkedTo(v, userID) {
			return errcode.ErrClaimSelf
		}
		if claimPendingFor(v, userID) {
			return errcode.ErrClaimPending
		}
		player, e := s.addPlayer(ctx, groupID, v, value, nil)
		if e != nil {
			return e
		}
		return s.claims.Save(ctx, groupID, &Claim{UserID: userID, PlayerID: player.ID})
	case "approve":
		if !owner {
			return errcode.ErrForbidden
		}
		index := slices.IndexFunc(v.Claims, func(c *Claim) bool { return c.UserID == target })
		if index < 0 {
			return errcode.ErrNotFound
		}
		if !slices.ContainsFunc(v.Members, func(m *Member) bool { return m.UserID == target }) {
			return errcode.ErrForbidden
		}
		if slices.ContainsFunc(
			v.Players,
			func(p *Player) bool { return p.Linked() && *p.Account == target },
		) {
			return errcode.ErrClaimAccount
		}
		playerID := v.Claims[index].PlayerID
		for _, p := range v.Players {
			if p.ID != playerID {
				continue
			}
			if p.Linked() {
				return errcode.ErrClaimPlayer
			}
			account := target
			p.Account = &account
			if err = s.players.Save(ctx, groupID, p); err != nil {
				return err
			}
			return s.claims.Delete(ctx, groupID, target)
		}
		return errcode.ErrNotFound
	case "reject":
		if !owner {
			return errcode.ErrForbidden
		}
		return s.claims.Delete(ctx, groupID, target)
	default:
		return errcode.ErrActionUnsupported
	}
}

func playerLinkedTo(v *Snapshot, userID string) bool {
	return slices.ContainsFunc(
		v.Players,
		func(p *Player) bool { return p.Linked() && *p.Account == userID },
	)
}

func claimPendingFor(v *Snapshot, userID string) bool {
	return slices.ContainsFunc(v.Claims, func(c *Claim) bool { return c.UserID == userID })
}
