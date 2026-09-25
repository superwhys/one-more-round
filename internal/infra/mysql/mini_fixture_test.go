package mysql_test

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/app/services"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
	"github.com/superwhys/one-more-round/web"
)

// miniFixtureWechat lets DevTools exercise real account rules without sending a
// temporary code or any application secret to an external WeChat environment.
type miniFixtureWechat struct{}

// ExchangeCode keeps the same fixture identity across repeated wx.login calls.
func (miniFixtureWechat) ExchangeCode(context.Context, string) (ports.WechatIdentity, error) {
	return ports.WechatIdentity{AppID: "mini-ui-fixture", OpenID: "fixed-devtools-user"}, nil
}

// TestWechatMiniFixture is an explicitly enabled local-only interactive server.
// All credentials, mailbox retrieval and shutdown routes exist only in this test.
func TestWechatMiniFixture(t *testing.T) {
	if os.Getenv("OMR_MINI_FIXTURE") != "1" {
		t.Skip("set OMR_MINI_FIXTURE=1 through scripts/mini-fixture.sh for local DevTools")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatal("fixture requires unused localhost port 8080:", err)
	}
	defer listener.Close()
	s := setup(t)
	ctx := context.Background()
	const email = "mini-existing@example.test"
	const trial = "mini-ui-trial"
	owner, _ := s.signup(t, email)
	if err := s.client.Gorm.Exec("UPDATE omr_challenges SET sent=? WHERE email=?", time.Now().Add(-time.Minute), email).Error; err != nil {
		t.Fatal(err)
	}
	group, err := s.groups.Create(ctx, owner.ID, &dto.CreateGroupReq{Name: "开发者工具测试小组", PlayerName: "我"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.AddPlayer(ctx, group.ID, owner.ID, &dto.AddPlayerReq{Name: "好友"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "璀璨宝石"}); err != nil {
		t.Fatal(err)
	}
	_, groupToken, err := s.groups.Invite(ctx, group.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.repos.Trial().Create(ctx, secure.Hash(trial), time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	auth := services.NewAuthApp(&services.AppContext{Repos: s.repos, Mailer: s.inbox, Wechat: miniFixtureWechat{}})
	backend := api.NewAPI("mini-ui-fixture", &config.Runtime{Origin: "http://127.0.0.1:8080"}, auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).SetupRouter()
	frontend, err := web.NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", backend))
	mux.Handle("/", frontend)
	fixture := map[string]string{"origin": "http://127.0.0.1:8080", "trial": trial, "existing_email": email, "group_token": groupToken, "group_id": group.ID}
	mux.HandleFunc("GET /__test__/fixture", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fixture)
	})
	mux.HandleFunc("GET /__test__/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"code": s.inbox.code(r.URL.Query().Get("email"))})
	})
	stop, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()
	mux.HandleFunc("POST /__test__/stop", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
		cancel()
	})
	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	t.Logf("MINI_FIXTURE_READY origin=%s trial=%s existing_email=%s", fixture["origin"], trial, email)
	t.Log("Fixture: GET /__test__/fixture; mailbox: GET /__test__/code?email=...; stop: POST /__test__/stop")
	select {
	case <-stop.Done():
	case err = <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			t.Error(err)
		}
	}
	shutdown, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err = server.Shutdown(shutdown); err != nil {
		t.Error(err)
	}
}
