package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql/mapper"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

type roundRepository struct {
	db *gorm.DB
}

// ListByGroup returns the group's rounds, newest first.
func (r *roundRepository) ListByGroup(ctx context.Context, groupID string) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).
		Where(q.GroupID.Eq(groupID), q.DeletedAt.IsNull()).
		Order(q.Played.Desc(), q.ID.Desc()).
		Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*diary.Round, 0, len(rows))
	for _, m := range rows {
		round, err := mapper.RoundModelToDomain(m)
		if err != nil {
			return nil, err
		}
		items = append(items, round)
	}
	return items, nil
}

// ListDeletedByGroup returns rounds still inside the recovery window.
func (r *roundRepository) ListDeletedByGroup(
	ctx context.Context,
	groupID string,
	after time.Time,
) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(q.GroupID.Eq(groupID), q.DeletedAt.Gte(after)).
		Order(q.DeletedAt.Desc()).
		Find()
	return r.decode(rows, err)
}

// ListDeletedBefore returns a bounded cleanup batch across groups.
func (r *roundRepository) ListDeletedBefore(
	ctx context.Context,
	before time.Time,
	limit int,
) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(q.DeletedAt.Lt(before)).
		Order(q.DeletedAt).
		Limit(limit).
		Find()
	return r.decode(rows, err)
}

func (r *roundRepository) decode(rows []*models.Round, err error) ([]*diary.Round, error) {
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*diary.Round, 0, len(rows))
	for _, m := range rows {
		round, e := mapper.RoundModelToDomain(m)
		if e != nil {
			return nil, e
		}
		items = append(items, round)
	}
	return items, nil
}

// Save inserts the round or overwrites its stored document.
func (r *roundRepository) Save(ctx context.Context, groupID string, round *diary.Round) error {
	row, err := mapper.RoundDomainToModel(round)
	if err != nil {
		return err
	}
	row.GroupID = groupID
	q := queryOf(r.db).Round
	return mapErr(
		q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
			string(
				q.GameID.ColumnName(),
			),
			string(q.Played.ColumnName()),
			string(q.Version.ColumnName()),
			string(q.Body.ColumnName()),
			string(q.DeletedAt.ColumnName()),
		})}).Create(row),
	)
}

// ReassignGame moves every round of the source game, including recycle-bin
// rows. The indexed column and JSON document must agree for reads and stats.
func (r *roundRepository) ReassignGame(
	ctx context.Context,
	groupID, sourceGameID, targetGameID string,
) error {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(q.GroupID.Eq(groupID), q.GameID.Eq(sourceGameID)).
		Find()
	if err != nil {
		return mapErr(err)
	}
	for _, row := range rows {
		stored, err := mapper.RoundModelToDomain(row)
		if err != nil {
			return err
		}
		if stored.GameID != sourceGameID {
			return errcode.ErrConflict.WithMessage("对局游戏资料不一致，暂不能合并")
		}
		stored.GameID = targetGameID
		updated, err := mapper.RoundDomainToModel(stored)
		if err != nil {
			return err
		}
		result, err := q.WithContext(ctx).
			Where(q.GroupID.Eq(groupID), q.ID.Eq(row.ID), q.GameID.Eq(sourceGameID)).
			UpdateSimple(q.GameID.Value(targetGameID), q.Body.Value(updated.Body))
		if err != nil {
			return mapErr(err)
		}
		if result.RowsAffected != 1 {
			return errcode.ErrConflict.WithMessage("对局已变化，请刷新后再合并")
		}
	}
	return nil
}

// Delete removes the round of the group.
func (r *roundRepository) Delete(ctx context.Context, groupID, id string) error {
	queries := queryOf(r.db)
	if _, err := queries.RoundShare.WithContext(ctx).
		Where(queries.RoundShare.GroupID.Eq(groupID), queries.RoundShare.RoundID.Eq(id)).
		Delete(); err != nil {
		return mapErr(err)
	}
	if err := (&commentRepository{db: r.db}).DeleteByRound(ctx, groupID, id); err != nil {
		return err
	}
	q := queries.Round
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id)).Delete()
	return mapErr(err)
}

// SaveShare creates or rotates the single public link of a round.
func (r *roundRepository) SaveShare(ctx context.Context, share *diary.Share) error {
	q := queryOf(r.db).RoundShare
	row := &models.RoundShare{
		RoundID:   share.RoundID,
		GroupID:   share.GroupID,
		TokenHash: share.TokenHash,
		CreatedBy: share.CreatedBy,
		CreatedAt: share.CreatedAt,
		RevokedAt: share.RevokedAt,
	}
	return mapErr(
		q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
			string(
				q.GroupID.ColumnName(),
			),
			string(q.TokenHash.ColumnName()),
			string(q.CreatedBy.ColumnName()),
			string(q.CreatedAt.ColumnName()),
			string(q.RevokedAt.ColumnName()),
		})}).Create(row),
	)
}

// GetShare returns the link state of a round.
func (r *roundRepository) GetShare(
	ctx context.Context,
	groupID, roundID string,
) (*diary.Share, error) {
	q := queryOf(r.db).RoundShare
	m, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID)).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return &diary.Share{
		RoundID:   m.RoundID,
		GroupID:   m.GroupID,
		TokenHash: m.TokenHash,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
		RevokedAt: m.RevokedAt,
	}, nil
}

// ResolveShare returns the active link identified by a token digest.
func (r *roundRepository) ResolveShare(
	ctx context.Context,
	tokenHash string,
) (*diary.Share, error) {
	q := queryOf(r.db).RoundShare
	m, err := q.WithContext(ctx).Where(q.TokenHash.Eq(tokenHash), q.RevokedAt.IsNull()).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return &diary.Share{
		RoundID:   m.RoundID,
		GroupID:   m.GroupID,
		TokenHash: m.TokenHash,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
	}, nil
}

// RevokeShare disables the current public link of a round.
func (r *roundRepository) RevokeShare(
	ctx context.Context,
	groupID, roundID string,
	at time.Time,
) error {
	q := queryOf(r.db).RoundShare
	result, err := q.WithContext(ctx).
		Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID), q.RevokedAt.IsNull()).
		Update(q.RevokedAt, at)
	if err != nil {
		return mapErr(err)
	}
	if result.RowsAffected == 0 {
		return errcode.ErrNotFound
	}
	return nil
}

type idempotencyRepository struct {
	db *gorm.DB
}

// Get returns the stored fingerprint and round of a submission key.
func (r *idempotencyRepository) Get(
	ctx context.Context,
	groupID, userID, key string,
) (string, string, error) {
	q := queryOf(r.db).Idempotency
	m, err := q.WithContext(ctx).
		Where(q.GroupID.Eq(groupID), q.UserID.Eq(userID), q.RequestKey.Eq(key)).
		Take()
	if err != nil {
		return "", "", mapErr(err)
	}
	return m.Hash, m.RoundID, nil
}

// Create stores the fingerprint of a submission key.
func (r *idempotencyRepository) Create(
	ctx context.Context,
	groupID, userID, key, hash, roundID string,
) error {
	return mapErr(
		queryOf(
			r.db,
		).Idempotency.WithContext(ctx).
			Create(&models.Idempotency{GroupID: groupID, UserID: userID, RequestKey: key, Hash: hash, RoundID: roundID}),
	)
}
