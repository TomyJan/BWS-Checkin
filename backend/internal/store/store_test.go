package store

import "testing"

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
