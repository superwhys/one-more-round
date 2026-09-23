package mysql

import (
	"context"

	"github.com/superwhys/one-more-round/internal/domain/diary"
	"github.com/superwhys/one-more-round/internal/infra/mysql/mapper"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
	"gorm.io/gorm"
)

// commentRepository persists round comments through the generated Query.
type commentRepository struct {
	db *gorm.DB
}

// ListByRound returns the round's comments, oldest first, with the total count.
func (r *commentRepository) ListByRound(ctx context.Context, groupID, roundID string, offset, limit int) ([]*diary.Comment, int, error) {
	q := queryOf(r.db).RoundComment
	rows, total, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID)).Order(q.Created, q.ID).FindByPage(offset, limit)
	if err != nil {
		return nil, 0, mapErr(err)
	}
	items := make([]*diary.Comment, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapper.CommentModelToDomain(row))
	}
	return items, int(total), nil
}

// Get returns one comment of the round.
func (r *commentRepository) Get(ctx context.Context, groupID, roundID, id string) (*diary.Comment, error) {
	q := queryOf(r.db).RoundComment
	row, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID), q.ID.Eq(id)).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return mapper.CommentModelToDomain(row), nil
}

// Save inserts a comment.
func (r *commentRepository) Save(ctx context.Context, c *diary.Comment) error {
	return mapErr(queryOf(r.db).RoundComment.WithContext(ctx).Create(mapper.CommentDomainToModel(c)))
}

// Delete removes one comment of the round and any replies that point at it.
func (r *commentRepository) Delete(ctx context.Context, groupID, roundID, id string) error {
	q := queryOf(r.db).RoundComment
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID)).Where(q.WithContext(ctx).Where(q.ID.Eq(id)).Or(q.ParentID.Eq(id))).Delete()
	return mapErr(err)
}

// DeleteByRound permanently removes every comment of the round.
func (r *commentRepository) DeleteByRound(ctx context.Context, groupID, roundID string) error {
	queries := queryOf(r.db)
	if _, err := queries.CommentIdempotency.WithContext(ctx).Where(queries.CommentIdempotency.GroupID.Eq(groupID), queries.CommentIdempotency.RoundID.Eq(roundID)).Delete(); err != nil {
		return mapErr(err)
	}
	q := queries.RoundComment
	_, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.RoundID.Eq(roundID)).Delete()
	return mapErr(err)
}

// commentIdempotencyRepository stores the fingerprint of a comment submission.
type commentIdempotencyRepository struct {
	db *gorm.DB
}

// Get returns the stored fingerprint and comment of a submission key.
func (r *commentIdempotencyRepository) Get(ctx context.Context, groupID, userID, roundID, key string) (string, string, error) {
	q := queryOf(r.db).CommentIdempotency
	row, err := q.WithContext(ctx).Where(q.GroupID.Eq(groupID), q.UserID.Eq(userID), q.RoundID.Eq(roundID), q.RequestKey.Eq(key)).Take()
	if err != nil {
		return "", "", mapErr(err)
	}
	return row.Hash, row.CommentID, nil
}

// Create stores the fingerprint of a submission key.
func (r *commentIdempotencyRepository) Create(ctx context.Context, groupID, userID, roundID, key, hash, commentID string) error {
	return mapErr(queryOf(r.db).CommentIdempotency.WithContext(ctx).Create(&models.CommentIdempotency{
		GroupID: groupID, UserID: userID, RoundID: roundID, RequestKey: key, Hash: hash, CommentID: commentID,
	}))
}
