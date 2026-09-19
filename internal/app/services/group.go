package services

import (
	"context"
	"slices"
	"sort"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// GroupApp handles groups, their players, games, invitations and membership.
type GroupApp struct {
	repos     ports.Repositories
	converter *converter.Converter
}

// NewGroupApp builds the group application service.
func NewGroupApp(ctx *AppContext) *GroupApp {
	return &GroupApp{repos: ctx.Repos, converter: ctx.Converter}
}

// List returns the groups the account belongs to.
func (a *GroupApp) List(ctx context.Context, userID string) ([]dto.Group, error) {
	groups, err := groupService(a.repos).List(ctx, userID)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []*group.Group{}
	}
	return a.converter.GroupDomainListToDTOList(groups), nil
}

// Create creates a group and the owner's linked player profile atomically.
func (a *GroupApp) Create(ctx context.Context, userID string, req *dto.CreateGroupReq) (dto.Group, error) {
	var created *group.Group
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		created, e = groupService(repos).Create(ctx, userID, req.Name)
		if e != nil {
			return e
		}
		_, e = groupService(repos).AddOwnerPlayer(ctx, created.ID, userID, req.PlayerName)
		return e
	}); err != nil {
		return dto.Group{}, err
	}
	return a.converter.GroupDomainToDTO(created), nil
}

// Snapshot returns the group with the games and players ordered by recent
// activity, the recent locations, and the claims the account may see.
func (a *GroupApp) Snapshot(ctx context.Context, groupID, userID string) (dto.Snapshot, error) {
	var snapshot *group.Snapshot
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		current, e := groupService(repos).RequireMember(ctx, groupID, userID)
		if e != nil {
			return e
		}
		rounds, e := repos.Round().ListByGroup(ctx, groupID)
		if e != nil {
			return e
		}
		recentGames, recentPlayers := map[string]int{}, map[string]int{}
		current.Locations = []string{}
		for _, r := range rounds {
			if _, ok := recentGames[r.GameID]; !ok {
				recentGames[r.GameID] = len(recentGames) + 1
			}
			for _, id := range r.Players {
				if _, ok := recentPlayers[id]; !ok {
					recentPlayers[id] = len(recentPlayers) + 1
				}
			}
			if r.Location != "" && !slices.Contains(current.Locations, r.Location) && len(current.Locations) < 10 {
				current.Locations = append(current.Locations, r.Location)
			}
		}
		rank := func(seen map[string]int, id string) int {
			if n, ok := seen[id]; ok {
				return n
			}
			return len(seen) + 1
		}
		sort.SliceStable(current.Games, func(i, j int) bool {
			return rank(recentGames, current.Games[i].ID) < rank(recentGames, current.Games[j].ID)
		})
		sort.SliceStable(current.Players, func(i, j int) bool {
			return rank(recentPlayers, current.Players[i].ID) < rank(recentPlayers, current.Players[j].ID)
		})
		if !current.IsOwner(userID) {
			current.Claims = slices.DeleteFunc(current.Claims, func(c *group.Claim) bool { return c.UserID != userID })
		}
		snapshot = current
		return nil
	}); err != nil {
		return dto.Snapshot{}, err
	}
	return a.converter.SnapshotDomainToDTO(snapshot), nil
}

// AddPlayer creates a nickname profile of the group.
func (a *GroupApp) AddPlayer(ctx context.Context, groupID, userID string, req *dto.AddPlayerReq) (dto.Player, error) {
	var created *group.Player
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		created, e = groupService(repos).AddPlayer(ctx, groupID, userID, req.Name)
		return e
	}); err != nil {
		return dto.Player{}, err
	}
	return a.converter.PlayerDomainToDTO(created), nil
}

// AddGame adds a game to the group catalogue, reusing the entry with the same
// local name.
func (a *GroupApp) AddGame(ctx context.Context, groupID, userID string, req *dto.AddGameReq) (dto.Game, error) {
	var resolved *game.Game
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		var e error
		resolved, e = gameService(repos).Resolve(ctx, groupID, req.Name)
		return e
	}); err != nil {
		return dto.Game{}, err
	}
	return a.converter.GameDomainToDTO(resolved), nil
}

// SearchExternalGames reports that the external catalogue is unavailable; the
// manual catalogue stays usable.
func (a *GroupApp) SearchExternalGames(ctx context.Context, groupID, userID string) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		return errcode.ErrBGGUnavailable
	})
}

// Manage applies an owner or member action on the group.
func (a *GroupApp) Manage(ctx context.Context, groupID, userID string, req *dto.ManageReq) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if req.Action == "alias" {
			if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
				return e
			}
			_, e := gameService(repos).Rename(ctx, groupID, req.Target, req.Value)
			return e
		}
		return groupService(repos).Manage(ctx, groupID, userID, req.Action, req.Target, req.Value)
	})
}

// Invite creates a group invitation and returns the plaintext token once.
func (a *GroupApp) Invite(ctx context.Context, groupID, userID string) (dto.Invite, string, error) {
	var invited *group.Invite
	var token string
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		invited, token, e = groupService(repos).Invite(ctx, groupID, userID, time.Now().UTC())
		return e
	}); err != nil {
		return dto.Invite{}, "", err
	}
	return a.converter.InviteDomainToDTO(invited), token, nil
}

// Invites lists the group's invitations for the owner.
func (a *GroupApp) Invites(ctx context.Context, groupID, userID string) ([]dto.Invite, error) {
	var invites []*group.Invite
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		invites, e = groupService(repos).Invites(ctx, groupID, userID)
		return e
	}); err != nil {
		return nil, err
	}
	return a.converter.InviteDomainListToDTOList(invites), nil
}

// PreviewInvite returns only the invited group's ID and name before login.
func (a *GroupApp) PreviewInvite(ctx context.Context, token string) (dto.InvitePreview, error) {
	var invited *group.Group
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		invited, e = groupService(repos).InvitedGroup(ctx, token, time.Now().UTC())
		return e
	})
	if err != nil {
		return dto.InvitePreview{}, err
	}
	return a.converter.GroupDomainToInvitePreviewDTO(invited), nil
}

// Join accepts a group invitation and returns the joined group.
func (a *GroupApp) Join(ctx context.Context, userID string, req *dto.JoinReq) (string, error) {
	var groupID string
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		var e error
		groupID, e = groupService(repos).Join(ctx, userID, req.Token, time.Now().UTC())
		return e
	}); err != nil {
		return "", err
	}
	return groupID, nil
}
