package services

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// RoundApp handles the recorded rounds of a group.
type RoundApp struct {
	repos     ports.Repositories
	converter *converter.Converter
}

// NewRoundApp builds the round application service.
func NewRoundApp(ctx *AppContext) *RoundApp {
	return &RoundApp{repos: ctx.Repos, converter: ctx.Converter}
}

// List returns the filtered timeline of the group with its statistics.
func (a *RoundApp) List(ctx context.Context, groupID, userID string, req *dto.ListRoundsReq) (dto.Page, error) {
	filter := diary.Filter{From: req.From, To: req.To, Game: req.Game, Player: req.Player, Offset: req.Offset, Limit: req.Limit}
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
	return a.converter.PageDomainToDTO(page), nil
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
	return a.converter.RoundDomainToDTO(found), nil
}

// Save validates a submitted round and stores it as a new record or an edit.
// A repeated submission key with the same content returns the stored record; a
// repeated key with different content is a conflict.
func (a *RoundApp) Save(ctx context.Context, userID string, req *dto.SaveRoundReq) (dto.Round, error) {
	groupID, key := req.GroupID, req.IdempotencyKey
	input := req.Round
	normalizeRound(&input)
	input.ID = req.RoundID
	input.Author, input.UpdatedBy, input.UpdatedAt = "", "", time.Time{}
	if err := a.converter.RoundDTOToDomain(&input, groupID).Validate(); err != nil {
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
		round := a.converter.RoundDTOToDomain(&input, groupID)
		round.Author = userID
		round.Version = 1
		var released []string
		if req.RoundID != "" {
			index := slices.IndexFunc(rounds, func(old *diary.Round) bool { return old.ID == req.RoundID })
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
			if e = repos.Idempotency().Create(ctx, groupID, userID, key, fingerprint, round.ID); e != nil {
				return e
			}
		}
		saved = round
		return nil
	})
	if err != nil {
		return dto.Round{}, err
	}
	return a.converter.RoundDomainToDTO(saved), nil
}

// Delete removes a round at the version the client last saw and releases its
// photos.
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
			for _, id := range r.Photos {
				p, e := repos.Photo().Get(ctx, req.GroupID, id)
				if e != nil {
					return e
				}
				p.Detach(now)
				if e = repos.Photo().Save(ctx, req.GroupID, p); e != nil {
					return e
				}
			}
			return repos.Round().Delete(ctx, req.GroupID, req.RoundID)
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
