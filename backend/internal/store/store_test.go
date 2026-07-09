package store

import (
	"database/sql"
	"errors"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"bws-checkin/backend/internal/domain"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func TestUserIDIsUUIDString(t *testing.T) {
	s := newTestStore(t)
	user := mustCreateUser(t, s, "oidc-user", "User")
	if !uuidPattern.MatchString(user.ID) {
		t.Fatalf("user ID = %q, want UUID string", user.ID)
	}
	loaded, err := s.UserByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("load user by UUID: %v", err)
	}
	if loaded.ID != user.ID {
		t.Fatalf("loaded ID = %q, want %q", loaded.ID, user.ID)
	}
}

func TestCreateGroupRejectsDuplicateID(t *testing.T) {
	s := newTestStore(t)
	user := mustCreateUser(t, s, "oidc-owner", "Owner")

	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: "bw2026-fri", Name: "BW2026 周五", Day: "friday", OwnerUserID: user.ID,
	}); err != nil {
		t.Fatalf("create group: %v", err)
	}

	err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: "bw2026-fri", Name: "重复组", Day: "friday", OwnerUserID: user.ID,
	})
	if err == nil {
		t.Fatal("expected duplicate group ID to fail")
	}
}

func TestCompleteTaskIsIdempotent(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	member := mustCreateUser(t, s, "oidc-member", "Member")
	mustCreateGroup(t, s, "bw2026-fri", owner.ID)
	if err := s.JoinGroup(t.Context(), "bw2026-fri", member.ID); err != nil {
		t.Fatalf("join group: %v", err)
	}

	if err := s.MarkComplete(t.Context(), "bw2026-fri", "rainbow-station", member.ID, owner.ID); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	if err := s.MarkComplete(t.Context(), "bw2026-fri", "rainbow-station", member.ID, owner.ID); err != nil {
		t.Fatalf("repeat mark complete: %v", err)
	}

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected seeded tasks")
	}
	if tasks[0].CompletedCount != 1 {
		t.Fatalf("completed count = %d, want 1", tasks[0].CompletedCount)
	}
	if tasks[0].TotalCount != 2 {
		t.Fatalf("total count = %d, want 2", tasks[0].TotalCount)
	}
}

func TestGroupTasksIncludeDisplayMetadata(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	mustCreateGroup(t, s, "bw2026-fri", owner.ID)

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("expected seeded tasks")
	}
	task := tasks[0]
	if task.GroupName == "" {
		t.Fatal("expected task group name")
	}
	if task.Title == "" {
		t.Fatal("expected task title")
	}
	if task.RewardCoins <= 0 {
		t.Fatalf("reward coins = %d, want positive", task.RewardCoins)
	}
	if task.Description == "" {
		t.Fatal("expected task description")
	}
}

func TestGroupTasksFilterByEventDate(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: "bw2026-day2", Name: "BW2026 7 月 11 日", Day: "20260711", OwnerUserID: owner.ID,
	}); err != nil {
		t.Fatalf("create group with event date: %v", err)
	}

	tasks, err := s.GroupTasks(t.Context(), "bw2026-day2")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if len(tasks) == 0 {
		t.Fatal("tasks length = 0, want day tasks")
	}
	for _, task := range tasks {
		if task.EventDay != "20260711" {
			t.Fatalf("task %s event day = %q, want 20260711", task.ID, task.EventDay)
		}
	}
}

