package router

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestOversizedPhotoRejectedBeforeApplication(t *testing.T) {
	for _, size := range []int{2*1024*1024 + 1, 2*1024*1024 + 64*1024 + 1} {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		file, err := form.CreateFormFile("photo", "large.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(make([]byte, size)); err != nil {
			t.Fatal(err)
		}
		if err = form.Close(); err != nil {
			t.Fatal(err)
		}
		engine := gin.New()
		engine.Use(gin.RecoveryWithWriter(io.Discard))
		// A rejected body must not invoke the application or create upload metadata.
		engine.POST("/photos", uploadPhotoHandler(nil))
		request := httptest.NewRequest(http.MethodPost, "/photos", &body)
		request.Header.Set("Content-Type", form.FormDataContentType())
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		var payload struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
			t.Fatal(err)
		}
		if response.Code != http.StatusBadRequest || payload.Message != "请选择不超过 2 MB 的图片" {
			t.Fatalf("%d bytes: status=%d message=%q", size, response.Code, payload.Message)
		}
	}
}

type uploadFunc func(context.Context, string, string, io.Reader) (string, error)

func (fn uploadFunc) Upload(ctx context.Context, group, user string, r io.Reader) (string, error) {
	return fn(ctx, group, user, r)
}

type unreadUploadBody struct{ t *testing.T }

func (b unreadUploadBody) Read([]byte) (int, error) {
	b.t.Error("busy request body was read")
	return 0, io.EOF
}
func (unreadUploadBody) Close() error { return nil }

func TestUploadAdmissionAndRelease(t *testing.T) {
	for _, panicUpload := range []bool{false, true} {
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		engine := gin.New()
		engine.Use(gin.RecoveryWithWriter(io.Discard))
		engine.POST("/photos", uploadPhotoHandler(uploadFunc(func(context.Context, string, string, io.Reader) (string, error) {
			close(entered)
			<-release
			if panicUpload {
				panic("test panic")
			}
			return "photo", nil
		})))
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		file, err := form.CreateFormFile("photo", "test.png")
		if err != nil {
			t.Fatal(err)
		}
		file.Write([]byte("photo"))
		form.Close()
		first := httptest.NewRequest(http.MethodPost, "/photos", &body)
		first.Header.Set("Content-Type", form.FormDataContentType())
		go func() { defer close(done); engine.ServeHTTP(httptest.NewRecorder(), first) }()
		select {
		case <-entered:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("upload never entered")
		}
		blocked := httptest.NewRequest(http.MethodPost, "/photos", unreadUploadBody{t})
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, blocked)
		close(release)
		<-done
		if response.Code != http.StatusServiceUnavailable || response.Header().Get("Retry-After") != "2" {
			t.Fatalf("busy request: %d %s", response.Code, response.Body.String())
		}
		// Even a panic releases the slot. A subsequent malformed request is parsed.
		response = httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/photos", nil))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("slot leaked: %d", response.Code)
		}
	}
}
