package httpapi

import (
	"bytes"
	"context"
	"crypto"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bws-checkin/backend/internal/bilibili"
	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/store"
	"bws-checkin/backend/internal/tasksync"
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

func TestSessionRejectsTamperedCookie(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, Session: SessionConfig{Secret: "test-secret"}})
	cookies := loginForTest(t, h, "TomyJan")
	if len(cookies) == 0 {
		t.Fatal("expected login cookie")
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	tampered := *cookies[0]
	tampered.Value = "999." + tampered.Value
	me.AddCookie(&tampered)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("me status = %d, want 401", w.Code)
	}
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

func TestOIDCLoginAndCallbackCreatesSession(t *testing.T) {
	s := newTestStore(t)
	issuer := newOIDCTestProvider(t)
	h := NewRouter(Deps{
		Store: s,
		OIDC: OIDCConfig{
			IssuerURL:         issuer.URL,
			ClientID:          "bws-client",
			ClientSecret:      "bws-secret",
			RedirectURL:       "http://app.test/auth/oidc/callback",
			PostLoginRedirect: "http://app.test/",
		},
	})

	login := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusFound {
		t.Fatalf("login status = %d, body = %s", w.Code, w.Body.String())
	}
	location := w.Header().Get("Location")
	authURL, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse auth redirect: %v", err)
	}
	if authURL.Path != "/authorize" {
		t.Fatalf("auth path = %q, want /authorize", authURL.Path)
	}
	if authURL.Query().Get("client_id") != "bws-client" {
		t.Fatalf("client_id = %q", authURL.Query().Get("client_id"))
	}
	if authURL.Query().Get("redirect_uri") != "http://app.test/auth/oidc/callback" {
		t.Fatalf("redirect_uri = %q", authURL.Query().Get("redirect_uri"))
	}
	if authURL.Query().Get("state") == "" {
		t.Fatal("state is empty")
	}

	callback := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=auth-code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	for _, c := range w.Result().Cookies() {
		callback.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, callback)
	if w.Code != http.StatusFound {
		t.Fatalf("callback status = %d, body = %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Location") != "http://app.test/" {
		t.Fatalf("callback location = %q", w.Header().Get("Location"))
	}

	me := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range w.Result().Cookies() {
		me.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "Alice") {
		t.Fatalf("expected OIDC display name in response, got %s", w.Body.String())
	}
}

func TestOIDCCallbackRejectsInvalidState(t *testing.T) {
	s := newTestStore(t)
	issuer := newOIDCTestProvider(t)
	h := NewRouter(Deps{
		Store: s,
		OIDC: OIDCConfig{
			IssuerURL:         issuer.URL,
			ClientID:          "bws-client",
			ClientSecret:      "bws-secret",
			RedirectURL:       "http://app.test/auth/oidc/callback",
			PostLoginRedirect: "http://app.test/",
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=auth-code&state=bad", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("callback status = %d, want 400", w.Code)
	}
}

func TestOIDCCallbackRejectsInvalidIDTokenIssuer(t *testing.T) {
	s := newTestStore(t)
	issuer := newOIDCTestProviderWithIDTokenIssuer(t, "https://wrong-issuer.example")
	h := NewRouter(Deps{
		Store: s,
		OIDC: OIDCConfig{
			IssuerURL:         issuer.URL,
			ClientID:          "bws-client",
			ClientSecret:      "bws-secret",
			RedirectURL:       "http://app.test/auth/oidc/callback",
			PostLoginRedirect: "http://app.test/",
		},
	})

	login := httptest.NewRequest(http.MethodGet, "/auth/oidc/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, login)
	if w.Code != http.StatusFound {
		t.Fatalf("login status = %d, body = %s", w.Code, w.Body.String())
	}
	authURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse auth redirect: %v", err)
	}

	callback := httptest.NewRequest(http.MethodGet, "/auth/oidc/callback?code=auth-code&state="+url.QueryEscape(authURL.Query().Get("state")), nil)
	for _, c := range w.Result().Cookies() {
		callback.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, callback)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("callback status = %d, want 502", w.Code)
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

func TestGroupTasksUseAuthenticatedQRImageAPI(t *testing.T) {
	s := newTestStore(t)
	uploadDir := t.TempDir()
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: uploadDir})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	memberID := userIDForCookies(t, h, memberCookies)

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", validPNG(t))
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/group/tasks?groupId=bw2026-fri", nil)
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if strings.Contains(w.Body.String(), "/uploads/") {
		t.Fatalf("group tasks must not expose uploads URL, got %s", w.Body.String())
	}
	qrURL := "/api/v1/user/qr?userId=" + memberID
	if !strings.Contains(w.Body.String(), qrURL) {
		t.Fatalf("expected authenticated QR API URL %q, got %s", qrURL, w.Body.String())
	}

	qrReq := httptest.NewRequest(http.MethodGet, qrURL, nil)
	for _, c := range ownerCookies {
		qrReq.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, qrReq)
	if w.Code != http.StatusOK {
		t.Fatalf("qr status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content type = %q, want image/png", ct)
	}
}

func TestBilibiliAccountRequiresLogin(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/account", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("account status = %d, want 401", w.Code)
	}
}

