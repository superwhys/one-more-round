package mysql_test

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"io/fs"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/domain/photo"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// interruptedFiles simulates remote failures after bytes have already been saved.
type interruptedFiles struct {
	ports.PhotoFiles
	afterSave func(context.Context, string) error
	remove    func(context.Context, string) error
	readErr   error
}

func (f *interruptedFiles) Save(ctx context.Context, id string, r io.Reader) error {
	if err := f.PhotoFiles.Save(ctx, id, r); err != nil {
		return err
	}
	if f.afterSave != nil {
		return f.afterSave(ctx, id)
	}
	return nil
}

func (f *interruptedFiles) Remove(ctx context.Context, id string) error {
	if f.remove != nil {
		if err := f.remove(ctx, id); err != nil {
			return err
		}
	}
	return f.PhotoFiles.Remove(ctx, id)
}

func (f *interruptedFiles) Read(ctx context.Context, id string, thumb bool) (*ports.PhotoContent, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.PhotoFiles.Read(ctx, id, thumb)
}

func uploadData(t *testing.T) io.Reader {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 16, 16))); err != nil {
		t.Fatal(err)
	}
	return &b
}

func TestPhotoFailedUploadAndCleanupRetry(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "cleanup@example.com")
	g, err := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "清理重试", PlayerName: "玩家"})
	if err != nil {
		t.Fatal(err)
	}
	files := &interruptedFiles{
		PhotoFiles: &photos.Files{Root: s.photoRoot},
		afterSave:  func(context.Context, string) error { return errcode.ErrPhotoStorage },
		remove:     func(context.Context, string) error { return errcode.ErrPhotoStorage },
	}
	app := services.NewPhotoApp(&services.AppContext{Repos: s.repos, Photos: files})
	if id, err := app.Upload(ctx, g.ID, u.ID, uploadData(t)); err == nil || id != "" {
		t.Fatal("failed upload reported success")
	}
	pending, err := s.repos.Photo().ListCleanup(ctx, services.PhotoCutoff(time.Now()), 100)
	if err != nil || len(pending) != 1 || pending[0].State != photo.StateDeleting {
		t.Fatalf("failed deletion not retained: %v %v", pending, err)
	}
	id := pending[0].ID
	if err := app.RequireAccess(ctx, g.ID, u.ID, id); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("failed upload exposed: %v", err)
	}
	for _, thumb := range []bool{false, true} {
		content, err := files.PhotoFiles.Read(ctx, id, thumb)
		if err != nil {
			t.Fatal("failure fixture did not leave remote bytes")
		}
		content.Body.Close()
	}
	if err := services.CleanPhotos(ctx, s.repos, files, services.PhotoCutoff(time.Now())); !errors.Is(err, errcode.ErrPhotoStorage) {
		t.Fatalf("cleanup failure hidden: %v", err)
	}
	files.remove = nil
	if err := services.CleanPhotos(ctx, s.repos, files, services.PhotoCutoff(time.Now())); err != nil {
		t.Fatal(err)
	}
	if _, err := s.repos.Photo().Get(ctx, g.ID, id); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("completed deletion retained row: %v", err)
	}
	for _, thumb := range []bool{false, true} {
		if _, err := files.PhotoFiles.Read(ctx, id, thumb); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("cleanup did not remove both objects")
		}
	}
}

func TestPhotoMembershipRevokedDuringUpload(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "photo-owner@example.com")
	member, _ := s.signup(t, "photo-member@example.com")
	g, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "上传中移除", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.groups.Invite(ctx, g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: token}); err != nil {
		t.Fatal(err)
	}
	var storedID string
	files := &interruptedFiles{PhotoFiles: &photos.Files{Root: s.photoRoot}}
	files.afterSave = func(ctx context.Context, id string) error {
		storedID = id
		return s.groups.Manage(ctx, g.ID, owner.ID, &dto.ManageReq{Action: "remove", Target: member.ID})
	}
	app := services.NewPhotoApp(&services.AppContext{Repos: s.repos, Photos: files})
	if _, err := app.Upload(ctx, g.ID, member.ID, uploadData(t)); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("revoked member completed upload: %v", err)
	}
	if _, err := s.repos.Photo().Get(ctx, g.ID, storedID); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("revoked upload left metadata: %v", err)
	}
	if _, err := files.Read(ctx, storedID, false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("revoked upload left file")
	}
}

func TestPhotoCanceledUploadCompensates(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "photo-canceled@example.com")
	g, err := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "取消上传", PlayerName: "玩家"})
	if err != nil {
		t.Fatal(err)
	}
	requestCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var storedID string
	files := &interruptedFiles{PhotoFiles: &photos.Files{Root: s.photoRoot}}
	files.afterSave = func(ctx context.Context, id string) error { storedID = id; cancel(); return ctx.Err() }
	files.remove = func(ctx context.Context, _ string) error {
		if ctx.Err() != nil {
			t.Error("compensation inherited canceled context")
		}
		return nil
	}
	app := services.NewPhotoApp(&services.AppContext{Repos: s.repos, Photos: files})
	if _, err := app.Upload(requestCtx, g.ID, u.ID, uploadData(t)); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if _, err := s.repos.Photo().Get(ctx, g.ID, storedID); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatal("canceled upload not cleaned")
	}
	if _, err := files.Read(ctx, storedID, false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("canceled upload left file")
	}
}

