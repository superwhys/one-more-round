package dto

import "time"

// Notification is one private activity item for the current account.
type Notification struct {
	ID      string     `json:"id"`
	GroupID string     `json:"group_id"`
	Kind    string     `json:"kind"`
	Title   string     `json:"title"`
	Body    string     `json:"body"`
	Link    string     `json:"link"`
	Created time.Time  `json:"created"`
	ReadAt  *time.Time `json:"read_at"`
}

// NotificationPage includes the unread badge count.
type NotificationPage struct {
	Items  []Notification `json:"items"`
	Unread int            `json:"unread"`
}

// ReadNotificationReq marks one item read for the signed-in account.
type ReadNotificationReq struct {
	ID string `uri:"id" validate:"required"`
}
