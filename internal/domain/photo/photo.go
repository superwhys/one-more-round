// Package photo owns the metadata of uploaded round photos. The image files
// themselves live outside the database behind the application ports.
package photo

import (
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

// Photo is the metadata of one uploaded image of a group.
type Photo struct {
	ID      string
	GroupID string
	Owner   string
	RoundID string
	Created time.Time
}

// Attached reports whether the photo belongs to a round.
func (p *Photo) Attached() bool { return p.RoundID != "" }

// ReadableBy reports whether the account may read the photo: an unattached
// upload belongs to its uploader only, an attached photo to the group.
func (p *Photo) ReadableBy(userID string) bool { return p.Attached() || p.Owner == userID }

// Attach binds the photo to a round on behalf of the account.
func (p *Photo) Attach(roundID, userID string) error {
	if p.Attached() && p.RoundID != roundID {
		return errcode.ErrPhotoLinked
	}
	if !p.Attached() && p.Owner != userID {
		return errcode.ErrForbidden
	}
	p.RoundID = roundID
	return nil
}

// Detach releases the photo and restarts its retention window.
func (p *Photo) Detach(now time.Time) {
	p.RoundID = ""
	p.Created = now
}
