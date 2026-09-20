package notification

import "context"

// IRepository stores private account notifications.
type IRepository interface {
	ListByUser(ctx context.Context, userID string, limit int) ([]*Notification, error)
	Create(ctx context.Context, item *Notification) error
	Save(ctx context.Context, item *Notification) error
	Get(ctx context.Context, userID, id string) (*Notification, error)
}
