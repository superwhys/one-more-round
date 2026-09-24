package worker

import (
	"context"
	"time"

	"github.com/miebyte/goutils/logging"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/infra/photos"
)

func PhotoCleanup(
	repos ports.Repositories,
	photoFiles *photos.OSS,
) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			err := services.CleanDeletedRounds(
				ctx,
				repos,
				time.Now().UTC().Add(-services.RoundRecycleRetention),
			)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				logging.Error("Deleted round cleanup failed")
			}

			err = services.CleanPhotos(
				ctx,
				repos,
				photoFiles,
				services.PhotoCutoff(time.Now().UTC()),
			)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				logging.Error("Temporary photo cleanup failed")
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
			}
		}
	}
}
