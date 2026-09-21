package router

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/miebyte/goutils/ginutils"

	"github.com/superwhys/one-more-round/api/common"
	"github.com/superwhys/one-more-round/internal/app/dto"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// bindRequest 在独立路由上执行一次自动绑定，返回绑定后的请求。body 非 nil 时按
// JSON 提交，headers 用于设置请求头。绑定或校验失败即视为测试失败。
func bindRequest[Q any](t *testing.T, method, route, target string, body any, headers map[string]string) *Q {
	t.Helper()
	var bound *Q
	engine := gin.New()
	engine.Any(route, ginutils.RequestHandler(func(ctx *gin.Context, req *Q) {
		bound = req
		ctx.JSON(http.StatusOK, nil)
	}))
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(encoded)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("%s: status=%d body=%s", target, response.Code, response.Body.String())
	}
	if bound == nil {
		t.Fatalf("%s: request was not bound", target)
	}
	return bound
}

// TestGroupPathBinding 只有分组路径参数的接口由绑定填充分组 ID。
func TestGroupPathBinding(t *testing.T) {
	req := bindRequest[dto.GroupPathReq](t, http.MethodGet, "/groups/:group", "/groups/g1", nil, nil)
	if req.GroupID != "g1" {
		t.Fatalf("group: want g1, got %q", req.GroupID)
	}
}

// TestRoundPathBinding 对局路径参数由绑定填充，且查询参数不能覆盖它们。
func TestRoundPathBinding(t *testing.T) {
	req := bindRequest[dto.RoundPathReq](t, http.MethodGet, "/groups/:group/rounds/:id", "/groups/g1/rounds/r1", nil, nil)
	if req.GroupID != "g1" || req.RoundID != "r1" {
		t.Fatalf("group=%q round=%q", req.GroupID, req.RoundID)
	}
	overridden := bindRequest[dto.RoundPathReq](t, http.MethodGet, "/groups/:group/rounds/:id", "/groups/g1/rounds/r1?GroupID=g2&RoundID=r2", nil, nil)
	if overridden.GroupID != "g1" || overridden.RoundID != "r1" {
		t.Fatalf("query overrode the path: group=%q round=%q", overridden.GroupID, overridden.RoundID)
	}
}

// TestRecapQueryBinding 回顾的周期来自查询参数，分组仍取自路径。
func TestRecapQueryBinding(t *testing.T) {
	req := bindRequest[dto.RecapReq](t, http.MethodGet, "/groups/:group/rounds/recap", "/groups/g1/rounds/recap?period=2026-09&GroupID=g2", nil, nil)
	if req.GroupID != "g1" || req.Period != "2026-09" {
		t.Fatalf("group=%q period=%q", req.GroupID, req.Period)
	}
}

// TestListRoundsQueryBinding 列表接口的筛选与分页由绑定填充：缺省 limit 为 30，
// has_photos 不参与自动绑定，以免空值被当成 false 筛选。
func TestListRoundsQueryBinding(t *testing.T) {
	route, target := "/groups/:group/rounds", "/groups/g1/rounds"
	req := bindRequest[dto.ListRoundsReq](t, http.MethodGet, route, target+"?from=&mode=&q=&has_photos=", nil, nil)
	if req.GroupID != "g1" || req.Limit != 30 {
		t.Fatalf("group=%q default limit=%d", req.GroupID, req.Limit)
	}
	if req.HasPhotos != nil {
		t.Fatalf("has_photos: want no restriction, got %v", *req.HasPhotos)
	}

	bounded := bindRequest[dto.ListRoundsReq](t, http.MethodGet, route, target+"?limit=100&offset=30&from=2026-01-01&mode=coop&q=%E6%89%93%E9%80%9A&GroupID=g2&has_photos=false", nil, nil)
	if bounded.GroupID != "g1" || bounded.Limit != 100 || bounded.Offset != 30 {
		t.Fatalf("group=%q limit=%d offset=%d", bounded.GroupID, bounded.Limit, bounded.Offset)
	}
	if bounded.From != "2026-01-01" || bounded.Mode != "coop" || bounded.Query != "打通" {
		t.Fatalf("filters: from=%q mode=%q q=%q", bounded.From, bounded.Mode, bounded.Query)
	}
	if bounded.HasPhotos != nil {
		t.Fatalf("has_photos still bound automatically: %v", *bounded.HasPhotos)
	}
}

