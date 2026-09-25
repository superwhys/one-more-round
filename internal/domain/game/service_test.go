package game

import (
	"context"
	"errors"
	"testing"

	"github.com/superwhys/one-more-round/internal/errcode"
)

type memGames struct{ items []*Game }

func (m *memGames) Save(_ context.Context, _ string, g *Game) error {
	for i, old := range m.items {
		if old.ID == g.ID {
			m.items[i] = g
			return nil
		}
	}
	m.items = append(m.items, g)
	return nil
}

func (m *memGames) ListByGroup(context.Context, string) ([]*Game, error) {
	return m.items, nil
}

// Delete removes one stored game from the test repository.
func (m *memGames) Delete(_ context.Context, _ string, id string) error {
	for i, game := range m.items {
		if game.ID == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}
	return errcode.ErrNotFound
}

func TestImportReusesExternalID(t *testing.T) {
	repo := &memGames{}
	svc := NewService(repo)
	first, err := svc.Import(
		context.Background(),
		"g",
		13,
		"卡坦岛",
		"Catan",
		"https://cf.geekdo-images.com/catan.jpg",
	)
	if err != nil || first.Name != "卡坦岛" || first.Original != "Catan" || first.BGGID == nil ||
		*first.BGGID != 13 {
		t.Fatal(first, err)
	}
	again, err := svc.Import(context.Background(), "g", 13, "另一个名字", "Catan", "")
	if err != nil || again.ID != first.ID || again.Name != "卡坦岛" {
		t.Fatal(again, err)
	}
	if _, err = svc.Import(
		context.Background(),
		"g",
		14,
		"卡坦岛",
		"Catan: Traveler",
		"",
	); !errcode.ErrConflict.Is(
		err,
	) {
		t.Fatal(err)
	}
}

func TestAttachCoverLinksWithoutExternalCover(t *testing.T) {
	id := 13
	repo := &memGames{items: []*Game{{ID: "local", Name: "本组名字"}}}
	svc := NewService(repo)
	linked, err := svc.AttachCover(context.Background(), "g", "local", id, "Catan", "")
	if err != nil || linked.BGGID == nil || *linked.BGGID != id || linked.Name != "本组名字" ||
		linked.Original != "Catan" || linked.Cover != "" {
		t.Fatalf("linked = %+v, err = %v", linked, err)
	}
}

func TestAttachCoverKeepsExistingAssociationAndCover(t *testing.T) {
	id := 13
	repo := &memGames{items: []*Game{{
		ID: "linked", Name: "本组名字", BGGID: &id, Original: "Catan", Cover: "https://example.com/cover.jpg",
	}}}
	svc := NewService(repo)
	linked, err := svc.AttachCover(context.Background(), "g", "linked", id, "Catan", "")
	if err != nil || linked.Cover != "https://example.com/cover.jpg" {
		t.Fatalf("linked = %+v, err = %v", linked, err)
	}
	if _, err = svc.AttachCover(context.Background(), "g", "linked", 14, "Other", ""); !errcode.ErrConflict.Is(err) {
		t.Fatalf("different BGG ID should conflict: %v", err)
	}
}

func TestValidateMerge(t *testing.T) {
	bggID := 13
	source := &Game{ID: "manual", Name: "手动条目"}
	target := &Game{ID: "bgg", Name: "保留名称", BGGID: &bggID}
	if err := ValidateMerge(source, target); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		source *Game
		target *Game
		want   error
	}{
		{"missing source", nil, target, errcode.ErrNotFound},
		{"missing target", source, nil, errcode.ErrNotFound},
		{"same game", target, target, errcode.ErrBadRequest},
		{"linked source", target, source, errcode.ErrConflict},
		{"manual target", source, &Game{ID: "other"}, errcode.ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateMerge(tc.source, tc.target); !errors.Is(err, tc.want) {
				t.Fatalf("merge validation error = %v, want %v", err, tc.want)
			}
		})
	}
}
