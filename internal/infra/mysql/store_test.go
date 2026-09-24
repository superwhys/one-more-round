package mysql_test

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/miebyte/goutils/mysqlutils"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
	storepkg "github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

func TestSearchRecapNotificationsAndExport(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "feature-owner@example.com")
	member, _ := s.signup(t, "feature-member@example.com")
	group, err := s.groups.Create(
		ctx,
		owner.ID,
		&dto.CreateGroupReq{Name: "功能小组", PlayerName: "组主"},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := s.groups.Invite(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: token}); err != nil {
		t.Fatal(err)
	}
	ownerNotices, err := s.notifications.List(ctx, owner.ID)
	if err != nil || ownerNotices.Unread != 1 || ownerNotices.Items[0].Kind != "member_joined" {
		t.Fatalf("owner notifications = %#v, %v", ownerNotices, err)
	}

	player, err := s.groups.AddPlayer(ctx, group.ID, owner.ID, &dto.AddPlayerReq{Name: "朋友"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "记忆测试"})
	if err != nil {
		t.Fatal(err)
	}
	minutes := 60
	saved, err := s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{
			GroupID:        group.ID,
			IdempotencyKey: secure.NewID(),
			Round: dto.Round{
				GameID:  game.ID,
				Date:    "2026-09-20",
				Mode:    "coop",
				Outcome: "win",
				Players: []string{player.ID},
				Memory:  "第一次打通",
				Minutes: &minutes,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	hasPhotos := false
	page, err := s.rounds.List(
		ctx,
		group.ID,
		owner.ID,
		&dto.ListRoundsReq{
			Query:     "打通",
			Mode:      "coop",
			Outcome:   "win",
			HasPhotos: &hasPhotos,
			Limit:     30,
		},
	)
	if err != nil || page.Total != 1 || page.Items[0].ID != saved.ID {
		t.Fatalf("advanced search = %#v, %v", page, err)
	}
	recap, err := s.rounds.Recap(ctx, group.ID, owner.ID, "2026-09")
	if err != nil || recap.Rounds != 1 || recap.Minutes != 60 || recap.TopGame != game.ID {
		t.Fatalf("recap = %#v, %v", recap, err)
	}

	backup, err := s.groups.Export(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(backup), int64(len(backup)))
	if err != nil {
		t.Fatal(err)
	}
	entries := map[string]bool{}
	for _, file := range archive.File {
		entries[file.Name] = true
	}
	if !entries["one-more-round.json"] || !entries["rounds.csv"] {
		t.Fatalf("export entries = %#v", entries)
	}
	if _, err = s.groups.Export(ctx, group.ID, member.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("member export error = %v", err)
	}

	if err = s.groups.Manage(
		ctx,
		group.ID,
		member.ID,
		&dto.ManageReq{Action: "claim", Target: player.ID},
	); err != nil {
		t.Fatal(err)
	}
	ownerNotices, err = s.notifications.List(ctx, owner.ID)
	if err != nil || ownerNotices.Unread < 2 {
		t.Fatalf("claim notification = %#v, %v", ownerNotices, err)
	}
	if err = s.groups.Manage(
		ctx,
		group.ID,
		owner.ID,
		&dto.ManageReq{Action: "approve", Target: member.ID},
	); err != nil {
		t.Fatal(err)
	}
	memberNotices, err := s.notifications.List(ctx, member.ID)
	if err != nil || memberNotices.Unread != 1 || memberNotices.Items[0].Kind != "claim_approved" {
		t.Fatalf("member notifications = %#v, %v", memberNotices, err)
	}
	if err = s.notifications.Read(ctx, member.ID, memberNotices.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	memberNotices, _ = s.notifications.List(ctx, member.ID)
	if memberNotices.Unread != 0 {
		t.Fatalf("notification not read: %#v", memberNotices)
	}
}

// testOrigin is the origin the HTTP tests send and the cookie rules expect.
const testOrigin = "http://localhost:8080"

// inbox collects the verification codes the application asks the mailer to send.
type inbox struct {
	mu    sync.Mutex
	codes map[string]string
}

// SendCode records the code of one address.
func (m *inbox) SendCode(_ context.Context, email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[email] = code
	return nil
}

// code returns the last code sent to the address.
func (m *inbox) code(email string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[email]
}

// stack is the wired application the integration tests exercise.
type stack struct {
	client        *storepkg.Client
	repos         *storepkg.RepositoryFactory
	auth          *services.AuthApp
	groups        *services.GroupApp
	rounds        *services.RoundApp
	photos        *services.PhotoApp
	notifications *services.NotificationApp
	comments      *services.CommentApp
	photoRoot     string
	inbox         *inbox
}

// skipUnlessMySQL returns the address of the disposable test instance, or skips
// the test when none is configured.
func skipUnlessMySQL(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("OMR_TEST_MYSQL")
	if addr == "" {
		t.Skip("OMR_TEST_MYSQL not configured; requires an isolated local MySQL instance")
	}
	return addr
}

// setup creates an isolated database, wires the application and returns it.
func setup(t *testing.T) *stack {
	t.Helper()
	addr := skipUnlessMySQL(t)
	root, err := sql.Open("mysql", "root@tcp("+addr+")/?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	name := "omr_test_" + newTestDatabaseName()
	if _, err = root.ExecContext(
		context.Background(),
		"CREATE DATABASE "+name+" CHARACTER SET utf8mb4 COLLATE utf8mb4_bin",
	); err != nil {
		t.Fatal(err)
	}
	client, err := storepkg.Open(
		mysqlutils.MysqlConfig{Instance: addr, Database: name, Username: "root", PoolSize: 10},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		client.Close()
		root.Exec("DROP DATABASE " + name)
		root.Close()
	})
	if err = client.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	repos := storepkg.NewRepositoryFactory(client.Gorm)
	mailer := &inbox{codes: map[string]string{}}
	photoRoot := t.TempDir()
	appCtx := &services.AppContext{
		Repos:  repos,
		Mailer: mailer,
		Photos: &photos.Files{Root: photoRoot},
	}
	return &stack{
		client:        client,
		repos:         repos,
		auth:          services.NewAuthApp(appCtx),
		groups:        services.NewGroupApp(appCtx),
		rounds:        services.NewRoundApp(appCtx),
		photos:        services.NewPhotoApp(appCtx),
		notifications: services.NewNotificationApp(appCtx),
		comments:      services.NewCommentApp(appCtx),
		photoRoot:     photoRoot,
		inbox:         mailer,
	}
}

// signup registers an account through a trial invitation and returns it with its
// session token.
func (s *stack) signup(t *testing.T, email string) (dto.User, string) {
	t.Helper()
	ctx := context.Background()
	token := secure.NewID()
	if err := s.repos.Trial().
		Create(ctx, secure.Hash(token), time.Now().UTC().Add(7*24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: email, Invite: token},
		"127.0.0.1",
	); err != nil {
		t.Fatal(err)
	}
	user, session, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: s.inbox.code(email)})
	if err != nil {
		t.Fatal(err)
	}
	return user.User, session
}

