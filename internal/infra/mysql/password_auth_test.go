package mysql_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/miebyte/authkit"
	"github.com/miebyte/goutils/mysqlutils"
	"gorm.io/gorm"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/infra/mysql"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
	"github.com/superwhys/one-more-round/web"
)

const passwordTestSecret = "a-test7 "

// passwordInbox captures only fixture mail; the tests never use an SMTP server.
type passwordInbox struct {
	mu    sync.Mutex
	codes map[string]string
}

// SendCode stores the verification proof for the isolated browser fixture.
func (m *passwordInbox) SendCode(_ context.Context, email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[email] = code
	return nil
}

// code returns the most recently delivered fixture proof without racing requests.
func (m *passwordInbox) code(email string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.codes[email]
}

// passwordFixture owns an independently created database and the real services.
type passwordFixture struct {
	db     *gorm.DB
	repos  *mysql.RepositoryFactory
	ctx    *services.AppContext
	auth   *services.AuthApp
	groups *services.GroupApp
	inbox  *passwordInbox
}

// newPasswordFixture creates and later removes a random test database only.
func newPasswordFixture(t *testing.T) *passwordFixture {
	t.Helper()
	instance := os.Getenv("OMR_TEST_MYSQL")
	if instance == "" {
		t.Skip("OMR_TEST_MYSQL is unset; real MySQL password integration was not run")
	}
	admin, err := sql.Open("mysql", "root@tcp("+instance+")/?parseTime=true")
	if err != nil {
		t.Fatal(err)
	}
	database := "omr_password_" + secure.NewID()[:24]
	if _, err = admin.Exec(
		"CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin",
	); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE `" + database + "`"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	client, err := mysql.Open(
		mysqlutils.MysqlConfig{
			Instance: instance,
			Database: database,
			Username: "root",
			Charset:  "utf8mb4",
			PoolSize: 8,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	repos := mysql.NewRepositoryFactory(client.Gorm)
	inbox := &passwordInbox{codes: map[string]string{}}
	appCtx := &services.AppContext{Repos: repos, Mailer: inbox}
	return &passwordFixture{
		db:     client.Gorm,
		repos:  repos,
		ctx:    appCtx,
		auth:   passwordAuthApp(t, appCtx),
		groups: services.NewGroupApp(appCtx),
		inbox:  inbox,
	}
}

// passwordAuthApp assembles authkit with the host's existing repository boundary.
func passwordAuthApp(t *testing.T, appCtx *services.AppContext) *services.AuthApp {
	t.Helper()
	service, err := services.NewAuthkitService(appCtx)
	if err != nil {
		t.Fatal(err)
	}
	return services.NewAuthApp(appCtx, service)
}

// trial creates one-use admission in the same test database as the auth store.
func (f *passwordFixture) trial(t *testing.T, token string, expires time.Time) string {
	t.Helper()
	if err := f.repos.Trial().
		Create(context.Background(), secure.Hash(token), expires); err != nil {
		t.Fatal(err)
	}
	return token
}

// register creates a password account and verifies a usable opaque session.
func (f *passwordFixture) register(
	t *testing.T,
	username, invite, groupToken string,
) *dto.LoginResp {
	t.Helper()
	result, token, err := f.auth.RegisterPassword(
		context.Background(),
		&dto.PasswordRegisterReq{
			Username:   username,
			Identifier: username,
			Password:   passwordTestSecret,
			Invite:     invite,
			GroupToken: groupToken,
		},
	)
	if err != nil || token == "" || result == nil {
		t.Fatalf(
			"password registration failed: result=%v token-present=%v error=%v",
			result,
			token != "",
			err,
		)
	}
	account, err := f.auth.Authenticate(context.Background(), token)
	if err != nil || account.ID != result.ID || account.Username != username {
		t.Fatalf("registered session: account=%v error=%v", account, err)
	}
	return result
}

// count checks committed rows directly so rollback assertions cover all stores.
func (f *passwordFixture) count(t *testing.T, table string) int64 {
	t.Helper()
	var count int64
	if err := f.db.Table(table).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// TestPasswordRegistrationAdmission verifies invitations, conflicts and login independence.
func TestPasswordRegistrationAdmission(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	for i, req := range []*dto.PasswordRegisterReq{
		{Username: "no-invite", Identifier: "no-invite", Password: passwordTestSecret},
		{Username: "bad-invite", Identifier: "bad-invite", Password: passwordTestSecret, Invite: "missing"},
		{Username: "expired-invite", Identifier: "expired-invite", Password: passwordTestSecret, Invite: f.trial(t, "expired", time.Now().Add(-time.Hour))},
	} {
		if _, _, err := f.auth.RegisterPassword(
			ctx,
			req,
		); !errors.Is(
			err,
			errcode.ErrTrialInvalid,
		) {
			t.Fatalf("invalid admission %d: %v", i, err)
		}
	}
	if f.count(t, "auth_accounts") != 0 || f.count(t, "auth_sessions") != 0 {
		t.Fatal("rejected admission created an account or session")
	}
	invite := f.trial(t, "valid", time.Now().Add(time.Hour))
	user := f.register(t, "桌游玩家", invite, "")
	if user.Email != "" || user.GroupID != "" {
		t.Fatalf("password account gained unintended email/group: %+v", user)
	}
	if _, _, err := f.auth.RegisterPassword(
		ctx,
		&dto.PasswordRegisterReq{
			Username:   "another-player",
			Identifier: "another-player",
			Password:   passwordTestSecret,
			Invite:     invite,
		},
	); !errors.Is(
		err,
		errcode.ErrTrialInvalid,
	) {
		t.Fatalf("one-use trial was reused: %v", err)
	}
	second := f.trial(t, "duplicate-trial", time.Now().Add(time.Hour))
	if _, _, err := f.auth.RegisterPassword(
		ctx,
		&dto.PasswordRegisterReq{
			Username:   "桌游玩家",
			Identifier: "桌游玩家",
			Password:   passwordTestSecret,
			Invite:     second,
		},
	); !errors.Is(
		err,
		errcode.ErrConflict,
	) {
		t.Fatalf("duplicate password identifier: %v", err)
	}
	// Conflict cannot consume admission intended for a corrected login identifier.
	f.register(t, "new-player", second, "")
	result, token, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "桌游玩家", Password: passwordTestSecret},
		"existing",
	)
	if err != nil || token == "" || result.ID != user.ID {
		t.Fatalf("existing login without invite: result=%v error=%v", result, err)
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "桌游玩家", Password: strings.TrimSpace(passwordTestSecret)},
		"trimmed",
	); !errors.Is(
		err,
		errcode.ErrUnauthorized,
	) {
		t.Fatalf("password whitespace was not significant: %v", err)
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "missing-player", Password: passwordTestSecret},
		"unknown",
	); !errors.Is(
		err,
		errcode.ErrUnauthorized,
	) {
		t.Fatalf("unknown password identifier: %v", err)
	}
}

