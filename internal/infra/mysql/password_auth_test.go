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
	if _, err = admin.Exec("CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4 COLLATE utf8mb4_bin"); err != nil {
		_ = admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE `" + database + "`"); err != nil {
			t.Error(err)
		}
		_ = admin.Close()
	})
	client, err := mysql.Open(mysqlutils.MysqlConfig{Instance: instance, Database: database, Username: "root", Charset: "utf8mb4", PoolSize: 8})
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
	return &passwordFixture{db: client.Gorm, repos: repos, ctx: appCtx, auth: passwordAuthApp(t, appCtx), groups: services.NewGroupApp(appCtx), inbox: inbox}
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
	if err := f.repos.Trial().Create(context.Background(), secure.Hash(token), expires); err != nil {
		t.Fatal(err)
	}
	return token
}

// register creates a password account and verifies a usable opaque session.
func (f *passwordFixture) register(t *testing.T, username, invite, groupToken string) *dto.LoginResp {
	t.Helper()
	result, token, err := f.auth.RegisterPassword(context.Background(), &dto.PasswordRegisterReq{Username: username, Password: passwordTestSecret, Invite: invite, GroupToken: groupToken})
	if err != nil || token == "" || result == nil {
		t.Fatalf("password registration failed: result=%v token-present=%v error=%v", result, token != "", err)
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
		{Username: "no-invite", Password: passwordTestSecret},
		{Username: "bad-invite", Password: passwordTestSecret, Invite: "missing"},
		{Username: "expired-invite", Password: passwordTestSecret, Invite: f.trial(t, "expired", time.Now().Add(-time.Hour))},
	} {
		if _, _, err := f.auth.RegisterPassword(ctx, req); !errors.Is(err, errcode.ErrTrialInvalid) {
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
	if _, _, err := f.auth.RegisterPassword(ctx, &dto.PasswordRegisterReq{Username: "another-player", Password: passwordTestSecret, Invite: invite}); !errors.Is(err, errcode.ErrTrialInvalid) {
		t.Fatalf("one-use trial was reused: %v", err)
	}
	second := f.trial(t, "duplicate-trial", time.Now().Add(time.Hour))
	if _, _, err := f.auth.RegisterPassword(ctx, &dto.PasswordRegisterReq{Username: "桌游玩家", Password: passwordTestSecret, Invite: second}); !errors.Is(err, errcode.ErrConflict) {
		t.Fatalf("duplicate username: %v", err)
	}
	// Conflict cannot consume admission intended for a corrected username.
	f.register(t, "new-player", second, "")
	result, token, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: "桌游玩家", Password: passwordTestSecret}, "existing")
	if err != nil || token == "" || result.ID != user.ID {
		t.Fatalf("existing login without invite: result=%v error=%v", result, err)
	}
	if _, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: "桌游玩家", Password: strings.TrimSpace(passwordTestSecret)}, "trimmed"); !errors.Is(err, errcode.ErrUnauthorized) {
		t.Fatalf("password whitespace was not significant: %v", err)
	}
	if _, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: "missing-player", Password: passwordTestSecret}, "unknown"); !errors.Is(err, errcode.ErrUnauthorized) {
		t.Fatalf("unknown username: %v", err)
	}
}

// TestPasswordInvalidInput covers authkit's Unicode bounds without consuming admission.
func TestPasswordInvalidInput(t *testing.T) {
	f := newPasswordFixture(t)
	invite := f.trial(t, "invalid-input-trial", time.Now().Add(time.Hour))
	cases := []struct{ username, password string }{
		{"ab", passwordTestSecret}, {strings.Repeat("字", 65), passwordTestSecret},
		{"has space", passwordTestSecret}, {"name@example.test", passwordTestSecret},
		{"has\ncontrol", passwordTestSecret}, {"valid-player", strings.Repeat("密", 7)},
		{"valid-player", strings.Repeat("密", 129)},
	}
	for i, c := range cases {
		if _, _, err := f.auth.RegisterPassword(context.Background(), &dto.PasswordRegisterReq{Username: c.username, Password: c.password, Invite: invite}); !errors.Is(err, errcode.ErrBadRequest) {
			t.Fatalf("invalid input %d: %v", i, err)
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
			user, _, err := f.auth.RegisterPassword(ctx, &dto.PasswordRegisterReq{Username: username, Password: c.password, Invite: invite})
			if err != nil {
				t.Fatalf("eight-character password registration failed: %v", err)
			}
			result, token, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: username, Password: c.password}, username)
			if err != nil || token == "" || result == nil || result.ID != user.ID {
				t.Fatalf("eight-character password login failed: result=%v error=%v", result, err)
			}
		})
	}
}