// uploadPhoto stores a tiny generated image and returns its identifier.
func (s *stack) uploadPhoto(t *testing.T, groupID, userID string) string {
	t.Helper()
	source := image.NewRGBA(image.Rect(0, 0, 4, 4))
	source.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, source); err != nil {
		t.Fatal(err)
	}
	id, err := s.photos.Upload(context.Background(), groupID, userID, &buffer)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestRealMySQLDiary(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, session := s.signup(t, "owner@example.com")
	member, _ := s.signup(t, "member@example.com")
	outsider, _ := s.signup(t, "outsider@example.com")
	g, e := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "测试小组", PlayerName: "组主"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.groups.Snapshot(ctx, g.ID, outsider.ID); e == nil {
		t.Fatal("outsider read group")
	}
	_, invite, e := s.groups.Invite(ctx, g.ID, owner.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: invite}); e != nil {
		t.Fatal(e)
	}
	a, e := s.groups.AddPlayer(ctx, g.ID, owner.ID, &dto.AddPlayerReq{Name: "阿林"})
	if e != nil {
		t.Fatal(e)
	}
	b, e := s.groups.AddPlayer(ctx, g.ID, owner.ID, &dto.AddPlayerReq{Name: "小周"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.groups.AddPlayer(ctx, g.ID, owner.ID, &dto.AddPlayerReq{Name: "阿林"}); e == nil {
		t.Fatal("duplicate player")
	}
	game, e := s.groups.AddGame(ctx, g.ID, owner.ID, &dto.AddGameReq{Name: "璀璨宝石"})
	if e != nil {
		t.Fatal(e)
	}
	negative := "-1.2500"
	zero := "0"
	r := dto.Round{
		GameID:  game.ID,
		Date:    "2026-09-18",
		Mode:    "individual",
		Outcome: "win",
		Players: []string{a.ID, b.ID},
		Winners: []string{a.ID, b.ID},
		Scores:  map[string]*string{a.ID: &negative, b.ID: &zero},
		Photos:  []string{},
		Teams:   []dto.Team{},
	}
	key := secure.NewID()
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			saved, e := s.rounds.Save(
				ctx,
				owner.ID,
				&dto.SaveRoundReq{Round: r, GroupID: g.ID, IdempotencyKey: key},
			)
			if e != nil {
				errs <- e
			} else {
				ids <- saved.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	id := ""
	for v := range ids {
		if id != "" && id != v {
			t.Fatal("duplicate round")
		}
		id = v
	}
	changed := r
	changed.Memory = "changed"
	if _, e = s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{Round: changed, GroupID: g.ID, IdempotencyKey: key},
	); e == nil {
		t.Fatal("same key different body accepted")
	}
	saved, e := s.rounds.Get(ctx, g.ID, owner.ID, id)
	if e != nil {
		t.Fatal(e)
	}
	if saved.Scores[a.ID] == nil || *saved.Scores[a.ID] != negative {
		t.Fatal("lost decimal precision")
	}
	if _, e = s.rounds.Save(
		ctx,
		member.ID,
		&dto.SaveRoundReq{Round: saved, GroupID: g.ID, RoundID: id},
	); e == nil {
		t.Fatal("member edits other record")
	}
	saved.Memory = "新回忆"
	updated, e := s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{Round: saved, GroupID: g.ID, RoundID: id},
	)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{Round: saved, GroupID: g.ID, RoundID: id},
	); e == nil {
		t.Fatal("stale update")
	}
	if e = s.rounds.Delete(
		ctx,
		owner.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: id, Version: saved.Version},
	); e == nil {
		t.Fatal("stale delete")
	}
	page, e := s.rounds.List(ctx, g.ID, owner.ID, &dto.ListRoundsReq{Limit: 30})
	if e != nil {
		t.Fatal(e)
	}
	if page.Total != 1 || len(page.Stats) != 2 || page.Stats[0].Wins != 1 ||
		page.Stats[0].Samples != 1 {
		t.Fatalf("bad stats %#v", page)
	}
	for _, outcome := range []string{"draw", "unknown"} {
		extra := r
		extra.Outcome = outcome
		extra.Winners = nil
		if _, e = s.rounds.Save(
			ctx,
			owner.ID,
			&dto.SaveRoundReq{Round: extra, GroupID: g.ID, IdempotencyKey: secure.NewID()},
		); e != nil {
			t.Fatal(e)
		}
	}
	page, e = s.rounds.List(ctx, g.ID, owner.ID, &dto.ListRoundsReq{Limit: 1, Player: a.ID})
	if e != nil || page.Total != 3 || len(page.Items) != 1 || page.Stats[0].Samples != 2 {
		t.Fatalf("bad filtered stats: %#v %v", page, e)
	}
	other, e := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "其他小组", PlayerName: "组主"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{Round: r, GroupID: other.ID, IdempotencyKey: secure.NewID()},
	); e == nil {
		t.Fatal("cross-group resources accepted")
	}
	if e = s.groups.Manage(
		ctx,
		g.ID,
		member.ID,
		&dto.ManageReq{Action: "claim", Target: a.ID},
	); e != nil {
		t.Fatal(e)
	}
	if e = s.groups.Manage(
		ctx,
		g.ID,
		owner.ID,
		&dto.ManageReq{Action: "approve", Target: member.ID},
	); e != nil {
		t.Fatal(e)
	}
	if e = s.groups.Manage(
		ctx,
		g.ID,
		member.ID,
		&dto.ManageReq{Action: "claim", Target: b.ID},
	); e == nil {
		t.Fatal("account bound twice")
	}
	s.uploadPhoto(t, g.ID, member.ID)
	if e = s.groups.Manage(
		ctx,
		g.ID,
		owner.ID,
		&dto.ManageReq{Action: "remove", Target: member.ID},
	); e != nil {
		t.Fatal(e)
	}
	if _, e = s.rounds.List(ctx, g.ID, member.ID, &dto.ListRoundsReq{Limit: 30}); e == nil {
		t.Fatal("removed member can read")
	}
	if _, e = s.rounds.Save(
		ctx,
		member.ID,
		&dto.SaveRoundReq{Round: r, GroupID: g.ID, IdempotencyKey: secure.NewID()},
	); e == nil {
		t.Fatal("removed member can write")
	}
	if e = s.rounds.Delete(
		ctx,
		owner.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: id, Version: updated.Version},
	); e != nil {
		t.Fatal(e)
	}
	page, _ = s.rounds.List(ctx, g.ID, owner.ID, &dto.ListRoundsReq{Limit: 30})
	if page.Total != 2 {
		t.Fatal("delete not reflected")
	}
	if _, e = s.auth.Authenticate(ctx, session); e != nil {
		t.Fatal(e)
	}
	if e = s.auth.Logout(ctx, session); e != nil {
		t.Fatal(e)
	}
	if _, e = s.auth.Authenticate(ctx, session); e == nil {
		t.Fatal("logout did not revoke session")
	}
}

