package services

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"slices"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// RoundApp handles the recorded rounds of a group.
type RoundApp struct {
	repos ports.Repositories
	files ports.PhotoFiles
}

// NewRoundApp builds the round application service.
func NewRoundApp(ctx *AppContext) *RoundApp {
	return &RoundApp{repos: ctx.Repos, files: ctx.Photos}
}

// ShareStatus returns whether the caller-managed round has an active link.
func (a *RoundApp) ShareStatus(
	ctx context.Context,
	groupID, userID, roundID string,
) (dto.RoundShareStatus, error) {
	var status dto.RoundShareStatus
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := requireShareManager(ctx, repos, groupID, userID, roundID); err != nil {
			return err
		}
		share, err := repos.Round().GetShare(ctx, groupID, roundID)
		if errors.Is(err, errcode.ErrNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		status.Active, status.CreatedAt = share.Active(), share.CreatedAt
		return nil
	})
	return status, err
}

// CreateShare creates or rotates a round's public bearer link.
func (a *RoundApp) CreateShare(
	ctx context.Context,
	groupID, userID, roundID string,
) (dto.RoundShareToken, error) {
	token := secure.NewID()
	created := time.Now().UTC()
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := requireShareManager(ctx, repos, groupID, userID, roundID); err != nil {
			return err
		}
		return repos.Round().
			SaveShare(ctx, &diary.Share{RoundID: roundID, GroupID: groupID, TokenHash: secure.Hash(token), CreatedBy: userID, CreatedAt: created})
	})
	if err != nil {
		return dto.RoundShareToken{}, err
	}
	return dto.RoundShareToken{Token: token, CreatedAt: created}, nil
}

// RevokeShare immediately disables a round's current public link.
func (a *RoundApp) RevokeShare(ctx context.Context, groupID, userID, roundID string) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := requireShareManager(ctx, repos, groupID, userID, roundID); err != nil {
			return err
		}
		return repos.Round().RevokeShare(ctx, groupID, roundID, time.Now().UTC())
	})
}

// PublicRound resolves an active bearer token to a deliberately sanitized view.
func (a *RoundApp) PublicRound(ctx context.Context, token string) (dto.PublicRound, error) {
	if len(token) != 64 {
		return dto.PublicRound{}, errcode.ErrNotFound
	}
	var result dto.PublicRound
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		share, err := repos.Round().ResolveShare(ctx, secure.Hash(token))
		if err != nil {
			return errcode.ErrNotFound
		}
		round, err := findRound(ctx, repos, share.GroupID, share.RoundID)
		if err != nil {
			return errcode.ErrNotFound
		}
		snapshot, err := repos.Group().Snapshot(ctx, share.GroupID)
		if err != nil {
			return err
		}
		result = publicRoundDTO(round, snapshot)
		return nil
	})
	return result, err
}

// PublicPhoto opens a photo only while the share is active and the photo still
// belongs to the shared round.
func (a *RoundApp) PublicPhoto(
	ctx context.Context,
	token, photoID string,
) (*ports.PhotoContent, error) {
	shared, err := a.PublicRound(ctx, token)
	if err != nil || !slices.Contains(shared.Photos, photoID) {
		return nil, errcode.ErrNotFound
	}
	content, err := a.files.Read(ctx, photoID, false)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errcode.ErrNotFound
	}
	return content, err
}

func requireShareManager(
	ctx context.Context,
	repos ports.Repositories,
	groupID, userID, roundID string,
) (*diary.Round, error) {
	access, err := groupService(repos).RequireMember(ctx, groupID, userID)
	if err != nil {
		return nil, err
	}
	round, err := findRound(ctx, repos, groupID, roundID)
	if err != nil {
		return nil, err
	}
	if round.Author != userID && !access.IsOwner(userID) {
		return nil, errcode.ErrForbidden
	}
	return round, nil
}

