package mysql

import (
	"context"

	"github.com/superwhys/one-more-round/internal/converter"
	"github.com/superwhys/one-more-round/internal/domain/notification"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type notificationRepository struct {
	db        *gorm.DB
	converter *converter.Converter
}

func (r *notificationRepository) ListByUser(ctx context.Context, userID string, limit int) ([]*notification.Notification, error) {
	q := queryOf(r.db).Notification
	rows, err := q.WithContext(ctx).Where(q.UserID.Eq(userID)).Order(q.Created.Desc(), q.ID.Desc()).Limit(limit).Find()
	if err != nil {
		return nil, mapErr(err)
	}
	items := make([]*notification.Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, r.converter.NotificationModelToDomain(row))
	}
	return items, nil
}

func (r *notificationRepository) Create(ctx context.Context, item *notification.Notification) error {
	q := queryOf(r.db).Notification
	return mapErr(q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(r.converter.NotificationDomainToModel(item)))
}

func (r *notificationRepository) Save(ctx context.Context, item *notification.Notification) error {
	q := queryOf(r.db).Notification
	_, err := q.WithContext(ctx).Where(q.ID.Eq(item.ID), q.UserID.Eq(item.UserID)).Update(q.ReadAt, item.ReadAt)
	return mapErr(err)
}

func (r *notificationRepository) Get(ctx context.Context, userID, id string) (*notification.Notification, error) {
	q := queryOf(r.db).Notification
	row, err := q.WithContext(ctx).Where(q.ID.Eq(id), q.UserID.Eq(userID)).Take()
	if err != nil {
		return nil, mapErr(err)
	}
	return r.converter.NotificationModelToDomain(row), nil
}