func TestOTPAndInvites(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	if e := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: "new@example.com", Invite: "invalid"},
		"local",
	); e != nil {
		t.Fatal(e)
	}
	if _, _, e := s.auth.Login(
		ctx,
		&dto.LoginReq{Email: "new@example.com", Code: s.inbox.code("new@example.com")},
	); e == nil {
		t.Fatal("registered without trial")
	}
	u, _ := s.signup(t, "valid@example.com")
	if _, _, e := s.auth.Login(
		ctx,
		&dto.LoginReq{Email: "valid@example.com", Code: s.inbox.code("valid@example.com")},
	); e == nil {
		t.Fatal("reused code")
	}
	if e := s.auth.SendCode(ctx, &dto.SendCodeReq{Email: "valid@example.com"}, "local"); e == nil {
		t.Fatal("resend throttle ignored")
	}
	trial := secure.NewID()
	if e := s.repos.Trial().
		Create(ctx, secure.Hash(trial), time.Now().UTC().Add(7*24*time.Hour)); e != nil {
		t.Fatal(e)
	}
	if e := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: "attempts@example.com", Invite: trial},
		"local",
	); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 5; i++ {
		if _, _, e := s.auth.Login(
			ctx,
			&dto.LoginReq{Email: "attempts@example.com", Code: "wrong"},
		); e == nil {
			t.Fatal("wrong code accepted")
		}
	}
	if _, _, e := s.auth.Login(
		ctx,
		&dto.LoginReq{Email: "attempts@example.com", Code: s.inbox.code("attempts@example.com")},
	); e == nil {
		t.Fatal("attempt cap ignored")
	}
	g, e := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "邀请", PlayerName: "发起人"})
	if e != nil {
		t.Fatal(e)
	}
	invite, token, e := s.groups.Invite(ctx, g.ID, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.groups.Manage(
		ctx,
		g.ID,
		u.ID,
		&dto.ManageReq{Action: "revoke", Target: invite.ID},
	); e != nil {
		t.Fatal(e)
	}
	if _, e = s.groups.Join(ctx, u.ID, &dto.JoinReq{Token: token}); e == nil {
		t.Fatal("revoked invite accepted")
	}
	// Expiry is persisted, not a frontend timer.
	s.client.Gorm.Exec(
		"UPDATE omr_challenges SET expires=? WHERE email=?",
		time.Now().UTC().Add(-time.Hour),
		"attempts@example.com",
	)
}

