package converter

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/photo"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

// PhotoModelToDomain converts a photo row into the domain entity.
func (c *Converter) PhotoModelToDomain(m *models.Photo) *photo.Photo {
	if m == nil {
		return nil
	}
	return &photo.Photo{ID: m.ID, GroupID: m.GroupID, Owner: m.Owner, RoundID: m.RoundID, Created: m.Created, State: photo.State(m.State)}
}

// PhotoDomainToModel converts the photo entity into its row of group groupID.
func (c *Converter) PhotoDomainToModel(groupID string, p *photo.Photo) *models.Photo {
	if p == nil {
		return nil
	}
	return &models.Photo{ID: p.ID, GroupID: groupID, Owner: p.Owner, RoundID: p.RoundID, Created: p.Created, State: string(p.State)}
}

// PhotoDomainToDTO converts the photo entity into the API DTO.
func (c *Converter) PhotoDomainToDTO(p *photo.Photo) dto.Photo {
	if p == nil {
		return dto.Photo{}
	}
	return dto.Photo{ID: p.ID, Owner: p.Owner, RoundID: p.RoundID, Created: p.Created}
}
