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
	// ListUnattached returns at most limit unbound photos created before the
	// given time.
	ListUnattached(ctx context.Context, before time.Time, limit int) ([]*Photo, error)
	// DeleteUnattached removes an unbound photo created before the given time
	// and reports whether a row was deleted.
	DeleteUnattached(ctx context.Context, groupID, id string, before time.Time) (bool, error)
}