func TestPlayerProfileOnboarding(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "profile-owner@example.com")
	member, _ := s.signup(t, "profile-member@example.com")

	g, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "档案小组", PlayerName: "小林"})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := s.groups.Snapshot(ctx, g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Players) != 1 || snapshot.Players[0].Account == nil ||
		*snapshot.Players[0].Account != owner.ID {
		t.Fatalf("owner profile not created and linked: %#v", snapshot.Players)
	}
	groupsBefore, err := s.groups.List(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Create(
		ctx,
		owner.ID,
		&dto.CreateGroupReq{Name: "应回滚", PlayerName: ""},
	); err == nil {
		t.Fatal("group created without owner player name")
	}
	groupsAfter, err := s.groups.List(ctx, owner.ID)
	if err != nil || len(groupsAfter) != len(groupsBefore) {
		t.Fatalf(
			"invalid owner profile did not roll back group: before=%d after=%d err=%v",
			len(groupsBefore),
			len(groupsAfter),
			err,
		)
	}

	_, token, err := s.groups.Invite(ctx, g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.Join(ctx, member.ID, &dto.JoinReq{Token: token}); err != nil {
		t.Fatal(err)
	}
	if err = s.groups.Manage(
		ctx,
		g.ID,
		member.ID,
		&dto.ManageReq{Action: "claim-new", Value: "小周"},
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.groups.Snapshot(ctx, g.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	var created *dto.Player
	for i := range snapshot.Players {
		if snapshot.Players[i].Name == "小周" {
			created = &snapshot.Players[i]
		}
	}
	if len(snapshot.Claims) != 1 || len(snapshot.Players) != 2 || created == nil ||
		created.Account != nil ||
		snapshot.Claims[0].PlayerID != created.ID {
		t.Fatalf(
			"new member profile and claim not created together: %#v %#v",
			snapshot.Players,
			snapshot.Claims,
		)
	}
	if err = s.groups.Manage(
		ctx,
		g.ID,
		member.ID,
		&dto.ManageReq{Action: "claim-new", Value: "重复"},
	); err != errcode.ErrClaimPending {
		t.Fatalf("duplicate claim error = %v", err)
	}
	if err = s.groups.Manage(
		ctx,
		g.ID,
		owner.ID,
		&dto.ManageReq{Action: "approve", Target: member.ID},
	); err != nil {
		t.Fatal(err)
	}
	snapshot, err = s.groups.Snapshot(ctx, g.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	created = nil
	for i := range snapshot.Players {
		if snapshot.Players[i].Name == "小周" {
			created = &snapshot.Players[i]
		}
	}
	if len(snapshot.Claims) != 0 || created == nil || created.Account == nil ||
		*created.Account != member.ID {
		t.Fatalf("approved member profile not linked: %#v %#v", snapshot.Players, snapshot.Claims)
	}
}

func TestPhotoPermissionsRollbackAndCleanup(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	u, _ := s.signup(t, "photos@example.com")
	other, _ := s.signup(t, "friend@example.com")
	g, e := s.groups.Create(ctx, u.ID, &dto.CreateGroupReq{Name: "照片", PlayerName: "拍照人"})
	if e != nil {
		t.Fatal(e)
	}
	_, inv, e := s.groups.Invite(ctx, g.ID, u.ID)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.groups.Join(ctx, other.ID, &dto.JoinReq{Token: inv}); e != nil {
		t.Fatal(e)
	}
	p, _ := s.groups.AddPlayer(ctx, g.ID, u.ID, &dto.AddPlayerReq{Name: "玩家"})
	game, _ := s.groups.AddGame(ctx, g.ID, u.ID, &dto.AddGameReq{Name: "合作游戏"})
	photo := s.uploadPhoto(t, g.ID, u.ID)
	if e = s.photos.RequireAccess(ctx, g.ID, other.ID, photo); e == nil {
		t.Fatal("another member read unattached photo")
	}
	r := dto.Round{
		GameID:  game.ID,
		Date:    "2026-09-18",
		Mode:    "coop",
		Outcome: "win",
		Players: []string{p.ID},
		Photos:  []string{photo, "missing"},
	}
	if _, e = s.rounds.Save(
		ctx,
		u.ID,
		&dto.SaveRoundReq{Round: r, GroupID: g.ID, IdempotencyKey: secure.NewID()},
	); e == nil {
		t.Fatal("missing photo accepted")
	}
	if e = s.photos.RequireAccess(ctx, g.ID, other.ID, photo); e == nil {
		t.Fatal("transaction leaked photo association")
	}
	r.Photos = []string{photo}
	saved, e := s.rounds.Save(
		ctx,
		u.ID,
		&dto.SaveRoundReq{Round: r, GroupID: g.ID, IdempotencyKey: secure.NewID()},
	)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.photos.RequireAccess(ctx, g.ID, other.ID, photo); e != nil {
		t.Fatal(e)
	}
	content, e := s.photos.Read(ctx, g.ID, other.ID, photo, false)
	if e != nil {
		t.Fatal(e)
	}
	content.Body.Close()
	if e = s.groups.Manage(
		ctx,
		g.ID,
		u.ID,
		&dto.ManageReq{Action: "remove", Target: other.ID},
	); e != nil {
		t.Fatal(e)
	}
	if e = s.photos.RequireAccess(ctx, g.ID, other.ID, photo); e == nil {
		t.Fatal("removed member photo access")
	}
	called := false
	files := countingFiles{inner: &photos.Files{Root: s.photoRoot}, removed: &called}
	// The cutoff is moved forward so the test does not wait for the retention
	// window; a detached upload is then older than the boundary.
	cutoff := time.Now().UTC().Add(time.Hour)
	if e = services.CleanPhotos(ctx, s.repos, files, cutoff); e != nil {
		t.Fatal(e)
	}
	if called {
		t.Fatal("cleaned attached photo")
	}
	if e = s.rounds.Delete(
		ctx,
		u.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: saved.ID, Version: saved.Version},
	); e != nil {
		t.Fatal(e)
	}
	if e = services.CleanPhotos(ctx, s.repos, files, cutoff); e != nil {
		t.Fatal(e)
	}
	if called {
		t.Fatal("cleaned photo while round was recoverable")
	}
	bin, e := s.rounds.RecycleBin(ctx, g.ID, u.ID)
	if e != nil || len(bin) != 1 {
		t.Fatalf("recycle bin = %#v, %v", bin, e)
	}
	restored, e := s.rounds.Restore(
		ctx,
		u.ID,
		&dto.RestoreRoundReq{GroupID: g.ID, RoundID: saved.ID, Version: bin[0].Version},
	)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.rounds.Delete(
		ctx,
		u.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: restored.ID, Version: restored.Version},
	); e != nil {
		t.Fatal(e)
	}
	if e = services.CleanDeletedRounds(ctx, s.repos, cutoff); e != nil {
		t.Fatal(e)
	}
	if e = services.CleanPhotos(ctx, s.repos, files, cutoff); e != nil {
		t.Fatal(e)
	}
	if !called {
		t.Fatal("did not clean photo after recycle retention")
	}
}

func TestRoundCommentsPermissionsAndCleanup(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	owner, _ := s.signup(t, "comment-owner@example.com")
	member, _ := s.signup(t, "comment-member@example.com")
	outsider, _ := s.signup(t, "comment-outsider@example.com")
	g, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "评论小组", PlayerName: "组主"})
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
	player, err := s.groups.AddPlayer(ctx, g.ID, owner.ID, &dto.AddPlayerReq{Name: "朋友"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, g.ID, owner.ID, &dto.AddGameReq{Name: "评论测试"})
	if err != nil {
		t.Fatal(err)
	}
	saved, err := s.rounds.Save(
		ctx,
		owner.ID,
		&dto.SaveRoundReq{
			GroupID:        g.ID,
			IdempotencyKey: secure.NewID(),
			Round: dto.Round{
				GameID:  game.ID,
				Date:    "2026-09-20",
				Mode:    "coop",
				Outcome: "win",
				Players: []string{player.ID},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}

	parentID := "missing"
	if _, err = s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "回复不存在的评论",
			ParentID:       &parentID,
		},
	); !errors.Is(
		err,
		errcode.ErrCommentParent,
	) {
		t.Fatalf("missing parent error = %v", err)
	}
	if _, err = s.comments.Create(
		ctx,
		outsider.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "局外留言",
		},
	); !errors.Is(
		err,
		errcode.ErrForbidden,
	) {
		t.Fatalf("outsider comment error = %v", err)
	}

	key := secure.NewID()
	root, err := s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: key,
			Body:           "今晚这局真好看",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: key,
			Body:           "今晚这局真好看",
		},
	)
	if err != nil || again.ID != root.ID {
		t.Fatalf("idempotent retry = %#v, %v", again, err)
	}
	if _, err = s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: key,
			Body:           "换一句",
		},
	); !errors.Is(
		err,
		errcode.ErrIdempotencyBody,
	) {
		t.Fatalf("idempotent conflict = %v", err)
	}

	reply, err := s.comments.Create(
		ctx,
		owner.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "下一局再来",
			ParentID:       &root.ID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "再套一层",
			ParentID:       &reply.ID,
		},
	); !errors.Is(
		err,
		errcode.ErrCommentParent,
	) {
		t.Fatalf("nested reply error = %v", err)
	}
	if err = s.comments.Delete(
		ctx,
		member.ID,
		&dto.DeleteRoundCommentReq{GroupID: g.ID, RoundID: saved.ID, CommentID: reply.ID},
	); !errors.Is(
		err,
		errcode.ErrForbidden,
	) {
		t.Fatalf("member deleted others = %v", err)
	}

	page, err := s.comments.List(
		ctx,
		owner.ID,
		&dto.ListRoundCommentsReq{GroupID: g.ID, RoundID: saved.ID, Limit: 30},
	)
	if err != nil || page.Total != 2 || len(page.Items) != 2 || page.Items[0].ID != root.ID ||
		page.Items[1].ID != reply.ID {
		t.Fatalf("list = %#v, %v", page, err)
	}
	ownerNotices, err := s.notifications.List(ctx, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNotificationKind(ownerNotices.Items, "round_commented") {
		t.Fatalf("owner missing round comment notice: %#v", ownerNotices.Items)
	}
	memberNotices, err := s.notifications.List(ctx, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !hasNotificationKind(memberNotices.Items, "round_comment_replied") {
		t.Fatalf("member missing reply notice: %#v", memberNotices.Items)
	}

	if err = s.comments.Delete(
		ctx,
		owner.ID,
		&dto.DeleteRoundCommentReq{GroupID: g.ID, RoundID: saved.ID, CommentID: root.ID},
	); err != nil {
		t.Fatal(err)
	}
	page, err = s.comments.List(
		ctx,
		owner.ID,
		&dto.ListRoundCommentsReq{GroupID: g.ID, RoundID: saved.ID, Limit: 30},
	)
	if err != nil || page.Total != 0 {
		t.Fatalf("cascade delete = %#v, %v", page, err)
	}

	kept, err := s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "删局后应保留",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.rounds.Delete(
		ctx,
		owner.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: saved.ID, Version: saved.Version},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = s.comments.Create(
		ctx,
		member.ID,
		&dto.CreateRoundCommentReq{
			GroupID:        g.ID,
			RoundID:        saved.ID,
			IdempotencyKey: secure.NewID(),
			Body:           "回收站里不能评",
		},
	); !errors.Is(
		err,
		errcode.ErrNotFound,
	) {
		t.Fatalf("recycle-bin comment error = %v", err)
	}
	bin, err := s.rounds.RecycleBin(ctx, g.ID, owner.ID)
	if err != nil || len(bin) != 1 {
		t.Fatalf("recycle bin = %#v, %v", bin, err)
	}
	restored, err := s.rounds.Restore(
		ctx,
		owner.ID,
		&dto.RestoreRoundReq{GroupID: g.ID, RoundID: saved.ID, Version: bin[0].Version},
	)
	if err != nil {
		t.Fatal(err)
	}
	page, err = s.comments.List(
		ctx,
		owner.ID,
		&dto.ListRoundCommentsReq{GroupID: g.ID, RoundID: restored.ID, Limit: 30},
	)
	if err != nil || page.Total != 1 || page.Items[0].ID != kept.ID {
		t.Fatalf("restored comments = %#v, %v", page, err)
	}
	if err = s.rounds.Delete(
		ctx,
		owner.ID,
		&dto.DeleteRoundReq{GroupID: g.ID, RoundID: restored.ID, Version: restored.Version},
	); err != nil {
		t.Fatal(err)
	}
	if err = services.CleanDeletedRounds(
		ctx,
		s.repos,
		time.Now().UTC().Add(time.Hour),
	); err != nil {
		t.Fatal(err)
	}
	leftover, total, err := s.repos.Comment().ListByRound(ctx, g.ID, restored.ID, 0, 30)
	if err != nil || total != 0 || len(leftover) != 0 {
		t.Fatalf("permanent delete left comments = %#v total=%d err=%v", leftover, total, err)
	}
}

func hasNotificationKind(items []dto.Notification, kind string) bool {
	for _, item := range items {
		if item.Kind == kind {
			return true
		}
	}
	return false
}

// countingFiles records whether a file removal happened during a cleanup pass.
type countingFiles struct {
	inner   *photos.Files
	removed *bool
}

func (f countingFiles) Save(ctx context.Context, id string, r io.Reader) error {
	return f.inner.Save(ctx, id, r)
}

func (f countingFiles) Read(
	ctx context.Context,
	id string,
	thumb bool,
) (*ports.PhotoContent, error) {
	return f.inner.Read(ctx, id, thumb)
}

func (f countingFiles) Remove(ctx context.Context, id string) error {
	*f.removed = true
	return f.inner.Remove(ctx, id)
}

func TestHTTPAuthenticationAndCSRF(t *testing.T) {
	s := setup(t)
	u, session := s.signup(t, "api@example.com")
	handler := api.NewAPI("test", &config.Runtime{Origin: testOrigin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
		SetupRouter()
	call := func(method, path, origin, cookie string, payload any) *httptest.ResponseRecorder {
		t.Helper()
		data, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, path, bytes.NewReader(data))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if cookie != "" {
			req.AddCookie(&http.Cookie{Name: "omr_session", Value: cookie})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}
	if w := call("GET", "/v1/me", "", "", nil); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := call("GET", "/v1/me", "", session, nil); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := call(
		"POST",
		"/v1/groups",
		"https://evil.example",
		session,
		map[string]string{"name": "bad"},
	); w.Code != 403 {
		t.Fatal("CSRF accepted")
	}
	w := call(
		"POST",
		"/v1/groups",
		testOrigin,
		session,
		map[string]string{"name": "API 小组", "player_name": "接口用户"},
	)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	groups, e := s.groups.List(context.Background(), u.ID)
	if e != nil || len(groups) != 1 {
		t.Fatal("group not persisted")
	}
	if w = call("GET", "/v1/groups/"+groups[0].ID+"/bgg/search", "", session, nil); w.Code != 503 {
		t.Fatal("missing BGG not reported")
	}
	if w = call(
		"GET",
		"/v1/groups/"+groups[0].ID+"/rounds?limit=1000",
		"",
		session,
		nil,
	); w.Code != 400 {
		t.Fatal("unbounded page accepted")
	}
	if w = call(
		"POST",
		"/v1/auth/logout",
		testOrigin,
		session,
		map[string]string{},
	); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = call("GET", "/v1/me", "", session, nil); w.Code != 401 {
		t.Fatal("logged-out HTTP session accepted")
	}
}