func TestMigrateLegacyGroupDayConstraintAllowsEventDates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open legacy db: %v", err)
	}
	_, err = db.Exec(`
		CREATE TABLE users (
			id TEXT PRIMARY KEY,
			oidc_subject TEXT NOT NULL UNIQUE,
			display_name TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			qr_image_path TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE groups (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			day TEXT NOT NULL CHECK (day IN ('friday', 'saturday', 'sunday')),
			description TEXT NOT NULL DEFAULT '',
			owner_user_id TEXT NOT NULL REFERENCES users(id),
			created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE group_members (
			group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role TEXT NOT NULL CHECK (role IN ('owner', 'member')),
			joined_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (group_id, user_id)
		);
		INSERT INTO users (id, oidc_subject, display_name) VALUES ('owner-id', 'oidc-owner', 'Owner');
		INSERT INTO groups (id, name, day, owner_user_id) VALUES ('bw2026', 'BW2026', 'friday', 'owner-id');
		INSERT INTO group_members (group_id, user_id, role) VALUES ('bw2026', 'owner-id', 'owner');
	`)
	if err != nil {
		t.Fatalf("seed legacy db: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy db: %v", err)
	}

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open migrated store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.UpdateGroup(t.Context(), UpdateGroupInput{
		ID: "bw2026", Name: "BW2026 7 月 11 日", Day: "20260711", Description: "day2",
	}); err != nil {
		t.Fatalf("update group to event date after migration: %v", err)
	}
	group, err := s.GroupByID(t.Context(), "bw2026", "owner-id")
	if err != nil {
		t.Fatalf("load migrated group: %v", err)
	}
	if group.Day != "20260711" {
		t.Fatalf("group day = %q, want 20260711", group.Day)
	}
}

