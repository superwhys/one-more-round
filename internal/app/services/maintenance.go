package services

import (
	"context"
	"errors"
	"time"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// PhotoRetention is how long an upload may stay unassociated before the cleanup
// removes it.
const PhotoRetention = 7 * 24 * time.Hour

// PhotoCleanupBatch caps the uploads one cleanup pass inspects.
const PhotoCleanupBatch = 1000

// PhotoCutoff returns the retention boundary: uploads created before it and
// still unassociated are removed by the cleanup.
func PhotoCutoff(now time.Time) time.Time { return now.Add(-PhotoRetention) }

// CleanPhotos claims expired uploads under the same group lock as round saving.
// Objects are deleted outside the transaction, with metadata retained for retry.
func CleanPhotos(ctx context.Context, repos ports.Repositories, files ports.PhotoFiles, cutoff time.Time) error {
	items, err := repos.Photo().ListCleanup(ctx, cutoff, PhotoCleanupBatch)
	if err != nil {
		return err
	}
	var result error
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return errors.Join(result, err)
		}
		claimed := false
		err = repos.WithTransaction(ctx, func(tx ports.Repositories) error {
			if _, e := tx.Group().GetByID(ctx, item.GroupID); e != nil {
				return e
			}
			p, e := tx.Photo().Get(ctx, item.GroupID, item.ID)
			if errors.Is(e, errcode.ErrNotFound) {
				return nil
			}
			if e != nil {
				return e
			}
			claimed = p.BeginDeletion(cutoff)
			if !claimed {
				return nil
			}
			return tx.Photo().Save(ctx, item.GroupID, p)
		})
		if err != nil {
			result = errors.Join(result, err)
			continue
		}
		if claimed {
			result = errors.Join(result, deletePhotoFiles(ctx, repos, files, item.GroupID, item.ID))
		}
	}
	return result
}

func deletePhotoFiles(ctx context.Context, repos ports.Repositories, files ports.PhotoFiles, groupID, id string) error {
	if err := files.Remove(ctx, id); err != nil {
		return err
	}
	return repos.Photo().DeletePending(ctx, groupID, id)
}