// TestPasswordRegistrationDisplayName keeps display names separate from login bindings.
func TestPasswordRegistrationDisplayName(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	for _, req := range []*dto.PasswordRegisterReq{
		{Identifier: "display-first", Username: "shared-display"},
		{Identifier: "display-second", Username: "shared-display"},
		{Identifier: "empty-display"},
		{Identifier: "display-with-space", Username: "展示名称 @团队"},
		{Identifier: " Password@Example.TEST ", Username: "邮箱登录玩家"},
	} {
		req.Password = passwordTestSecret
		req.Invite = f.trial(t, secure.NewID(), time.Now().Add(time.Hour))
		user, token, err := f.auth.RegisterPassword(ctx, req)
		if err != nil || user.Username != req.Username || user.Email != "" {
			t.Fatalf(
				"registration changed display name or created an email binding: user=%v error=%v",
				user,
				err,
			)
		}
		account, err := f.auth.Authenticate(ctx, token)
		if err != nil || account.Username != req.Username {
			t.Fatalf("display name was not preserved: account=%v error=%v", account, err)
		}
		identifier, err := authkit.NormalizeUsername(req.Identifier)
		if err != nil {
			t.Fatal(err)
		}
		bound, err := f.repos.Auth().Accounts().GetPasswordIdentifier(ctx, user.ID)
		if err != nil || bound != identifier {
			t.Fatalf("normalized password binding=%q error=%v", bound, err)
		}
		login, _, err := f.auth.LoginPassword(
			ctx,
			&dto.PasswordLoginReq{Identifier: req.Identifier, Password: passwordTestSecret},
			identifier,
		)
		if err != nil || login.ID != user.ID || login.Username != req.Username {
			t.Fatalf("login by independent identifier: user=%v error=%v", login, err)
		}
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "shared-display", Password: passwordTestSecret},
		"display-only",
	); !errors.Is(
		err,
		errcode.ErrInvalidCredentials,
	) {
		t.Fatalf("display name was accepted as a login binding: %v", err)
	}
}