func findRound(
	ctx context.Context,
	repos ports.Repositories,
	groupID, roundID string,
) (*diary.Round, error) {
	rounds, err := repos.Round().ListByGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	for _, round := range rounds {
		if round.ID == roundID {
			return round, nil
		}
	}
	return nil, errcode.ErrNotFound
}

func publicRoundDTO(round *diary.Round, snapshot *group.Snapshot) dto.PublicRound {
	playerNames := make(map[string]string, len(snapshot.Players))
	for _, player := range snapshot.Players {
		playerNames[player.ID] = player.Name
	}
	gameName := "桌游"
	for _, game := range snapshot.Games {
		if game.ID == round.GameID {
			gameName = game.Name
			break
		}
	}
	players := make([]dto.PublicPlayer, 0, len(round.Players))
	for _, id := range round.Players {
		players = append(
			players,
			dto.PublicPlayer{Name: playerNames[id], Score: round.Scores[id], Winner: round.Won(id)},
		)
	}
	teams := make([]dto.PublicTeam, 0, len(round.Teams))
	for _, team := range round.Teams {
		names := make([]string, 0, len(team.Players))
		for _, id := range team.Players {
			names = append(names, playerNames[id])
		}
		teams = append(
			teams,
			dto.PublicTeam{Name: team.Name, Players: names, Score: team.Score, Winner: team.Winner},
		)
	}
	return dto.PublicRound{
		GroupName: snapshot.Group.Name,
		GameName:  gameName,
		Date:      round.Date,
		Mode:      round.Mode,
		Outcome:   round.Outcome,
		Players:   players,
		Teams:     teams,
		TeamScore: round.TeamScore,
		Memory:    round.Memory,
		Photos:    round.Photos,
	}
}

// List returns the filtered timeline of the group with its statistics.
func (a *RoundApp) List(
	ctx context.Context,
	groupID, userID string,
	req *dto.ListRoundsReq,
) (dto.Page, error) {
	filter := diary.Filter{
		From:      req.From,
		To:        req.To,
		Game:      req.Game,
		Player:    req.Player,
		Query:     req.Query,
		Mode:      req.Mode,
		Outcome:   req.Outcome,
		HasPhotos: req.HasPhotos,
		Offset:    req.Offset,
		Limit:     req.Limit,
	}
	if err := diary.ValidateFilter(filter); err != nil {
		return dto.Page{}, err
	}
	var rounds []*diary.Round
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		var e error
		rounds, e = repos.Round().ListByGroup(ctx, groupID)
		return e
	}); err != nil {
		return dto.Page{}, err
	}
	page, err := diary.List(rounds, filter)
	if err != nil {
		return dto.Page{}, err
	}
	return mapper.PageDomainToDTO(page), nil
}

// Recap returns full-period highlights independent of timeline pagination.
func (a *RoundApp) Recap(ctx context.Context, groupID, userID, period string) (dto.Recap, error) {
	from, to, err := recapRange(period)
	if err != nil {
		return dto.Recap{}, err
	}
	var rounds []*diary.Round
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		var listErr error
		rounds, listErr = repos.Round().ListByGroup(ctx, groupID)
		return listErr
	})
	if err != nil {
		return dto.Recap{}, err
	}
	return mapper.RecapDomainToDTO(period, diary.BuildRecap(rounds, from, to)), nil
}

func recapRange(period string) (string, string, error) {
	if len(period) == 4 {
		start, err := time.Parse("2006", period)
		if err != nil {
			return "", "", errcode.ErrBadRequest.WithMessage("回顾年份无效")
		}
		return start.Format("2006-01-02"), start.AddDate(1, 0, -1).Format("2006-01-02"), nil
	}
	start, err := time.Parse("2006-01", period)
	if err != nil {
		return "", "", errcode.ErrBadRequest.WithMessage("回顾月份无效")
	}
	return start.Format("2006-01-02"), start.AddDate(0, 1, -1).Format("2006-01-02"), nil
}

