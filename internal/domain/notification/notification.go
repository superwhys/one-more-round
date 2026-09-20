package notification

import "time"

// Notification is a private, account-scoped activity item.
type Notification struct {
	ID, UserID, GroupID string
	Kind, Title, Body   string
	Link, DedupeKey     string
	Created             time.Time
	ReadAt              *time.Time
}

// Read marks the item read without changing its event time.
func (n *Notification) Read(now time.Time) {
	if n.ReadAt == nil {
		n.ReadAt = &now
	}
}
