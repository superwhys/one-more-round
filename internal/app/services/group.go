package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/domain/game"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// GroupApp handles groups, their players, games, invitations and membership.
type GroupApp struct {
	repos     ports.Repositories
	photos    ports.PhotoFiles
	converter *converter.Converter
}

// NewGroupApp builds the group application service.
func NewGroupApp(ctx *AppContext) *GroupApp {
	return &GroupApp{repos: ctx.Repos, photos: ctx.Photos, converter: ctx.Converter}
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
		current, err := groupService(repos).RequireMember(ctx, groupID, userID)
		if err != nil {
			return err
		}
		if err = groupService(repos).Manage(ctx, groupID, userID, req.Action, req.Target, req.Value); err != nil {
			return err
		}
		now := time.Now().UTC()
		switch req.Action {
		case "claim", "claim-new":
			return createNotification(ctx, repos, current.Group.Owner, groupID, "claim_requested", "新的玩家关联申请", "有成员申请关联玩家档案，请前往小组页面处理。", "/group", "claim-requested:"+groupID+":"+userID+":"+secure.NewID(), now)
		case "approve":
			return createNotification(ctx, repos, req.Target, groupID, "claim_approved", "玩家关联已通过", "组主已确认你的玩家档案关联。", "/group", "claim-approved:"+groupID+":"+req.Target+":"+secure.NewID(), now)
		case "reject":
			return createNotification(ctx, repos, req.Target, groupID, "claim_rejected", "玩家关联未通过", "组主未通过本次玩家档案关联，你可以重新申请。", "/group", "claim-rejected:"+groupID+":"+req.Target+":"+secure.NewID(), now)
		}
		return nil
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
		now := time.Now().UTC()
		service := groupService(repos)
		invited, e := service.InvitedGroup(ctx, req.Token, now)
		if e != nil {
			return e
		}
		_, memberErr := service.RequireMember(ctx, invited.ID, userID)
		alreadyMember := memberErr == nil
		if memberErr != nil && !errors.Is(memberErr, errcode.ErrForbidden) {
			return memberErr
		}
		groupID, e = service.Join(ctx, userID, req.Token, now)
		if e != nil || alreadyMember || invited.Owner == userID {
			return e
		}
		return createNotification(ctx, repos, invited.Owner, invited.ID, "member_joined", "有朋友加入了小组", "一位新成员通过邀请加入了你的小组。", "/group", "member-joined:"+invited.ID+":"+userID, now)
	}); err != nil {
		return "", err
	}
	return groupID, nil
}

// Export builds an owner-only ZIP backup with JSON, CSV and original photos.
func (a *GroupApp) Export(ctx context.Context, groupID, userID string) ([]byte, error) {
	var snapshot *group.Snapshot
	var rounds, deleted []*diary.Round
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		current, e := groupService(repos).RequireMember(ctx, groupID, userID)
		if e != nil {
			return e
		}
		if !current.IsOwner(userID) {
			return errcode.ErrForbidden
		}
		snapshot = current
		if rounds, e = repos.Round().ListByGroup(ctx, groupID); e != nil {
			return e
		}
		deleted, e = repos.Round().ListDeletedByGroup(ctx, groupID, time.Now().UTC().Add(-RoundRecycleRetention))
		return e
	})
	if err != nil {
		return nil, err
	}

	manifest := struct {
		ExportedAt time.Time    `json:"exported_at"`
		Snapshot   dto.Snapshot `json:"snapshot"`
		Rounds     []dto.Round  `json:"rounds"`
		RecycleBin []dto.Round  `json:"recycle_bin"`
	}{time.Now().UTC(), a.converter.SnapshotDomainToDTO(snapshot), a.converter.RoundDomainListToDTOList(rounds), a.converter.RoundDomainListToDTOList(deleted)}

	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	jsonFile, e := archive.Create("one-more-round.json")
	if e == nil {
		e = json.NewEncoder(jsonFile).Encode(manifest)
	}
	if e != nil {
		_ = archive.Close()
		return nil, e
	}
	csvFile, e := archive.Create("rounds.csv")
	if e != nil {
		_ = archive.Close()
		return nil, e
	}
	writer := csv.NewWriter(csvFile)
	_ = writer.Write([]string{"id", "date", "game", "mode", "outcome", "players", "location", "minutes", "memory", "deleted_at"})
	gameNames, playerNames := map[string]string{}, map[string]string{}
	for _, item := range snapshot.Games {
		gameNames[item.ID] = item.Name
	}
	for _, item := range snapshot.Players {
		playerNames[item.ID] = item.Name
	}
	all := append(append([]*diary.Round{}, rounds...), deleted...)
	photoIDs := map[string]bool{}
	for _, round := range all {
		players := make([]string, 0, len(round.Players))
		for _, id := range round.Players {
			players = append(players, playerNames[id])
		}
		minutes := ""
		if round.Minutes != nil {
			minutes = strconv.Itoa(*round.Minutes)
		}
		deletedAt := ""
		if round.DeletedAt != nil {
			deletedAt = round.DeletedAt.Format(time.RFC3339)
		}
		_ = writer.Write([]string{round.ID, round.Date, gameNames[round.GameID], round.Mode, round.Outcome, strings.Join(players, "、"), round.Location, minutes, round.Memory, deletedAt})
		for _, id := range round.Photos {
			photoIDs[id] = true
		}
	}
	writer.Flush()
	if e = writer.Error(); e != nil {
		_ = archive.Close()
		return nil, e
	}
	for id := range photoIDs {
		content, readErr := a.photos.Read(ctx, id, false)
		if readErr != nil {
			_ = archive.Close()
			return nil, readErr
		}
		entry, createErr := archive.Create("photos/" + id + ".jpg")
		if createErr == nil {
			_, createErr = io.Copy(entry, content.Body)
		}
		closeErr := content.Body.Close()
		if createErr != nil {
			_ = archive.Close()
			return nil, createErr
		}
		if closeErr != nil {
			_ = archive.Close()
			return nil, closeErr
		}
	}
	if err = archive.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
