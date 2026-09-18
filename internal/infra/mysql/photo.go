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
		string(q.RoundID.ColumnName()),
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

// ListUnattached returns at most limit unbound photos created before the given
// time.
func (r *photoRepository) ListUnattached(ctx context.Context, before time.Time, limit int) ([]*photo.Photo, error) {
	q := queryOf(r.db).Photo
	rows, err := q.WithContext(ctx).Where(q.RoundID.Eq(""), q.Created.Lt(before)).Limit(limit).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*photo.Photo, 0, len(rows))
	for _, m := range rows {
		items = append(items, r.converter.PhotoModelToDomain(m))
	}
	return items, nil
}

// DeleteUnattached removes an unbound photo created before the given time and
// reports whether a row was deleted.
func (r *photoRepository) DeleteUnattached(ctx context.Context, groupID, id string, before time.Time) (bool, error) {
	q := queryOf(r.db).Photo
	result, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.ID.Eq(id), q.RoundID.Eq(""), q.Created.Lt(before)).Delete()
	if err != nil {
		return false, mapErr(err)
	}
	return result.RowsAffected == 1, nil
}
