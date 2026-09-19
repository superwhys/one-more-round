package photo

import (
	"context"
	"time"
)

// IPhotoRepository reads and writes photo metadata of a group.
type IPhotoRepository interface {
	// Save inserts the photo metadata, replacing an existing round binding.
	Save(ctx context.Context, groupID string, p *Photo) error
	// Get returns the metadata of one photo of the group.
	Get(ctx context.Context, groupID, id string) (*Photo, error)
	// ListCleanup returns expired unbound uploads and previously claimed deletions.
	ListCleanup(ctx context.Context, before time.Time, limit int) ([]*Photo, error)
	// DeletePending removes metadata only after its claimed objects were deleted.
	DeletePending(ctx context.Context, groupID, id string) error
}
