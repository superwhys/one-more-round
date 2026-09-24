package mysql

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/infra/mysql/mapper"
)

type gameRepository struct {
	db *gorm.DB
}

// Save inserts the game or updates its editable columns.
func (r *gameRepository) Save(ctx context.Context, groupID string, g *game.Game) error {
	q := queryOf(r.db).Game
	return mapErr(
		q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
			string(
				q.Name.ColumnName(),
			),
			string(q.Original.ColumnName()),
			string(q.BGGID.ColumnName()),
			string(q.Cover.ColumnName()),
		})}).Create(mapper.GameDomainToModel(groupID, g)),
	)
}

// ListByGroup returns the group's games ordered by local name.
func (r *gameRepository) ListByGroup(ctx context.Context, groupID string) ([]*game.Game, error) {
	q := queryOf(r.db).Game
	rows, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID)).Order(q.Name).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*game.Game, 0, len(rows))
	for _, m := range rows {
		items = append(items, mapper.GameModelToDomain(m))
	}
	return items, nil
}
