package identity

import (
	"context"
	"time"
)

// IUserRepository reads and creates accounts.
type IUserRepository interface {
	// GetByEmail returns the account of the address, or errcode.ErrNotFound.
	GetByEmail(ctx context.Context, email string) (*User, error)
	// Create inserts the account.
	Create(ctx context.Context, u *User) error
	// GetBySessionToken returns the account of a live session digest.
	GetBySessionToken(ctx context.Context, hash string) (*User, error)
}

// IVerifyCodeRepository stores the pending code of each address. Get returns a
// zero-valued placeholder for an address that has none yet.
type IVerifyCodeRepository interface {
	Get(ctx context.Context, email string) (*Challenge, error)
	Save(ctx context.Context, c *Challenge) error
}

// IRateRepository counts send attempts per identifier in an hourly window.
type IRateRepository interface {
	// Hit records one attempt and fails with errcode.ErrTooManyRequests once
	// the limit of the current window is reached.
	Hit(ctx context.Context, id string, now time.Time, limit int) error
}

// ITrialRepository manages the one-use registration invitations.
type ITrialRepository interface {
	// Create stores a trial invitation holding the token digest.
	Create(ctx context.Context, hash string, expires time.Time) error
	// Consume marks an unused, unexpired invitation as used.
	Consume(ctx context.Context, hash string, now time.Time) error
	// Revoke marks an invitation as used so it cannot register an account.
	Revoke(ctx context.Context, hash string) error
}

// ISessionRepository stores login sessions by token digest.
type ISessionRepository interface {
	Create(ctx context.Context, s *Session) error
	Delete(ctx context.Context, hash string) error
}