func TestBilibiliLoginQRCodeFlowBindsAccountAndGeneratesQR(t *testing.T) {
	s := newTestStore(t)
	biliServer := newBilibiliLoginTestServer(t)
	h := NewRouter(Deps{
		Store:                s,
		DevAuth:              true,
		UploadDir:            t.TempDir(),
		Bilibili:             bilibili.NewClient(bilibili.ClientOptions{PassportBaseURL: biliServer.URL, APIBaseURL: biliServer.URL, HTTPClient: biliServer.Client()}),
		BilibiliCookieSecret: "local-test-cookie-secret",
	})
	cookies := loginForTest(t, h, "TomyJan")
	userID := userIDForCookies(t, h, cookies)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/bilibili/login/qrcode/create", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "qr-key") || !strings.Contains(w.Body.String(), "https://passport.bilibili.com/qrcode") {
		t.Fatalf("unexpected qrcode create response: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"imageDataUrl":"data:image/png;base64,`) {
		t.Fatalf("expected qrcode image data URL in response: %s", w.Body.String())
	}

	req = jsonRequest(t, http.MethodPost, "/api/v1/bilibili/login/qrcode/poll", map[string]any{"qrcodeKey": "qr-key"})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "confirmed") {
		t.Fatalf("expected confirmed poll response, got %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/account", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), `"bound":true`) || !strings.Contains(w.Body.String(), `"mid":"123456"`) {
		t.Fatalf("unexpected account response: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "session-value") || strings.Contains(w.Body.String(), "refresh-token") {
		t.Fatalf("account response leaked secret: %s", w.Body.String())
	}

	req = jsonRequest(t, http.MethodPost, "/api/v1/me/qr/source/set", map[string]any{"source": "bilibili_generated"})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/user/qr?userId="+userID, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("generated qr status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("generated qr content type = %q, want image/png", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("generated qr has invalid png header: %x", w.Body.Bytes()[:4])
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/bilibili/account/unbind", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/bilibili/account", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), `"bound":false`) {
		t.Fatalf("expected unbound account response, got %s", w.Body.String())
	}
}

func TestTaskSyncAPI(t *testing.T) {
	s := newTestStore(t)
	syncer := tasksync.New(s, &httpTaskSource{tasks: []tasksync.Task{{
		ExternalID: "4001", GroupName: "8.1馆", Name: "官方任务", Title: "官方任务",
		VenueID: "1", VenueName: "8.1馆", EventDay: "20260710", SortOrder: 10,
	}}}, tasksync.Config{Now: func() time.Time { return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC) }})
	h := NewRouter(Deps{Store: s, DevAuth: true, TaskSync: syncer})
	cookies := loginForTest(t, h, "TomyJan")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/task/sync", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/task/sync/status", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "2026-07-10T12:00:00Z") {
		t.Fatalf("expected sync status timestamp, got %s", w.Body.String())
	}
}

func TestTaskImageProxyRequiresLoginAndUsesTaskImageURL(t *testing.T) {
	imageServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") == "" {
			t.Fatal("expected referer header for upstream task image request")
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(validPNG(t))
	}))
	t.Cleanup(imageServer.Close)

	s := newTestStore(t)
	if err := s.ReplaceBilibiliTasks(t.Context(), []store.SyncedTaskInput{{
		ID: "bilibili:4002:20260710:1", ExternalID: "4002", GroupName: "8.1馆",
		Name: "官方图片任务", Title: "官方图片任务", ImageURL: imageServer.URL + "/logo.png",
		VenueID: "1", VenueName: "8.1馆", EventDay: "20260710", SortOrder: 10,
	}}); err != nil {
		t.Fatalf("replace tasks: %v", err)
	}
	h := NewRouter(Deps{Store: s, DevAuth: true})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/task/image?taskId=bilibili:4002:20260710:1", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("image status = %d, want 401", w.Code)
	}

	cookies := loginForTest(t, h, "TomyJan")
	req = httptest.NewRequest(http.MethodGet, "/api/v1/task/image?taskId=bilibili:4002:20260710:1", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("image status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content type = %q, want image/png", ct)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("proxied image has invalid png header: %x", w.Body.Bytes()[:4])
	}
}

func TestGroupTaskSyncRequiresOwnerBilibiliAccount(t *testing.T) {
	s := newTestStore(t)
	syncer := tasksync.New(s, &httpTaskSource{tasks: []tasksync.Task{{
		ExternalID: "4101", GroupName: "1.1馆", Name: "创建者任务", Title: "创建者任务",
		VenueID: "2", VenueName: "1.1馆", EventDay: "20260711", SortOrder: 10,
	}}}, tasksync.Config{})
	h := NewRouter(Deps{Store: s, DevAuth: true, TaskSync: syncer})
	ownerCookies := loginForTest(t, h, "Owner")

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-day2", "name": "BW2026 7 月 11 日", "day": "20260711",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/task/sync", map[string]any{"groupId": "bw2026-day2"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("sync status = %d, body = %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"code":"creator_bilibili_account_required"`) {
		t.Fatalf("expected creator account error, got %s", w.Body.String())
	}
}

