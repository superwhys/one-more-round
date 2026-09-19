package mysql

import (
	"context"
	"errors"
	"time"

	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type groupRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// ListByUser returns the groups the account belongs to.
func (r *groupRepository) ListByUser(ctx context.Context, userID string) ([]*group.Group, error) {
	q := queryOf(r.db)
	g, member := q.Group, q.Member
	rows, err := g.WithContext(ctx).Select(g.ALL).Join(member, member.GroupID.EqCol(g.ID)).Where(member.UserID.Eq(userID)).Order(g.ID).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*group.Group, 0, len(rows))
	for _, m := range rows {
		items = append(items, r.converter.GroupModelToDomain(m))
	}
	return items, nil
}

// GetByID returns the group under a write lock.
func (r *groupRepository) GetByID(ctx context.Context, id string) (*group.Group, error) {
	q := queryOf(r.db).Group
	m, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Clauses(clause.Locking{Strength: "UPDATE"}).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return r.converter.GroupModelToDomain(m), nil
}

// Create stores the group and its first member.
func (r *groupRepository) Create(ctx context.Context, g *group.Group, ownerID string) error {
	if err := queryOf(r.db).Group.WithContext(ctx).Create(r.converter.GroupDomainToModel(g)); err != nil {
		return mapErr(err)
	}
	return r.AddMember(ctx, g.ID, ownerID)
}

// Save updates the group name and owner.
func (r *groupRepository) Save(ctx context.Context, g *group.Group) error {
	q := queryOf(r.db).Group
	_, err := q.WithContext(ctx).Where(q.ID.Eq(g.ID)).UpdateSimple(q.Name.Value(g.Name), q.Owner.Value(g.Owner))
	return mapErr(err)
}

// Snapshot reads the group with its members, players, games and claims; the
// group row is locked so member changes cannot interleave with the read.
func (r *groupRepository) Snapshot(ctx context.Context, id string) (*group.Snapshot, error) {
	s := &group.Snapshot{Members: []*group.Member{}, Players: []*group.Player{}, Games: []*game.Game{}, Claims: []*group.Claim{}}
	g, err := r.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.Group = g
	q := queryOf(r.db)
	users, member := q.User, q.Member
	rows, err := users.WithContext(ctx).Select(users.ALL).Join(member, member.UserID.EqCol(users.ID)).Where(member.GroupID.Eq(id)).Order(users.ID).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	for _, m := range rows {
		s.Members = append(s.Members, r.converter.MemberUserModelToDomain(m))
	}
	players, err := q.Player.WithContext(ctx).Where(q.Player.GroupID.Eq(id)).Order(q.Player.Name).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	for _, m := range players {
		s.Players = append(s.Players, r.converter.PlayerModelToDomain(m))
	}
	games, err := q.Game.WithContext(ctx).Where(q.Game.GroupID.Eq(id)).Order(q.Game.Name).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	for _, m := range games {
		s.Games = append(s.Games, r.converter.GameModelToDomain(m))
	}
	claims, err := q.Claim.WithContext(ctx).Where(q.Claim.GroupID.Eq(id)).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	for _, m := range claims {
		s.Claims = append(s.Claims, r.converter.ClaimModelToDomain(m))
	}
	return s, nil
}

// AddMember adds the account to the group, ignoring an existing membership.
func (r *groupRepository) AddMember(ctx context.Context, groupID, userID string) error {
	return mapErr(queryOf(r.db).Member.WithContext(ctx).Clauses(clause.Insert{Modifier: "IGNORE"}).Create(&models.Member{GroupID: groupID, UserID: userID}))
}

// RemoveMember removes the membership together with the account's claim.
func (r *groupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	if err := (&claimRepository{db: r.db, converter: r.converter}).Delete(ctx, groupID, userID); err != nil {
		return err
	}
	q := queryOf(r.db).Member
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.UserID.Eq(userID)).Delete()
	return mapErr(err)
}

type playerRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// Save inserts the player, or updates the account link of an existing profile.
func (r *playerRepository) Save(ctx context.Context, groupID string, p *group.Player) error {
	q := queryOf(r.db).Player
	count, err := q.WithContext(ctx).Where(q.ID.Eq(p.ID), q.GroupID.Eq(groupID)).Count()
	if err != nil {
		return mapErr(err)
	}
	if count > 0 {
		// Update accepts a nil pointer and writes NULL when an account is unlinked.
		_, err = q.WithContext(ctx).Where(q.ID.Eq(p.ID), q.GroupID.Eq(groupID)).Update(q.Account, p.Account)
		return mapErr(err)
	}
	return mapErr(q.WithContext(ctx).Create(r.converter.PlayerDomainToModel(groupID, p)))
}

type claimRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// Save stores the account's claim, replacing an earlier one.
func (r *claimRepository) Save(ctx context.Context, groupID string, c *group.Claim) error {
	q := queryOf(r.db).Claim
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
		string(q.PlayerID.ColumnName()),
	})}).Create(r.converter.ClaimDomainToModel(groupID, c)))
}

// Delete removes the account's pending claim.
func (r *claimRepository) Delete(ctx context.Context, groupID, userID string) error {
	q := queryOf(r.db).Claim
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.UserID.Eq(userID)).Delete()
	return mapErr(err)
}

type inviteRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// Create stores an invitation holding the token digest.
func (r *inviteRepository) Create(ctx context.Context, groupID string, inv *group.Invite, hash string) error {
	return mapErr(queryOf(r.db).Invite.WithContext(ctx).Create(r.converter.InviteDomainToModel(groupID, hash, inv)))
}

// ListByGroup returns the group's invitations, newest expiry first.
func (r *inviteRepository) ListByGroup(ctx context.Context, groupID string) ([]*group.Invite, error) {
	q := queryOf(r.db).Invite
	rows, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID)).Order(q.Expires.Desc()).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*group.Invite, 0, len(rows))
	for _, m := range rows {
		items = append(items, r.converter.InviteModelToDomain(m))
	}
	return items, nil
}

// Revoke disables an unused invitation.
func (r *inviteRepository) Revoke(ctx context.Context, groupID, id string) error {
	q := queryOf(r.db).Invite
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id)).UpdateSimple(q.Revoked.Value(true))
	return mapErr(err)
}

// Resolve returns the group of a live, unrevoked invitation digest.
func (r *inviteRepository) Resolve(ctx context.Context, hash string, now time.Time) (string, error) {
	q := queryOf(r.db).Invite
	m, err := q.WithContext(ctx).Where(q.Hash.Eq(hash)).Take()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", errcode.ErrInviteInvalid
	}
	if err != nil {
		return "", mapErr(err)
	}
	if m.Revoked {
		return "", errcode.ErrInviteRevoked
	}
	if !m.Expires.After(now) {
		return "", errcode.ErrInviteExpired
	}
	return m.GroupID, nil
}
