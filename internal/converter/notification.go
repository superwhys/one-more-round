package converter

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/notification"
	"github.com/superwhys/one-more-round/internal/infra/mysql/models"
)

func (c *Converter) NotificationModelToDomain(m *models.Notification) *notification.Notification {
	if m == nil {
		return nil
	}
	return &notification.Notification{ID: m.ID, UserID: m.UserID, GroupID: m.GroupID, Kind: m.Kind, Title: m.Title, Body: m.Body, Link: m.Link, DedupeKey: m.DedupeKey, Created: m.Created, ReadAt: m.ReadAt}
}

func (c *Converter) NotificationDomainToModel(n *notification.Notification) *models.Notification {
	if n == nil {
		return nil
	}
	return &models.Notification{ID: n.ID, UserID: n.UserID, GroupID: n.GroupID, Kind: n.Kind, Title: n.Title, Body: n.Body, Link: n.Link, DedupeKey: n.DedupeKey, Created: n.Created, ReadAt: n.ReadAt}
}

func (c *Converter) NotificationDomainToDTO(n *notification.Notification) dto.Notification {
	return dto.Notification{ID: n.ID, GroupID: n.GroupID, Kind: n.Kind, Title: n.Title, Body: n.Body, Link: n.Link, Created: n.Created, ReadAt: n.ReadAt}
}
