package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql/mapper"
)

// trialRepository persists the host's one-use registration invitations.
type trialRepository struct {
	db *gorm.DB
}

// Create stores a trial invitation holding the token digest.
func (r *trialRepository) Create(ctx context.Context, hash string, expires time.Time) error {
	return mapErr(
		queryOf(
			r.db,
		).Trial.WithContext(ctx).
			Create(mapper.TrialDomainToModel(&identity.Trial{Hash: hash, Expires: expires})),
	)
}

// Consume marks an unused, unexpired trial invitation as used.
func (r *trialRepository) Consume(ctx context.Context, hash string, now time.Time) error {
	q := queryOf(r.db).Trial
	res, err := q.WithContext(ctx).
		Where(q.Hash.Eq(hash), q.Consumed.Is(false), q.Expires.Gt(now)).
		UpdateSimple(q.Consumed.Value(true))
	if err != nil {
		return mapErr(err)
	}
	if res.RowsAffected != 1 {
		return errcode.ErrTrialInvalid
	}
	return nil
}

// Revoke marks a trial invitation as used so it cannot register an account.
func (r *trialRepository) Revoke(ctx context.Context, hash string) error {
	q := queryOf(r.db).Trial
	_, err := q.WithContext(ctx).Where(q.Hash.Eq(hash)).UpdateSimple(q.Consumed.Value(true))
	return mapErr(err)
}