// TestPasswordInvalidInput covers authkit's Unicode bounds without consuming admission.
func TestPasswordInvalidInput(t *testing.T) {
	f := newPasswordFixture(t)
	invite := f.trial(t, "invalid-input-trial", time.Now().Add(time.Hour))
	cases := []struct{ identifier, password string }{
		{"", passwordTestSecret},
		{"ab", passwordTestSecret},
		{strings.Repeat("字", 65), passwordTestSecret},
		{"has space", passwordTestSecret},
		{"name@", passwordTestSecret},
		{"has\ncontrol", passwordTestSecret},
		{"valid-player", strings.Repeat("密", 7)},
		{"valid-player", strings.Repeat("密", 129)},
	}
	for i, c := range cases {
		if _, _, err := f.auth.RegisterPassword(
			context.Background(),
			&dto.PasswordRegisterReq{
				Identifier: c.identifier,
				Password:   c.password,
				Invite:     invite,
			},
		); !errors.Is(
			err,
			errcode.ErrBadRequest,
		) {
			t.Fatalf("invalid input %d: %v", i, err)
		}
	}
	for _, display := range []string{strings.Repeat("名", 65), "display\nname"} {
		if _, _, err := f.auth.RegisterPassword(
			context.Background(),
			&dto.PasswordRegisterReq{
				Identifier: "valid-player",
				Username:   display,
				Password:   passwordTestSecret,
				Invite:     invite,
			},
		); !errors.Is(
			err,
			errcode.ErrBadRequest,
		) {
			t.Fatalf("invalid display name accepted: %v", err)
		}
	}
	f.register(t, "valid-player", invite, "")
}

// TestPasswordMinimumLength accepts eight Unicode characters for registration and login.
func TestPasswordMinimumLength(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	for _, c := range []struct{ name, password string }{
		{"ascii", passwordTestSecret},
		{"unicode", strings.Repeat("密", 8)},
	} {
		t.Run(c.name, func(t *testing.T) {
			username := "minimum-" + c.name
			invite := f.trial(t, username, time.Now().Add(time.Hour))
			user, _, err := f.auth.RegisterPassword(
				ctx,
				&dto.PasswordRegisterReq{
					Username:   username,
					Identifier: username,
					Password:   c.password,
					Invite:     invite,
				},
			)
			if err != nil {
				t.Fatalf("eight-character password registration failed: %v", err)
			}
			result, token, err := f.auth.LoginPassword(
				ctx,
				&dto.PasswordLoginReq{Identifier: username, Password: c.password},
				username,
			)
			if err != nil || token == "" || result == nil || result.ID != user.ID {
				t.Fatalf("eight-character password login failed: result=%v error=%v", result, err)
			}
		})
	}
}

// passwordRollback forces failure after the complete host transaction callback.
type passwordRollback struct{ ports.Repositories }

// WithTransaction makes account, session, admission and joining rollback together.
func (r passwordRollback) WithTransaction(
	ctx context.Context,
	fn func(ports.Repositories) error,
) error {
	return r.Repositories.WithTransaction(ctx, func(tx ports.Repositories) error {
		if err := fn(tx); err != nil {
			return err
		}
		return errors.New("fixture forces registration transaction rollback")
	})
}

