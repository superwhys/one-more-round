package mysql_test

import (
	"context"
	"encoding/json"
	"io"
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
	"github.com/superwhys/one-more-round/web"
)

// Uses the real API and an isolated database/local photo directory, never OSS.
func TestPhotoLimitsBrowser(t *testing.T) {
	if os.Getenv("OMR_PHOTO_BROWSER_TEST") != "1" {
		t.Skip("set OMR_PHOTO_BROWSER_TEST=1 with Playwright and Chrome installed")
	}
	s := setup(t)
	ctx := context.Background()
	user, session := s.signup(t, "photo-browser@example.com")
	group, err := s.groups.Create(
		ctx,
		user.ID,
		&dto.CreateGroupReq{Name: "照片限制验收", PlayerName: "小林"},
	)
	if err != nil {
		t.Fatal(err)
	}
	game, err := s.groups.AddGame(ctx, group.ID, user.ID, &dto.AddGameReq{Name: "合作桌游"})
	if err != nil {
		t.Fatal(err)
	}
	photo, err := io.ReadAll(uploadData(t))
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
	backend := api.NewAPI("photo-browser-test", &config.Runtime{Origin: origin}, s.auth, s.groups, s.rounds, s.photos, s.notifications, s.comments).
		SetupRouter()
	mux.Handle("/api/", http.StripPrefix("/api", backend))
	mux.Handle("/", frontend)
	server.Start()
	defer server.Close()
	fixture, err := json.Marshal(
		map[string]any{
			"origin":   origin,
			"session":  session,
			"groupID":  group.ID,
			"userID":   user.ID,
			"gameID":   game.ID,
			"gameName": game.Name,
			"photo":    photo,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../../scripts/test-photo-limits.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Env = append(os.Environ(), "OMR_PHOTO_BROWSER_FIXTURE="+string(fixture))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("photo browser checks failed: %v\n%s", err, output)
	}
	t.Log(string(output))
}
