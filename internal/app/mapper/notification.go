package mapper

import (
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/domain/notification"
)

// NotificationDomainToDTO converts a private notification into the API DTO.
func NotificationDomainToDTO(n *notification.Notification) dto.Notification {
	return dto.Notification{
		ID:      n.ID,
		GroupID: n.GroupID,
		Kind:    n.Kind,
		Title:   n.Title,
		Body:    n.Body,
		Link:    n.Link,
		Created: n.Created,
		ReadAt:  n.ReadAt,
	}
}
