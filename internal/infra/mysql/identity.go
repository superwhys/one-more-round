package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql/mapper"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

type userRepository struct {
	db *gorm.DB
}

// GetByEmail returns the account of the address.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*identity.User, error) {
	q := queryOf(r.db).User
	m, err := q.WithContext(ctx).Where(q.Email.Eq(email)).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return mapper.UserModelToDomain(m), nil
}

// Create inserts the account.
func (r *userRepository) Create(ctx context.Context, u *identity.User) error {
	return mapErr(queryOf(r.db).User.WithContext(ctx).Create(mapper.UserDomainToModel(u)))
}

// GetBySessionToken returns the account of a live session digest.
func (r *userRepository) GetBySessionToken(
	ctx context.Context,
	hash string,
) (*identity.User, error) {
	q := queryOf(r.db)
	u, session := q.User, q.Session
	m, err := u.WithContext(ctx).
		Select(u.ALL).
		Join(session, session.UserID.EqCol(u.ID)).
		Where(session.Hash.Eq(hash), session.Expires.Gt(time.Now().UTC())).
		Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return mapper.UserModelToDomain(m), nil
}

type verifyCodeRepository struct {
	db *gorm.DB
}

// Get returns the pending code, creating a placeholder row for an address that
// has none yet so the row can be locked for the enclosing transaction.
func (r *verifyCodeRepository) Get(ctx context.Context, email string) (*identity.Challenge, error) {
	c := queryOf(r.db).Challenge
	epoch := time.Unix(0, 0).UTC()
	if err := c.WithContext(ctx).
		Clauses(clause.Insert{Modifier: "IGNORE"}).
		Create(&models.Challenge{Email: email, Expires: epoch, Sent: epoch}); err != nil {
		return nil, mapErr(err)
	}
	m, err := c.WithContext(ctx).
		Where(c.Email.Eq(email)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return mapper.ChallengeModelToDomain(m), nil
}

// Save stores the code, its limits and the readiness flag.
func (r *verifyCodeRepository) Save(ctx context.Context, ch *identity.Challenge) error {
	q := queryOf(r.db).Challenge
	_, err := q.WithContext(ctx).Where(q.Email.Eq(ch.Email)).UpdateSimple(
		q.Hash.Value(ch.Hash), q.InviteHash.Value(ch.Invite), q.Expires.Value(ch.Expires),
		q.Sent.Value(ch.Sent), q.Attempts.Value(ch.Attempts), q.Ready.Value(ch.Ready),
	)
	return mapErr(err)
}

type rateRepository struct {
	db *gorm.DB
}

// Hit counts one send attempt inside an hourly window.
func (r *rateRepository) Hit(ctx context.Context, id string, now time.Time, limit int) error {
	q := queryOf(r.db).Rate
	if err := q.WithContext(ctx).
		Clauses(clause.Insert{Modifier: "IGNORE"}).
		Create(&models.Rate{ID: id, Starts: now}); err != nil {
		return mapErr(err)
	}
	m, err := q.WithContext(ctx).
		Where(q.ID.Eq(id)).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Take()
	if err != nil {
		return mapErr(err)
	}
	if now.Sub(m.Starts) >= time.Hour {
		m.Starts = now
		m.Hits = 0
	}
	if m.Hits >= limit {
		return errcode.ErrTooManyRequests
	}
	_, err = q.WithContext(ctx).
		Where(q.ID.Eq(id)).
		UpdateSimple(q.Starts.Value(m.Starts), q.Hits.Value(m.Hits+1))
	return mapErr(err)
}

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

type sessionRepository struct {
	db *gorm.DB
}

// Create stores the session of a token digest.
func (r *sessionRepository) Create(ctx context.Context, s *identity.Session) error {
	return mapErr(queryOf(r.db).Session.WithContext(ctx).Create(mapper.SessionDomainToModel(s)))
}

// Delete removes the session row of a token digest.
func (r *sessionRepository) Delete(ctx context.Context, hash string) error {
	q := queryOf(r.db).Session
	_, err := q.WithContext(ctx).Where(q.Hash.Eq(hash)).Delete()
	return mapErr(err)
}