func TestGroupTaskSyncUsesOwnerBilibiliAccount(t *testing.T) {
	s := newTestStore(t)
	source := &httpAccountTaskSource{tasks: []tasksync.Task{{
		ExternalID: "4201", GroupName: "1.1馆", Name: "创建者任务", Title: "创建者任务",
		VenueID: "2", VenueName: "1.1馆", EventDay: "20260711", SortOrder: 10,
	}}}
	syncer := tasksync.New(s, source, tasksync.Config{})
	h := NewRouter(Deps{Store: s, DevAuth: true, TaskSync: syncer})
	ownerCookies := loginForTest(t, h, "Owner")
	ownerID := userIDForCookies(t, h, ownerCookies)
	if err := s.SaveBilibiliAccount(t.Context(), domain.BilibiliAccount{
		UserID: ownerID, MID: "123456", Uname: "OwnerBili", CookieCiphertext: "ciphertext",
	}); err != nil {
		t.Fatalf("save owner bilibili account: %v", err)
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-day2", "name": "BW2026 7 月 11 日", "day": "20260711",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/task/sync", map[string]any{"groupId": "bw2026-day2"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if source.accountUserID != ownerID {
		t.Fatalf("account user id = %q, want owner %q", source.accountUserID, ownerID)
	}
}

func TestManualTaskActionsRejectLiveCompletion(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	memberID := userIDForCookies(t, h, memberCookies)

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	if err := s.UpsertLiveTaskCompletion(t.Context(), store.LiveTaskCompletionInput{
		GroupID:       "bw2026-fri",
		TaskID:        "rainbow-station",
		TargetUserID:  memberID,
		Status:        domain.CompletionStatusLiveCompleted,
		LiveCheckedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("upsert live completion: %v", err)
	}

	for _, path := range []string{"/api/v1/task/complete", "/api/v1/task/uncomplete"} {
		req = jsonRequest(t, http.MethodPost, path, map[string]any{"groupId": "bw2026-fri", "taskId": "rainbow-station", "userId": memberID})
		for _, c := range ownerCookies {
			req.AddCookie(c)
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assertBusinessError(t, w, "live_completion_locked")
	}
}

func TestRefreshTaskStatusWritesLiveCompletion(t *testing.T) {
	s := newTestStore(t)
	biliServer := newBilibiliPointsStatusTestServer(t, true)
	h := NewRouter(Deps{
		Store:                s,
		DevAuth:              true,
		Bilibili:             bilibili.NewClient(bilibili.ClientOptions{APIBaseURL: biliServer.URL, PassportBaseURL: biliServer.URL, HTTPClient: biliServer.Client()}),
		BilibiliCookieSecret: "cookie-secret",
	})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	memberID := userIDForCookies(t, h, memberCookies)
	encryptedCookies, err := bilibili.EncryptCookieJar("cookie-secret", []*http.Cookie{{Name: "SESSDATA", Value: "session-value"}})
	if err != nil {
		t.Fatalf("encrypt cookies: %v", err)
	}
	if err := s.SaveBilibiliAccount(t.Context(), domain.BilibiliAccount{
		UserID:           memberID,
		MID:              "123456",
		Uname:            "member",
		CookieCiphertext: encryptedCookies,
	}); err != nil {
		t.Fatalf("save bilibili account: %v", err)
	}
	if err := s.ReplaceBilibiliTasks(t.Context(), []store.SyncedTaskInput{{
		ID: "bilibili:5001:20260710:1", ExternalID: "5001", GroupName: "8.1馆",
		Name: "官方任务", Title: "官方任务", VenueID: "1", VenueName: "8.1馆", EventDay: "20260710", SortOrder: 10,
	}}); err != nil {
		t.Fatalf("replace tasks: %v", err)
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/task/status/refresh", map[string]any{
		"groupId": "bw2026-fri", "taskId": "bilibili:5001:20260710:1", "userId": memberID,
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	entry := completionForMember(t, tasks[0], memberID)
	if entry.Status != domain.CompletionStatusLiveCompleted || entry.Source != domain.CompletionSourceLive || !entry.Completed {
		t.Fatalf("completion = %+v", entry)
	}
	if entry.CanToggle || !entry.CanRefresh {
		t.Fatalf("live controls = canToggle:%v canRefresh:%v", entry.CanToggle, entry.CanRefresh)
	}
}

func TestRefreshTaskStatusFailureMarksLiveStaleWithoutDowngrade(t *testing.T) {
	s := newTestStore(t)
	biliServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "remote failed", http.StatusBadGateway)
	}))
	defer biliServer.Close()
	h := NewRouter(Deps{
		Store:                s,
		DevAuth:              true,
		Bilibili:             bilibili.NewClient(bilibili.ClientOptions{APIBaseURL: biliServer.URL, PassportBaseURL: biliServer.URL, HTTPClient: biliServer.Client()}),
		BilibiliCookieSecret: "cookie-secret",
	})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	memberID := userIDForCookies(t, h, memberCookies)
	encryptedCookies, err := bilibili.EncryptCookieJar("cookie-secret", []*http.Cookie{{Name: "SESSDATA", Value: "session-value"}})
	if err != nil {
		t.Fatalf("encrypt cookies: %v", err)
	}
	if err := s.SaveBilibiliAccount(t.Context(), domain.BilibiliAccount{
		UserID:           memberID,
		MID:              "123456",
		Uname:            "member",
		CookieCiphertext: encryptedCookies,
	}); err != nil {
		t.Fatalf("save bilibili account: %v", err)
	}
	if err := s.ReplaceBilibiliTasks(t.Context(), []store.SyncedTaskInput{{
		ID: "bilibili:5001:20260710:1", ExternalID: "5001", GroupName: "8.1馆",
		Name: "官方任务", Title: "官方任务", VenueID: "1", VenueName: "8.1馆", EventDay: "20260710", SortOrder: 10,
	}}); err != nil {
		t.Fatalf("replace tasks: %v", err)
	}

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	if err := s.UpsertLiveTaskCompletion(t.Context(), store.LiveTaskCompletionInput{
		GroupID: "bw2026-fri", TaskID: "bilibili:5001:20260710:1", TargetUserID: memberID,
		Status: domain.CompletionStatusLiveCompleted, LiveCheckedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), UpdatedAt: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("upsert live completion: %v", err)
	}

	req = jsonRequest(t, http.MethodPost, "/api/v1/task/status/refresh", map[string]any{
		"groupId": "bw2026-fri", "taskId": "bilibili:5001:20260710:1", "userId": memberID,
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertBusinessError(t, w, "task_status_refresh_failed")

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	entry := completionForMember(t, tasks[0], memberID)
	if entry.Status != domain.CompletionStatusLiveCompleted || !entry.Completed || !entry.LiveStale {
		t.Fatalf("completion after failed refresh = %+v", entry)
	}
}

func TestJoinMissingGroupReturnsStableBusinessError(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	cookies := loginForTest(t, h, "TomyJan")

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "missing-group"})
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("join status = %d, want 200", w.Code)
	}
	assertBusinessError(t, w, "group_not_found")
	if strings.Contains(w.Body.String(), "sql:") {
		t.Fatalf("business error should not expose SQL detail, got %s", w.Body.String())
	}
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

func TestOwnerRemovesMemberAndMemberCannotRemove(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	ownerID := userIDForCookies(t, h, ownerCookies)
	memberID := userIDForCookies(t, h, memberCookies)

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/member/remove", map[string]any{
		"groupId": "bw2026-fri",
		"userId":  ownerID,
	})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("member remove status = %d, body = %s", w.Code, w.Body.String())
	}
	assertBusinessError(t, w, "owner_role_required")

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/member/remove", map[string]any{
		"groupId": "bw2026-fri",
		"userId":  ownerID,
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("owner self remove status = %d, body = %s", w.Code, w.Body.String())
	}
	assertBusinessError(t, w, "owner_remove_forbidden")

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/member/remove", map[string]any{
		"groupId": "bw2026-fri",
		"userId":  memberID,
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodGet, "/api/v1/group/tasks?groupId=bw2026-fri", nil)
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("tasks status = %d, body = %s", w.Code, w.Body.String())
	}
	tasks := decodeTasks(t, w)
	if len(tasks) == 0 {
		t.Fatal("expected tasks")
	}
	if tasks[0].TotalCount != 1 {
		t.Fatalf("total count = %d, want 1", tasks[0].TotalCount)
	}
	for _, entry := range tasks[0].Members {
		if entry.Member.ID == memberID {
			t.Fatalf("removed member is still listed in tasks: %+v", entry.Member)
		}
	}
}

