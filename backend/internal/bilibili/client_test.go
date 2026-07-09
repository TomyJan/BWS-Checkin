package bilibili_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bws-checkin/backend/internal/bilibili"
)

func TestCreateLoginQRCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/passport-login/web/qrcode/generate" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		writeJSON(t, w, map[string]any{
			"code": 0,
			"data": map[string]any{
				"url":        "https://passport.bilibili.com/qrcode",
				"qrcode_key": "qr-key",
			},
		})
	}))
	defer server.Close()

	client := bilibili.NewClient(bilibili.ClientOptions{
		PassportBaseURL: server.URL,
		APIBaseURL:      server.URL,
		HTTPClient:      server.Client(),
	})
	got, err := client.CreateLoginQRCode(t.Context())
	if err != nil {
		t.Fatalf("create login qrcode: %v", err)
	}
	if got.URL != "https://passport.bilibili.com/qrcode" || got.QRCodeKey != "qr-key" {
		t.Fatalf("qrcode = %+v", got)
	}
	if got.ExpiresAt.Before(time.Now().Add(2 * time.Minute)) {
		t.Fatalf("expires at = %v, want a future ttl", got.ExpiresAt)
	}
}

func TestPollLoginQRCodeStatusMappingAndCookies(t *testing.T) {
	tests := []struct {
		name       string
		biliCode   int
		wantStatus string
	}{
		{name: "pending scan", biliCode: 86101, wantStatus: bilibili.LoginStatusPendingScan},
		{name: "pending confirm", biliCode: 86090, wantStatus: bilibili.LoginStatusPendingConfirm},
		{name: "expired", biliCode: 86038, wantStatus: bilibili.LoginStatusExpired},
		{name: "confirmed", biliCode: 0, wantStatus: bilibili.LoginStatusConfirmed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/x/passport-login/web/qrcode/poll" {
					t.Fatalf("path = %s", r.URL.Path)
				}
				if got := r.URL.Query().Get("qrcode_key"); got != "qr-key" {
					t.Fatalf("qrcode_key = %q, want qr-key", got)
				}
				if tt.biliCode == 0 {
					http.SetCookie(w, &http.Cookie{Name: "SESSDATA", Value: "session-value", Domain: ".bilibili.com", Path: "/"})
				}
				writeJSON(t, w, map[string]any{
					"code": 0,
					"data": map[string]any{
						"code":          tt.biliCode,
						"message":       "status",
						"refresh_token": "refresh-token",
					},
				})
			}))
			defer server.Close()

			client := bilibili.NewClient(bilibili.ClientOptions{
				PassportBaseURL: server.URL,
				APIBaseURL:      server.URL,
				HTTPClient:      server.Client(),
			})
			got, err := client.PollLoginQRCode(t.Context(), "qr-key")
			if err != nil {
				t.Fatalf("poll login qrcode: %v", err)
			}
			if got.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.Status, tt.wantStatus)
			}
			if tt.biliCode == 0 {
				if got.RefreshToken != "refresh-token" {
					t.Fatalf("refresh token = %q", got.RefreshToken)
				}
				if len(got.Cookies) != 1 || got.Cookies[0].Name != "SESSDATA" || got.Cookies[0].Value != "session-value" {
					t.Fatalf("cookies = %+v", got.Cookies)
				}
			}
		})
	}
}

func TestNav(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/web-interface/nav" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if got := r.Header.Get("Cookie"); !strings.Contains(got, "SESSDATA=session-value") {
			t.Fatalf("cookie header = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"code": 0,
			"data": map[string]any{
				"isLogin": true,
				"mid":     123456,
				"uname":   "bws-user",
				"face":    "https://example.com/face.png",
			},
		})
	}))
	defer server.Close()

	client := bilibili.NewClient(bilibili.ClientOptions{
		PassportBaseURL: server.URL,
		APIBaseURL:      server.URL,
		HTTPClient:      server.Client(),
	})
	got, err := client.Nav(t.Context(), []*http.Cookie{{Name: "SESSDATA", Value: "session-value"}})
	if err != nil {
		t.Fatalf("nav: %v", err)
	}
	if got.MID != "123456" || got.Uname != "bws-user" || got.FaceURL != "https://example.com/face.png" {
		t.Fatalf("nav user = %+v", got)
	}
}

func TestOfflinePoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/x/activity/bws/offline/points" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("bid") != "202601" || r.URL.Query().Get("year") != "202601" || r.URL.Query().Get("fid") != "1" || r.URL.Query().Get("day") != "20260710" {
			t.Fatalf("query = %s", r.URL.RawQuery)
		}
		writeJSON(t, w, map[string]any{
			"code": 0,
			"data": map[string]any{
				"points_list": map[string]any{
					"20260710": map[string]any{
						"points": []map[string]any{
							{
								"id":       1001,
								"name":     "主舞台任务",
								"image":    "https://example.com/task.png",
								"unlocked": 5,
								"is_point": 1,
								"dic":      "完成主舞台互动。",
							},
						},
						"task": map[string]any{"total": 1, "cur_count": 1},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := bilibili.NewClient(bilibili.ClientOptions{
		PassportBaseURL: server.URL,
		APIBaseURL:      server.URL,
		HTTPClient:      server.Client(),
	})
	got, err := client.OfflinePoints(t.Context(), bilibili.OfflinePointsRequest{
		BID: 202601, Year: 202601, VenueID: 1, Day: "20260710",
	}, []*http.Cookie{{Name: "SESSDATA", Value: "session-value"}})
	if err != nil {
		t.Fatalf("offline points: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("points length = %d, want 1", len(got))
	}
	point := got[0]
	if point.ID != "1001" || point.Name != "主舞台任务" || point.ImageURL != "https://example.com/task.png" {
		t.Fatalf("point = %+v", point)
	}
	if point.RewardCoins != 5 || !point.Completed || point.Description != "完成主舞台互动。" || point.EventDay != "20260710" {
		t.Fatalf("point fields = %+v", point)
	}
}

func TestEncryptCookieJarRoundTrip(t *testing.T) {
	cookies := []*http.Cookie{
		{Name: "SESSDATA", Value: "session-value", Domain: ".bilibili.com", Path: "/", Expires: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC), HttpOnly: true},
		{Name: "bili_jct", Value: "csrf-value", Domain: ".bilibili.com", Path: "/"},
	}

	encrypted, err := bilibili.EncryptCookieJar("local-development-secret", cookies)
	if err != nil {
		t.Fatalf("encrypt cookie jar: %v", err)
	}
	if strings.Contains(encrypted, "session-value") || strings.Contains(encrypted, "csrf-value") {
		t.Fatalf("ciphertext leaked cookie value: %s", encrypted)
	}

	decrypted, err := bilibili.DecryptCookieJar("local-development-secret", encrypted)
	if err != nil {
		t.Fatalf("decrypt cookie jar: %v", err)
	}
	if len(decrypted) != 2 {
		t.Fatalf("decrypted cookies length = %d, want 2", len(decrypted))
	}
	if decrypted[0].Name != "SESSDATA" || decrypted[0].Value != "session-value" || !decrypted[0].HttpOnly {
		t.Fatalf("first cookie = %+v", decrypted[0])
	}
	if !decrypted[0].Expires.Equal(cookies[0].Expires) {
		t.Fatalf("expires = %v, want %v", decrypted[0].Expires, cookies[0].Expires)
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}
