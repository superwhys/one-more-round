package mysql_test

import (
	"context"
	"errors"
	"testing"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// TestGroupWishlist verifies membership, group scoping and idempotent updates.
func TestGroupWishlist(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "wish-owner@example.com")
	outsider, _ := s.signup(t, "wish-outsider@example.com")
	first, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "想玩小组", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "其他小组", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, first.ID, owner.ID, &dto.AddGameReq{Name: "待玩游戏"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.groups.AddGame(ctx, second.ID, owner.ID, &dto.AddGameReq{Name: "别组游戏"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.groups.SetGameWanted(ctx, first.ID, outsider.ID, game.ID, true); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("outsider update: %v", err)
	}
	if _, err = s.groups.ListWishlist(ctx, first.ID, outsider.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("outsider list: %v", err)
	}
	if err = s.groups.SetGameWanted(ctx, first.ID, owner.ID, other.ID, true); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("cross-group game: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err = s.groups.SetGameWanted(ctx, first.ID, owner.ID, game.ID, true); err != nil {
			t.Fatalf("add wish %d: %v", i, err)
		}
	}
	wishlist, err := s.groups.ListWishlist(ctx, first.ID, owner.ID)
	if err != nil || len(wishlist.GameIDs) != 1 || wishlist.GameIDs[0] != game.ID {
		t.Fatalf("wishlist = %#v, %v", wishlist, err)
	}
	for i := 0; i < 2; i++ {
		if err = s.groups.SetGameWanted(ctx, first.ID, owner.ID, game.ID, false); err != nil {
			t.Fatalf("remove wish %d: %v", i, err)
		}
	}
	wishlist, err = s.groups.ListWishlist(ctx, first.ID, owner.ID)
	if err != nil || len(wishlist.GameIDs) != 0 {
		t.Fatalf("cleared wishlist = %#v, %v", wishlist, err)
	}
}
