package store

import (
	"errors"
	"testing"
	"time"
)

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
	if !tasks[0].Members[1].Completed {
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
	if tasks[0].Members[1].Completed {
		t.Fatal("newer uncomplete state did not win")
	}
	if tasks[0].CompletedCount != 0 {
		t.Fatalf("completed count = %d, want 0", tasks[0].CompletedCount)
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

func mustCreateGroup(t *testing.T, s *Store, id string, ownerID int64) {
	t.Helper()
	if err := s.CreateGroup(t.Context(), CreateGroupInput{
		ID: id, Name: "BW2026 周五", Day: "friday", OwnerUserID: ownerID,
	}); err != nil {
		t.Fatalf("create group: %v", err)
	}
}
