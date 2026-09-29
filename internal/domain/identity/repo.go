package identity

import (
	"context"
	"time"
)

// ITrialRepository manages the one-use registration invitations.
type ITrialRepository interface {
	// Create stores a trial invitation holding the token digest.
	Create(ctx context.Context, hash string, expires time.Time) error
	// Consume marks an unused, unexpired invitation as used.
	Consume(ctx context.Context, hash string, now time.Time) error
	// Revoke marks an invitation as used so it cannot register an account.
	Revoke(ctx context.Context, hash string) error
}

// ICredentialRepository persists additional login credentials chosen by the
// application's account-linking rules.
type ICredentialRepository interface {
	// BindEmail adds an email credential to an existing account.
	BindEmail(ctx context.Context, accountID, email string) error
	// BindWechat assigns the locked WeChat credential to an existing account.
	BindWechat(ctx context.Context, accountID, openIDHash string) error
}
