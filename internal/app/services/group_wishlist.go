package services

import (
	"context"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// ListWishlist returns the group's wanted game IDs to a current member.
func (a *GroupApp) ListWishlist(ctx context.Context, groupID, userID string) (dto.Wishlist, error) {
	var ids []string
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := groupService(repos).RequireMember(ctx, groupID, userID); err != nil {
			return err
		}
		var err error
		ids, err = repos.GameWish().ListByGroup(ctx, groupID)
		return err
	}); err != nil {
		return dto.Wishlist{}, err
	}
	if ids == nil {
		ids = []string{}
	}
	return dto.Wishlist{GameIDs: ids}, nil
}

// SetGameWanted marks or clears a group game as wanted by a current member.
func (a *GroupApp) SetGameWanted(ctx context.Context, groupID, userID, gameID string, wanted bool) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		current, err := groupService(repos).RequireMember(ctx, groupID, userID)
		if err != nil {
			return err
		}
		found := false
		for _, game := range current.Games {
			if game.ID == gameID {
				found = true
				break
			}
		}
		if !found {
			return errcode.ErrNotFound
		}
		if wanted {
			return repos.GameWish().Add(ctx, groupID, gameID)
		}
		return repos.GameWish().Remove(ctx, groupID, gameID)
	})
}