// TestReadPhotoQueryBinding 照片读取的路径参数由绑定填充，查询仅控制缩略图。
func TestReadPhotoQueryBinding(t *testing.T) {
	route, target := "/groups/:group/photos/:id", "/groups/g1/photos/p1"
	req := bindRequest[dto.ReadPhotoReq](t, http.MethodGet, route, target+"?size=thumb&GroupID=g2&PhotoID=p2", nil, nil)
	if req.GroupID != "g1" || req.PhotoID != "p1" || req.Size != "thumb" {
		t.Fatalf("group=%q photo=%q size=%q", req.GroupID, req.PhotoID, req.Size)
	}
	empty := bindRequest[dto.ReadPhotoReq](t, http.MethodGet, route, target+"?size=", nil, nil)
	if empty.Size != "" {
		t.Fatalf("empty size: want no thumbnail, got %q", empty.Size)
	}
}

// TestPublicRoundBinding 分享令牌从请求头绑定，照片 ID 取自路径。
func TestPublicRoundBinding(t *testing.T) {
	header := map[string]string{"X-Round-Share": "token"}
	shared := bindRequest[dto.PublicRoundReq](t, http.MethodGet, "/shared-rounds", "/shared-rounds", nil, header)
	if shared.Token != "token" {
		t.Fatalf("token=%q", shared.Token)
	}
	photo := bindRequest[dto.PublicRoundPhotoReq](t, http.MethodGet, "/shared-rounds/photos/:photo", "/shared-rounds/photos/p1?PhotoID=p2", nil, header)
	if photo.Token != "token" || photo.PhotoID != "p1" {
		t.Fatalf("token=%q photo=%q", photo.Token, photo.PhotoID)
	}
	missing := bindRequest[dto.PublicRoundReq](t, http.MethodGet, "/shared-rounds", "/shared-rounds", nil, nil)
	if missing.Token != "" {
		t.Fatalf("token without header: want empty, got %q", missing.Token)
	}
}

// TestMalformedPagingReportsBusinessError 非法的分页参数在绑定阶段被拒绝，响应仍带业务码与 HTTP 状态。
func TestMalformedPagingReportsBusinessError(t *testing.T) {
	common.ConfigureRequestFailures()
	engine := gin.New()
	engine.Use(gin.RecoveryWithWriter(io.Discard))
	engine.GET("/groups/:group/rounds", listRoundsHandler(nil))

	response := httptest.NewRecorder()
	engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/groups/g1/rounds?offset=abc", nil))
	if status, code, message := failureEnvelope(t, response); status != http.StatusBadRequest || code != errcode.CodeBadRequest || message != errcode.ErrBadRequest.Message {
		t.Fatalf("status=%d code=%d message=%q", status, code, message)
	}
}

// TestRecapPeriodReportsBusinessError 缺失或非法的回顾周期由用例给出明确的业务错误，而不是绑定失败。
func TestRecapPeriodReportsBusinessError(t *testing.T) {
	common.ConfigureRequestFailures()
	cases := map[string]string{
		"/groups/g1/rounds/recap":                "回顾月份无效",
		"/groups/g1/rounds/recap?period=2026-13": "回顾月份无效",
		"/groups/g1/rounds/recap?period=abcd":    "回顾年份无效",
	}
	for target, want := range cases {
		engine := gin.New()
		engine.Use(gin.RecoveryWithWriter(io.Discard))
		engine.GET("/groups/:group/rounds/recap", recapHandler(nil))

		response := httptest.NewRecorder()
		engine.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if status, code, message := failureEnvelope(t, response); status != http.StatusBadRequest || code != errcode.CodeBadRequest || message != want {
			t.Fatalf("%s: status=%d code=%d message=%q", target, status, code, message)
		}
	}
}

// failureEnvelope 读出一次失败响应的 HTTP 状态、业务码与文案。
func failureEnvelope(t *testing.T, response *httptest.ResponseRecorder) (int, int, string) {
	t.Helper()
	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal %s: %v", response.Body.String(), err)
	}
	return response.Code, payload.Code, payload.Message
}

// TestPathGroupWinsOverBody 请求体不能改写路径里的分组，分组 ID 始终来自路径。
func TestPathGroupWinsOverBody(t *testing.T) {
	game := bindRequest[dto.AddGameReq](t, http.MethodPost, "/groups/:group/games", "/groups/g1/games", map[string]any{"name": "小世界", "GroupID": "g2"}, nil)
	if game.GroupID != "g1" || game.Name != "小世界" {
		t.Fatalf("group=%q name=%q", game.GroupID, game.Name)
	}
	manage := bindRequest[dto.ManageReq](t, http.MethodPost, "/groups/:group/manage", "/groups/g1/manage", map[string]any{"action": "rename", "value": "新名字", "GROUPID": "g2"}, nil)
	if manage.GroupID != "g1" || manage.Action != "rename" || manage.Value != "新名字" {
		t.Fatalf("group=%q action=%q value=%q", manage.GroupID, manage.Action, manage.Value)
	}
}
