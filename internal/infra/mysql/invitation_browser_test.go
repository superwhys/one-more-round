package mysql_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/miebyte/goutils/mysqlutils"
	"github.com/superwhys/one-more-round/api"
	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/infra/mail"
	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/web"
)

// This opt-in test serves the real embedded frontend and API against a disposable
// database. The inbox route exists only in this test server, never in the app.
func TestGroupInvitationBrowser(t *testing.T) {
	if os.Getenv("OMR_BROWSER_TEST") != "1" {
		t.Skip("set OMR_BROWSER_TEST=1 with Playwright and Chrome installed")
	}
	s := setup(t)
	owner, g, _, token := invitationFixture(t, s)
	inv, revoked, err := s.groups.Invite(context.Background(), g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.groups.Manage(context.Background(), g.ID, owner.ID, &dto.ManageReq{Action: "revoke", Target: inv.ID}); err != nil {
		t.Fatal(err)
	}
	inv, expired, err := s.groups.Invite(context.Background(), g.ID, owner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.client.Gorm.Exec("UPDATE omr_invites SET expires=? WHERE id=?", time.Now().Add(-time.Hour), inv.ID).Error; err != nil {
		t.Fatal(err)
	}
	other, err := s.groups.Create(context.Background(), owner.ID, &dto.CreateGroupReq{Name: "周末合作局", PlayerName: "组主"})
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := s.groups.Invite(context.Background(), other.ID, owner.ID)
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
	backend := api.NewAPI("browser-test", &config.Runtime{Origin: origin}, s.auth, s.groups, s.rounds, s.photos, s.notifications).SetupRouter()
	mux.Handle("/api/", http.StripPrefix("/api", backend))
	mux.HandleFunc("/__test__/code", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		json.NewEncoder(w).Encode(map[string]string{"code": s.inbox.code(r.URL.Query().Get("email"))})
	})
	mux.Handle("/", frontend)
	server.Start()
	defer server.Close()
	fixture, err := json.Marshal(map[string]string{"origin": origin, "token": token, "revoked": revoked, "expired": expired, "second": second, "name": g.Name, "secondName": other.Name})
	if err != nil {
		t.Fatal(err)
	}
	script, err := filepath.Abs("../../../scripts/test-invitations.mjs")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", script)
	cmd.Env = append(os.Environ(), "OMR_BROWSER_FIXTURE="+string(fixture))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("invitation browser checks failed: %v\n%s", err, output)
	}
	t.Log(string(output))
}

func TestInvitationStandaloneBinary(t *testing.T) {
	binary := os.Getenv("OMR_TEST_BINARY")
	if binary == "" {
		t.Skip("set OMR_TEST_BINARY to a freshly built binary")
	}
	s := setup(t)
	dir := t.TempDir()
	data, err := os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	binary = filepath.Join(dir, "one-more-round")
	if err = os.WriteFile(binary, data, 0700); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	listener.Close()
	origin := "http://" + address
	conf, err := json.Marshal(map[string]any{"app": config.Runtime{
		Origin: origin,
		OSS:    photos.OSSConfig{Bucket: "test-bucket", Region: "cn-shenzhen", Endpoint: "https://oss-cn-shenzhen.aliyuncs.com", Prefix: "image/", AccessID: "test-id", AccessSecret: "test-secret"},
		MySQL:  mysqlutils.MysqlConfig{Instance: os.Getenv("OMR_TEST_MYSQL"), Database: s.client.Gorm.Migrator().CurrentDatabase(), Username: "root"},
		SMTP:   mail.Config{Host: "127.0.0.1", Port: 2525, From: "test@example.com"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(dir, "config.json")
	if err = os.WriteFile(configFile, conf, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	cmd := exec.CommandContext(ctx, binary, "--configFile", configFile, "--listen", address)
	cmd.Dir = dir
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err = cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); cmd.Wait() })
	client := &http.Client{Timeout: time.Second}
	for {
		resp, e := client.Get(origin + "/api/v1/status")
		if e == nil {
			resp.Body.Close()
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("standalone server did not start")
		case <-time.After(50 * time.Millisecond):
		}
	}
	var index []byte
	for _, check := range []struct {
		path   string
		status int
	}{
		{"/", 200}, {"/join", 200}, {"/login", 200}, {"/group", 200},
		{"/api/v1/status", 200}, {"/api/v1/me", 401}, {"/api/v1/missing", 404},
		{"/assets/missing.js", 404}, {"/uploads/missing.jpg", 404},
	} {
		req, _ := http.NewRequest(http.MethodGet, origin+check.path, nil)
		req.Header.Set("Accept", "text/html")
		resp, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(resp.Body)
		resp.Body.Close()
		if e != nil || resp.StatusCode != check.status {
			t.Fatalf("%s: status=%d err=%v", check.path, resp.StatusCode, e)
		}
		if check.path == "/" {
			index = body
		}
	}
	asset := regexp.MustCompile(`src="(/assets/[^"]+\.js)"`).FindSubmatch(index)
	if len(asset) != 2 {
		t.Fatal("embedded entry script missing")
	}
	resp, err := client.Get(origin + string(asset[1]))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatal("embedded asset not served")
	}
}
