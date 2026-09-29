// Package identity owns the host registration invitations.
// Login credentials and sessions are managed by authkit.
package identity

import "time"

// TrialTTL is how long an unused trial invitation stays valid.
const TrialTTL = 7 * 24 * time.Hour

// Trial is a one-use invitation that grants registration.
type Trial struct {
	Hash     string
	Expires  time.Time
	Consumed bool
}
