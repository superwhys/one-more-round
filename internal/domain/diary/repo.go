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
	// SaveShare creates or rotates the single public link of a round.
	SaveShare(ctx context.Context, share *Share) error
	// GetShare returns the link state of a round.
	GetShare(ctx context.Context, groupID, roundID string) (*Share, error)
	// ResolveShare returns the active link identified by a token digest.
	ResolveShare(ctx context.Context, tokenHash string) (*Share, error)
	// RevokeShare disables the current public link of a round.
	RevokeShare(ctx context.Context, groupID, roundID string, at time.Time) error
}

// ICommentRepository reads and writes the comments of a round.
type ICommentRepository interface {
	// ListByRound returns the round's comments, oldest first, with the total count.
	ListByRound(
		ctx context.Context,
		groupID, roundID string,
		offset, limit int,
	) ([]*Comment, int, error)
	// Get returns one comment of the round.
	Get(ctx context.Context, groupID, roundID, id string) (*Comment, error)
	// Save inserts a comment.
	Save(ctx context.Context, c *Comment) error
	// Delete removes one comment of the round and any replies that point at it.
	Delete(ctx context.Context, groupID, roundID, id string) error
	// DeleteByRound permanently removes every comment of the round.
	DeleteByRound(ctx context.Context, groupID, roundID string) error
}

// ICommentIdempotencyRepository stores the fingerprint of a comment submission
// so a retried request reuses the comment it already created.
type ICommentIdempotencyRepository interface {
	// Get returns the stored fingerprint and comment of a submission key.
	Get(
		ctx context.Context,
		groupID, userID, roundID, key string,
	) (hash, commentID string, err error)
	// Create stores the fingerprint of a submission key.
	Create(ctx context.Context, groupID, userID, roundID, key, hash, commentID string) error
}

// IIdempotencyRepository stores the fingerprint of a round submission so a
// retried request reuses the record it already created.
type IIdempotencyRepository interface {
	// Get returns the stored fingerprint and round of a submission key.
	Get(ctx context.Context, groupID, userID, key string) (hash, roundID string, err error)
	// Create stores the fingerprint of a submission key.
	Create(ctx context.Context, groupID, userID, key, hash, roundID string) error
}
