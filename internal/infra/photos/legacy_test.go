package photos

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyOSSReadsUsePrimaryThenThumbnail(t *testing.T) {
	for _, primary := range []bool{true, false} {
		var paths []string
		store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
			paths = append(paths, r.URL.Path)
			if !primary && !strings.HasSuffix(r.URL.Path, "-thumb.jpg") {
				w.WriteHeader(404)
				io.WriteString(w, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			io.WriteString(w, "legacy-image")
		})
		for _, thumb := range []bool{false, true} {
			paths = nil
			content, err := store.Read(context.Background(), testPhotoID, thumb)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(content.Body)
			content.Body.Close()
			if err != nil || !bytes.Equal(body, []byte("legacy-image")) {
				t.Fatalf("legacy read: %q %v", body, err)
			}
			want := 1
			if !primary {
				want = 2
			}
			if len(paths) != want || strings.HasSuffix(paths[0], "-thumb.jpg") {
				t.Fatalf("read order: %v", paths)
			}
		}
	}
}

func TestLegacyLocalThumbnailFallbackAndRemoval(t *testing.T) {
	store := &Files{Root: t.TempDir()}
	path := filepath.Join(store.Root, photoName(testPhotoID, true))
	if err := os.WriteFile(path, []byte("legacy-thumbnail"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, thumb := range []bool{false, true} {
		content, err := store.Read(context.Background(), testPhotoID, thumb)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(content.Body)
		content.Body.Close()
		if err != nil || string(data) != "legacy-thumbnail" {
			t.Fatalf("legacy read: %q %v", data, err)
		}
	}
	if err := store.Remove(context.Background(), testPhotoID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy file not removed: %v", err)
	}
}
