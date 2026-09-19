package mysql

import (
	"context"
	"time"

	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/photo"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type photoRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// Save inserts the photo metadata, replacing its round binding.
func (r *photoRepository) Save(ctx context.Context, groupID string, p *photo.Photo) error {
	q := queryOf(r.db).Photo
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
		string(q.RoundID.ColumnName()), string(q.Created.ColumnName()), string(q.State.ColumnName()),
	})}).Create(r.converter.PhotoDomainToModel(groupID, p)))
}

// Get returns the metadata of one photo of the group.
func (r *photoRepository) Get(ctx context.Context, groupID, id string) (*photo.Photo, error) {
	q := queryOf(r.db).Photo
	m, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id)).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return r.converter.PhotoModelToDomain(m), nil
}

// ListCleanup includes failed deletions even when their retention cutoff changed.
func (r *photoRepository) ListCleanup(ctx context.Context, before time.Time, limit int) ([]*photo.Photo, error) {
	q := queryOf(r.db).Photo
	rows, err := q.WithContext(ctx).Where(q.RoundID.Eq("")).
		Where(q.WithContext(ctx).Where(q.Created.Lt(before)).Or(q.State.Eq(string(photo.StateDeleting)))).
		Order(q.Created, q.ID).Limit(limit).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*photo.Photo, 0, len(rows))
	for _, m := range rows {
		items = append(items, r.converter.PhotoModelToDomain(m))
	}
	return items, nil
}

// DeletePending is idempotent, including concurrent cleanup workers.
func (r *photoRepository) DeletePending(ctx context.Context, groupID, id string) error {
	q := queryOf(r.db).Photo
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id), q.RoundID.Eq(""), q.State.Eq(string(photo.StateDeleting))).Delete()
	return mapErr(err)
}
