package mysql_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// testWechat maps temporary test codes to provider identities without external calls.
type testWechat struct{}

// ExchangeCode supplies deterministic provider responses for the real database tests.
func (testWechat) ExchangeCode(_ context.Context, code string) (ports.WechatIdentity, error) {
	if code == "provider-failed" {
		return ports.WechatIdentity{}, errcode.ErrWechatLogin
	}
	return ports.WechatIdentity{AppID: "test-mini", OpenID: code}, nil
}

// wechatAuth wires the real transaction adapter with a deterministic provider.
func wechatAuth(s *stack) *services.AuthApp {
	return services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: s.inbox, Wechat: testWechat{}})
}

// freshEmailCode resets only the fixture clock before requesting a real code.
func freshEmailCode(t *testing.T, s *stack, email string) string {
	t.Helper()
	if err := s.client.Gorm.Exec("UPDATE omr_challenges SET sent=? WHERE email=?", time.Now().Add(-time.Minute), email).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.auth.SendCode(context.Background(), &dto.SendCodeReq{Email: email}, "wechat-test"); err != nil {
		t.Fatal(err)
	}
	return s.inbox.code(email)
}

// trialToken creates one isolated invitation for a fixture account.
func trialToken(t *testing.T, s *stack) string {
	t.Helper()
	token := secure.NewID()
	if err := s.repos.Trial().Create(context.Background(), secure.Hash(token), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return token
}

// TestWechatFirstLoginBindsExistingEmail preserves the original account and its
// groups, keeps email login working and rejects a second WeChat identity.
func TestWechatFirstLoginBindsExistingEmail(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	ctx := context.Background()
	owner, _ := s.signup(t, "existing-wx@example.com")
	group, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "原有记录小组", PlayerName: "我"})
	if err != nil {
		t.Fatal(err)
	}
	code := freshEmailCode(t, s, owner.Email)
	result, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "wechat-one", Email: strings.ToUpper(owner.Email), EmailCode: code})
	if err != nil || result.ID != owner.ID || result.Email != owner.Email {
		t.Fatalf("existing account = %#v, %v", result, err)
	}
	if _, err = s.groups.Snapshot(ctx, group.ID, result.ID); err != nil {
		t.Fatal(err)
	}
	if current, err := auth.Authenticate(ctx, result.Token); err != nil || current.ID != owner.ID {
		t.Fatalf("session = %#v, %v", current, err)
	}
	if _, err = auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "wechat-two", Email: owner.Email, EmailCode: code}); err != errcode.ErrChallengeInvalid {
		t.Fatalf("reused email code = %v", err)
	}
	code = freshEmailCode(t, s, owner.Email)
	if _, err = auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "wechat-two", Email: owner.Email, EmailCode: code}); err != errcode.ErrWechatBound {
		t.Fatalf("second WeChat identity = %v", err)
	}
	// Failed binding must not consume the proof or alter email login.
	if existing, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: owner.Email, Code: code}); err != nil || existing.ID != owner.ID {
		t.Fatalf("email login = %#v, %v", existing, err)
	}
	if again, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "wechat-one"}); err != nil || again.ID != owner.ID {
		t.Fatalf("repeat WeChat login = %#v, %v", again, err)
	}
}

// TestWechatInvitationAndConcurrentRegistration uses one invitation once while
// concurrent logins to the same identity share exactly one account.
func TestWechatInvitationAndConcurrentRegistration(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	ctx := context.Background()
	if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "no-invitation"}); err != errcode.ErrTrialInvalid {
		t.Fatalf("uninvited registration = %v", err)
	}
	invite := trialToken(t, s)
	type loginResult struct {
		result *dto.WechatLoginResp
		err    error
	}
	results := make(chan loginResult, 6)
	for range cap(results) {
		go func() {
			result, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "concurrent-wx", Invite: invite})
			results <- loginResult{result, err}
		}()
	}
	id := ""
	for range cap(results) {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.result.Email != "" || (id != "" && result.result.ID != id) {
			t.Fatalf("duplicate or synthetic email account = %#v", result.result)
		}
		id = result.result.ID
	}
	var count int64
	if err := s.client.Gorm.Table("omr_users").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("users = %d, %v", count, err)
	}
	if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "other-wx", Invite: invite}); err != errcode.ErrTrialInvalid {
		t.Fatalf("reused invitation = %v", err)
	}
	if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "another-wx", Invite: trialToken(t, s)}); err != nil {
		t.Fatalf("second nullable-email account: %v", err)
	}
}

