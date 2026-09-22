package services

import (
	"context"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/mapper"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/notification"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// NotificationApp serves the current account's private activity center.
type NotificationApp struct {
	repos ports.Repositories
}

func NewNotificationApp(ctx *AppContext) *NotificationApp {
	return &NotificationApp{repos: ctx.Repos}
}

// List creates due invitation reminders and returns the newest activity items.
func (a *NotificationApp) List(ctx context.Context, userID string) (dto.NotificationPage, error) {
	now := time.Now().UTC()
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		groups, err := groupService(repos).List(ctx, userID)
		if err != nil {
			return err
		}
		for _, current := range groups {
			if current.Owner != userID {
				continue
			}
			invites, err := groupService(repos).Invites(ctx, current.ID, userID)
			if err != nil {
				return err
			}
			for _, invite := range invites {
				if invite.Revoked || !invite.Expires.After(now) || invite.Expires.Sub(now) > 24*time.Hour {
					continue
				}
				if err = createNotification(ctx, repos, userID, current.ID, "invite_expiring", "小组邀请即将到期", "有一条邀请将在 24 小时内到期。", "/group", "invite-expiring:"+invite.ID, now); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return dto.NotificationPage{}, err
	}
	items, err := a.repos.Notification().ListByUser(ctx, userID, 100)
	if err != nil {
		return dto.NotificationPage{}, err
	}
	page := dto.NotificationPage{Items: make([]dto.Notification, 0, len(items))}
	for _, item := range items {
		page.Items = append(page.Items, mapper.NotificationDomainToDTO(item))
		if item.ReadAt == nil {
			page.Unread++
		}
	}
	return page, nil
}

// Read marks one notification belonging to the current account as read.
func (a *NotificationApp) Read(ctx context.Context, userID, id string) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		item, err := repos.Notification().Get(ctx, userID, id)
		if err != nil {
			return err
		}
		item.Read(time.Now().UTC())
		return repos.Notification().Save(ctx, item)
	})
}

func createNotification(ctx context.Context, repos ports.Repositories, userID, groupID, kind, title, body, link, dedupe string, now time.Time) error {
	return repos.Notification().Create(ctx, &notification.Notification{ID: secure.NewID(), UserID: userID, GroupID: groupID, Kind: kind, Title: title, Body: body, Link: link, DedupeKey: secure.Hash(dedupe), Created: now})
}
