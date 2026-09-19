package photos_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/png"
	"io"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/infra/photos"
	"github.com/superwhys/one-more-round/internal/pkg/secure"
)

// Explicit opt-in: writes only two randomly named test objects and removes them.
// Credentials are read from an ignored config file and never included in output.
func TestOSSLive(t *testing.T) {
	path := os.Getenv("OMR_TEST_OSS_CONFIG")
	if path == "" {
		t.Skip("OMR_TEST_OSS_CONFIG not configured")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read OSS test config")
	}
	var c struct {
		App struct {
			OSS photos.OSSConfig `json:"oss"`
		} `json:"app"`
	}
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatal("invalid OSS test config")
	}
	store, err := photos.NewOSS(c.App.OSS)
	if err != nil {
		t.Fatal(err)
	}
	id := secure.NewID()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := store.Remove(ctx, id); err != nil {
			t.Errorf("test object cleanup failed (photo ID %s): %v", id, err)
		}
	})
	var upload bytes.Buffer
	if err := png.Encode(&upload, image.NewRGBA(image.Rect(0, 0, 900, 600))); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := store.Save(ctx, id, &upload); err != nil {
		t.Fatal("OSS upload failed:", err)
	}
	for _, thumb := range []bool{false, true} {
		content, err := store.Read(ctx, id, thumb)
		if err != nil {
			t.Fatal("OSS read failed:", err)
		}
		cfg, format, err := image.DecodeConfig(content.Body)
		_, drainErr := io.Copy(io.Discard, content.Body)
		closeErr := content.Body.Close()
		want := 900
		if thumb {
			want = 480
		}
		if err != nil || drainErr != nil || closeErr != nil || format != "jpeg" || cfg.Width != want {
			t.Fatal("OSS image verification failed")
		}
	}
	if err := store.Remove(ctx, id); err != nil {
		t.Fatal("OSS delete failed:", err)
	}
	for _, thumb := range []bool{false, true} {
		if _, err := store.Read(ctx, id, thumb); !errors.Is(err, fs.ErrNotExist) {
			t.Fatal("OSS deletion was not confirmed:", err)
		}
	}
}
