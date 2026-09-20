package mysql

import (
	"context"
	"time"

	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type roundRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// ListByGroup returns the group's rounds, newest first.
func (r *roundRepository) ListByGroup(ctx context.Context, groupID string) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.DeletedAt.IsNull()).Order(q.Played.Desc(), q.ID.Desc()).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*diary.Round, 0, len(rows))
	for _, m := range rows {
		round, err := r.converter.RoundModelToDomain(m)
		if err != nil {
			return nil, err
		}
		items = append(items, round)
	}
	return items, nil
}

// ListDeletedByGroup returns rounds still inside the recovery window.
func (r *roundRepository) ListDeletedByGroup(ctx context.Context, groupID string, after time.Time) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(q.GroupID.Eq(groupID), q.DeletedAt.Gte(after)).Order(q.DeletedAt.Desc()).Find()
	return r.decode(rows, err)
}

// ListDeletedBefore returns a bounded cleanup batch across groups.
func (r *roundRepository) ListDeletedBefore(ctx context.Context, before time.Time, limit int) ([]*diary.Round, error) {
	q := queryOf(r.db).Round
	rows, err := q.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).Where(q.DeletedAt.Lt(before)).Order(q.DeletedAt).Limit(limit).Find()
	return r.decode(rows, err)
}

func (r *roundRepository) decode(rows []*models.Round, err error) ([]*diary.Round, error) {
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*diary.Round, 0, len(rows))
	for _, m := range rows {
		round, e := r.converter.RoundModelToDomain(m)
		if e != nil {
			return nil, e
		}
		items = append(items, round)
	}
	return items, nil
}

// Save inserts the round or overwrites its stored document.
func (r *roundRepository) Save(ctx context.Context, groupID string, round *diary.Round) error {
	row, err := r.converter.RoundDomainToModel(round)
	if err != nil {
		return err
	}
	row.GroupID = groupID
	q := queryOf(r.db).Round
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
		string(q.GameID.ColumnName()), string(q.Played.ColumnName()), string(q.Version.ColumnName()), string(q.Body.ColumnName()), string(q.DeletedAt.ColumnName()),
	})}).Create(row))
}

// Delete removes the round of the group.
func (r *roundRepository) Delete(ctx context.Context, groupID, id string) error {
	q := queryOf(r.db).Round
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id)).Delete()
	return mapErr(err)
}

type idempotencyRepository struct {
	db *gorm.DB
}

// Get returns the stored fingerprint and round of a submission key.
func (r *idempotencyRepository) Get(ctx context.Context, groupID, userID, key string) (string, string, error) {
	q := queryOf(r.db).Idempotency
	m, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.UserID.Eq(userID), q.RequestKey.Eq(key)).Take()
	if err != nil {
		return "", "", mapErr(err)
	}
	return m.Hash, m.RoundID, nil
}

// Create stores the fingerprint of a submission key.
func (r *idempotencyRepository) Create(ctx context.Context, groupID, userID, key, hash, roundID string) error {
	return mapErr(queryOf(r.db).Idempotency.WithContext(ctx).Create(&models.Idempotency{GroupID: groupID, UserID: userID, RequestKey: key, Hash: hash, RoundID: roundID}))
}
