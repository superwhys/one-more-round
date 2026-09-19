package router

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/superwhys/one-more-round/internal/app/ports"
	"github.com/superwhys/one-more-round/internal/errcode"
)

type photoReadFunc func(context.Context, string, string, string, bool) (*ports.PhotoContent, error)

func (f photoReadFunc) Read(ctx context.Context, group, user, id string, thumb bool) (*ports.PhotoContent, error) {
	return f(ctx, group, user, id, thumb)
}

type checkedPhotoBody struct {
	read   func([]byte) (int, error)
	closed bool
}

func (b *checkedPhotoBody) Read(p []byte) (int, error) { return b.read(p) }
func (b *checkedPhotoBody) Close() error               { b.closed = true; return nil }

func servePhoto(reader photoReader, writer http.ResponseWriter, query string) {
	engine := gin.New()
	engine.GET("/groups/:group/photos/:id", readPhotoHandler(reader))
	engine.ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/groups/group/photos/photo"+query, nil))
}

func TestPhotoHandlerForwardsChunksAndClosesBody(t *testing.T) {
	for _, thumb := range []bool{false, true} {
		response := httptest.NewRecorder()
		const size = 1024 * 1024
		read := 0
		body := &checkedPhotoBody{read: func(p []byte) (int, error) {
			if read != response.Body.Len() {
				return 0, errors.New("body was buffered instead of forwarded")
			}
			if read == size {
				return 0, io.EOF
			}
			n := min(len(p), 4096, size-read)
			for i := range p[:n] {
				p[i] = 'x'
			}
			read += n
			return n, nil
		}}
		reader := photoReadFunc(func(_ context.Context, group, user, id string, small bool) (*ports.PhotoContent, error) {
			if group != "group" || id != "photo" || small != thumb {
				t.Error("photo route parameters changed")
			}
			return &ports.PhotoContent{Body: body, Size: size}, nil
		})
		query := ""
		if thumb {
			query = "?size=thumb"
		}
		servePhoto(reader, response, query)
		if response.Code != 200 || response.Body.Len() != size || !body.closed {
			t.Fatalf("incomplete/unclosed stream: status=%d bytes=%d closed=%t", response.Code, response.Body.Len(), body.closed)
		}
		for key, want := range map[string]string{"Content-Type": "image/jpeg", "Content-Length": strconv.Itoa(size), "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff"} {
			if got := response.Header().Get(key); got != want {
				t.Fatalf("%s=%q; want %q", key, got, want)
			}
		}
	}
}

func TestPhotoStreamFailuresCloseWithoutAppendingJSON(t *testing.T) {
	for _, started := range []bool{false, true} {
		response := httptest.NewRecorder()
		first := true
		body := &checkedPhotoBody{read: func(p []byte) (int, error) {
			if started && first {
				first = false
				return copy(p, "jpeg"), nil
			}
			return 0, errcode.ErrPhotoStorage
		}}
		servePhoto(photoReadFunc(func(context.Context, string, string, string, bool) (*ports.PhotoContent, error) {
			return &ports.PhotoContent{Body: body, Size: 100}, nil
		}), response, "")
		if !body.closed {
			t.Fatal("failed stream not closed")
		}
		if started {
			if response.Code != 200 || response.Body.String() != "jpeg" || response.Header().Get("Content-Length") != "100" {
				t.Fatal("error JSON appended to a partial image")
			}
		} else {
			if response.Code != 503 || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/json") || response.Header().Get("Content-Length") != "" {
				t.Fatalf("initial read error did not become a valid error response: %d %v", response.Code, response.Header())
			}
		}
	}
}

type disconnectedWriter struct{ header http.Header }

func (w *disconnectedWriter) Header() http.Header       { return w.header }
func (w *disconnectedWriter) WriteHeader(int)           {}
func (w *disconnectedWriter) Write([]byte) (int, error) { return 0, errors.New("client disconnected") }

func TestPhotoClientWriteFailureClosesBody(t *testing.T) {
	reads := 0
	body := &checkedPhotoBody{read: func(p []byte) (int, error) { reads++; return copy(p, "chunk"), nil }}
	reader := photoReadFunc(func(context.Context, string, string, string, bool) (*ports.PhotoContent, error) {
		return &ports.PhotoContent{Body: body, Size: 100}, nil
	})
	servePhoto(reader, &disconnectedWriter{header: make(http.Header)}, "")
	if !body.closed || reads != 1 {
		t.Fatalf("disconnected client did not stop stream: closed=%t reads=%d", body.closed, reads)
	}
}
