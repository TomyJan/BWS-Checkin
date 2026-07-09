package httpapi

import (
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
