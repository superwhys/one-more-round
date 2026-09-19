package photos

import (
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"
)

func TestOSSReadReturnsBeforeBodyCompletes(t *testing.T) {
	release := make(chan struct{})
	store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "6")
		io.WriteString(w, "abc")
		w.(http.Flusher).Flush()
		select {
		case <-release:
			io.WriteString(w, "def")
		case <-r.Context().Done():
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	content, err := store.Read(ctx, testPhotoID, false)
	if err != nil {
		t.Fatalf("opening must not wait for the complete body: %v", err)
	}
	defer content.Body.Close()
	if content.Size != 6 {
		t.Fatalf("content length lost: %d", content.Size)
	}
	chunk := make([]byte, 3)
	if _, err := io.ReadFull(content.Body, chunk); err != nil || string(chunk) != "abc" {
		t.Fatalf("first chunk unavailable after opening: %q %v", chunk, err)
	}
	close(release)
	if _, err := io.ReadFull(content.Body, chunk); err != nil || string(chunk) != "def" {
		t.Fatalf("remaining body unavailable: %q %v", chunk, err)
	}
	if _, err := content.Body.Read(chunk); !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected end of stream: %v", err)
	}
}

func TestOSSStreamCancellationAndCloseStopUpstream(t *testing.T) {
	for _, action := range []string{"cancel", "close"} {
		t.Run(action, func(t *testing.T) {
			upstreamDone := make(chan struct{})
			store := testOSS(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "1")
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				<-r.Context().Done()
				close(upstreamDone)
			})
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			content, err := store.Read(ctx, testPhotoID, false)
			if err != nil {
				t.Fatal(err)
			}
			defer content.Body.Close()
			readDone := make(chan error, 1)
			go func() { _, err := content.Body.Read(make([]byte, 1)); readDone <- err }()
			if action == "cancel" {
				cancel()
			} else if err := content.Body.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-readDone:
				if err == nil {
					t.Fatal("canceled stream returned success")
				}
				if action == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("cancellation lost: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("body read did not stop")
			}
			select {
			case <-upstreamDone:
			case <-time.After(time.Second):
				t.Fatal("upstream request not canceled")
			}
		})
	}
}
