package mysql

import (
	"context"

	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type gameRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

// Save inserts the game or updates its editable columns.
func (r *gameRepository) Save(ctx context.Context, groupID string, g *game.Game) error {
	q := queryOf(r.db).Game
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{
		string(q.Name.ColumnName()), string(q.Original.ColumnName()), string(q.BGGID.ColumnName()),
	})}).Create(r.converter.GameDomainToModel(groupID, g)))
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
		items = append(items, r.converter.GameModelToDomain(m))
	}
	return items, nil
}
