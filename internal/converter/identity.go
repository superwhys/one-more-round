package converter

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// UserModelToDomain converts an account row into the domain entity.
func (c *Converter) UserModelToDomain(m *models.User) *identity.User {
	if m == nil {
		return nil
	}
	return &identity.User{ID: m.ID, Email: m.Email}
}

// UserDomainToModel converts the account entity into its row.
func (c *Converter) UserDomainToModel(u *identity.User) *models.User {
	if u == nil {
		return nil
	}
	return &models.User{ID: u.ID, Email: u.Email}
}

// UserDomainToDTO converts the account entity into the API DTO.
func (c *Converter) UserDomainToDTO(u *identity.User) *dto.User {
	if u == nil {
		return nil
	}
	return &dto.User{ID: u.ID, Email: u.Email}
}

// ChallengeModelToDomain converts a verification code row into the domain entity.
func (c *Converter) ChallengeModelToDomain(m *models.Challenge) *identity.Challenge {
	if m == nil {
		return nil
	}
	return &identity.Challenge{Email: m.Email, Hash: m.Hash, Invite: m.InviteHash, Expires: m.Expires, Sent: m.Sent, Attempts: m.Attempts, Ready: m.Ready}
}

// SessionDomainToModel converts the session entity into its row.
func (c *Converter) SessionDomainToModel(s *identity.Session) *models.Session {
	if s == nil {
		return nil
	}
	return &models.Session{Hash: s.Hash, UserID: s.UserID, Expires: s.Expires}
}

// TrialDomainToModel converts a trial invitation into its row.
func (c *Converter) TrialDomainToModel(t *identity.Trial) *models.Trial {
	if t == nil {
		return nil
	}
	return &models.Trial{Hash: t.Hash, Expires: t.Expires, Consumed: t.Consumed}
}

// TrialModelToDomain converts a trial invitation row into the domain entity.
func (c *Converter) TrialModelToDomain(m *models.Trial) *identity.Trial {
	if m == nil {
		return nil
	}
	return &identity.Trial{Hash: m.Hash, Expires: m.Expires, Consumed: m.Consumed}
}
