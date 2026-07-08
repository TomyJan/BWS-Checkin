package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bws-checkin/backend/internal/store"
)

func TestDevLoginAndMe(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})

	login := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login?name=TomyJan", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d", w.Code)
	}
	assertOK(t, w)

	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range w.Result().Cookies() {
		me.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusOK {
		t.Fatalf("me status = %d", w.Code)
	}
	assertOK(t, w)
}

func TestMeRequiresLogin(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestGroupsAndTasksFlow(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	cookies := loginForTest(t, h, "TomyJan")

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id":          "bw2026-fri",
		"name":        "BW2026 周五",
		"day":         "friday",
		"description": "测试组",
	})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("create group status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/group/tasks?groupId=bw2026-fri", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tasks status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)
}

func TestDuplicateGroupReturnsBusinessErrorWithHTTP200(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	cookies := loginForTest(t, h, "TomyJan")

	body := map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday",
	}
	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", body)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/create", body)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("duplicate group status = %d, want 200", w.Code)
	}
	assertBusinessError(t, w, "group_id_conflict")
}

func TestQRUploadRequiresLogin(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: t.TempDir()})

	req := multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", []byte{0x89, 'P', 'N', 'G'})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
}

func TestQRUploadUpdatesCurrentUser(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: t.TempDir()})
	cookies := loginForTest(t, h, "TomyJan")

	req := multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", []byte{0x89, 'P', 'N', 'G'})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)

	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range cookies {
		me.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusOK {
		t.Fatalf("me status = %d", w.Code)
	}
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "/uploads/") {
		t.Fatalf("expected qrImageUrl in response, got %s", w.Body.String())
	}
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func loginForTest(t *testing.T, h http.Handler, name string) []*http.Cookie {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/dev/login?name="+name, nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status = %d", w.Code)
	}
	return w.Result().Cookies()
}

func assertOK(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var body struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v, body = %s", err, w.Body.String())
	}
	if !body.OK {
		t.Fatalf("ok = false, body = %s", w.Body.String())
	}
}

func assertBusinessError(t *testing.T, w *httptest.ResponseRecorder, code string) {
	t.Helper()
	var body struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v, body = %s", err, w.Body.String())
	}
	if body.OK {
		t.Fatalf("ok = true, want false, body = %s", w.Body.String())
	}
	if body.Error.Code != code {
		t.Fatalf("error code = %q, want %q, body = %s", body.Error.Code, code, w.Body.String())
	}
}

func jsonRequest(t *testing.T, method, path string, body any) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode request body: %v", err)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func multipartRequest(t *testing.T, path string, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
