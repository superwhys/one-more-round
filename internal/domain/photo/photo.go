// Package photo owns the metadata of uploaded round photos. The image files
// themselves live outside the database behind the application ports.
package photo

import (
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

type State string

// MaxUploadBytes limits each original upload before image processing.
const MaxUploadBytes = 2 * 1024 * 1024

const (
	StateReady     State = "ready"
	StateUploading State = "uploading"
	StateDeleting  State = "deleting"
)

// Photo is the metadata of one uploaded image of a group.
type Photo struct {
	ID      string
	GroupID string
	Owner   string
	RoundID string
	Created time.Time
	State   State
}

// Attached reports whether the photo belongs to a round.
func (p *Photo) Attached() bool { return p.RoundID != "" }

// ReadableBy reports whether the account may read the photo: an unattached
// upload belongs to its uploader only, an attached photo to the group.
func (p *Photo) ReadableBy(userID string) bool {
	return p.State == StateReady && (p.Attached() || p.Owner == userID)
}

// Attach binds the photo to a round on behalf of the account.
func (p *Photo) Attach(roundID, userID string) error {
	if p.State != StateReady {
		return errcode.ErrPhotoNotFound
	}
	if p.Attached() && p.RoundID != roundID {
		return errcode.ErrPhotoLinked
	}
	if !p.Attached() && p.Owner != userID {
		return errcode.ErrForbidden
	}
	p.RoundID = roundID
	return nil
}

// BeginDeletion claims an expired upload while excluding further attachments.
// A previously claimed deletion can be retried regardless of the cutoff.
func (p *Photo) BeginDeletion(cutoff time.Time) bool {
	if p.Attached() || (p.State != StateDeleting && !p.Created.Before(cutoff)) {
		return false
	}
	p.State = StateDeleting
	return true
}

// Detach releases the photo and restarts its retention window.
func (p *Photo) Detach(now time.Time) {
	p.RoundID = ""
	p.Created = now
}