// TestWechatGroupInvitationAtomicity covers joining, revocation and transaction rollback.
func TestWechatGroupInvitationAtomicity(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	ctx := context.Background()
	owner, g, inv, invite := invitationFixture(t, s)
	result, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "group-friend", GroupToken: invite})
	if err != nil || result.GroupID != g.ID {
		t.Fatalf("joined account = %#v, %v", result, err)
	}
	if _, err = s.groups.Snapshot(ctx, g.ID, result.ID); err != nil {
		t.Fatal(err)
	}
	broken := services.NewAuthApp(&services.AppContext{Repos: failingJoinRepos{s.repos}, Mailer: s.inbox, Wechat: testWechat{}})
	if _, err = broken.WechatLogin(ctx, &dto.WechatLoginReq{Code: "rollback-wx", GroupToken: invite}); err != errJoinFailure {
		t.Fatalf("join failure = %v", err)
	}
	var count int64
	if err = s.client.Gorm.Table("omr_wechat_accounts").Where("openid_hash = ?", secure.Hash("rollback-wx")).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("rolled back identity = %d, %v", count, err)
	}
	if _, err = auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "rollback-wx", GroupToken: invite}); err != nil {
		t.Fatal(err)
	}
	if err = s.groups.Manage(ctx, g.ID, owner.ID, &dto.ManageReq{Action: "revoke", Target: inv.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "revoked-wx", GroupToken: invite}); err != errcode.ErrInviteRevoked {
		t.Fatalf("revoked registration = %v", err)
	}
}

// TestWechatBindEmailNeverMerges binds only unused mailboxes, rotates the session,
// and retains both independently registered accounts when their emails conflict.
func TestWechatBindEmailNeverMerges(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	ctx := context.Background()
	existing, _ := s.signup(t, "separate@example.com")
	wx, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "independent", Invite: trialToken(t, s)})
	if err != nil {
		t.Fatal(err)
	}
	g, err := s.groups.Create(ctx, wx.ID, &dto.CreateGroupReq{Name: "微信小组", PlayerName: "微信玩家"})
	if err != nil {
		t.Fatal(err)
	}
	code := freshEmailCode(t, s, existing.Email)
	if _, err = auth.BindWechatEmail(ctx, wx.ID, wx.Token, &dto.BindEmailReq{Email: existing.Email, Code: code}); err != errcode.ErrEmailAccountConflict {
		t.Fatalf("independent binding = %v", err)
	}
	if _, err = auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "independent", Email: existing.Email, EmailCode: code}); err != errcode.ErrEmailAccountConflict {
		t.Fatalf("login merge = %v", err)
	}
	if _, err = s.groups.Snapshot(ctx, g.ID, existing.ID); !errors.Is(err, errcode.ErrForbidden) {
		t.Fatalf("cross-account data moved: %v", err)
	}
	newEmail := "newly-bound@example.com"
	code = freshEmailCode(t, s, newEmail)
	bound, err := auth.BindWechatEmail(ctx, wx.ID, wx.Token, &dto.BindEmailReq{Email: newEmail, Code: code})
	if err != nil || bound.ID != wx.ID || bound.Email != newEmail || bound.Token == wx.Token {
		t.Fatalf("bound account = %#v, %v", bound, err)
	}
	if _, err = auth.Authenticate(ctx, wx.Token); err != errcode.ErrUnauthorized {
		t.Fatalf("old session survived rotation: %v", err)
	}
	if _, err = auth.Authenticate(ctx, bound.Token); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.BindWechatEmail(ctx, bound.ID, bound.Token, &dto.BindEmailReq{Email: newEmail, Code: code}); err != errcode.ErrChallengeInvalid {
		t.Fatalf("binding proof reused: %v", err)
	}
	code = freshEmailCode(t, s, newEmail)
	if login, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: newEmail, Code: code}); err != nil || login.ID != wx.ID {
		t.Fatalf("new email login = %#v, %v", login, err)
	}
}

