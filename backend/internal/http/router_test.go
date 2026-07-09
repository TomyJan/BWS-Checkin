package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthzReturnsOK(t *testing.T) {
	handler := NewRouter(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "ok")
	}
}

func TestRouterServesFrontendFallback(t *testing.T) {
	handler := NewRouter(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/groups/example", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "BWS Checkin") {
		t.Fatalf("frontend body = %q", rec.Body.String())
	}
}

func TestAPIRouteDoesNotFallbackToFrontend(t *testing.T) {
	handler := NewRouter(Deps{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "BWS Checkin") {
		t.Fatalf("api route unexpectedly served frontend: %s", rec.Body.String())
	}
}

func TestRouterWritesStructuredRequestLog(t *testing.T) {
	var out bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: slog.LevelInfo}))
	handler := NewRouterWithLogger(Deps{}, logger)

	req := httptest.NewRequest(http.MethodGet, "/healthz?secret=hidden", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var entry map[string]any
	if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
		t.Fatalf("request log is not JSON: %v\n%s", err, out.String())
	}
	if entry["msg"] != "http_request" {
		t.Fatalf("msg = %v, want http_request", entry["msg"])
	}
	if entry["method"] != http.MethodGet {
		t.Fatalf("method = %v, want %s", entry["method"], http.MethodGet)
	}
	if entry["path"] != "/healthz" {
		t.Fatalf("path = %v, want /healthz", entry["path"])
	}
	if _, ok := entry["duration_ms"].(float64); !ok {
		t.Fatalf("duration_ms missing or not numeric: %#v", entry["duration_ms"])
	}
	if entry["status"] != float64(http.StatusOK) {
		t.Fatalf("status = %v, want %d", entry["status"], http.StatusOK)
	}
	if strings.Contains(out.String(), "secret=hidden") {
		t.Fatalf("request log leaked query string: %s", out.String())
	}
}
