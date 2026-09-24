package mysql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/domain/group"
	"github.com/superwhys/one-more-round/internal/errcode"
)

func invitationRequest(
	t *testing.T,
	handler http.Handler,
	path string,
	body any,
) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1"+path, bytes.NewReader(data))
	req.Header.Set("Origin", testOrigin)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func invitationFixture(t *testing.T, s *stack) (dto.User, dto.Group, dto.Invite, string) {
	t.Helper()
	ctx := context.Background()
	owner, _ := s.signup(t, "invite-owner@example.com")
	g, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "周五桌游局", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	inv, token, err := s.groups.Invite(ctx, g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	return owner, g, inv, token
}

func TestGroupInvitationRegistrationAndReuse(t *testing.T) {
	s := setup(t)
	_, g, _, token := invitationFixture(t, s)
	handler := api.NewAPI("test", &config.Runtime{Origin: testOrigin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
		SetupRouter()
	for _, email := range []string{"friend-one@example.com", "friend-two@example.com"} {
		rec := invitationRequest(
			t,
			handler,
			"/auth/code",
			map[string]string{"email": email, "group_token": token},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf("send code: %d", rec.Code)
		}
		rec = invitationRequest(
			t,
			handler,
			"/auth/login",
			map[string]string{"email": email, "code": s.inbox.code(email), "group_token": token},
		)
		if rec.Code != http.StatusOK {
			t.Fatalf(
				"group invitation should register and join without a trial: %d %s",
				rec.Code,
				rec.Body.String(),
			)
		}
		var result struct {
			Data struct {
				ID      string `json:"id"`
				GroupID string `json:"group_id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Data.ID == "" || result.Data.GroupID != g.ID {
			t.Fatal("login did not return the account and joined group")
		}
		if len(rec.Result().Cookies()) != 1 {
			t.Fatal("successful join did not issue a session")
		}
		if _, err := s.groups.Snapshot(context.Background(), g.ID, result.Data.ID); err != nil {
			t.Fatal(err)
		}
		// Joining again is harmless and does not duplicate the membership.
		for range 2 {
			if _, err := s.groups.Join(
				context.Background(),
				result.Data.ID,
				&dto.JoinReq{Token: token},
			); err != nil {
				t.Fatal(err)
			}
		}
		rec = invitationRequest(
			t,
			handler,
			"/auth/login",
			map[string]string{"email": email, "code": s.inbox.code(email), "group_token": token},
		)
		if rec.Code == http.StatusOK {
			t.Fatal("verification code was reused")
		}
	}
	groups, err := s.repos.Group().Snapshot(context.Background(), g.ID)
	if err != nil || len(groups.Members) != 3 {
		t.Fatalf("members: %v, err: %v", groups, err)
	}
}

func TestGroupInvitationExistingAccountLogin(t *testing.T) {
	s := setup(t)
	_, g, _, token := invitationFixture(t, s)
	user, _ := s.signup(t, "existing@example.com")
	if err := s.client.Gorm.Exec(
		"UPDATE omr_challenges SET sent=? WHERE email=?",
		time.Now().Add(-time.Minute),
		user.Email,
	).Error; err != nil {
		t.Fatal(err)
	}
	handler := api.NewAPI("test", &config.Runtime{Origin: testOrigin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
		SetupRouter()
	rec := invitationRequest(
		t,
		handler,
		"/auth/code",
		map[string]string{"email": user.Email, "group_token": token},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("send: %d", rec.Code)
	}
	rec = invitationRequest(
		t,
		handler,
		"/auth/login",
		map[string]string{
			"email":       user.Email,
			"code":        s.inbox.code(user.Email),
			"group_token": token,
		},
	)
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := s.groups.Snapshot(context.Background(), g.ID, user.ID); err != nil {
		t.Fatal("existing account was not joined:", err)
	}
}

func TestGroupInvitationPreviewAndInvalidation(t *testing.T) {
	for _, state := range []string{"revoked", "expired", "unknown"} {
		t.Run(state, func(t *testing.T) {
			s := setup(t)
			owner, g, inv, token := invitationFixture(t, s)
			handler := api.NewAPI("test", &config.Runtime{Origin: testOrigin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
				SetupRouter()
			rec := invitationRequest(
				t,
				handler,
				"/auth/group-invite",
				map[string]string{"token": token},
			)
			if rec.Code != http.StatusOK {
				t.Fatalf("preview: %d", rec.Code)
			}
			var preview struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
				t.Fatal(err)
			}
			if len(preview.Data) != 2 || preview.Data["group_id"] != g.ID ||
				preview.Data["name"] != g.Name {
				t.Fatal("preview must expose only group ID and name")
			}
			email := "pending@example.com"
			rec = invitationRequest(
				t,
				handler,
				"/auth/code",
				map[string]string{"email": email, "group_token": token},
			)
			if rec.Code != http.StatusOK {
				t.Fatalf("send: %d", rec.Code)
			}
			want := "小组邀请无效，请向组主索取新链接"
			switch state {
			case "revoked":
				want = "小组邀请已撤销，请向组主索取新链接"
				if err := s.groups.Manage(
					context.Background(),
					g.ID,
					owner.ID,
					&dto.ManageReq{Action: "revoke", Target: inv.ID},
				); err != nil {
					t.Fatal(err)
				}
			case "expired":
				want = "小组邀请已过期，请向组主索取新链接"
				if err := s.client.Gorm.Exec(
					"UPDATE omr_invites SET expires=? WHERE id=?",
					time.Now().Add(-time.Hour),
					inv.ID,
				).Error; err != nil {
					t.Fatal(err)
				}
			case "unknown":
				token = "unknown-token"
			}
			for _, request := range []struct {
				path string
				body any
			}{
				{"/auth/group-invite", map[string]string{"token": token}},
				{"/auth/code", map[string]string{"email": "another@example.com", "group_token": token}},
				{"/auth/login", map[string]string{"email": email, "code": s.inbox.code(email), "group_token": token}},
			} {
				rec = invitationRequest(t, handler, request.path, request.body)
				var result struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if rec.Code != http.StatusBadRequest || result.Message != want {
					t.Fatalf("%s: %d %s", request.path, rec.Code, result.Message)
				}
				if len(rec.Result().Cookies()) != 0 {
					t.Fatal("failed login issued a session")
				}
			}
			if _, err := s.repos.User().
				GetByEmail(context.Background(), email); !errors.Is(
				err,
				errcode.ErrNotFound,
			) {
				t.Fatal("invalid invitation registered an account", err)
			}
		})
	}
}

type (
	failingJoinRepos struct{ ports.Repositories }
	failingJoinGroup struct{ group.IGroupRepository }
)

var errJoinFailure = errors.New("injected membership write failure")

func (r failingJoinRepos) WithTransaction(
	ctx context.Context,
	fn func(ports.Repositories) error,
) error {
	return r.Repositories.WithTransaction(
		ctx,
		func(tx ports.Repositories) error { return fn(failingJoinRepos{tx}) },
	)
}

func (r failingJoinRepos) Group() group.IGroupRepository {
	return failingJoinGroup{r.Repositories.Group()}
}
func (failingJoinGroup) AddMember(context.Context, string, string) error { return errJoinFailure }

func TestGroupInvitationRegistrationRollsBackOnJoinFailure(t *testing.T) {
	s := setup(t)
	_, _, _, token := invitationFixture(t, s)
	email := "rollback@example.com"
	if err := s.auth.SendCode(
		context.Background(),
		&dto.SendCodeReq{Email: email},
		"local",
	); err != nil {
		t.Fatal(err)
	}
	var request dto.LoginReq
	raw, _ := json.Marshal(
		map[string]string{"email": email, "code": s.inbox.code(email), "group_token": token},
	)
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	auth := services.NewAuthApp(
		&services.AppContext{Repos: failingJoinRepos{s.repos}, Mailer: s.inbox},
	)
	if _, _, err := auth.Login(context.Background(), &request); !errors.Is(err, errJoinFailure) {
		t.Fatalf("expected injected failure: %v", err)
	}
	if _, err := s.repos.User().
		GetByEmail(context.Background(), email); !errors.Is(
		err,
		errcode.ErrNotFound,
	) {
		t.Fatal("registration was not rolled back", err)
	}
	// Rollback must preserve the code so a normal retry can finish.
	if _, _, err := s.auth.Login(context.Background(), &request); err != nil {
		t.Fatal("retry after rollback:", err)
	}
}

func TestGroupInvitationConcurrentLoginAndAttemptLimit(t *testing.T) {
	s := setup(t)
	_, g, _, token := invitationFixture(t, s)
	ctx := context.Background()
	email := "concurrent-invite@example.com"
	if err := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: email, GroupToken: token},
		"local",
	); err != nil {
		t.Fatal(err)
	}
	req := &dto.LoginReq{Email: email, Code: s.inbox.code(email), GroupToken: token}
	errors := make(chan error, 6)
	for range cap(errors) {
		go func() { _, _, err := s.auth.Login(ctx, req); errors <- err }()
	}
	successes := 0
	for range cap(errors) {
		if err := <-errors; err == nil {
			successes++
		} else if err != errcode.ErrChallengeInvalid {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful concurrent logins = %d", successes)
	}
	snapshot, err := s.repos.Group().Snapshot(ctx, g.ID)
	if err != nil || len(snapshot.Members) != 2 {
		t.Fatalf("duplicate membership: %v %v", snapshot, err)
	}
	email = "attempt-limit-invite@example.com"
	if err := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: email, GroupToken: token},
		"local",
	); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, _, err := s.auth.Login(
			ctx,
			&dto.LoginReq{Email: email, Code: "wrong", GroupToken: token},
		); err != errcode.ErrChallengeMismatch {
			t.Fatalf("wrong code: %v", err)
		}
	}
	if _, _, err := s.auth.Login(
		ctx,
		&dto.LoginReq{Email: email, Code: s.inbox.code(email), GroupToken: token},
	); err != errcode.ErrChallengeInvalid {
		t.Fatalf("attempt limit: %v", err)
	}
}

type observedGroupRepos struct {
	ports.Repositories
	reached chan struct{}
	once    *sync.Once
}
type observedGroup struct {
	group.IGroupRepository
	reached chan struct{}
	once    *sync.Once
}

func (r observedGroupRepos) WithTransaction(
	ctx context.Context,
	fn func(ports.Repositories) error,
) error {
	return r.Repositories.WithTransaction(
		ctx,
		func(tx ports.Repositories) error { return fn(observedGroupRepos{tx, r.reached, r.once}) },
	)
}

func (r observedGroupRepos) Group() group.IGroupRepository {
	return observedGroup{r.Repositories.Group(), r.reached, r.once}
}

func (r observedGroup) GetByID(ctx context.Context, id string) (*group.Group, error) {
	r.once.Do(func() { close(r.reached) })
	return r.IGroupRepository.GetByID(ctx, id)
}

func TestGroupInvitationConcurrentRevokeBlocksRegistration(t *testing.T) {
	s := setup(t)
	_, g, inv, token := invitationFixture(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	email := "revoke-race@example.com"
	if err := s.auth.SendCode(
		ctx,
		&dto.SendCodeReq{Email: email, GroupToken: token},
		"local",
	); err != nil {
		t.Fatal(err)
	}
	reached := make(chan struct{})
	auth := services.NewAuthApp(
		&services.AppContext{
			Repos:  observedGroupRepos{s.repos, reached, &sync.Once{}},
			Mailer: s.inbox,
		},
	)
	result := make(chan error, 1)
	err := s.repos.WithTransaction(ctx, func(repos ports.Repositories) error {
		if _, err := repos.Group().GetByID(ctx, g.ID); err != nil {
			return err
		}
		go func() {
			_, _, err := auth.Login(
				ctx,
				&dto.LoginReq{Email: email, Code: s.inbox.code(email), GroupToken: token},
			)
			result <- err
		}()
		// Login has read the valid invite but must wait for the revoker's lock.
		select {
		case <-reached:
		case <-ctx.Done():
			return ctx.Err()
		}
		return repos.Invite().Revoke(ctx, g.ID, inv.ID)
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = <-result; err != errcode.ErrInviteRevoked {
		t.Fatalf("concurrent revocation: %v", err)
	}
	if _, err = s.repos.User().GetByEmail(ctx, email); !errors.Is(err, errcode.ErrNotFound) {
		t.Fatal("revoked invitation registered an account", err)
	}
}