// invitedGroup creates the group invitation through real application services.
func (f *passwordFixture) invitedGroup(t *testing.T) (dto.Group, string) {
	t.Helper()
	owner := f.register(t, "group-owner", f.trial(t, "owner-trial", time.Now().Add(time.Hour)), "")
	group, err := f.groups.Create(
		context.Background(),
		owner.ID,
		&dto.CreateGroupReq{Name: "密码测试小组", PlayerName: "组主"},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, token, err := f.groups.Invite(context.Background(), group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	return group, token
}

// TestPasswordRegistrationRollback checks that registration never leaves partial host data.
func TestPasswordRegistrationRollback(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	group, groupToken := f.invitedGroup(t)
	invite := f.trial(t, "rollback-trial", time.Now().Add(time.Hour))
	appCtx := &services.AppContext{Repos: passwordRollback{f.repos}, Mailer: f.inbox}
	failing := passwordAuthApp(t, appCtx)
	for _, req := range []*dto.PasswordRegisterReq{
		{Username: "rollback-trial-user", Identifier: "rollback-trial-user", Password: passwordTestSecret, Invite: invite},
		{Username: "rollback-group-user", Identifier: "rollback-group-user", Password: passwordTestSecret, GroupToken: groupToken},
	} {
		before := map[string]int64{}
		for _, table := range []string{"auth_accounts", "auth_bindings", "auth_sessions", "omr_members", "omr_notifications"} {
			before[table] = f.count(t, table)
		}
		if _, _, err := failing.RegisterPassword(ctx, req); err == nil {
			t.Fatal("expected transaction failure")
		}
		for table, count := range before {
			if f.count(t, table) != count {
				t.Fatalf("failed registration left committed rows in %s", table)
			}
		}
	}
	f.register(t, "trial-retry", invite, "")
	joined := f.register(t, "group-retry", "", groupToken)
	if joined.GroupID != group.ID || f.count(t, "omr_members") != 2 ||
		f.count(t, "omr_notifications") != 1 {
		t.Fatalf("atomic group registration: result=%+v", joined)
	}
	snapshot, err := f.groups.Snapshot(ctx, group.ID, joined.ID)
	if err != nil {
		t.Fatal(err)
	}
	var memberUsername string
	for _, member := range snapshot.Members {
		if member.UserID == joined.ID {
			memberUsername = member.Username
		}
	}
	if memberUsername != "group-retry" {
		t.Fatalf("password member's username missing in snapshot: %q", memberUsername)
	}
}

// server exposes the real API and embedded production frontend on one origin.
func (f *passwordFixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	origin := "http://" + listener.Addr().String()
	backend := api.NewAPI(
		&config.Runtime{Origin: origin},
		f.auth,
		f.groups,
		services.NewRoundApp(f.ctx),
		services.NewPhotoApp(f.ctx),
		services.NewNotificationApp(f.ctx),
		services.NewCommentApp(f.ctx),
	)
	frontend, err := web.NewHandler()
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", backend.SetupRouter()))
	mux.HandleFunc("/__test__/code", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).
			Encode(map[string]string{"code": f.inbox.code(r.URL.Query().Get("email"))})
	})
	mux.Handle("/", frontend)
	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

// passwordRequest invokes the same Origin and Cookie protocol as the browser.
func passwordRequest(
	t *testing.T,
	server *httptest.Server,
	method, path string,
	payload any,
	cookie *http.Cookie,
) (*http.Response, map[string]any) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", server.URL)
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
	}
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return response, body
}

// TestPasswordSessionHTTP verifies error statuses, session cookie, /me and revocation.
func TestPasswordSessionHTTP(t *testing.T) {
	f := newPasswordFixture(t)
	server := f.server(t)
	invite := f.trial(t, "http-trial", time.Now().Add(time.Hour))
	response, _ := passwordRequest(
		t,
		server,
		"POST",
		"/api/v1/auth/password/register",
		map[string]string{
			"username": "http-player",
			"password": passwordTestSecret,
			"invite":   invite,
		},
		nil,
	)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("display name was accepted without an identifier: status=%d", response.StatusCode)
	}
	response, body := passwordRequest(
		t,
		server,
		"POST",
		"/api/v1/auth/password/register",
		map[string]string{
			"username":   "http-player",
			"identifier": "http-player",
			"password":   passwordTestSecret,
			"invite":     invite,
		},
		nil,
	)
	if response.StatusCode != http.StatusOK ||
		body["data"].(map[string]any)["username"] != "http-player" {
		t.Fatalf("register response: status=%d body=%v", response.StatusCode, body)
	}
	var cookie *http.Cookie
	for _, c := range response.Cookies() {
		if c.Name == common.SessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode ||
		cookie.Path != "/" ||
		cookie.Value == "" {
		t.Fatal("registration did not set a private Strict session cookie")
	}
	response, body = passwordRequest(t, server, "GET", "/api/v1/me", nil, cookie)
	if response.StatusCode != 200 || body["data"].(map[string]any)["username"] != "http-player" ||
		body["data"].(map[string]any)["password_identifier"] != "http-player" {
		t.Fatalf("/me: status=%d body=%v", response.StatusCode, body)
	}
	response, _ = passwordRequest(t, server, "POST", "/api/v1/auth/logout", nil, cookie)
	if response.StatusCode != 200 || len(response.Cookies()) != 1 ||
		response.Cookies()[0].MaxAge != -1 {
		t.Fatal("logout did not clear session cookie")
	}
	response, _ = passwordRequest(t, server, "GET", "/api/v1/me", nil, cookie)
	if response.StatusCode != 401 {
		t.Fatal("logged-out session still authenticates")
	}
	for _, rejected := range []struct {
		password string
		status   int
	}{{"a-wrong-password-value", 401}, {"", 400}} {
		response, _ = passwordRequest(
			t,
			server,
			"POST",
			"/api/v1/auth/password/login",
			map[string]string{"identifier": "http-player", "password": rejected.password},
			nil,
		)
		if response.StatusCode != rejected.status {
			t.Fatalf("invalid password login status=%d", response.StatusCode)
		}
	}
	response, _ = passwordRequest(
		t,
		server,
		"POST",
		"/api/v1/auth/password/login",
		map[string]string{"identifier": "http-player", "password": passwordTestSecret},
		nil,
	)
	if response.StatusCode != 200 || len(response.Cookies()) != 1 {
		t.Fatal("existing password account could not log in without an invitation")
	}
}

