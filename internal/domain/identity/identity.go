// Package identity owns accounts, verification codes, sessions and the trial
// invitations that gate first-time registration.
package identity

import (
	"crypto/subtle"
	"net/mail"
	"strings"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

const (
	// CodeTTL is how long a verification code stays valid.
	CodeTTL = 10 * time.Minute
	// ResendInterval is the minimum delay between two codes for one address.
	ResendInterval = time.Minute
	// MaxAttempts caps the failed verifications of one code.
	MaxAttempts = 5
	// SessionTTL is the lifetime of a login session.
	SessionTTL = 30 * 24 * time.Hour
	// TrialTTL is how long an unused trial invitation stays valid.
	TrialTTL = 7 * 24 * time.Hour
	// EmailRateLimit is the hourly code limit per address.
	EmailRateLimit = 10
	// IPRateLimit is the hourly code limit per client address.
	IPRateLimit = 30
)

// User is a login account.
type User struct {
	ID    string
	Email string
}

// Challenge is the pending verification code of one email address.
type Challenge struct {
	Email    string
	Hash     string
	Invite   string
	Expires  time.Time
	Sent     time.Time
	Attempts int
	Ready    bool
}

// Expired reports whether the code is past its lifetime.
func (c *Challenge) Expired(now time.Time) bool { return !now.Before(c.Expires) }

// AttemptsExhausted reports whether the code spent all its attempts.
func (c *Challenge) AttemptsExhausted() bool { return c.Attempts >= MaxAttempts }

// Throttled reports whether a new code was requested too soon.
func (c *Challenge) Throttled(now time.Time) bool { return now.Sub(c.Sent) < ResendInterval }

// Matches compares the submitted code against the stored digest.
func (c *Challenge) Matches(email, code string) bool {
	return subtle.ConstantTimeCompare([]byte(c.Hash), []byte(secure.Hash(email+code))) == 1
}

// Session is a stored login session identified by a token digest.
type Session struct {
	Hash    string
	UserID  string
	Expires time.Time
}

// Trial is a one-use invitation that grants registration.
type Trial struct {
	Hash     string
	Expires  time.Time
	Consumed bool
}

// NormalizeEmail validates and lowercases a login address.
func NormalizeEmail(v string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	a, err := mail.ParseAddress(v)
	if err != nil || a.Address != v || len(v) > 254 {
		return "", errcode.ErrInvalidEmail
	}
	return v, nil
}
