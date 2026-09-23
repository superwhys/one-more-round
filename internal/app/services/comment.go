package services

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// CommentApp serves the group-private comments of a round.
type CommentApp struct {
	repos ports.Repositories
}

// NewCommentApp builds the comment application service.
func NewCommentApp(ctx *AppContext) *CommentApp {
	return &CommentApp{repos: ctx.Repos}
}

// commentFingerprint is the idempotent payload of one comment submission.
type commentFingerprint struct {
	Body     string  `json:"body"`
	ParentID *string `json:"parent_id"`
}

// List returns a page of comments for a living round the caller can read.
func (a *CommentApp) List(ctx context.Context, userID string, req *dto.ListRoundCommentsReq) (dto.Paginated[dto.RoundComment], error) {
	if req.Limit < 1 || req.Limit > 100 || req.Offset < 0 {
		return dto.Paginated[dto.RoundComment]{}, errcode.ErrPageRange
	}
	var items []*diary.Comment
	var total int
	err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, req.GroupID, userID); e != nil {
			return e
		}
		if _, e := findRound(ctx, repos, req.GroupID, req.RoundID); e != nil {
			return e
		}
		var e error
		items, total, e = repos.Comment().ListByRound(ctx, req.GroupID, req.RoundID, req.Offset, req.Limit)
		return e
	})
	if err != nil {
		return dto.Paginated[dto.RoundComment]{}, err
	}
	return dto.Paginated[dto.RoundComment]{Total: total, Items: mapper.CommentDomainListToDTOList(items)}, nil
}

// Create stores a root comment or a one-level reply. A repeated submission key
// with the same content returns the stored comment; a repeated key with
// different content is a conflict.
func (a *CommentApp) Create(ctx context.Context, userID string, req *dto.CreateRoundCommentReq) (dto.RoundComment, error) {
	if len(req.IdempotencyKey) < 16 || len(req.IdempotencyKey) > 128 {
		return dto.RoundComment{}, errcode.ErrIdempotencyKey.WithMessage("缺少有效提交标识，请重新发表")
	}
	comment := &diary.Comment{GroupID: req.GroupID, RoundID: req.RoundID, Author: userID, Body: req.Body, ParentID: req.ParentID}
	if err := comment.Validate(); err != nil {
		return dto.RoundComment{}, errcode.ErrBadRequest.WithMessage(err.Error())
	}
	payload, err := json.Marshal(commentFingerprint{Body: comment.Body, ParentID: comment.ParentID})
	if err != nil {
		return dto.RoundComment{}, err
	}
	fingerprint := secure.Hash(string(payload))
	var saved *diary.Comment
	err = a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, req.GroupID, userID); e != nil {
			return e
		}
		round, e := findRound(ctx, repos, req.GroupID, req.RoundID)
		if e != nil {
			return e
		}
		// 同键先复用已有评论；内容变了视为冲突，评论被删则不能再用该键。
		hash, previous, e := repos.CommentIdempotency().Get(ctx, req.GroupID, userID, req.RoundID, req.IdempotencyKey)
		if e == nil {
			if hash != fingerprint {
				return errcode.ErrIdempotencyBody
			}
			existing, getErr := repos.Comment().Get(ctx, req.GroupID, req.RoundID, previous)
			if getErr != nil {
				if errors.Is(getErr, errcode.ErrNotFound) {
					return errcode.ErrIdempotencyGone
				}
				return getErr
			}
			saved = existing
			return nil
		}
		if !errors.Is(e, errcode.ErrNotFound) {
			return e
		}
		// 回复只能指向同局根评论，不能再套一层。
		var parent *diary.Comment
		if !comment.Root() {
			parent, e = repos.Comment().Get(ctx, req.GroupID, req.RoundID, *comment.ParentID)
			if e != nil {
				if errors.Is(e, errcode.ErrNotFound) {
					return errcode.ErrCommentParent
				}
				return e
			}
			if !parent.Root() {
				return errcode.ErrCommentParent
			}
		}
		comment.ID = secure.NewID()
		comment.Created = time.Now().UTC()
		if e = repos.Comment().Save(ctx, comment); e != nil {
			return e
		}
		if e = repos.CommentIdempotency().Create(ctx, req.GroupID, userID, req.RoundID, req.IdempotencyKey, fingerprint, comment.ID); e != nil {
			return e
		}
		if e = notifyRoundComment(ctx, repos, userID, round, comment, parent); e != nil {
			return e
		}
		saved = comment
		return nil
	})
	if err != nil {
		return dto.RoundComment{}, err
	}
	return mapper.CommentDomainToDTO(saved), nil
}

// Delete removes a comment the caller authored, or any comment when the caller
// is the group owner. Deleting a root comment also removes its replies.
func (a *CommentApp) Delete(ctx context.Context, userID string, req *dto.DeleteRoundCommentReq) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		access, e := groupService(repos).RequireMember(ctx, req.GroupID, userID)
		if e != nil {
			return e
		}
		if _, e = findRound(ctx, repos, req.GroupID, req.RoundID); e != nil {
			return e
		}
		comment, e := repos.Comment().Get(ctx, req.GroupID, req.RoundID, req.CommentID)
		if e != nil {
			return e
		}
		if comment.Author != userID && !access.IsOwner(userID) {
			return errcode.ErrForbidden
		}
		return repos.Comment().Delete(ctx, req.GroupID, req.RoundID, comment.ID)
	})
}

// notifyRoundComment tells the round author or the parent comment author about a new comment.
func notifyRoundComment(ctx context.Context, repos ports.Repositories, userID string, round *diary.Round, comment, parent *diary.Comment) error {
	now := time.Now().UTC()
	link := "/rounds/" + round.ID
	if parent != nil {
		if parent.Author == userID {
			return nil
		}
		return createNotification(ctx, repos, parent.Author, round.GroupID, "round_comment_replied", "有人回复了你的评论", "小组成员回复了你在一局里的评论。", link, "round-comment-replied:"+comment.ID, now)
	}
	if round.Author == userID {
		return nil
	}
	return createNotification(ctx, repos, round.Author, round.GroupID, "round_commented", "有人评论了你记录的对局", "小组成员在你记录的一局里留下了评论。", link, "round-commented:"+comment.ID, now)
}
