package services

import (
	"context"
	"errors"
	"io"
	"io/fs"
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

// Upload tracks the ID before writing any objects. Pending uploads cannot be
// read or attached, and failed compensation remains discoverable by cleanup.
func (a *PhotoApp) Upload(ctx context.Context, groupID, userID string, r io.Reader) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	id := secure.NewID()
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := groupService(repos).RequireMember(ctx, groupID, userID); err != nil {
			return err
		}
		return repos.Photo().Save(ctx, groupID, &photo.Photo{
			ID: id, GroupID: groupID, Owner: userID, Created: time.Now().UTC(), State: photo.StateUploading,
		})
	}); err != nil {
		return "", err
	}
	if err := a.files.Save(ctx, id, r); err != nil {
		return "", errors.Join(err, a.discard(ctx, groupID, id))
	}
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := groupService(repos).RequireMember(ctx, groupID, userID); err != nil {
			return err
		}
		p, err := repos.Photo().Get(ctx, groupID, id)
		if err != nil {
			return err
		}
		if p.State != photo.StateUploading {
			return errcode.ErrPhotoNotFound
		}
		p.State = photo.StateReady
		return repos.Photo().Save(ctx, groupID, p)
	}); err != nil {
		return "", errors.Join(err, a.discard(ctx, groupID, id))
	}
	return id, nil
}

// Read opens a stored image after checking member access. The caller closes it.
func (a *PhotoApp) Read(ctx context.Context, groupID, userID, id string, thumb bool) (*ports.PhotoContent, error) {
	if err := a.RequireAccess(ctx, groupID, userID, id); err != nil {
		return nil, err
	}
	content, err := a.files.Read(ctx, id, thumb)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errcode.ErrNotFound
	}
	return content, err
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
		if p.State != photo.StateReady {
			return errcode.ErrNotFound
		}
		if !p.ReadableBy(userID) {
			return errcode.ErrForbidden
		}
		return nil
	})
}

// discard gets a short independent deadline so a canceled upload can still be
// claimed for deletion. An outage leaves its uploading row for later cleanup.
func (a *PhotoApp) discard(ctx context.Context, groupID, id string) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := a.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := repos.Group().GetByID(ctx, groupID); err != nil {
			return err
		}
		p, err := repos.Photo().Get(ctx, groupID, id)
		if err != nil {
			return err
		}
		if p.Attached() {
			return errcode.ErrPhotoLinked
		}
		p.State = photo.StateDeleting
		return repos.Photo().Save(ctx, groupID, p)
	}); err != nil {
		return err
	}
	return deletePhotoFiles(ctx, a.repos, a.files, groupID, id)
}
