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
	// Import stores an external game, reusing the same external id in the group.
	Import(
		ctx context.Context,
		groupID string,
		bggID int,
		localName, original, cover string,
	) (*Game, error)
	// AttachCover links an existing game to an external entry and stores its cover.
	AttachCover(
		ctx context.Context,
		groupID, target string,
		bggID int,
		original, cover string,
	) (*Game, error)
}

var _ IService = (*service)(nil)

type service struct {
	games IGameRepository
}

// NewService builds the game service from its repository.
func NewService(games IGameRepository) IService { return &service{games: games} }

// validName reports whether a user-provided name fits the column limit.
func validName(
	v string,
) bool {
	return strings.TrimSpace(v) != "" && utf8.RuneCountInString(v) <= 255
}

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

// Import stores an external game. The same external id is reused; a different
// game that already uses the local name is left unchanged.
func (s *service) Import(
	ctx context.Context,
	groupID string,
	bggID int,
	localName, original, cover string,
) (*Game, error) {
	original = strings.TrimSpace(original)
	localName = strings.TrimSpace(localName)
	cover = strings.TrimSpace(cover)
	if localName == "" {
		localName = original
	}
	if bggID <= 0 || !validName(original) || !validName(localName) || !validCover(cover) {
		return nil, errcode.ErrGameName
	}
	games, err := s.games.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, old := range games {
		if old.BGGID != nil && *old.BGGID == bggID {
			if old.Cover == "" && cover != "" {
				old.Cover = cover
				if err = s.games.Save(ctx, groupID, old); err != nil {
					return nil, err
				}
			}
			return old, nil
		}
	}
	for _, old := range games {
		if old.SameName(localName) {
			return nil, errcode.ErrConflict.WithMessage("架上已有同名桌游，请换一个本组名称")
		}
	}
	id := bggID
	g := &Game{ID: secure.NewID(), Name: localName, Original: original, BGGID: &id, Cover: cover}
	if err = s.games.Save(ctx, groupID, g); err != nil {
		return nil, err
	}
	return g, nil
}

// AttachCover stores an external cover on a game already on the shelf. The
// local name stays as the group wrote it.
func (s *service) AttachCover(
	ctx context.Context,
	groupID, target string,
	bggID int,
	original, cover string,
) (*Game, error) {
	original = strings.TrimSpace(original)
	cover = strings.TrimSpace(cover)
	if bggID <= 0 || !validName(original) || !validCover(cover) || cover == "" {
		return nil, errcode.ErrBGGSearch.WithMessage("这款桌游没有可用封面")
	}
	games, err := s.games.ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	var current *Game
	for _, old := range games {
		if old.BGGID != nil && *old.BGGID == bggID && old.ID != target {
			return nil, errcode.ErrConflict.WithMessage("架上已有这款桌游")
		}
		if old.ID == target {
			current = old
		}
	}
	if current == nil {
		return nil, errcode.ErrNotFound
	}
	id := bggID
	current.BGGID = &id
	current.Cover = cover
	if current.Original == "" {
		current.Original = original
	}
	if err = s.games.Save(ctx, groupID, current); err != nil {
		return nil, err
	}
	return current, nil
}

// validCover accepts an empty cover or one https image address.
func validCover(v string) bool {
	return v == "" || (strings.HasPrefix(v, "https://") && utf8.RuneCountInString(v) <= 512)
}