// TestPasswordLoginRateCommit proves rejection commits rate counters across host transactions.
func TestPasswordLoginRateCommit(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	user, _, err := f.auth.RegisterPassword(ctx, &dto.PasswordRegisterReq{
		Identifier: "rate-player", Username: "rate-display", Password: passwordTestSecret,
		Invite: f.trial(t, "rate-trial", time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	email := "rate-player@example.test"
	if err := f.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		return repos.Credential().BindEmail(ctx, user.ID, email)
	}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < authkit.PasswordAccountRateLimit; i++ {
		if _, _, err := f.auth.LoginPassword(
			ctx,
			&dto.PasswordLoginReq{Identifier: email, Password: passwordTestSecret},
			"rate-ip",
		); !errors.Is(
			err,
			errcode.ErrInvalidCredentials,
		) {
			t.Fatalf("unbound password email login=%v", err)
		}
	}
	for i := 0; i < authkit.PasswordAccountRateLimit; i++ {
		_, _, err := f.auth.LoginPassword(
			ctx,
			&dto.PasswordLoginReq{Identifier: "rate-player", Password: "a-wrong-password-value"},
			"rate-ip",
		)
		if !errors.Is(err, errcode.ErrUnauthorized) {
			t.Fatalf("password binding attempt %d response=%v", i, err)
		}
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "rate-player", Password: passwordTestSecret},
		"rate-ip",
	); !errors.Is(
		err,
		errcode.ErrTooManyRequests,
	) {
		t.Fatalf("rejected attempts did not limit the password binding: %v", err)
	}
}

// emailAccount creates an existing email identity through real code verification.
func (f *passwordFixture) emailAccount(
	t *testing.T,
	email, invite string,
) (*dto.LoginResp, string) {
	t.Helper()
	ctx := context.Background()
	if err := f.auth.SendCode(ctx, &dto.SendCodeReq{Email: email}, email); err != nil {
		t.Fatal(err)
	}
	user, token, err := f.auth.Login(
		ctx,
		&dto.LoginReq{Email: email, Code: f.inbox.code(email), Invite: invite},
	)
	if err != nil {
		t.Fatal(err)
	}
	return user, token
}

// TestPasswordSetExistingEmail preserves identity and atomically rotates all sessions.
func TestPasswordSetExistingEmail(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	email := "existing-player@example.test"
	user, oldToken := f.emailAccount(
		t,
		email,
		f.trial(t, "email-account-trial", time.Now().Add(time.Hour)),
	)
	group, err := f.groups.Create(
		ctx,
		user.ID,
		&dto.CreateGroupReq{Name: "原来的小组", PlayerName: "原玩家"},
	)
	if err != nil {
		t.Fatal(err)
	}
	game, err := f.groups.AddGame(ctx, group.ID, user.ID, &dto.AddGameReq{Name: "原来的游戏"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := f.groups.Snapshot(ctx, group.ID, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	rounds := services.NewRoundApp(f.ctx)
	history, err := rounds.Save(ctx, user.ID, &dto.SaveRoundReq{
		GroupID:        group.ID,
		IdempotencyKey: secure.NewID(),
		Round: dto.Round{
			GameID:  game.ID,
			Date:    "2026-09-01",
			Mode:    "coop",
			Outcome: "win",
			Players: []string{before.Players[0].ID},
			Memory:  "保留在原账号的对局",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherToken := secure.NewID()
	if err := f.repos.Auth().
		Sessions().
		Create(ctx, &authkit.Session{Hash: secure.Hash(otherToken), AccountID: user.ID, Expires: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	f.register(t, "taken-player", f.trial(t, "taken-trial", time.Now().Add(time.Hour)), "")
	for _, req := range []*dto.SetPasswordReq{
		{Password: passwordTestSecret},
		{Identifier: "taken-player", Password: passwordTestSecret},
		{Identifier: "email-player", Password: strings.Repeat("密", 7)},
	} {
		if _, _, err := f.auth.SetPassword(ctx, user.ID, oldToken, req); err == nil {
			t.Fatal("invalid password setup succeeded")
		}
		for _, token := range []string{oldToken, otherToken} {
			if _, err := f.auth.Authenticate(ctx, token); err != nil {
				t.Fatal("failed password setup revoked a valid session")
			}
		}
	}
	newUser, currentToken, err := f.auth.SetPassword(
		ctx,
		user.ID,
		oldToken,
		&dto.SetPasswordReq{Identifier: "email-player", Password: passwordTestSecret},
	)
	if err != nil || newUser.ID != user.ID || newUser.Email != email ||
		newUser.Username != user.Username ||
		newUser.PasswordIdentifier != "email-player" {
		t.Fatalf("existing email identity changed: user=%v error=%v", newUser, err)
	}
	for _, token := range []string{oldToken, otherToken} {
		if _, err := f.auth.Authenticate(ctx, token); !errors.Is(err, errcode.ErrUnauthorized) {
			t.Fatalf("previous session was not revoked: %v", err)
		}
	}
	if _, err := f.auth.Authenticate(ctx, currentToken); err != nil {
		t.Fatalf("replacement session unusable: %v", err)
	}
	snapshot, err := f.groups.Snapshot(ctx, group.ID, user.ID)
	if err != nil || snapshot.Group.Owner != user.ID || len(snapshot.Players) != 1 ||
		len(snapshot.Games) != 1 ||
		snapshot.Games[0].ID != game.ID {
		t.Fatalf("password setup changed existing group data: snapshot=%+v error=%v", snapshot, err)
	}
	preserved, err := rounds.Get(ctx, group.ID, user.ID, history.ID)
	if err != nil || preserved.Author != user.ID || preserved.GameID != game.ID ||
		preserved.Memory != history.Memory ||
		preserved.Version != history.Version {
		t.Fatalf("password setup changed historical round: round=%+v error=%v", preserved, err)
	}
	result, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "email-player", Password: passwordTestSecret},
		"email-player",
	)
	if err != nil || result.ID != user.ID || result.Email != email {
		t.Fatalf("password login by binding: result=%v error=%v", result, err)
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: email, Password: passwordTestSecret},
		"email-alias",
	); !errors.Is(
		err,
		errcode.ErrInvalidCredentials,
	) {
		t.Fatalf("verified email was borrowed as a password alias: %v", err)
	}
	if _, _, err := f.auth.SetPassword(
		ctx,
		user.ID,
		currentToken,
		&dto.SetPasswordReq{Identifier: "different-player", Password: passwordTestSecret},
	); !errors.Is(
		err,
		errcode.ErrConflict,
	) {
		t.Fatalf("existing password identifier changed unexpectedly: %v", err)
	}
	if _, err := f.auth.Authenticate(ctx, currentToken); err != nil {
		t.Fatal("conflicting password identifier change revoked current session")
	}
	rollbackCtx := &services.AppContext{Repos: passwordRollback{f.repos}, Mailer: f.inbox}
	if _, _, err := passwordAuthApp(
		t,
		rollbackCtx,
	).SetPassword(ctx, user.ID, currentToken, &dto.SetPasswordReq{Password: "another-password-for-tests"}); err == nil {
		t.Fatal("expected password setup transaction rollback")
	}
	if _, err := f.auth.Authenticate(ctx, currentToken); err != nil {
		t.Fatal("failed password setup transaction revoked current session")
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "email-player", Password: passwordTestSecret},
		"after-rollback",
	); err != nil {
		t.Fatalf("failed password setup changed the credential: %v", err)
	}
	server := f.server(t)
	response, _ := passwordRequest(
		t,
		server,
		"POST",
		"/api/v1/auth/password/set",
		map[string]string{"identifier": "email-player", "password": passwordTestSecret},
		nil,
	)
	if response.StatusCode != 401 {
		t.Fatalf("unauthenticated password setup status=%d", response.StatusCode)
	}
	cookie := &http.Cookie{Name: common.SessionCookie, Value: currentToken}
	response, body := passwordRequest(
		t,
		server,
		"POST",
		"/api/v1/auth/password/set",
		map[string]string{"password": passwordTestSecret},
		cookie,
	)
	if response.StatusCode != 200 || body["data"].(map[string]any)["id"] != user.ID ||
		body["data"].(map[string]any)["password_identifier"] != "email-player" ||
		len(response.Cookies()) != 1 {
		t.Fatalf("password setup HTTP response: status=%d body=%v", response.StatusCode, body)
	}
	response, body = passwordRequest(t, server, "GET", "/api/v1/me", nil, response.Cookies()[0])
	if response.StatusCode != 200 ||
		body["data"].(map[string]any)["password_identifier"] != "email-player" {
		t.Fatal("password setup did not preserve the browser's active session")
	}
}

// TestPasswordSetPreservesDisplayName resets a binding without treating display as a credential.
func TestPasswordSetPreservesDisplayName(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	user, oldToken, err := f.auth.RegisterPassword(ctx, &dto.PasswordRegisterReq{
		Identifier: "reset-player", Username: "different-display", Password: passwordTestSecret,
		Invite: f.trial(t, "reset-display-trial", time.Now().Add(time.Hour)),
	})
	if err != nil {
		t.Fatal(err)
	}
	newPassword := "replacement-password"
	updated, token, err := f.auth.SetPassword(
		ctx,
		user.ID,
		oldToken,
		&dto.SetPasswordReq{Password: newPassword},
	)
	if err != nil || updated.Username != "different-display" ||
		updated.PasswordIdentifier != "reset-player" {
		t.Fatalf("password reset changed display name or binding: user=%v error=%v", updated, err)
	}
	if _, err := f.auth.Authenticate(ctx, oldToken); !errors.Is(err, errcode.ErrUnauthorized) {
		t.Fatalf("password reset did not revoke the previous session: %v", err)
	}
	if _, err := f.auth.Authenticate(ctx, token); err != nil {
		t.Fatalf("replacement session unusable: %v", err)
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "reset-player", Password: passwordTestSecret},
		"reset-old-password",
	); !errors.Is(
		err,
		errcode.ErrInvalidCredentials,
	) {
		t.Fatalf("password reset retained the old password: %v", err)
	}
	login, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "reset-player", Password: newPassword},
		"reset-new-password",
	)
	if err != nil || login.ID != user.ID || login.Username != "different-display" {
		t.Fatalf("reset password binding unusable: user=%v error=%v", login, err)
	}
	if _, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: "different-display", Password: newPassword},
		"reset-display",
	); !errors.Is(
		err,
		errcode.ErrInvalidCredentials,
	) {
		t.Fatalf("password reset bound the display name: %v", err)
	}
}

