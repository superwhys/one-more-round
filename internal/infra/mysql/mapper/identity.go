// Package mapper translates between MySQL persistence models and domain
// entities. It must not depend on application DTOs.
package mapper

import (
	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// TrialDomainToModel converts a trial invitation into its row.
func TrialDomainToModel(t *identity.Trial) *models.Trial {
	if t == nil {
		return nil
	}
	return &models.Trial{Hash: t.Hash, Expires: t.Expires, Consumed: t.Consumed}
}
