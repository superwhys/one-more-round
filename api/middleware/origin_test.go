package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/superwhys/one-more-round/api/common"
)

// TestMiniProgramOriginBoundary ensures explicit mini-program transport cannot
// make ambient browser cookies bypass the existing CSRF boundary.
func TestMiniProgramOriginBoundary(t *testing.T) {
	const origin = "https://round.example"
	for _, test := range []struct {
		name, path, origin, cookie, auth, client, content string
		allow                                             bool
	}{
		{name: "browser", path: "/v1/groups", origin: origin, cookie: "omr_session=browser", allow: true},
		{name: "foreign-browser", path: "/v1/groups", origin: "https://evil.example", cookie: "omr_session=browser"},
		{name: "missing-origin-cookie", path: "/v1/groups", cookie: "omr_session=browser"},
		{name: "bearer", path: "/v1/groups", auth: "Bearer " + strings.Repeat("a", 64), allow: true},
		{name: "bearer-with-cookie", path: "/v1/groups", auth: "Bearer " + strings.Repeat("a", 64), cookie: "omr_session=browser"},
		{name: "bearer-foreign-origin", path: "/v1/groups", auth: "Bearer " + strings.Repeat("a", 64), origin: "https://evil.example"},
		{name: "invalid-bearer", path: "/v1/groups", auth: "Bearer invalid"},
		{name: "wechat-login", path: "/v1/auth/wx-login", client: "wechat-mini", content: "application/json", allow: true},
		{name: "email-code", path: "/v1/auth/code", client: "wechat-mini", content: "application/json; charset=utf-8", allow: true},
		{name: "invite-preview", path: "/v1/auth/group-invite", client: "wechat-mini", content: "application/json", allow: true},
		{name: "form-login", path: "/v1/auth/wx-login", client: "wechat-mini", content: "application/x-www-form-urlencoded"},
		{name: "cookie-login", path: "/v1/auth/wx-login", cookie: "omr_session=browser", client: "wechat-mini", content: "application/json"},
		{name: "missing-header", path: "/v1/auth/wx-login", content: "application/json"},
		{name: "unlisted-path", path: "/v1/groups", client: "wechat-mini", content: "application/json"},
		{name: "email-cookie-login", path: "/v1/auth/login", client: "wechat-mini", content: "application/json"},
	} {
		t.Run(test.name, func(t *testing.T) {
			router := gin.New()
			router.Use(OriginMiddleware(origin))
			router.POST(test.path, func(c *gin.Context) { c.Status(http.StatusNoContent) })
			request := httptest.NewRequest(http.MethodPost, test.path, nil)
			request.Header.Set("Origin", test.origin)
			request.Header.Set("Cookie", test.cookie)
			request.Header.Set("Authorization", test.auth)
			request.Header.Set("X-OMR-Client", test.client)
			request.Header.Set("Content-Type", test.content)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if (response.Code == http.StatusNoContent) != test.allow {
				t.Fatalf("status = %d, allow = %v", response.Code, test.allow)
			}
			if response.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("unexpected CORS access")
			}
		})
	}
}

// TestInvalidAuthorizationNeverUsesCookie rejects fallback to ambient credentials.
func TestInvalidAuthorizationNeverUsesCookie(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	context.Request.AddCookie(&http.Cookie{Name: common.SessionCookie, Value: "cookie-token"})
	context.Request.Header.Set("Authorization", "Bearer bad")
	if common.SessionToken(context) != "" {
		t.Fatal("invalid authorization fell back to cookie")
	}
}
