// Package mapper translates between MySQL persistence models and domain
// entities. It must not depend on application DTOs.
package mapper

import (
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// UserModelToDomain converts an account row into the domain entity.
func UserModelToDomain(m *models.User) *identity.User {
	if m == nil {
		return nil
	}
	user := &identity.User{ID: m.ID}
	if m.Email != nil {
		user.Email = *m.Email
	}
	return user
}

// UserDomainToModel converts the account entity into its row.
func UserDomainToModel(u *identity.User) *models.User {
	if u == nil {
		return nil
	}
	m := &models.User{ID: u.ID}
	if u.Email != "" {
		m.Email = &u.Email
	}
	return m
}

// ChallengeModelToDomain converts a verification code row into the domain entity.
func ChallengeModelToDomain(m *models.Challenge) *identity.Challenge {
	if m == nil {
		return nil
	}
	return &identity.Challenge{
		Email:    m.Email,
		Hash:     m.Hash,
		Invite:   m.InviteHash,
		Expires:  m.Expires,
		Sent:     m.Sent,
		Attempts: m.Attempts,
		Ready:    m.Ready,
	}
}

// SessionDomainToModel converts the session entity into its row.
func SessionDomainToModel(s *identity.Session) *models.Session {
	if s == nil {
		return nil
	}
	return &models.Session{Hash: s.Hash, UserID: s.UserID, Expires: s.Expires}
}

// TrialDomainToModel converts a trial invitation into its row.
func TrialDomainToModel(t *identity.Trial) *models.Trial {
	if t == nil {
		return nil
	}
	return &models.Trial{Hash: t.Hash, Expires: t.Expires, Consumed: t.Consumed}
}