// passwordAccountPause holds the setter's first account lock until the email flow starts.
type passwordAccountPause struct {
	locked  chan struct{}
	release chan struct{}
	once    sync.Once
}

// passwordPausedRepositories preserves the real transaction while wrapping its auth store.
type passwordPausedRepositories struct {
	ports.Repositories
	pause *passwordAccountPause
}

// WithTransaction propagates the pause to repositories bound to the real transaction.
func (r passwordPausedRepositories) WithTransaction(
	ctx context.Context,
	fn func(ports.Repositories) error,
) error {
	return r.Repositories.WithTransaction(ctx, func(repos ports.Repositories) error {
		return fn(passwordPausedRepositories{Repositories: repos, pause: r.pause})
	})
}

// Auth wraps only the setter's blacklist operations without changing persistence.
func (r passwordPausedRepositories) Auth() authkit.Store {
	return passwordPausedStore{Store: r.Repositories.Auth(), pause: r.pause}
}

// passwordPausedStore forwards all operations except the account-lock pause.
type passwordPausedStore struct {
	authkit.Store
	pause *passwordAccountPause
}

// Blacklist exposes the real blacklist repository with one controlled pause.
func (s passwordPausedStore) Blacklist() authkit.BlacklistRepository {
	return passwordPausedBlacklist{BlacklistRepository: s.Store.Blacklist(), pause: s.pause}
}

