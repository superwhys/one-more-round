package game

import (
	"context"
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

func TestImportReusesExternalID(t *testing.T) {
	repo := &memGames{}
	svc := NewService(repo)
	first, err := svc.Import(context.Background(), "g", 13, "卡坦岛", "Catan", "https://cf.geekdo-images.com/catan.jpg")
	if err != nil || first.Name != "卡坦岛" || first.Original != "Catan" || first.BGGID == nil || *first.BGGID != 13 {
		t.Fatal(first, err)
	}
	again, err := svc.Import(context.Background(), "g", 13, "另一个名字", "Catan", "")
	if err != nil || again.ID != first.ID || again.Name != "卡坦岛" {
		t.Fatal(again, err)
	}
	if _, err = svc.Import(context.Background(), "g", 14, "卡坦岛", "Catan: Traveler", ""); !errcode.ErrConflict.Is(err) {
		t.Fatal(err)
	}
}
