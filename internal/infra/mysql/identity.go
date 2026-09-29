package mysql

import (
	"context"
	"time"

	"github.com/miebyte/authkit"
	authmodels "github.com/miebyte/authkit/mysql/models"
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

// credentialRepository persists credentials linked by application use cases.
type credentialRepository struct {
	db *gorm.DB
}

// BindEmail inserts an email credential for an existing account.
func (r *credentialRepository) BindEmail(ctx context.Context, accountID, email string) error {
	return mapErr(queryOf(r.db).Binding.WithContext(ctx).Create(&authmodels.Binding{
		Method:     authkit.MethodEmail,
		Identifier: email,
		AccountID:  &accountID,
	}))
}

// BindWechat assigns the placeholder locked by authkit to an existing account.
func (r *credentialRepository) BindWechat(
	ctx context.Context,
	accountID, openIDHash string,
) error {
	q := queryOf(r.db).Binding
	result, err := q.WithContext(ctx).
		Where(
			q.Method.Eq(authkit.MethodWechat),
			q.Identifier.Eq(openIDHash),
			q.AccountID.IsNull(),
		).
		UpdateSimple(q.AccountID.Value(accountID))
	if err != nil {
		return mapErr(err)
	}
	if result.RowsAffected != 1 {
		return errcode.ErrConflict
	}
	return nil
}
