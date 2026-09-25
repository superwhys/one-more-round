package game

import "context"

// IWishRepository stores the games a group wants to play.
type IWishRepository interface {
	// ListByGroup returns game IDs the group wants to play.
	ListByGroup(ctx context.Context, groupID string) ([]string, error)
	// Add marks one game as wanted and is safe to repeat.
	Add(ctx context.Context, groupID, gameID string) error
	// Remove clears one wanted game and is safe to repeat.
	Remove(ctx context.Context, groupID, gameID string) error
}