func TestSyncTaskCompletionKeepsNewestState(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	member := mustCreateUser(t, s, "oidc-member", "Member")
	mustCreateGroup(t, s, "bw2026-fri", owner.ID)
	if err := s.JoinGroup(t.Context(), "bw2026-fri", member.ID); err != nil {
		t.Fatalf("join group: %v", err)
	}

	oldTime := time.Date(2026, 7, 8, 10, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(5 * time.Minute)
	if err := s.SyncTaskCompletion(t.Context(), SyncTaskCompletionInput{
		GroupID: "bw2026-fri", TaskID: "rainbow-station", TargetUserID: member.ID, CheckedByUserID: owner.ID, Completed: true, UpdatedAt: newTime,
	}); err != nil {
		t.Fatalf("new complete sync: %v", err)
	}
	if err := s.SyncTaskCompletion(t.Context(), SyncTaskCompletionInput{
		GroupID: "bw2026-fri", TaskID: "rainbow-station", TargetUserID: member.ID, CheckedByUserID: owner.ID, Completed: false, UpdatedAt: oldTime,
	}); err != nil {
		t.Fatalf("old uncomplete sync: %v", err)
	}

	tasks, err := s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	if !memberCompletion(t, tasks[0], member.ID).Completed {
		t.Fatal("older offline state overwrote newer completion")
	}

	if err := s.SyncTaskCompletion(t.Context(), SyncTaskCompletionInput{
		GroupID: "bw2026-fri", TaskID: "rainbow-station", TargetUserID: member.ID, CheckedByUserID: owner.ID, Completed: false, UpdatedAt: newTime.Add(time.Minute),
	}); err != nil {
		t.Fatalf("new uncomplete sync: %v", err)
	}
	tasks, err = s.GroupTasks(t.Context(), "bw2026-fri")
	if err != nil {
		t.Fatalf("group tasks after uncomplete: %v", err)
	}
	if memberCompletion(t, tasks[0], member.ID).Completed {
		t.Fatal("newer uncomplete state did not win")
	}
	if tasks[0].CompletedCount != 0 {
		t.Fatalf("completed count = %d, want 0", tasks[0].CompletedCount)
	}
}

func TestBilibiliAccountAndQRSource(t *testing.T) {
	s := newTestStore(t)
	user := mustCreateUser(t, s, "oidc-bilibili", "Bilibili User")
	expiresAt := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	validatedAt := expiresAt.Add(-time.Hour)

	account := domain.BilibiliAccount{
		UserID:                 user.ID,
		MID:                    "123456",
		Uname:                  "bws-user",
		FaceURL:                "https://example.com/face.png",
		CookieCiphertext:       "cookie-ciphertext",
		CookieExpiresAt:        &expiresAt,
		RefreshTokenCiphertext: "refresh-token-ciphertext",
		LastValidatedAt:        &validatedAt,
	}
	if err := s.SaveBilibiliAccount(t.Context(), account); err != nil {
		t.Fatalf("save bilibili account: %v", err)
	}

	loaded, err := s.BilibiliAccount(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("load bilibili account: %v", err)
	}
	if loaded.MID != "123456" || loaded.Uname != "bws-user" || loaded.FaceURL != "https://example.com/face.png" {
		t.Fatalf("loaded account = %+v", loaded)
	}
	if loaded.CookieCiphertext != "cookie-ciphertext" || loaded.RefreshTokenCiphertext != "refresh-token-ciphertext" {
		t.Fatalf("loaded account secret fields not preserved: %+v", loaded)
	}
	if loaded.CookieExpiresAt == nil || !loaded.CookieExpiresAt.Equal(expiresAt) {
		t.Fatalf("cookie expires at = %v, want %v", loaded.CookieExpiresAt, expiresAt)
	}
	if loaded.LastValidatedAt == nil || !loaded.LastValidatedAt.Equal(validatedAt) {
		t.Fatalf("last validated at = %v, want %v", loaded.LastValidatedAt, validatedAt)
	}

	if err := s.SetUserQRSource(t.Context(), user.ID, domain.QRSourceBilibiliGenerated); err != nil {
		t.Fatalf("set qr source: %v", err)
	}
	updated, err := s.UserByID(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("load user after qr source update: %v", err)
	}
	if updated.QRSource != domain.QRSourceBilibiliGenerated {
		t.Fatalf("qr source = %q, want %q", updated.QRSource, domain.QRSourceBilibiliGenerated)
	}

	if err := s.UnbindBilibiliAccount(t.Context(), user.ID); err != nil {
		t.Fatalf("unbind bilibili account: %v", err)
	}
	if _, err := s.BilibiliAccount(t.Context(), user.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("load unbound bilibili account err = %v, want sql.ErrNoRows", err)
	}
}

func TestLiveCompletionLocksManualSync(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-live-owner", "Owner")
	member := mustCreateUser(t, s, "oidc-live-member", "Member")
	mustCreateGroup(t, s, "bw2026-live", owner.ID)
	if err := s.JoinGroup(t.Context(), "bw2026-live", member.ID); err != nil {
		t.Fatalf("join group: %v", err)
	}

	liveCheckedAt := time.Date(2026, 7, 10, 13, 0, 0, 0, time.UTC)
	if err := s.UpsertLiveTaskCompletion(t.Context(), LiveTaskCompletionInput{
		GroupID:       "bw2026-live",
		TaskID:        "rainbow-station",
		TargetUserID:  member.ID,
		Status:        domain.CompletionStatusLiveCompleted,
		LiveCheckedAt: liveCheckedAt,
		UpdatedAt:     liveCheckedAt,
	}); err != nil {
		t.Fatalf("upsert live completion: %v", err)
	}

	err := s.SyncTaskCompletion(t.Context(), SyncTaskCompletionInput{
		GroupID:         "bw2026-live",
		TaskID:          "rainbow-station",
		TargetUserID:    member.ID,
		CheckedByUserID: owner.ID,
		Completed:       false,
		UpdatedAt:       liveCheckedAt.Add(time.Minute),
	})
	if !errors.Is(err, ErrLiveCompletionLocked) {
		t.Fatalf("manual sync after live err = %v, want ErrLiveCompletionLocked", err)
	}

	tasks, err := s.GroupTasks(t.Context(), "bw2026-live")
	if err != nil {
		t.Fatalf("group tasks: %v", err)
	}
	entry := memberCompletion(t, tasks[0], member.ID)
	if !entry.Completed {
		t.Fatal("live completed state was changed by manual sync")
	}
	if entry.Status != domain.CompletionStatusLiveCompleted || entry.Source != domain.CompletionSourceLive {
		t.Fatalf("completion status/source = %q/%q, want live completed/live", entry.Status, entry.Source)
	}
	if entry.CanToggle {
		t.Fatal("live completion can toggle, want false")
	}
	if !entry.CanRefresh {
		t.Fatal("live completion can refresh = false, want true")
	}
	if entry.LiveCheckedAt == nil || !entry.LiveCheckedAt.Equal(liveCheckedAt) {
		t.Fatalf("live checked at = %v, want %v", entry.LiveCheckedAt, liveCheckedAt)
	}
}

func TestGroupUpdateLockAndArchive(t *testing.T) {
	s := newTestStore(t)
	owner := mustCreateUser(t, s, "oidc-owner", "Owner")
	member := mustCreateUser(t, s, "oidc-member", "Member")
	mustCreateGroup(t, s, "bw2026-fri", owner.ID)

	if err := s.UpdateGroup(t.Context(), UpdateGroupInput{
		ID: "bw2026-fri", Name: "BW2026 周六", Day: "saturday", Description: "更新后的说明",
	}); err != nil {
		t.Fatalf("update group: %v", err)
	}
	group, err := s.GroupByID(t.Context(), "bw2026-fri", owner.ID)
	if err != nil {
		t.Fatalf("load group: %v", err)
	}
	if group.Name != "BW2026 周六" || group.Day != "saturday" || group.Description != "更新后的说明" {
		t.Fatalf("group after update = %+v", group)
	}

	if err := s.SetGroupJoinLocked(t.Context(), "bw2026-fri", true); err != nil {
		t.Fatalf("lock group: %v", err)
	}
	group, err = s.GroupByID(t.Context(), "bw2026-fri", owner.ID)
	if err != nil {
		t.Fatalf("load locked group: %v", err)
	}
	if !group.JoinLocked {
		t.Fatal("group JoinLocked = false, want true")
	}
	if err := s.JoinGroup(t.Context(), "bw2026-fri", member.ID); !errors.Is(err, ErrGroupJoinLocked) {
		t.Fatalf("join locked group err = %v, want ErrGroupJoinLocked", err)
	}

	if err := s.SetGroupJoinLocked(t.Context(), "bw2026-fri", false); err != nil {
		t.Fatalf("unlock group: %v", err)
	}
	if err := s.JoinGroup(t.Context(), "bw2026-fri", member.ID); err != nil {
		t.Fatalf("join unlocked group: %v", err)
	}

	if err := s.ArchiveGroup(t.Context(), "bw2026-fri"); err != nil {
		t.Fatalf("archive group: %v", err)
	}
	group, err = s.GroupByID(t.Context(), "bw2026-fri", owner.ID)
	if err != nil {
		t.Fatalf("load archived group: %v", err)
	}
	if group.ArchivedAt == nil {
		t.Fatal("group ArchivedAt = nil, want timestamp")
	}
	lateMember := mustCreateUser(t, s, "oidc-late", "Late")
	if err := s.JoinGroup(t.Context(), "bw2026-fri", lateMember.ID); !errors.Is(err, ErrGroupArchived) {
		t.Fatalf("join archived group err = %v, want ErrGroupArchived", err)
	}
	if err := s.SyncTaskCompletion(t.Context(), SyncTaskCompletionInput{
		GroupID: "bw2026-fri", TaskID: "rainbow-station", TargetUserID: member.ID, CheckedByUserID: owner.ID, Completed: true, UpdatedAt: time.Now().UTC(),
	}); !errors.Is(err, ErrGroupArchived) {
		t.Fatalf("sync archived group err = %v, want ErrGroupArchived", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenMemory()
	if err != nil {
		t.Fatalf("open memory store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustCreateUser(t *testing.T, s *Store, subject, name string) User {
	t.Helper()
	user, err := s.UpsertUser(t.Context(), subject, name)
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func mustCreateGroup(t *testing.T, s *Store, id string, ownerID string) {
	t.Helper()
	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: id, Name: "BW2026 周五", Day: "friday", OwnerUserID: ownerID,
	}); err != nil {
		t.Fatalf("create group: %v", err)
	}
}

func memberCompletion(t *testing.T, task domain.TaskStatus, userID string) domain.MemberCompletion {
	t.Helper()
	for _, entry := range task.Members {
		if entry.Member.ID == userID {
			return entry
		}
	}
	t.Fatalf("member %s not found", userID)
	return domain.MemberCompletion{}
}