// passwordPausedBlacklist pauses after taking the real account lock once.
type passwordPausedBlacklist struct {
	authkit.BlacklistRepository
	pause *passwordAccountPause
}

// CheckAccount holds the acquired account lock until the competing email flow can contend.
func (r passwordPausedBlacklist) CheckAccount(ctx context.Context, accountID string) error {
	if err := r.BlacklistRepository.CheckAccount(ctx, accountID); err != nil {
		return err
	}
	var err error
	r.pause.once.Do(func() {
		close(r.pause.locked)
		select {
		case <-r.pause.release:
		case <-ctx.Done():
			err = ctx.Err()
		}
	})
	return err
}

// TestPasswordEmailLockOrder prevents email authentication and password setup from deadlocking.
func TestPasswordEmailLockOrder(t *testing.T) {
	f := newPasswordFixture(t)
	email := "concurrent-password@example.test"
	user, token := f.emailAccount(
		t,
		email,
		f.trial(t, "email-lock-trial", time.Now().Add(time.Hour)),
	)
	pause := &passwordAccountPause{locked: make(chan struct{}), release: make(chan struct{})}
	appCtx := &services.AppContext{
		Repos:  passwordPausedRepositories{Repositories: f.repos, pause: pause},
		Mailer: f.inbox,
	}
	setter := passwordAuthApp(t, appCtx)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var release sync.Once
	releaseSetter := func() { release.Do(func() { close(pause.release) }) }
	defer releaseSetter()
	setResult := make(chan error, 1)
	go func() {
		_, _, err := setter.SetPassword(
			ctx,
			user.ID,
			token,
			&dto.SetPasswordReq{Identifier: email, Password: passwordTestSecret},
		)
		setResult <- err
	}()
	select {
	case <-pause.locked:
	case <-ctx.Done():
		t.Fatal("setter did not acquire its account lock before the timeout")
	}
	emailStarted := make(chan struct{})
	emailLocked := make(chan struct{})
	emailResult := make(chan error, 1)
	go func() {
		emailResult <- f.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
			blacklist := repos.Auth().Blacklist()
			close(emailStarted)
			// Email authentication locks its credential before the account.
			if err := blacklist.Check(ctx, authkit.Credential{Method: authkit.MethodEmail, Identifier: email}); err != nil {
				return err
			}
			close(emailLocked)
			return blacklist.CheckAccount(ctx, user.ID)
		})
	}()
	select {
	case <-emailStarted:
	case <-ctx.Done():
		t.Fatal("email transaction did not start before the timeout")
	}
	// A correct setter already owns the email credential, so this flow waits;
	// the old order lets it own the credential while waiting for the account.
	timer := time.NewTimer(500 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-emailLocked:
	case <-timer.C:
	case <-ctx.Done():
		t.Fatal("email lock coordination timed out")
	}
	releaseSetter()
	var setErr, emailErr error
	select {
	case setErr = <-setResult:
	case <-ctx.Done():
		t.Fatal("password setup did not finish before the timeout")
	}
	select {
	case emailErr = <-emailResult:
	case <-ctx.Done():
		t.Fatal("email authentication lock flow did not finish before the timeout")
	}
	if setErr != nil || emailErr != nil {
		t.Fatalf(
			"concurrent password/email lock order failed: setter=%v email=%v",
			setErr,
			emailErr,
		)
	}
	identifier, err := f.repos.Auth().
		Accounts().
		GetPasswordIdentifier(context.Background(), user.ID)
	if err != nil || identifier != email {
		t.Fatalf(
			"successful password setup did not preserve its binding: identifier=%q error=%v",
			identifier,
			err,
		)
	}
}

