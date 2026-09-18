package services

import (
	"context"
	"time"

	"github.com/superwhys/one-more-round/internal/app/ports"
)

// PhotoRetention is how long an upload may stay unassociated before the cleanup
// removes it.
const PhotoRetention = 7 * 24 * time.Hour

// PhotoCleanupBatch caps the uploads one cleanup pass inspects.
const PhotoCleanupBatch = 1000

// PhotoCutoff returns the retention boundary: uploads created before it and
// still unassociated are removed by the cleanup.
func PhotoCutoff(now time.Time) time.Time { return now.Add(-PhotoRetention) }

// CleanPhotos removes the metadata of uploads created before the cutoff that
// stayed unassociated. Each deletion runs under the group lock that round saving
// also takes, so an upload being attached is never removed.
func CleanPhotos(ctx context.Context, repos ports.Repositories, files ports.PhotoFiles, cutoff time.Time) error {
	items, err := repos.Photo().ListUnattached(ctx, cutoff, PhotoCleanupBatch)
	if err != nil {
		return err
	}
	for _, item := range items {
		deleted := false
		err = repos.WithTransaction(ctx, func(tx ports.Repositories) error {
			if _, e := tx.Group().GetByID(ctx, item.GroupID); e != nil {
				return e
			}
			var e error
			deleted, e = tx.Photo().DeleteUnattached(ctx, item.GroupID, item.ID, cutoff)
			return e
		})
		if err != nil {
			return err
		}
		if deleted {
			files.Remove(item.ID)
		}
	}
	return nil
}
