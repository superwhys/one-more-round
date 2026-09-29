package mysql_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/miebyte/authkit"

	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/errcode"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// TestNewEmailRegistrationRequiresInvitationAtLogin makes sending a code
// independent of admission and checks invitation consumption only at registration.
func TestNewEmailRegistrationRequiresInvitationAtLogin(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	email := "first-registration@example.com"
	code := freshEmailCode(t, s, email)
	for _, invite := range []string{"", "invalid-trial"} {
		account, token, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: code, Invite: invite})
		if err != errcode.ErrTrialInvalid || account != nil || token != "" {
			t.Fatalf("uninvited registration = %#v, session_issued=%t, %v", account, token != "", err)
		}
	}
	if _, err := s.repos.Auth().Accounts().GetByEmail(ctx, email); !errors.Is(err, authkit.ErrNotFound) {
		t.Fatalf("denied registration persisted an account: %v", err)
	}
	invite := trialToken(t, s)
	account, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: code, Invite: invite})
	if err != nil || account.Email != email {
		t.Fatalf("invitation supplied at login did not register: %#v, %v", account, err)
	}
	otherEmail := "second-registration@example.com"
	otherCode := freshEmailCode(t, s, otherEmail)
	if _, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: otherEmail, Code: otherCode, Invite: invite}); err != errcode.ErrTrialInvalid {
		t.Fatalf("consumed trial admitted another account: %v", err)
	}
	code = freshEmailCode(t, s, email)
	if again, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: code, Invite: invite}); err != nil || again.ID != account.ID {
		t.Fatalf("consumed registration invitation blocked existing account: %#v, %v", again, err)
	}
}

// TestExistingAccountLoginIgnoresStaleInvitation keeps account access independent
// from registration invitations and requires an explicit action to join a group.
func TestExistingAccountLoginIgnoresStaleInvitation(t *testing.T) {
	s := setup(t)
	ctx := context.Background()
	_, otherGroup, _, validGroupToken := invitationFixture(t, s)
	account, session := s.signup(t, "admitted-existing@example.com")
	wxAuth := wechatAuth(s)
	wx, err := wxAuth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "existing-trial-wx", Invite: trialToken(t, s)})
	if err != nil {
		t.Fatal(err)
	}
	unusedTrial := trialToken(t, s)
	for _, groupToken := range []string{"", "stale-group-token", validGroupToken} {
		code := freshEmailCode(t, s, account.Email)
		login, _, err := s.auth.Login(ctx, &dto.LoginReq{
			Email: account.Email, Code: code, Invite: unusedTrial, GroupToken: groupToken,
		})
		if err != nil || login.ID != account.ID || login.GroupID != "" {
			t.Fatalf("existing login depended on or used registration admission: %#v, %v", login, err)
		}
		if _, err := s.groups.Snapshot(ctx, otherGroup.ID, account.ID); err != errcode.ErrForbidden {
			t.Fatalf("login implicitly granted access to another group: %v", err)
		}
		again, err := wxAuth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "existing-trial-wx", Invite: unusedTrial, GroupToken: groupToken})
		if err != nil || again.ID != wx.ID || again.GroupID != "" {
			t.Fatalf("existing WeChat login depended on registration admission: %#v, %v", again, err)
		}
		if _, err := s.groups.Snapshot(ctx, otherGroup.ID, wx.ID); err != errcode.ErrForbidden {
			t.Fatalf("WeChat login implicitly granted access to another group: %v", err)
		}
	}
	if err := s.repos.Trial().Consume(ctx, secure.Hash(unusedTrial), time.Now().UTC()); err != nil {
		t.Fatalf("existing account login consumed a new trial invitation: %v", err)
	}
	if current, err := s.auth.Authenticate(ctx, session); err != nil || current.ID != account.ID {
		t.Fatalf("original session lost permanent admission: %#v, %v", current, err)
	}
}

// TestRegisteredAccountsSurviveGroupInvitationInvalidation checks both email
// and WeChat accounts retain sessions, membership and future login eligibility.
func TestRegisteredAccountsSurviveGroupInvitationInvalidation(t *testing.T) {
	for _, state := range []string{"revoked", "expired"} {
		t.Run(state, func(t *testing.T) {
			s := setup(t)
			ctx := context.Background()
			owner, g, invite, token := invitationFixture(t, s)
			email := "permanent-admission@example.com"
			code := freshEmailCode(t, s, email)
			account, session, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: code, GroupToken: token})
			if err != nil || account.GroupID != g.ID {
				t.Fatalf("initial group registration = %#v, %v", account, err)
			}
			wxAuth := wechatAuth(s)
			wx, err := wxAuth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "permanent-wx", GroupToken: token})
			if err != nil || wx.GroupID != g.ID {
				t.Fatalf("initial WeChat registration = %#v, %v", wx, err)
			}
			if state == "revoked" {
				if err := s.groups.Manage(ctx, g.ID, owner.ID, &dto.ManageReq{Action: "revoke", Target: invite.ID}); err != nil {
					t.Fatal(err)
				}
			} else if err := s.client.Gorm.Exec("UPDATE omr_invites SET expires=? WHERE id=?", time.Now().Add(-time.Hour), invite.ID).Error; err != nil {
				t.Fatal(err)
			}
			for _, current := range []struct{ id, session string }{{account.ID, session}, {wx.ID, wx.Token}} {
				if authenticated, err := s.auth.Authenticate(ctx, current.session); err != nil || authenticated.ID != current.id {
					t.Fatalf("invitation invalidation revoked an account session: %#v, %v", authenticated, err)
				}
				if _, err := s.groups.Snapshot(ctx, g.ID, current.id); err != nil {
					t.Fatalf("invitation invalidation removed existing membership: %v", err)
				}
			}
			for _, stale := range []string{"", token, "invalid-group-token"} {
				code := freshEmailCode(t, s, email)
				login, _, err := s.auth.Login(ctx, &dto.LoginReq{Email: email, Code: code, Invite: "stale-trial", GroupToken: stale})
				if err != nil || login.ID != account.ID || login.GroupID != "" {
					t.Fatalf("existing email login after invitation invalidation = %#v, %v", login, err)
				}
				again, err := wxAuth.WechatLogin(ctx, &dto.WechatLoginReq{Code: "permanent-wx", Invite: "stale-trial", GroupToken: stale})
				if err != nil || again.ID != wx.ID || again.GroupID != "" {
					t.Fatalf("existing WeChat login after invitation invalidation = %#v, %v", again, err)
				}
			}
		})
	}
}
