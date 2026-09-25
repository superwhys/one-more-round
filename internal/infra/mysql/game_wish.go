package mysql

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// gameWishRepository persists the group's shared want-to-play set.
type gameWishRepository struct {
	db *gorm.DB
}

// ListByGroup returns the game IDs marked as wanted by the group.
func (r *gameWishRepository) ListByGroup(ctx context.Context, groupID string) ([]string, error) {
	q := queryOf(r.db).GameWish
	rows, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID)).Order(q.GameID).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.GameID)
	}
	return ids, nil
}

// Add marks a game as wanted and is safe to repeat.
func (r *gameWishRepository) Add(ctx context.Context, groupID, gameID string) error {
	q := queryOf(r.db).GameWish
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(
		&models.GameWish{GroupID: groupID, GameID: gameID},
	))
}

// Remove clears a wanted game and is safe to repeat.
func (r *gameWishRepository) Remove(ctx context.Context, groupID, gameID string) error {
	q := queryOf(r.db).GameWish
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.GameID.Eq(gameID)).Delete()
	return mapErr(err)
}