// passwordRollback forces failure after the complete host transaction callback.
type passwordRollback struct{ ports.Repositories }

// WithTransaction makes account, session, admission and joining rollback together.
func (r passwordRollback) WithTransaction(ctx context.Context, fn func(ports.Repositories) error) error {
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
	group, err := f.groups.Create(context.Background(), owner.ID, &dto.CreateGroupReq{Name: "密码测试小组", PlayerName: "组主"})
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
		{Username: "rollback-trial-user", Password: passwordTestSecret, Invite: invite},
		{Username: "rollback-group-user", Password: passwordTestSecret, GroupToken: groupToken},
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
	if joined.GroupID != group.ID || f.count(t, "omr_members") != 2 || f.count(t, "omr_notifications") != 1 {
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
	backend := api.NewAPI(&config.Runtime{Origin: origin}, f.auth, f.groups, services.NewRoundApp(f.ctx), services.NewPhotoApp(f.ctx), services.NewNotificationApp(f.ctx), services.NewCommentApp(f.ctx))
	frontend, err := web.NewHandler()
	if err != nil {
		_ = listener.Close()
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", backend.SetupRouter()))
	mux.HandleFunc("/__test__/code", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"code": f.inbox.code(r.URL.Query().Get("email"))})
	})
	mux.Handle("/", frontend)
	server := httptest.NewUnstartedServer(mux)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)
	return server
}

// passwordRequest invokes the same Origin and Cookie protocol as the browser.
func passwordRequest(t *testing.T, server *httptest.Server, method, path string, payload any, cookie *http.Cookie) (*http.Response, map[string]any) {
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
	response, body := passwordRequest(t, server, "POST", "/api/v1/auth/password/register", map[string]string{"username": "http-player", "password": passwordTestSecret, "invite": invite}, nil)
	if response.StatusCode != http.StatusOK || body["data"].(map[string]any)["username"] != "http-player" {
		t.Fatalf("register response: status=%d body=%v", response.StatusCode, body)
	}
	var cookie *http.Cookie
	for _, c := range response.Cookies() {
		if c.Name == common.SessionCookie {
			cookie = c
		}
	}
	if cookie == nil || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Value == "" {
		t.Fatal("registration did not set a private Strict session cookie")
	}
	response, body = passwordRequest(t, server, "GET", "/api/v1/me", nil, cookie)
	if response.StatusCode != 200 || body["data"].(map[string]any)["username"] != "http-player" {
		t.Fatalf("/me: status=%d body=%v", response.StatusCode, body)
	}
	response, _ = passwordRequest(t, server, "POST", "/api/v1/auth/logout", nil, cookie)
	if response.StatusCode != 200 || len(response.Cookies()) != 1 || response.Cookies()[0].MaxAge != -1 {
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
		response, _ = passwordRequest(t, server, "POST", "/api/v1/auth/password/login", map[string]string{"identifier": "http-player", "password": rejected.password}, nil)
		if response.StatusCode != rejected.status {
			t.Fatalf("invalid password login status=%d", response.StatusCode)
		}
	}
	response, _ = passwordRequest(t, server, "POST", "/api/v1/auth/password/login", map[string]string{"identifier": "http-player", "password": passwordTestSecret}, nil)
	if response.StatusCode != 200 || len(response.Cookies()) != 1 {
		t.Fatal("existing password account could not log in without an invitation")
	}
}

// TestPasswordLoginRateCommit proves rejection commits rate counters across host transactions.
func TestPasswordLoginRateCommit(t *testing.T) {
	f := newPasswordFixture(t)
	f.register(t, "rate-player", f.trial(t, "rate-trial", time.Now().Add(time.Hour)), "")
	ctx := context.Background()
	limited := false
	for i := 0; i < 100; i++ {
		_, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: "rate-player", Password: "a-wrong-password-value"}, "rate-ip")
		if errors.Is(err, errcode.ErrTooManyRequests) {
			limited = true
			break
		}
		if !errors.Is(err, errcode.ErrUnauthorized) {
			t.Fatalf("wrong password response=%v", err)
		}
	}
	if !limited {
		t.Fatal("password login rejection rolled back rate counters")
	}
	if _, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: "rate-player", Password: passwordTestSecret}, "rate-ip"); !errors.Is(err, errcode.ErrTooManyRequests) {
		t.Fatalf("correct password bypassed active rate limit: %v", err)
	}
}