func TestPhotoCleanupClaimBlocksConcurrentAttachment(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "photo-claim@example.com")
	g, err := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "清理和关联", PlayerName: "玩家"})
	if err != nil {
		t.Fatal(err)
	}
	player, err := s.groups.AddPlayer(ctx, g.ID, u.ID, &dto.AddPlayerReq{Name: "同伴"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, g.ID, u.ID, &dto.AddGameReq{Name: "合作"})
	if err != nil {
		t.Fatal(err)
	}
	id := s.uploadPhoto(t, g.ID, u.ID)
	files := &interruptedFiles{PhotoFiles: &photos.Files{Root: s.photoRoot}}
	files.remove = func(ctx context.Context, removing string) error {
		if removing != id {
			t.Fatal("wrong object claimed")
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := s.photos.RequireAccess(ctx, g.ID, u.ID, id); !errors.Is(err, errcode.ErrNotFound) {
			t.Errorf("claimed photo readable or transaction held during IO: %v", err)
		}
		_, err := s.rounds.Save(ctx, u.ID, &dto.SaveRoundReq{GroupID: g.ID, IdempotencyKey: secure.NewID(), Round: dto.Round{GameID: game.ID, Date: "2026-09-19", Mode: "coop", Outcome: "win", Players: []string{player.ID}, Photos: []string{id}}})
		if !errors.Is(err, errcode.ErrPhotoNotFound) {
			t.Errorf("claimed photo attached: %v", err)
		}
		return nil
	}
	if err := services.CleanPhotos(ctx, s.repos, files, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
}

func TestPhotoStorageFailureIsNotNotFound(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "photo-read@example.com")
	g, err := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "读取错误", PlayerName: "玩家"})
	if err != nil {
		t.Fatal(err)
	}
	id := s.uploadPhoto(t, g.ID, u.ID)
	files := &interruptedFiles{PhotoFiles: &photos.Files{Root: s.photoRoot}, readErr: errcode.ErrPhotoStorage}
	app := services.NewPhotoApp(&services.AppContext{Repos: s.repos, Photos: files})
	if _, err := app.Read(ctx, g.ID, "non-member", id, false); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("non-member reached image storage: %v", err)
	}
	if _, err := app.Read(ctx, g.ID, u.ID, id, false); !errors.Is(err, errcode.ErrPhotoStorage) {
		t.Fatalf("storage outage reported as missing photo: %v", err)
	}
	files.readErr = fs.ErrNotExist
	if _, err := app.Read(ctx, g.ID, u.ID, id, false); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatalf("missing object not mapped: %v", err)
	}
}

func TestPhotoDetachPersistsRetentionAndAbandonedUploadCleanup(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "photo-retention@example.com")
	g, err := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "照片保留期", PlayerName: "玩家"})
	if err != nil {
		t.Fatal(err)
	}
	player, err := s.groups.AddPlayer(ctx, g.ID, u.ID, &dto.AddPlayerReq{Name: "同伴"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, g.ID, u.ID, &dto.AddGameReq{Name: "合作"})
	if err != nil {
		t.Fatal(err)
	}
	id := s.uploadPhoto(t, g.ID, u.ID)
	p, err := s.repos.Photo().Get(ctx, g.ID, id)
	if err != nil {
		t.Fatal(err)
	}
	p.Created = time.Now().Add(-8 * 24 * time.Hour)
	if err = s.repos.Photo().Save(ctx, g.ID, p); err != nil {
		t.Fatal(err)
	}
	round, err := s.rounds.Save(ctx, u.ID, &dto.SaveRoundReq{GroupID: g.ID, IdempotencyKey: secure.NewID(), Round: dto.Round{GameID: game.ID, Date: "2026-09-19", Mode: "coop", Outcome: "win", Players: []string{player.ID}, Photos: []string{id}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.rounds.Delete(ctx, u.ID, &dto.DeleteRoundReq{GroupID: g.ID, RoundID: round.ID, Version: round.Version}); err != nil {
		t.Fatal(err)
	}
	files := &photos.Files{Root: s.photoRoot}
	if err = services.CleanPhotos(ctx, s.repos, files, services.PhotoCutoff(time.Now())); err != nil {
		t.Fatal(err)
	}
	p, err = s.repos.Photo().Get(ctx, g.ID, id)
	if err != nil || !p.Attached() {
		t.Fatalf("recoverable round photo detached early: %#v, %v", p, err)
	}
	if err = services.CleanDeletedRounds(ctx, s.repos, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	p, err = s.repos.Photo().Get(ctx, g.ID, id)
	if err != nil || p.Attached() || p.Created.Before(time.Now().Add(-time.Minute)) {
		t.Fatalf("expired round photo retention not persisted: %#v, %v", p, err)
	}
	// A crash after the first object was written leaves an uploading row. Its
	// ID must remain discoverable and become reclaimable after retention.
	p.State, p.Created = photo.StateUploading, time.Now().Add(-8*24*time.Hour)
	if err = s.repos.Photo().Save(ctx, g.ID, p); err != nil {
		t.Fatal(err)
	}
	if err = services.CleanPhotos(ctx, s.repos, files, services.PhotoCutoff(time.Now())); err != nil {
		t.Fatal(err)
	}
	if _, err = files.Read(ctx, id, false); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal("abandoned upload not reclaimed")
	}
}