// RecycleBin lists rounds deleted during the seven-day recovery window.
func (a *RoundApp) RecycleBin(ctx context.Context, groupID, userID string) ([]dto.Round, error) {
	var rounds []*diary.Round
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		var e error
		rounds, e = repos.Round().
			ListDeletedByGroup(ctx, groupID, time.Now().UTC().Add(-RoundRecycleRetention))
		return e
	})
	if err != nil {
		return nil, err
	}
	return mapper.RoundDomainListToDTOList(rounds), nil
}

// Restore returns one recoverable round to the timeline.
func (a *RoundApp) Restore(
	ctx context.Context,
	userID string,
	req *dto.RestoreRoundReq,
) (dto.Round, error) {
	var restored *diary.Round
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		access, e := groupService(repos).RequireMember(ctx, req.GroupID, userID)
		if e != nil {
			return e
		}
		rounds, e := repos.Round().
			ListDeletedByGroup(ctx, req.GroupID, time.Now().UTC().Add(-RoundRecycleRetention))
		if e != nil {
			return e
		}
		for _, round := range rounds {
			if round.ID != req.RoundID {
				continue
			}
			if round.Author != userID && !access.IsOwner(userID) {
				return errcode.ErrForbidden
			}
			if round.Version != req.Version {
				return errcode.ErrRoundStale
			}
			now := time.Now().UTC()
			round.DeletedAt = nil
			round.Version++
			round.UpdatedBy, round.UpdatedAt = userID, now
			if e = repos.Round().Save(ctx, req.GroupID, round); e != nil {
				return e
			}
			restored = round
			return nil
		}
		return errcode.ErrNotFound
	})
	if err != nil {
		return dto.Round{}, err
	}
	return mapper.RoundDomainToDTO(restored), nil
}

// Get returns one round of the group.
func (a *RoundApp) Get(ctx context.Context, groupID, userID, id string) (dto.Round, error) {
	var found *diary.Round
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		rounds, e := repos.Round().ListByGroup(ctx, groupID)
		if e != nil {
			return e
		}
		for _, r := range rounds {
			if r.ID == id {
				found = r
				return nil
			}
		}
		return errcode.ErrNotFound
	}); err != nil {
		return dto.Round{}, err
	}
	return mapper.RoundDomainToDTO(found), nil
}

