package tasksync_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"bws-checkin/backend/internal/bilibili"
	"bws-checkin/backend/internal/domain"
	"bws-checkin/backend/internal/store"
	"bws-checkin/backend/internal/tasksync"
)

func TestSyncWritesBilibiliTasks(t *testing.T) {
	st := newTestStore(t)
	source := &fakeSource{tasks: []tasksync.Task{
		{
			ExternalID:  "1001",
			GroupName:   "8.1馆",
			Name:        "主舞台任务",
			Title:       "完成主舞台任务",
			RewardCoins: 5,
			Description: "完成主舞台互动。",
			ImageURL:    "https://example.com/task.png",
			VenueID:     "1",
			VenueName:   "8.1馆",
			EventDay:    "20260710",
			SortOrder:   10,
		},
	}}
	syncer := tasksync.New(st, source, tasksync.Config{Now: fixedNow})

	if err := syncer.Sync(t.Context()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	owner := mustCreateUser(t, st, "oidc-owner", "Owner")
	mustCreateGroup(t, st, "bw2026-fri", owner.ID)
	tasks, err := st.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	task := findTask(t, tasks, "bilibili:1001:20260710:1")
	if task.ExternalID != "1001" || task.GroupName != "8.1馆" || task.ImageURL != "https://example.com/task.png" {
		t.Fatalf("synced task = %+v", task)
	}
	if task.RewardCoins != 5 || task.Description != "完成主舞台互动。" || task.SyncSource != "bilibili" {
		t.Fatalf("synced task fields = %+v", task)
	}

	status, err := st.TaskSyncState(t.Context())
	if err != nil {
		t.Fatalf("sync state: %v", err)
	}
	if status.LastSuccessAt == nil || !status.LastSuccessAt.Equal(fixedNow()) {
		t.Fatalf("last success = %v, want %v", status.LastSuccessAt, fixedNow())
	}
	if status.LastErrorAt != nil || status.LastErrorCode != "" {
		t.Fatalf("unexpected error state = %+v", status)
	}
}

func TestSyncFailureKeepsExistingTasks(t *testing.T) {
	st := newTestStore(t)
	syncer := tasksync.New(st, &fakeSource{err: errors.New("remote unavailable")}, tasksync.Config{Now: fixedNow})

	if err := syncer.Sync(t.Context()); err == nil {
		t.Fatal("sync err = nil, want remote error")
	}

	owner := mustCreateUser(t, st, "oidc-owner", "Owner")
	mustCreateGroup(t, st, "bw2026-fri", owner.ID)
	tasks, err := st.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected default tasks to remain after failed sync")
	}

	status, err := st.TaskSyncState(t.Context())
	if err != nil {
		t.Fatalf("sync state: %v", err)
	}
	if status.LastErrorAt == nil || status.LastErrorCode != "remote_unavailable" {
		t.Fatalf("error state = %+v", status)
	}
}

func TestEnsureFreshTriggersAsyncRefresh(t *testing.T) {
	st := newTestStore(t)
	now := fixedNow()
	source := &fakeSource{tasks: []tasksync.Task{{ExternalID: "2001", GroupName: "1.1馆", Name: "点位", Title: "点位", VenueID: "2", VenueName: "1.1馆", EventDay: "20260710", SortOrder: 20}}}
	var queued func()
	syncer := tasksync.New(st, source, tasksync.Config{
		Now:      func() time.Time { return now },
		FreshTTL: 5 * time.Minute,
		Async: func(fn func()) {
			queued = fn
		},
	})

	if err := st.RecordTaskSyncSuccess(t.Context(), now.Add(-10*time.Minute)); err != nil {
		t.Fatalf("record old success: %v", err)
	}
	if syncer.EnsureFresh(t.Context()) {
		t.Fatal("EnsureFresh returned true, want false when it only queues refresh")
	}
	if queued == nil {
		t.Fatal("expected async refresh to be queued")
	}
	queued()

	tasks, err := st.GroupTasks(t.Context(), mustGroup(t, st))
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	findTask(t, tasks, "bilibili:2001:20260710:2")
}

func TestBilibiliSourceUsesStoredAccountCookies(t *testing.T) {
	st := newTestStore(t)
	user := mustCreateUser(t, st, "oidc-bili", "Bili")
	encryptedCookies, err := bilibili.EncryptCookieJar("cookie-secret", []*http.Cookie{{Name: "SESSDATA", Value: "session-value"}})
	if err != nil {
		t.Fatalf("encrypt cookies: %v", err)
	}
	if err := st.SaveBilibiliAccount(t.Context(), domain.BilibiliAccount{
		UserID:           user.ID,
		MID:              "123456",
		Uname:            "bws-user",
		CookieCiphertext: encryptedCookies,
	}); err != nil {
		t.Fatalf("save bilibili account: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Cookie"); got != "SESSDATA=session-value" {
			t.Fatalf("cookie header = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"code": 0,
			"data": map[string]any{
				"points_list": map[string]any{
					r.URL.Query().Get("day"): map[string]any{
						"points": []map[string]any{
							{"id": 3001, "name": "官方点位", "image": "https://example.com/official.png", "unlocked": 3, "dic": "官方任务说明。"},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	source := tasksync.NewBilibiliSource(tasksync.BilibiliSourceConfig{
		Store:        st,
		Client:       bilibili.NewClient(bilibili.ClientOptions{APIBaseURL: server.URL, PassportBaseURL: server.URL, HTTPClient: server.Client()}),
		CookieSecret: "cookie-secret",
		BID:          202601,
		Year:         202601,
		Days:         []string{"20260710"},
		Venues:       []tasksync.Venue{{ID: 1, Name: "8.1馆"}},
	})

	tasks, err := source.FetchTasks(t.Context())
	if err != nil {
		t.Fatalf("fetch tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("tasks length = %d, want 1", len(tasks))
	}
	task := tasks[0]
	if task.ExternalID != "3001" || task.GroupName != "8.1馆" || task.VenueID != "1" || task.EventDay != "20260710" {
		t.Fatalf("task = %+v", task)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("write json: %v", err)
	}
}

type fakeSource struct {
	mu    sync.Mutex
	tasks []tasksync.Task
	err   error
}

func (f *fakeSource) FetchTasks(ctx context.Context) ([]tasksync.Task, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]tasksync.Task(nil), f.tasks...), nil
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.OpenMemory()
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func mustCreateUser(t *testing.T, st *store.Store, subject string, name string) domain.User {
	t.Helper()
	user, err := st.UpsertUser(t.Context(), subject, name)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateGroup(t *testing.T, st *store.Store, id string, ownerID string) {
	t.Helper()
	if err := st.CreateGroup(t.Context(), store.CreateGroupInput{ID: id, Name: "BW2026 周五", Day: "friday", OwnerUserID: ownerID}); err != nil {
		t.Fatalf("create group: %v", err)
	}
}

func mustGroup(t *testing.T, st *store.Store) string {
	t.Helper()
	owner := mustCreateUser(t, st, "oidc-group-owner", "Owner")
	groupID := "bw2026-fri"
	mustCreateGroup(t, st, groupID, owner.ID)
	return groupID
}

func findTask(t *testing.T, tasks []domain.TaskStatus, id string) domain.TaskStatus {
	t.Helper()
	for _, task := range tasks {
		if task.ID == id {
			return task
		}
	}
	t.Fatalf("task %s not found in %+v", id, tasks)
	return domain.TaskStatus{}
}