func TestGroupManagementRequiresOwnerAndControlsJoinArchive(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	lateCookies := loginForTest(t, h, "Late")
	memberID := userIDForCookies(t, h, memberCookies)

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/update", map[string]any{
		"groupId": "bw2026-fri", "name": "BW2026 周六", "day": "saturday", "description": "更新后的说明",
	})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertBusinessError(t, w, "owner_role_required")

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/update", map[string]any{
		"groupId": "bw2026-fri", "name": "BW2026 周六", "day": "saturday", "description": "更新后的说明",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "BW2026 周六") || !strings.Contains(w.Body.String(), "更新后的说明") {
		t.Fatalf("expected updated group in response, got %s", w.Body.String())
	}

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join-lock", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range lateCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertBusinessError(t, w, "group_join_locked")

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join-unlock", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/archive", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range lateCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertBusinessError(t, w, "group_archived")

	req = jsonRequest(t, http.MethodPost, "/api/v1/task/complete", map[string]any{
		"groupId": "bw2026-fri", "taskId": "rainbow-station", "userId": memberID,
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertBusinessError(t, w, "group_archived")

	req = httptest.NewRequest(http.MethodGet, "/api/v1/group/list", nil)
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if strings.Contains(w.Body.String(), "bw2026-fri") {
		t.Fatalf("archived group should be hidden by default, got %s", w.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/group/list?includeArchived=1", nil)
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)
	if !strings.Contains(w.Body.String(), "bw2026-fri") {
		t.Fatalf("archived group should be listed with includeArchived=1, got %s", w.Body.String())
	}
}

func TestAuditLogsKeyBusinessActions(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: t.TempDir()})
	ownerCookies := loginForTest(t, h, "Owner")
	memberCookies := loginForTest(t, h, "Member")
	memberID := userIDForCookies(t, h, memberCookies)

	req := jsonRequest(t, http.MethodPost, "/api/v1/group/create", map[string]any{
		"id": "bw2026-fri", "name": "BW2026 周五", "day": "friday",
	})
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = jsonRequest(t, http.MethodPost, "/api/v1/group/join", map[string]any{"groupId": "bw2026-fri"})
	for _, c := range memberCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	actions := []struct {
		path string
		body map[string]any
	}{
		{"/api/v1/group/update", map[string]any{"groupId": "bw2026-fri", "name": "BW2026 周六", "day": "saturday"}},
		{"/api/v1/group/join-lock", map[string]any{"groupId": "bw2026-fri"}},
		{"/api/v1/group/join-unlock", map[string]any{"groupId": "bw2026-fri"}},
		{"/api/v1/task/complete", map[string]any{"groupId": "bw2026-fri", "taskId": "rainbow-station", "userId": memberID}},
		{"/api/v1/task/uncomplete", map[string]any{"groupId": "bw2026-fri", "taskId": "rainbow-station", "userId": memberID}},
		{"/api/v1/group/member/remove", map[string]any{"groupId": "bw2026-fri", "userId": memberID}},
	}
	for _, action := range actions {
		req = jsonRequest(t, http.MethodPost, action.path, action.body)
		for _, c := range ownerCookies {
			req.AddCookie(c)
		}
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		assertOK(t, w)
	}

	req = multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", validPNG(t))
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/qr/delete", nil)
	for _, c := range ownerCookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	assertOK(t, w)

	logs, err := s.AuditLogs(t.Context())
	if err != nil {
		t.Fatalf("audit logs: %v", err)
	}
	wantActions := []string{
		"group.update",
		"group.join_lock",
		"group.join_unlock",
		"task.complete",
		"task.uncomplete",
		"group.member_remove",
		"qr.upload",
		"qr.delete",
	}
	for _, want := range wantActions {
		if !hasAuditAction(logs, want) {
			t.Fatalf("missing audit action %q in %+v", want, logs)
		}
	}
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
	uploadDir := t.TempDir()
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: uploadDir})
	cookies := loginForTest(t, h, "TomyJan")
	userID := userIDForCookies(t, h, cookies)

	req := multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", validPNG(t))
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
	if !strings.Contains(w.Body.String(), "/api/v1/user/qr?userId="+userID) {
		t.Fatalf("expected QR API URL in response, got %s", w.Body.String())
	}
	if _, err := os.Stat(filepath.Join(uploadDir, userID+".png")); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}

	qrReq := httptest.NewRequest(http.MethodGet, "/api/v1/user/qr?userId="+userID, nil)
	for _, c := range cookies {
		qrReq.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, qrReq)
	if w.Code != http.StatusOK {
		t.Fatalf("qr status = %d, body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("content type = %q, want image/png", ct)
	}

	req = multipartRequest(t, "/api/v1/me/qr/upload", "qr.jpg", validJPEG(t))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("replace upload status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)
	if _, err := os.Stat(filepath.Join(uploadDir, userID+".png")); !os.IsNotExist(err) {
		t.Fatalf("expected old png to be removed after replacement, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(uploadDir, userID+".jpg")); err != nil {
		t.Fatalf("expected replacement jpg to exist: %v", err)
	}

	deleteReq := httptest.NewRequest(http.MethodPost, "/api/v1/me/qr/delete", nil)
	for _, c := range cookies {
		deleteReq.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, deleteReq)
	if w.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", w.Code, w.Body.String())
	}
	assertOK(t, w)
	if _, err := os.Stat(filepath.Join(uploadDir, userID+".jpg")); !os.IsNotExist(err) {
		t.Fatalf("expected uploaded file to be removed, stat err = %v", err)
	}

	me = httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range cookies {
		me.AddCookie(c)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, me)
	if w.Code != http.StatusOK {
		t.Fatalf("me status after delete = %d", w.Code)
	}
	assertOK(t, w)
	if strings.Contains(w.Body.String(), "/api/v1/user/qr") {
		t.Fatalf("expected qrImageUrl to be cleared, got %s", w.Body.String())
	}
}

func TestQRImageAPIRequiresLogin(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: t.TempDir()})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/user/qr?userId=missing", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("qr status = %d, want 401", w.Code)
	}
}

