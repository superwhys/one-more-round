package wechat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/superwhys/one-more-round/internal/errcode"
)

// TestExchangeCode exercises provider validation and verifies that its secrets
// never escape in returned identity or errors.
func TestExchangeCode(t *testing.T) {
	for _, test := range []struct {
		name   string
		status int
		body   string
		want   error
	}{
		{"success", 200, `{"openid":"wx-person","session_key":"private-key","unionid":"unused"}`, nil},
		{"invalid-code", 200, `{"errcode":40029,"errmsg":"private-code"}`, errcode.ErrWechatCode},
		{"reused-code", 200, `{"errcode":40163}`, errcode.ErrWechatCode},
		{"provider-busy", 200, `{"errcode":-1}`, errcode.ErrWechatLogin},
		{"missing-identity", 200, `{"session_key":"private-key"}`, errcode.ErrWechatLogin},
		{"invalid-json", 200, `{`, errcode.ErrWechatLogin},
		{"oversized", 200, strings.Repeat(" ", 8192) + `{"openid":"wx-person"}`, errcode.ErrWechatLogin},
		{"failed-status", 503, `{"openid":"wx-person"}`, errcode.ErrWechatLogin},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("appid") != "app" || r.URL.Query().Get("secret") != "private-secret" ||
					r.URL.Query().Get("js_code") != "private-code" || r.URL.Query().Get("grant_type") != "authorization_code" {
					t.Error("missing server-side code2Session parameters")
				}
				w.WriteHeader(test.status)
				fmt.Fprint(w, test.body)
			}))
			defer server.Close()
			client := New(Config{AppID: "app", Secret: "private-secret"})
			client.endpoint = server.URL
			identity, err := client.ExchangeCode(context.Background(), "private-code")
			if err != test.want {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
			if err == nil && (identity.AppID != "app" || identity.OpenID != "wx-person") {
				t.Fatalf("identity = %#v", identity)
			}
			if err != nil && (strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), server.URL)) {
				t.Fatal("provider credentials leaked in error")
			}
		})
	}
}

// TestExchangeCodeTimeoutAndRedirect keeps both the exchange deadline and
// credentials inside the configured provider host.
func TestExchangeCodeTimeoutAndRedirect(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		client := New(Config{AppID: "app", Secret: "private-secret"})
		client.endpoint = server.URL
		client.http.Timeout = 20 * time.Millisecond
		if _, err := client.ExchangeCode(context.Background(), "private-code"); err != errcode.ErrWechatLogin {
			t.Fatalf("timeout error = %v", err)
		}
	})
	t.Run("redirect", func(t *testing.T) {
		target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { t.Error("followed provider redirect") }))
		defer target.Close()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, http.StatusFound) }))
		defer server.Close()
		client := New(Config{AppID: "app", Secret: "private-secret"})
		client.endpoint = server.URL
		if _, err := client.ExchangeCode(context.Background(), "private-code"); err != errcode.ErrWechatLogin {
			t.Fatalf("redirect error = %v", err)
		}
	})
}

// TestConfig rejects partial credentials while retaining optional email-only deployments.
func TestConfig(t *testing.T) {
	for _, config := range []Config{{AppID: "app"}, {Secret: "secret"}, {AppID: " app", Secret: "secret"}} {
		if config.Validate() == nil {
			t.Fatal("partial or whitespace credentials accepted")
		}
	}
	for _, config := range []Config{{}, {AppID: "app", Secret: "secret"}} {
		if err := config.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
