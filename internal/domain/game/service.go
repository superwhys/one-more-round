package game

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// IGameRepository reads and writes the games of a group.
type IGameRepository interface {
	// Save inserts the game or updates its editable columns.
	Save(ctx context.Context, groupID string, g *Game) error
	// ListByGroup returns the group's games ordered by local name.
	ListByGroup(ctx context.Context, groupID string) ([]*Game, error)
}

// IService defines the game catalogue use cases of one group.
type IService interface {
	// Resolve returns the existing game with that local name, or creates it.
	Resolve(ctx context.Context, groupID, name string) (*Game, error)
	// Rename sets a new local name for a game of the group.
	Rename(ctx context.Context, groupID, target, name string) (*Game, error)
}

var _ IService = (*service)(nil)

type service struct {
	games IGameRepository
}

// NewService builds the game service from its repository.
func NewService(games IGameRepository) IService { return &service{games: games} }

// validName reports whether a user-provided name fits the column limit.
func validName(v string) bool { return strings.TrimSpace(v) != "" && utf8.RuneCountInString(v) <= 255 }

// Resolve returns the existing game with that local name, or creates it.
func (s *service) Resolve(ctx context.Context, groupID, name string) (*Game, error) {
	name = strings.TrimSpace(name)
	if !validName(name) {
		return nil, errcode.ErrGameName
	}
	games, err := s.games.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, old := range games {
		if old.SameName(name) {
			return old, nil
		}
	}
	g := &Game{ID: secure.NewID(), Name: name}
	if err = s.games.Save(ctx, groupID, g); err != nil {
		return nil, err
	}
	return g, nil
}

// Rename sets a new local name for a game of the group.
func (s *service) Rename(ctx context.Context, groupID, target, name string) (*Game, error) {
	if !validName(name) {
		return nil, errcode.ErrGameAlias
	}
	games, err := s.games.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, g := range games {
		if g.ID == target {
			g.Name = strings.TrimSpace(name)
			if err = s.games.Save(ctx, groupID, g); err != nil {
				return nil, err
			}
			return g, nil
		}
	}
	return nil, errcode.ErrNotFound
}
