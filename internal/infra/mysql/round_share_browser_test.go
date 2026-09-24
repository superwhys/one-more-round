package mysql_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
	"github.com/superwhys/one-more-round/web"
)

// TestRoundShareBrowser exercises the authenticated share controls and the
// anonymous responsive page against the real API and a disposable database.
func TestRoundShareBrowser(t *testing.T) {
	if os.Getenv("OMR_SHARE_BROWSER_TEST") != "1" {
		t.Skip("set OMR_SHARE_BROWSER_TEST=1 with Playwright and Chrome installed")
	}
	s := setup(t)
	ctx := context.Background()
	owner, session := s.signup(t, "share-browser@example.com")
	group, err := s.groups.Create(
		ctx,
		owner.ID,
		&dto.CreateGroupReq{Name: "周五桌游组", PlayerName: "小林"},
	)
	if err != nil {
		t.Fatal(err)
	}
	player, err := s.groups.AddPlayer(ctx, group.ID, owner.ID, &dto.AddPlayerReq{Name: "阿周"})
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, group.ID, owner.ID, &dto.AddGameReq{Name: "璀璨宝石"})
	if err != nil {
		t.Fatal(err)
	}
	photo := s.uploadPhoto(t, group.ID, owner.ID)
	round, err := s.rounds.Save(
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
				Memory:  "最后一轮刚好凑齐",
				Photos:  []string{photo},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	frontend, err := web.NewHandler()
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	server := httptest.NewUnstartedServer(mux)
	origin := "http://" + server.Listener.Addr().String()
	backend := api.NewAPI("share-browser-test", &config.Runtime{Origin: origin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
		SetupRouter()
	mux.Handle("/api/", http.StripPrefix("/api", backend))
	mux.Handle("/", frontend)
	server.Start()
	defer server.Close()
	fixture, err := json.Marshal(
		map[string]string{
			"origin":     origin,
			"session":    session,
			"roundID":    round.ID,
			"groupName":  group.Name,
			"gameName":   game.Name,
			"playerName": player.Name,
			"memory":     round.Memory,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../../scripts/test-round-share.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Env = append(os.Environ(), "OMR_SHARE_BROWSER_FIXTURE="+string(fixture))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("share browser checks failed: %v\n%s", err, output)
	}
	t.Log(string(output))
}
