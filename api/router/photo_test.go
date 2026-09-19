package router

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

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
