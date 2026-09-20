package services

import (
	"context"
	"errors"
	"time"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// PhotoRetention is how long an upload may stay unassociated before the cleanup
// removes it.
const PhotoRetention = 7 * 24 * time.Hour

// PhotoCleanupBatch caps the uploads one cleanup pass inspects.
const PhotoCleanupBatch = 1000

// RoundRecycleRetention is how long a deleted round may be restored.
const RoundRecycleRetention = 7 * 24 * time.Hour

const roundCleanupBatch = 200

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

// CleanDeletedRounds permanently removes expired recycle-bin rows and releases
// their photos into the existing unassociated-photo retention workflow.
func CleanDeletedRounds(ctx context.Context, repos ports.Repositories, cutoff time.Time) error {
	items, err := repos.Round().ListDeletedBefore(ctx, cutoff, roundCleanupBatch)
	if err != nil {
		return err
	}
	for _, round := range items {
		if err = repos.WithTransaction(ctx, func(tx ports.Repositories) error {
			var current *diary.Round
			expired, e := tx.Round().ListDeletedBefore(ctx, cutoff, roundCleanupBatch)
			if e != nil {
				return e
			}
			for _, candidate := range expired {
				if candidate.ID == round.ID {
					current = candidate
					break
				}
			}
			if current == nil {
				return nil
			}
			for _, id := range current.Photos {
				p, e := tx.Photo().Get(ctx, round.GroupID, id)
				if errors.Is(e, errcode.ErrNotFound) {
					continue
				}
				if e != nil {
					return e
				}
				p.Detach(time.Now().UTC())
				if e = tx.Photo().Save(ctx, round.GroupID, p); e != nil {
					return e
				}
			}
			return tx.Round().Delete(ctx, current.GroupID, current.ID)
		}); err != nil {
			return err
		}
	}
	return nil
}
