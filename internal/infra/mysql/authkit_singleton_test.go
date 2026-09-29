package mysql_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// controlledAuthMailer exposes overlapping sends and one recoverable delivery
// failure while retaining the ordinary synchronized test mailbox.
type controlledAuthMailer struct {
	*inbox
	blockedEmail string
	entered      chan struct{}
	release      <-chan struct{}
	failEmail    string
	failed       atomic.Bool
}

// SendCode captures a proof, optionally pauses its delivery or fails it once.
func (m *controlledAuthMailer) SendCode(ctx context.Context, email, code string) error {
	if err := m.inbox.SendCode(ctx, email, code); err != nil {
		return err
	}
	if email == m.blockedEmail {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if email == m.failEmail && m.failed.CompareAndSwap(false, true) {
		return errors.New("temporary test delivery failure")
	}
	return nil
}

// TestAuthkitSharedServiceKeepsConcurrentMailboxesIsolated interleaves the two
// issuance transactions of separate addresses through the same AuthApp.
func TestAuthkitSharedServiceKeepsConcurrentMailboxesIsolated(t *testing.T) {
	s := setup(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	firstEmail, secondEmail := "shared-first@example.com", "shared-second@example.com"
	firstInvite, secondInvite := trialToken(t, s), trialToken(t, s)
	release := make(chan struct{})
	mailer := &controlledAuthMailer{
		inbox: s.inbox, blockedEmail: firstEmail,
		entered: make(chan struct{}), release: release,
	}
	auth := services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: mailer})
	completed := make(chan error, 1)
	go func() {
		completed <- auth.SendCode(ctx, &dto.SendCodeReq{Email: firstEmail}, "shared-first")
	}()
	select {
	case <-mailer.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := auth.SendCode(ctx, &dto.SendCodeReq{Email: secondEmail}, "shared-second"); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-completed; err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ email, invite string }{
		{firstEmail, firstInvite},
		{secondEmail, secondInvite},
	} {
		account, token, err := auth.Login(ctx, &dto.LoginReq{Email: input.email, Code: mailer.code(input.email), Invite: input.invite})
		if err != nil || account.Email != input.email {
			t.Fatalf("concurrent registration for %s = %#v, %v", input.email, account, err)
		}
		if current, err := auth.Authenticate(ctx, token); err != nil || current.ID != account.ID {
			t.Fatalf("same service lost authentication after sending: %#v, %v", current, err)
		}
		if err := s.repos.Trial().Consume(ctx, secure.Hash(input.invite), time.Now().UTC()); !errors.Is(err, errcode.ErrTrialInvalid) {
			t.Fatalf("registration did not consume its own invitation: %v", err)
		}
	}
}

// TestAuthkitSharedServiceRecoversAfterDeliveryFailure verifies one failed send
// leaves the same AuthApp usable for another mailbox and for a later retry.
func TestAuthkitSharedServiceRecoversAfterDeliveryFailure(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	failedEmail, otherEmail := "shared-failed@example.com", "shared-recovery@example.com"
	failedInvite, otherInvite := trialToken(t, s), trialToken(t, s)
	mailer := &controlledAuthMailer{inbox: s.inbox, failEmail: failedEmail}
	auth := services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: mailer})
	failedRequest := &dto.SendCodeReq{Email: failedEmail}
	if err := auth.SendCode(ctx, failedRequest, "shared-failure"); !errors.Is(err, errcode.ErrMailFailed) {
		t.Fatalf("first delivery = %v", err)
	}
	if _, _, err := auth.Login(ctx, &dto.LoginReq{Email: failedEmail, Code: mailer.code(failedEmail), Invite: failedInvite}); !errors.Is(err, errcode.ErrChallengeInvalid) {
		t.Fatalf("failed delivery activated its proof: %v", err)
	}
	if err := auth.SendCode(ctx, &dto.SendCodeReq{Email: otherEmail}, "shared-recovery"); err != nil {
		t.Fatalf("same service could not deliver after failure: %v", err)
	}
	other, token, err := auth.Login(ctx, &dto.LoginReq{Email: otherEmail, Code: mailer.code(otherEmail), Invite: otherInvite})
	if err != nil {
		t.Fatalf("same service could not register after failure: %v", err)
	}
	if current, err := auth.Authenticate(ctx, token); err != nil || current.ID != other.ID {
		t.Fatalf("same service could not authenticate after failure: %#v, %v", current, err)
	}
	if err := s.client.Gorm.Exec(
		"UPDATE auth_challenges SET sent=? WHERE email=?",
		time.Now().Add(-time.Minute), failedEmail,
	).Error; err != nil {
		t.Fatal(err)
	}
	if err := auth.SendCode(ctx, failedRequest, "shared-failure"); err != nil {
		t.Fatalf("same service could not retry failed delivery: %v", err)
	}
	if account, _, err := auth.Login(ctx, &dto.LoginReq{Email: failedEmail, Code: mailer.code(failedEmail), Invite: failedInvite}); err != nil || account.Email != failedEmail {
		t.Fatalf("failed mailbox lost its original invitation: %#v, %v", account, err)
	}
	if err := auth.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.Authenticate(ctx, token); !errors.Is(err, errcode.ErrUnauthorized) {
		t.Fatalf("same service could not revoke the original session: %v", err)
	}
}