// Save validates a submitted round and stores it as a new record or an edit.
// A repeated submission key with the same content returns the stored record; a
// repeated key with different content is a conflict.
func (a *RoundApp) Save(
	ctx context.Context,
	userID string,
	req *dto.SaveRoundReq,
) (dto.Round, error) {
	groupID, key := req.GroupID, req.IdempotencyKey
	input := req.Round
	normalizeRound(&input)
	input.ID = req.RoundID
	input.Author, input.UpdatedBy, input.UpdatedAt, input.DeletedAt = "", "", time.Time{}, nil
	if err := mapper.RoundDTOToDomain(&input, groupID).Validate(); err != nil {
		return dto.Round{}, errcode.ErrBadRequest.WithMessage(err.Error())
	}
	if req.RoundID == "" && (len(key) < 16 || len(key) > 128) {
		return dto.Round{}, errcode.ErrIdempotencyKey
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return dto.Round{}, err
	}
	fingerprint := secure.Hash(string(payload))
	var saved *diary.Round
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		access, e := groupService(repos).RequireMember(ctx, groupID, userID)
		if e != nil {
			return e
		}
		rounds, e := repos.Round().ListByGroup(ctx, groupID)
		if e != nil {
			return e
		}
		if req.RoundID == "" {
			hash, previous, e := repos.Idempotency().Get(ctx, groupID, userID, key)
			if e == nil {
				if hash != fingerprint {
					return errcode.ErrIdempotencyBody
				}
				for _, old := range rounds {
					if old.ID == previous {
						saved = old
						return nil
					}
				}
				return errcode.ErrIdempotencyGone
			}
			if !errors.Is(e, errcode.ErrNotFound) {
				return e
			}
		}
		if !access.HasGame(input.GameID) {
			return errcode.ErrGameNotInGroup
		}
		for _, p := range input.Players {
			if !access.HasPlayer(p) {
				return errcode.ErrPlayerNotInGroup
			}
		}
		round := mapper.RoundDTOToDomain(&input, groupID)
		round.Author = userID
		round.Version = 1
		var released []string
		if req.RoundID != "" {
			index := slices.IndexFunc(
				rounds,
				func(old *diary.Round) bool { return old.ID == req.RoundID },
			)
			if index < 0 {
				return errcode.ErrNotFound
			}
			old := rounds[index]
			if old.Author != userID && !access.IsOwner(userID) {
				return errcode.ErrForbidden
			}
			if old.Version != input.Version {
				return errcode.ErrRoundStale
			}
			round.Version = old.Version + 1
			round.Author = old.Author
			released = old.Photos
		} else {
			round.ID = secure.NewID()
		}
		now := time.Now().UTC()
		for _, id := range round.Photos {
			p, e := repos.Photo().Get(ctx, groupID, id)
			if e != nil {
				return errcode.ErrPhotoNotFound
			}
			if e = p.Attach(round.ID, userID); e != nil {
				return e
			}
			if e = repos.Photo().Save(ctx, groupID, p); e != nil {
				return e
			}
		}
		for _, id := range released {
			if slices.Contains(round.Photos, id) {
				continue
			}
			p, e := repos.Photo().Get(ctx, groupID, id)
			if e != nil {
				return e
			}
			p.Detach(now)
			if e = repos.Photo().Save(ctx, groupID, p); e != nil {
				return e
			}
		}
		round.UpdatedBy = userID
		round.UpdatedAt = now
		if e = repos.Round().Save(ctx, groupID, round); e != nil {
			return e
		}
		if req.RoundID == "" {
			if e = repos.Idempotency().
				Create(ctx, groupID, userID, key, fingerprint, round.ID); e != nil {
				return e
			}
		}
		saved = round
		return nil
	})
	if err != nil {
		return dto.Round{}, err
	}
	return mapper.RoundDomainToDTO(saved), nil
}

// Delete moves a round into the seven-day recycle bin.
func (a *RoundApp) Delete(ctx context.Context, userID string, req *dto.DeleteRoundReq) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		access, e := groupService(repos).RequireMember(ctx, req.GroupID, userID)
		if e != nil {
			return e
		}
		rounds, e := repos.Round().ListByGroup(ctx, req.GroupID)
		if e != nil {
			return e
		}
		for _, r := range rounds {
			if r.ID != req.RoundID {
				continue
			}
			if r.Author != userID && !access.IsOwner(userID) {
				return errcode.ErrForbidden
			}
			if r.Version != req.Version {
				return errcode.ErrRoundDeleteStale
			}
			now := time.Now().UTC()
			share, shareErr := repos.Round().GetShare(ctx, req.GroupID, req.RoundID)
			if shareErr != nil && !errors.Is(shareErr, errcode.ErrNotFound) {
				return shareErr
			}
			if shareErr == nil && share.Active() {
				if e = repos.Round().RevokeShare(ctx, req.GroupID, req.RoundID, now); e != nil {
					return e
				}
			}
			r.DeletedAt = &now
			r.Version++
			r.UpdatedBy, r.UpdatedAt = userID, now
			return repos.Round().Save(ctx, req.GroupID, r)
		}
		return errcode.ErrNotFound
	})
}

// normalizeRound gives a submitted round the non-nil collections the stored
// document expects, so JSON keeps `[]` and `{}` instead of null.
func normalizeRound(r *dto.Round) {
	if r.Players == nil {
		r.Players = []string{}
	}
	if r.Winners == nil {
		r.Winners = []string{}
	}
	if r.Scores == nil {
		r.Scores = map[string]*string{}
	}
	if r.Teams == nil {
		r.Teams = []dto.Team{}
	}
	if r.Photos == nil {
		r.Photos = []string{}
	}
}
