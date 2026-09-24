package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/domain/identity"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// fakeTrials records the invitation digests and expiries passed to the trial
// commands.
type fakeTrials struct {
	created       []string
	createdExpiry []time.Time
	revoked       []string
}

func (f *fakeTrials) Create(_ context.Context, hash string, expires time.Time) error {
	f.created = append(f.created, hash)
	f.createdExpiry = append(f.createdExpiry, expires)
	return nil
}

func (f *fakeTrials) Revoke(_ context.Context, hash string) error {
	f.revoked = append(f.revoked, hash)
	return nil
}

// TestIssueTrialWritesPrivateLink checks the written link, its length, the
// stored digest and the retention window against the freshly generated token.
func TestIssueTrialWritesPrivateLink(t *testing.T) {
	store := &fakeTrials{}
	path := filepath.Join(t.TempDir(), "trial-invitation.txt")
	before := time.Now().UTC()
	if err := IssueTrial(context.Background(), store, "http://127.0.0.1:8080", path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	link := strings.TrimSpace(string(raw))
	const prefix = "http://127.0.0.1:8080/login#trial="
	if !strings.HasPrefix(link, prefix) {
		t.Fatalf("unexpected invitation link %q", link)
	}
	token := strings.TrimPrefix(link, prefix)
	if len(token) != tokenLength {
		t.Fatalf("token length = %d, want %d", len(token), tokenLength)
	}
	if len(store.created) != 1 || store.created[0] != secure.Hash(token) {
		t.Fatalf("stored hashes = %v, want the hash of the written token", store.created)
	}
	if len(store.createdExpiry) != 1 ||
		store.createdExpiry[0].Before(before.Add(identity.TrialTTL-time.Minute)) {
		t.Fatalf("stored expiry = %v, want roughly seven days", store.createdExpiry)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %v, want 0600", info.Mode().Perm())
	}
}

// TestIssueTrialKeepsExistingFile checks that an existing invitation file is
// never overwritten and no invitation is stored.
func TestIssueTrialKeepsExistingFile(t *testing.T) {
	store := &fakeTrials{}
	path := filepath.Join(t.TempDir(), "trial-invitation.txt")
	if err := os.WriteFile(path, []byte("keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := IssueTrial(context.Background(), store, "http://127.0.0.1:8080", path); err == nil {
		t.Fatal("overwrote an existing invitation file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "keep\n" || len(store.created) != 0 {
		t.Fatalf("existing file changed: %q, stored %v", raw, store.created)
	}
}

// TestRevokeTrialConsumesInvitation checks that the token of the given file is
// the one revoked.
func TestRevokeTrialConsumesInvitation(t *testing.T) {
	store := &fakeTrials{}
	path := filepath.Join(t.TempDir(), "trial-invitation.txt")
	token := secure.NewID()
	if err := os.WriteFile(
		path,
		[]byte("http://127.0.0.1:8080/login#trial="+token+"\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := RevokeTrial(context.Background(), store, path); err != nil {
		t.Fatal(err)
	}
	if len(store.revoked) != 1 || store.revoked[0] != secure.Hash(token) {
		t.Fatalf("revoked hashes = %v, want the hash of the file token", store.revoked)
	}
}

// TestRevokeTrialRejectsInvalidFile checks that unusable files never revoke
// anything.
func TestRevokeTrialRejectsInvalidFile(t *testing.T) {
	for _, content := range []string{
		"",
		"not a link",
		"http://127.0.0.1:8080/login",
		"http://127.0.0.1:8080/login#trial=short",
		"http://127.0.0.1:8080/login#trial=%zz",
		"http://[::1",
	} {
		store := &fakeTrials{}
		path := filepath.Join(t.TempDir(), "trial-invitation.txt")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := RevokeTrial(context.Background(), store, path); err == nil {
			t.Errorf("accepted invalid invitation file %q", content)
		}
		if len(store.revoked) != 0 {
			t.Errorf("revoked %v from invalid file %q", store.revoked, content)
		}
	}
	if err := RevokeTrial(
		context.Background(),
		&fakeTrials{},
		filepath.Join(t.TempDir(), "missing.txt"),
	); err == nil {
		t.Error("accepted a missing invitation file")
	}
}

// TestRunTrialDispatch checks command selection, including the order between
// revoking and issuing.
func TestRunTrialDispatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "out.txt")
	store := &fakeTrials{}

	handled, err := runTrial(ctx, store, "http://127.0.0.1:8080", "", outputPath)
	if !handled || err != nil {
		t.Fatalf("issue trial: handled=%v err=%v", handled, err)
	}
	if len(store.created) != 1 {
		t.Fatalf("created = %v, want one invitation", store.created)
	}

	link, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	revokePath := filepath.Join(dir, "revoke.txt")
	if err := os.WriteFile(revokePath, link, 0o600); err != nil {
		t.Fatal(err)
	}
	handled, err = runTrial(ctx, store, "http://127.0.0.1:8080", revokePath, outputPath)
	if !handled || err != nil {
		t.Fatalf("revoke trial: handled=%v err=%v", handled, err)
	}
	if len(store.revoked) != 1 || len(store.created) != 1 {
		t.Fatalf("revoked = %v, created = %v, want revoking to win", store.revoked, store.created)
	}

	handled, err = runTrial(ctx, store, "http://127.0.0.1:8080", "", "")
	if handled || err != nil {
		t.Fatalf("no command: handled=%v err=%v", handled, err)
	}
}