// TestWechatVerificationLimitAndProviderFailures verifies failed guesses commit
// their attempt counts, and provider failures cannot create accounts.
func TestWechatVerificationLimitAndProviderFailures(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	ctx := context.Background()
	owner, _ := s.signup(t, "wrong-proof@example.com")
	code := freshEmailCode(t, s, owner.Email)
	for range 5 {
		if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "proof-wx", Email: owner.Email, EmailCode: "wrong"}); err != errcode.ErrChallengeMismatch {
			t.Fatalf("wrong code = %v", err)
		}
	}
	if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "proof-wx", Email: owner.Email, EmailCode: code}); err != errcode.ErrChallengeInvalid {
		t.Fatalf("attempt limit = %v", err)
	}
	if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "provider-failed", Invite: trialToken(t, s)}); err != errcode.ErrWechatLogin {
		t.Fatalf("provider failure = %v", err)
	}
	if _, err := s.auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "disabled"}); err != errcode.ErrWechatUnavailable {
		t.Fatalf("disabled integration = %v", err)
	}
	for _, req := range []*dto.WechatLoginReq{{Code: ""}, {Code: "some", Email: owner.Email}, {Code: "some", EmailCode: "123456"}} {
		if _, err := auth.WechatLogin(ctx, req); err != errcode.ErrBadRequest {
			t.Fatalf("incomplete input accepted: %v", err)
		}
	}
}

// TestWechatHTTPBearerSession covers the client contract, protected writes,
// private group access and logout without a browser Cookie or Origin.
func TestWechatHTTPBearerSession(t *testing.T) {
	s := setup(t)
	auth := wechatAuth(s)
	handler := api.NewAPI("test", &config.Runtime{Origin: testOrigin}, auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).SetupRouter()
	request := httptest.NewRequest(http.MethodPost, "/v1/auth/wx-login", strings.NewReader(`{"code":"http-wx","invite":"`+trialToken(t, s)+`"}`))
	request.Header.Set("X-OMR-Client", "wechat-mini")
	request.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, request)
	var login struct {
		Data dto.WechatLoginResp `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &login); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || login.Data.Token == "" || len(rec.Result().Cookies()) != 0 {
		t.Fatalf("mini login = %d %s", rec.Code, rec.Body.String())
	}
	for _, step := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/v1/me", "", 200},
		{http.MethodPost, "/v1/groups", `{"name":"小程序小组","player_name":"我"}`, 200},
		{http.MethodPost, "/v1/auth/logout", "", 200},
		{http.MethodGet, "/v1/me", "", 401},
	} {
		request = httptest.NewRequest(step.method, step.path, strings.NewReader(step.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Authorization", "Bearer "+login.Data.Token)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, request)
		if rec.Code != step.status {
			t.Fatalf("%s %s = %d %s", step.method, step.path, rec.Code, rec.Body.String())
		}
	}
}

// TestWechatMigrationPreservesExistingEmails verifies a legacy NOT NULL email
// schema can evolve without rewriting old addresses or inventing new ones.
func TestWechatMigrationPreservesExistingEmails(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	if err := s.client.Gorm.Exec("ALTER TABLE omr_users MODIFY email varchar(254) NOT NULL").Error; err != nil {
		t.Fatal(err)
	}
	owner, _ := s.signup(t, "legacy-account@example.com")
	if err := s.client.AutoMigrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.client.AutoMigrate(); err != nil {
		t.Fatal("repeat migration:", err)
	}
	existing, err := s.repos.User().GetByEmail(ctx, owner.Email)
	if err != nil || existing.ID != owner.ID {
		t.Fatalf("legacy email changed = %#v, %v", existing, err)
	}
	auth := wechatAuth(s)
	for _, code := range []string{"nullable-one", "nullable-two"} {
		if _, err := auth.WechatLogin(ctx, &dto.WechatLoginReq{Code: code, Invite: trialToken(t, s)}); err != nil {
			t.Fatalf("new nullable email: %v", err)
		}
	}
}
