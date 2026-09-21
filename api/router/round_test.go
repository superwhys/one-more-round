package router

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestParseHasPhotos 覆盖有无照片筛选的取值：缺省为空表示不做限制。
func TestParseHasPhotos(t *testing.T) {
	cases := []struct {
		raw     string
		want    *bool
		wantErr bool
	}{
		{raw: "", want: nil},
		{raw: "true", want: boolPtr(true)},
		{raw: "1", want: boolPtr(true)},
		{raw: "false", want: boolPtr(false)},
		{raw: "0", want: boolPtr(false)},
		{raw: "maybe", wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseHasPhotos(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("%q: want error, got %v", tc.raw, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%q: unexpected error: %v", tc.raw, err)
		}
		switch {
		case tc.want == nil && got != nil:
			t.Fatalf("%q: want no restriction, got %v", tc.raw, *got)
		case tc.want != nil && got == nil:
			t.Fatalf("%q: want %v, got no restriction", tc.raw, *tc.want)
		case tc.want != nil && *got != *tc.want:
			t.Fatalf("%q: want %v, got %v", tc.raw, *tc.want, *got)
		}
	}
}

// TestListRoundsRejectsInvalidHasPhotos 非法的 has_photos 在参数层被拒绝，不触达应用服务。
func TestListRoundsRejectsInvalidHasPhotos(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(gin.RecoveryWithWriter(io.Discard))
	engine.GET("/groups/:group/rounds", listRoundsHandler(nil))

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/g1/rounds?has_photos=maybe", nil))
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusBadRequest || payload.Message != "请求内容无效" {
		t.Fatalf("status=%d message=%q", response.Code, payload.Message)
	}
}

// boolPtr 返回布尔值的指针，用于构造期望值。
func boolPtr(value bool) *bool { return &value }