// emailAccount creates an existing email identity through real code verification.
func (f *passwordFixture) emailAccount(t *testing.T, email, invite string) (*dto.LoginResp, string) {
	t.Helper()
	ctx := context.Background()
	if err := f.auth.SendCode(ctx, &dto.SendCodeReq{Email: email}, email); err != nil {
		t.Fatal(err)
	}
	user, token, err := f.auth.Login(ctx, &dto.LoginReq{Email: email, Code: f.inbox.code(email), Invite: invite})
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
	user, oldToken := f.emailAccount(t, email, f.trial(t, "email-account-trial", time.Now().Add(time.Hour)))
	group, err := f.groups.Create(ctx, user.ID, &dto.CreateGroupReq{Name: "原来的小组", PlayerName: "原玩家"})
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
		GroupID: group.ID, IdempotencyKey: secure.NewID(),
		Round: dto.Round{GameID: game.ID, Date: "2026-09-01", Mode: "coop", Outcome: "win", Players: []string{before.Players[0].ID}, Memory: "保留在原账号的对局"},
	})
	if err != nil {
		t.Fatal(err)
	}
	otherToken := secure.NewID()
	if err := f.repos.Auth().Sessions().Create(ctx, &authkit.Session{Hash: secure.Hash(otherToken), AccountID: user.ID, Expires: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	f.register(t, "taken-player", f.trial(t, "taken-trial", time.Now().Add(time.Hour)), "")
	for _, req := range []*dto.SetPasswordReq{
		{Username: "taken-player", Password: passwordTestSecret},
		{Username: "email-player", Password: strings.Repeat("密", 7)},
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
	newUser, currentToken, err := f.auth.SetPassword(ctx, user.ID, oldToken, &dto.SetPasswordReq{Username: "email-player", Password: passwordTestSecret})
	if err != nil || newUser.ID != user.ID || newUser.Email != email || newUser.Username != "email-player" {
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
	if err != nil || snapshot.Group.Owner != user.ID || len(snapshot.Players) != 1 || len(snapshot.Games) != 1 || snapshot.Games[0].ID != game.ID {
		t.Fatalf("password setup changed existing group data: snapshot=%+v error=%v", snapshot, err)
	}
	preserved, err := rounds.Get(ctx, group.ID, user.ID, history.ID)
	if err != nil || preserved.Author != user.ID || preserved.GameID != game.ID || preserved.Memory != history.Memory || preserved.Version != history.Version {
		t.Fatalf("password setup changed historical round: round=%+v error=%v", preserved, err)
	}
	for _, identifier := range []string{"email-player", email} {
		result, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: identifier, Password: passwordTestSecret}, identifier)
		if err != nil || result.ID != user.ID || result.Email != email {
			t.Fatalf("password login by %s: result=%v error=%v", identifier, result, err)
		}
	}
	if _, _, err := f.auth.SetPassword(ctx, user.ID, currentToken, &dto.SetPasswordReq{Username: "different-player", Password: passwordTestSecret}); !errors.Is(err, errcode.ErrConflict) {
		t.Fatalf("existing username changed unexpectedly: %v", err)
	}
	if _, err := f.auth.Authenticate(ctx, currentToken); err != nil {
		t.Fatal("conflicting username change revoked current session")
	}
	rollbackCtx := &services.AppContext{Repos: passwordRollback{f.repos}, Mailer: f.inbox}
	if _, _, err := passwordAuthApp(t, rollbackCtx).SetPassword(ctx, user.ID, currentToken, &dto.SetPasswordReq{Password: "another-password-for-tests"}); err == nil {
		t.Fatal("expected password setup transaction rollback")
	}
	if _, err := f.auth.Authenticate(ctx, currentToken); err != nil {
		t.Fatal("failed password setup transaction revoked current session")
	}
	if _, _, err := f.auth.LoginPassword(ctx, &dto.PasswordLoginReq{Identifier: email, Password: passwordTestSecret}, "after-rollback"); err != nil {
		t.Fatalf("failed password setup changed the credential: %v", err)
	}
	server := f.server(t)
	response, _ := passwordRequest(t, server, "POST", "/api/v1/auth/password/set", map[string]string{"username": "email-player", "password": passwordTestSecret}, nil)
	if response.StatusCode != 401 {
		t.Fatalf("unauthenticated password setup status=%d", response.StatusCode)
	}
	cookie := &http.Cookie{Name: common.SessionCookie, Value: currentToken}
	response, body := passwordRequest(t, server, "POST", "/api/v1/auth/password/set", map[string]string{"password": passwordTestSecret}, cookie)
	if response.StatusCode != 200 || body["data"].(map[string]any)["id"] != user.ID || len(response.Cookies()) != 1 {
		t.Fatalf("password setup HTTP response: status=%d body=%v", response.StatusCode, body)
	}
	response, _ = passwordRequest(t, server, "GET", "/api/v1/me", nil, response.Cookies()[0])
	if response.StatusCode != 200 {
		t.Fatal("password setup did not preserve the browser's active session")
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
