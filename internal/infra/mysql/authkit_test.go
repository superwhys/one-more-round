package mysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// TestAuthkitOwnsIdentityTables prevents a second application-managed identity
// schema or persistent registration context from appearing in fresh databases.
func TestAuthkitOwnsIdentityTables(t *testing.T) {
	_, client := newRepos(t)
	for _, name := range []string{"auth_accounts", "auth_bindings", "auth_challenges", "auth_rates", "auth_sessions"} {
		if !client.Gorm.Migrator().HasTable(name) {
			t.Errorf("missing identity table %s", name)
		}
	}
	for _, name := range []string{"omr_users", "omr_wechat_accounts", "omr_challenges", "omr_rates", "omr_sessions", "omr_registration_intents"} {
		if client.Gorm.Migrator().HasTable(name) {
			t.Errorf("fresh schema still creates obsolete table %s", name)
		}
	}
}

// failedCodeDelivery retains the generated proof for verification without ever
// marking it successfully delivered.
type failedCodeDelivery struct {
	code string
}

// SendCode captures the code while simulating a mail provider failure.
func (m *failedCodeDelivery) SendCode(_ context.Context, _, code string) error {
	m.code = code
	return errors.New("test mail delivery failed")
}

// TestAuthkitMailFailureLeavesCodeUnusable verifies failed delivery retains
// resend limits but cannot register an account or consume its invitation.
func TestAuthkitMailFailureLeavesCodeUnusable(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	sender := &failedCodeDelivery{}
	auth := services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: sender})
	email, invite := "failed-mail@example.com", trialToken(t, s)
	request := &dto.SendCodeReq{Email: email}
	if err := auth.SendCode(ctx, request, "mail-failure-test"); err != errcode.ErrMailFailed {
		t.Fatalf("delivery failure = %v", err)
	}
	if sender.code == "" {
		t.Fatal("delivery failure did not exercise a generated code")
	}
	if _, _, err := auth.Login(ctx, &dto.LoginReq{Email: email, Code: sender.code, Invite: invite}); err != errcode.ErrChallengeInvalid {
		t.Fatalf("undelivered code accepted: %v", err)
	}
	if err := auth.SendCode(ctx, request, "mail-failure-test"); err != errcode.ErrResendTooSoon {
		t.Fatalf("failed delivery bypassed resend limit: %v", err)
	}
	if _, err := s.repos.Auth().Accounts().GetByEmail(ctx, email); !errors.Is(err, authkit.ErrNotFound) {
		t.Fatalf("failed delivery created an account: %v", err)
	}
	if err := s.client.Gorm.Exec("UPDATE auth_challenges SET sent=? WHERE email=?", time.Now().Add(-time.Minute), email).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.auth.SendCode(ctx, request, "mail-failure-test"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: s.inbox.code(email), Invite: invite}); err != nil {
		t.Fatalf("retry lost the invitation or challenge: %v", err)
	}
}

// delayedCodeDelivery exposes pending issuance while a later send completes first.
type delayedCodeDelivery struct {
	sent    chan string
	release <-chan struct{}
}

// SendCode delays mail completion while respecting cancellation.
func (m *delayedCodeDelivery) SendCode(ctx context.Context, _, code string) error {
	select {
	case m.sent <- code:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case <-m.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TestAuthkitSupersededDeliveryPreservesLatestCode prevents a delayed old mail
// completion from activating the wrong proof or disabling the newest one.
func TestAuthkitSupersededDeliveryPreservesLatestCode(t *testing.T) {
	s := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	email := "superseded@example.com"
	release := make(chan struct{})
	sender := &delayedCodeDelivery{sent: make(chan string, 1), release: release}
	auth := services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: sender})
	completed := make(chan error, 1)
	go func() {
		completed <- auth.SendCode(ctx, &dto.SendCodeReq{Email: email}, "superseded-test")
	}()
	select {
	case <-sender.sent:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := s.client.Gorm.Exec("UPDATE auth_challenges SET sent=? WHERE email=?", time.Now().Add(-time.Minute), email).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.auth.SendCode(ctx, &dto.SendCodeReq{Email: email}, "superseded-test"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-completed; err != errcode.ErrChallengeUpdated {
		t.Fatalf("superseded delivery = %v", err)
	}
	if _, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: s.inbox.code(email), Invite: trialToken(t, s)}); err != nil {
		t.Fatalf("newest issuance could not register: %v", err)
	}
}
