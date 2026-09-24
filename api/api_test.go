// Package api tests the HTTP assembly: the response envelope, the business code
// of failures and the availability of the generated documentation.
package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/superwhys/one-more-round/config"
	"github.com/superwhys/one-more-round/internal/errcode"
)

// TestStatusResponseEnvelope checks that a success response carries business code
// zero and the running build version.
func TestStatusResponseEnvelope(t *testing.T) {
	handler := NewAPI(
		"test-version",
		&config.Runtime{Origin: "http://localhost:8080"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).SetupRouter()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status: %d", rec.Code)
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != 0 || payload.Data.Version != "test-version" {
		t.Fatalf("unexpected payload: %s", rec.Body.String())
	}
}

// TestFailureKeepsBusinessCodeAndStatus checks that a rejected request reports
// the business code while the HTTP status carries the protocol semantics.
func TestFailureKeepsBusinessCodeAndStatus(t *testing.T) {
	handler := NewAPI(
		"test",
		&config.Runtime{Origin: "http://localhost:8080"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).SetupRouter()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/unknown", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status: %d", rec.Code)
	}
	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code == 0 || payload.Code == rec.Code {
		t.Fatalf("failure must carry a distinct business code: %s", rec.Body.String())
	}
}

// TestRejectedInputKeepsBusinessCodeAndStatus checks that a request rejected by
// binding or validation answers with the business code and status of this API
// instead of the default success envelope of the request binder.
func TestRejectedInputKeepsBusinessCodeAndStatus(t *testing.T) {
	handler := NewAPI(
		"test",
		&config.Runtime{Origin: "http://localhost:8080"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	).SetupRouter()
	rec := httptest.NewRecorder()
	// The invitation preview takes no session, so a missing token fails during
	// validation before any service is called.
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/group-invite",
		strings.NewReader("{}"),
	)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://localhost:8080")
	handler.ServeHTTP(rec, request)

	var payload struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusBadRequest || payload.Code != errcode.CodeBadRequest ||
		payload.Message != errcode.ErrBadRequest.Message {
		t.Fatalf(
			"rejected input: status=%d code=%d message=%q",
			rec.Code,
			payload.Code,
			payload.Message,
		)
	}
}

// TestSwaggerRouter checks that the generated documentation is served while
// production builds hide it.
func TestSwaggerRouter(t *testing.T) {
	api := NewAPI("test", &config.Runtime{}, nil, nil, nil, nil, nil, nil)
	docs := api.SwaggerRouter(false)
	for _, path := range []string{"/", "/doc.json"} {
		rec := httptest.NewRecorder()
		docs.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
			t.Fatalf("docs %s: %d", path, rec.Code)
		}
	}
	hidden := httptest.NewRecorder()
	api.SwaggerRouter(true).ServeHTTP(hidden, httptest.NewRequest(http.MethodGet, "/", nil))
	if hidden.Code != http.StatusNotFound {
		t.Fatalf("production must hide the docs: %d", hidden.Code)
	}
}
