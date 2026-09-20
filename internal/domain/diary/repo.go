package diary

import (
	"context"
	"time"
)

// IRoundRepository reads and writes the rounds of a group.
type IRoundRepository interface {
	// ListByGroup returns the group's rounds, newest first.
	ListByGroup(ctx context.Context, groupID string) ([]*Round, error)
	// ListDeletedByGroup returns recoverable rounds, newest deletion first.
	ListDeletedByGroup(ctx context.Context, groupID string, after time.Time) ([]*Round, error)
	// ListDeletedBefore returns rounds ready for permanent cleanup.
	ListDeletedBefore(ctx context.Context, before time.Time, limit int) ([]*Round, error)
	// Save inserts the round or overwrites its stored document.
	Save(ctx context.Context, groupID string, r *Round) error
	// Delete permanently removes an expired round of the group.
	Delete(ctx context.Context, groupID, id string) error
}

// IIdempotencyRepository stores the fingerprint of a round submission so a
// retried request reuses the record it already created.
type IIdempotencyRepository interface {
	// Get returns the stored fingerprint and round of a submission key.
	Get(ctx context.Context, groupID, userID, key string) (hash, roundID string, err error)
	// Create stores the fingerprint of a submission key.
	Create(ctx context.Context, groupID, userID, key, hash, roundID string) error
}