func TestQRUploadRejectsInvalidImageContent(t *testing.T) {
	s := newTestStore(t)
	h := NewRouter(Deps{Store: s, DevAuth: true, UploadDir: t.TempDir()})
	cookies := loginForTest(t, h, "TomyJan")

	req := multipartRequest(t, "/api/v1/me/qr/upload", "qr.png", []byte("not an image"))
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", w.Code, w.Body.String())
	}
	assertBusinessError(t, w, "invalid_image")
}

func TestBusinessErrorMessageSanitizesDatabaseDetails(t *testing.T) {
	databaseErrors := []string{
		"constraint failed: CHECK constraint failed: day IN ('friday', 'saturday', 'sunday') (275)",
		"UNIQUE constraint failed: groups.id",
		"SQL logic error: table tasks has no column named group_name (1)",
		"database is locked",
	}
	for _, fallback := range databaseErrors {
		got := businessErrorMessage("group_update_failed", fallback)
		if got != "操作失败，请稍后重试" {
			t.Fatalf("businessErrorMessage(%q) = %q, want generic message", fallback, got)
		}
		lower := strings.ToLower(got)
		if strings.Contains(lower, "constraint") || strings.Contains(lower, "sql") || strings.Contains(lower, "database") || strings.Contains(lower, "table") {
			t.Fatalf("sanitized message still leaks database detail: %q", got)
		}
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

type taskResponseItem struct {
	TotalCount int `json:"totalCount"`
	Members    []struct {
		Member struct {
			ID string `json:"id"`
		} `json:"member"`
	} `json:"members"`
}

func decodeTasks(t *testing.T, w *httptest.ResponseRecorder) []taskResponseItem {
	t.Helper()
	var body struct {
		OK   bool `json:"ok"`
		Data struct {
			Tasks []taskResponseItem `json:"tasks"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode tasks response: %v, body = %s", err, w.Body.String())
	}
	if !body.OK {
		t.Fatalf("ok = false, body = %s", w.Body.String())
	}
	return body.Data.Tasks
}

func completionForMember(t *testing.T, task domain.TaskStatus, userID string) domain.MemberCompletion {
	t.Helper()
	for _, entry := range task.Members {
		if entry.Member.ID == userID {
			return entry
		}
	}
	t.Fatalf("completion for user %s not found in %+v", userID, task.Members)
	return domain.MemberCompletion{}
}

func userIDForCookies(t *testing.T, h http.Handler, cookies []*http.Cookie) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", w.Code, w.Body.String())
	}
	var body struct {
		OK   bool `json:"ok"`
		Data struct {
			User struct {
				ID string `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode me response: %v, body = %s", err, w.Body.String())
	}
	if !body.OK {
		t.Fatalf("ok = false, body = %s", w.Body.String())
	}
	return body.Data.User.ID
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

func hasAuditAction(logs []store.AuditLog, action string) bool {
	for _, log := range logs {
		if log.Action == action {
			return true
		}
	}
	return false
}

type httpTaskSource struct {
	tasks []tasksync.Task
	err   error
}

func (s *httpTaskSource) FetchTasks(ctx context.Context) ([]tasksync.Task, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]tasksync.Task(nil), s.tasks...), nil
}

type httpAccountTaskSource struct {
	tasks         []tasksync.Task
	accountUserID string
}

func (s *httpAccountTaskSource) FetchTasks(ctx context.Context) ([]tasksync.Task, error) {
	return append([]tasksync.Task(nil), s.tasks...), nil
}

func (s *httpAccountTaskSource) FetchTasksForAccount(ctx context.Context, account domain.BilibiliAccount) ([]tasksync.Task, error) {
	s.accountUserID = account.UserID
	return append([]tasksync.Task(nil), s.tasks...), nil
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

func validPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func validJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.White)
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

func newBilibiliLoginTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/x/passport-login/web/qrcode/generate":
			writeJSON(w, http.StatusOK, map[string]any{
				"code": 0,
				"data": map[string]any{
					"url":        "https://passport.bilibili.com/qrcode",
					"qrcode_key": "qr-key",
				},
			})
		case "/x/passport-login/web/qrcode/poll":
			if got := r.URL.Query().Get("qrcode_key"); got != "qr-key" {
				t.Fatalf("qrcode_key = %q, want qr-key", got)
			}
			http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "session-value", Path: "/", HttpOnly: true})
			writeJSON(w, http.StatusOK, map[string]any{
				"code": 0,
				"data": map[string]any{
					"code":          0,
					"message":       "confirmed",
					"refresh_token": "refresh-token",
				},
			})
		case "/x/web-interface/nav":
			if got := r.Header.Get("Cookie"); !strings.Contains(got, "SESSDATA=session-value") {
				t.Fatalf("cookie header = %q", got)
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"code": 0,
				"data": map[string]any{
					"isLogin": true,
					"mid":     123456,
					"uname":   "bws-user",
					"face":    "https://example.com/face.png",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newBilibiliPointsStatusTestServer(t *testing.T, completed bool) *httptest.Server {
	t.Helper()
	isPoint := 0
	if completed {
		isPoint = 1
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/activity/bws/offline/points" {
			http.NotFound(w, r)
			return
		}
		if got := r.Header.Get("Cookie"); got != "SESSDATA=session-value" {
			t.Fatalf("cookie header = %q", got)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"code": 0,
			"data": map[string]any{
				"points_list": map[string]any{
					"20260710": map[string]any{
						"points": []map[string]any{
							{"id": 5001, "name": "官方任务", "unlocked": 3, "is_point": isPoint, "dic": "说明"},
						},
					},
				},
			},
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func newOIDCTestProvider(t *testing.T) *httptest.Server {
	return newOIDCTestProviderWithIDTokenIssuer(t, "")
}

func newOIDCTestProviderWithIDTokenIssuer(t *testing.T, idTokenIssuer string) *httptest.Server {
	t.Helper()
	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeJSON(w, http.StatusOK, map[string]string{
				"authorization_endpoint": server.URL + "/authorize",
				"token_endpoint":         server.URL + "/token",
				"userinfo_endpoint":      server.URL + "/userinfo",
				"jwks_uri":               server.URL + "/jwks",
			})
		case "/jwks":
			writeJSON(w, http.StatusOK, map[string]any{
				"keys": []map[string]string{
					{
						"kty": "RSA",
						"use": "sig",
						"kid": "test-key",
						"alg": "RS256",
						"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
						"e":   base64.RawURLEncoding.EncodeToString(bigEndianBytes(key.E)),
					},
				},
			})
		case "/token":
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse token form: %v", err)
			}
			if r.Form.Get("code") != "auth-code" {
				t.Fatalf("token code = %q", r.Form.Get("code"))
			}
			issuer := idTokenIssuer
			if issuer == "" {
				issuer = server.URL
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"access_token": "access-token",
				"id_token":     signedIDToken(t, key, issuer, "bws-client", "oidc-user-1"),
				"token_type":   "Bearer",
			})
		case "/userinfo":
			if r.Header.Get("Authorization") != "Bearer access-token" {
				t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
			}
			writeJSON(w, http.StatusOK, map[string]string{
				"sub":   "oidc-user-1",
				"name":  "Alice",
				"email": "alice@example.test",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func signedIDToken(t *testing.T, key *rsa.PrivateKey, issuer string, audience string, subject string) string {
	t.Helper()
	header := map[string]string{"alg": "RS256", "typ": "JWT", "kid": "test-key"}
	claims := map[string]any{
		"iss": issuer,
		"aud": audience,
		"sub": subject,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Add(-time.Minute).Unix(),
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("marshal id token header: %v", err)
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal id token claims: %v", err)
	}
	signingInput := base64.RawURLEncoding.EncodeToString(headerJSON) + "." + base64.RawURLEncoding.EncodeToString(claimsJSON)
	sum := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(cryptorand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("sign id token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func bigEndianBytes(value int) []byte {
	if value == 0 {
		return []byte{0}
	}
	var out []byte
	for value > 0 {
		out = append([]byte{byte(value & 0xff)}, out...)
		value >>= 8
	}
	return out
}
