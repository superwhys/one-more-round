package services

import (
	"context"
	"io"
	"time"

	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/domain/photo"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// PhotoApp handles photo uploads and authorized reads.
type PhotoApp struct {
	repos ports.Repositories
	files ports.PhotoFiles
}

// NewPhotoApp builds the photo application service.
func NewPhotoApp(ctx *AppContext) *PhotoApp {
	return &PhotoApp{repos: ctx.Repos, files: ctx.Photos}
}

// Upload stores an image file and records its metadata, removing the file again
// when the metadata cannot be written. An upload that failed does not block the
// rest of the round.
func (a *PhotoApp) Upload(ctx context.Context, groupID, userID string, r io.Reader) (string, error) {
	if err := a.requireMember(ctx, groupID, userID); err != nil {
		return "", err
	}
	id := secure.NewID()
	if err := a.files.Save(ctx, id, r); err != nil {
		return "", errcode.ErrPhotoUpload
	}
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		return repos.Photo().Save(ctx, groupID, &photo.Photo{ID: id, GroupID: groupID, Owner: userID, Created: time.Now().UTC()})
	}); err != nil {
		a.files.Remove(id)
		return "", err
	}
	return id, nil
}

// Read returns one stored image after checking member access.
func (a *PhotoApp) Read(ctx context.Context, groupID, userID, id string, thumb bool) ([]byte, error) {
	if err := a.RequireAccess(ctx, groupID, userID, id); err != nil {
		return nil, err
	}
	data, err := a.files.Read(id, thumb)
	if err != nil {
		return nil, errcode.ErrNotFound
	}
	return data, nil
}

// RequireAccess checks that the member may read the photo. An unattached upload
// is readable by its uploader only.
func (a *PhotoApp) RequireAccess(ctx context.Context, groupID, userID, id string) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, e := groupService(repos).RequireMember(ctx, groupID, userID); e != nil {
			return e
		}
		p, e := repos.Photo().Get(ctx, groupID, id)
		if e != nil {
			return e
		}
		if !p.ReadableBy(userID) {
			return errcode.ErrForbidden
		}
		return nil
	})
}

// requireMember fails unless the account is a current member of the group.
func (a *PhotoApp) requireMember(ctx context.Context, groupID, userID string) error {
	return a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		_, e := groupService(repos).RequireMember(ctx, groupID, userID)
		return e
	})
}