// TestPasswordEmailIdentifier remains usable independently of the verified email binding.
func TestPasswordEmailIdentifier(t *testing.T) {
	f := newPasswordFixture(t)
	ctx := context.Background()
	email := "password-email@example.test"
	user, oldToken := f.emailAccount(
		t,
		email,
		f.trial(t, "password-email-trial", time.Now().Add(time.Hour)),
	)
	updated, token, err := f.auth.SetPassword(
		ctx,
		user.ID,
		oldToken,
		&dto.SetPasswordReq{
			Identifier: " PASSWORD-EMAIL@Example.TEST ",
			Password:   passwordTestSecret,
		},
	)
	if err != nil || updated.Username != user.Username || updated.PasswordIdentifier != email {
		t.Fatalf("email password binding: user=%v error=%v", updated, err)
	}
	if _, err := f.auth.Authenticate(ctx, token); err != nil {
		t.Fatal(err)
	}
	if err := f.repos.Auth().AdminDeleteBinding(ctx, user.ID, authkit.MethodEmail); err != nil {
		t.Fatal(err)
	}
	login, _, err := f.auth.LoginPassword(
		ctx,
		&dto.PasswordLoginReq{Identifier: email, Password: passwordTestSecret},
		"independent-email-password",
	)
	if err != nil || login.ID != user.ID || login.Email != "" || login.Username != user.Username {
		t.Fatalf(
			"verified email removal affected the password binding: user=%v error=%v",
			login,
			err,
		)
	}
}

// TestPasswordAuthBrowser runs actual registration/login pages against the isolated API.
func TestPasswordAuthBrowser(t *testing.T) {
	if os.Getenv("OMR_PASSWORD_BROWSER_TEST") != "1" {
		t.Skip("OMR_PASSWORD_BROWSER_TEST is not 1; password browser regression was not run")
	}
	f := newPasswordFixture(t)
	group, token := f.invitedGroup(t)
	fixture := map[string]string{
		"origin": f.server(t).URL, "token": token, "groupID": group.ID, "groupName": group.Name,
		"trial":          f.trial(t, "browser-password-trial", time.Now().Add(time.Hour)),
		"duplicateTrial": f.trial(t, "browser-duplicate-trial", time.Now().Add(time.Hour)),
		"emailTrial":     f.trial(t, "browser-email-trial", time.Now().Add(time.Hour)),
		"password":       passwordTestSecret,
	}
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("node", filepath.Join(root, "scripts/test-password-auth.mjs"))
	command.Dir = root
	command.Env = append(os.Environ(), "OMR_BROWSER_FIXTURE="+string(encoded))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	if err != nil {
		t.Fatalf("password browser regression: %v", err)
	}
}
