package session

import (
	"context"
	"testing"

	"github.com/mpm/manfred/internal/store"
)

func setupTestStore(t *testing.T) (*SQLiteStore, func()) {
	t.Helper()

	db, err := store.OpenInMemory()
	if err != nil {
		t.Fatalf("open in-memory db: %v", err)
	}

	if err := db.Migrate(context.Background()); err != nil {
		db.Close()
		t.Fatalf("migrate db: %v", err)
	}

	sessionStore := NewSQLiteStore(db)

	cleanup := func() {
		db.Close()
	}

	return sessionStore, cleanup
}

func TestSQLiteStoreCreate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)

	err := store.Create(ctx, sess)
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	// Verify it was created
	got, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get() = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("Get() = nil, want session")
	}
	if got.ID != sess.ID {
		t.Errorf("ID = %q, want %q", got.ID, sess.ID)
	}
	if got.RepoOwner != sess.RepoOwner {
		t.Errorf("RepoOwner = %q, want %q", got.RepoOwner, sess.RepoOwner)
	}
	if got.IssueNumber != sess.IssueNumber {
		t.Errorf("IssueNumber = %d, want %d", got.IssueNumber, sess.IssueNumber)
	}
}

func TestSQLiteStoreCreateDuplicate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)

	err := store.Create(ctx, sess)
	if err != nil {
		t.Fatalf("Create() = %v, want nil", err)
	}

	// Try to create duplicate
	sess2 := NewSession("owner", "repo", 42)
	err = store.Create(ctx, sess2)
	if err == nil {
		t.Error("Create() duplicate = nil, want error")
	}
}

func TestSQLiteStoreGetByIssue(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)
	store.Create(ctx, sess)

	got, err := store.GetByIssue(ctx, "owner", "repo", 42)
	if err != nil {
		t.Fatalf("GetByIssue() = %v, want nil", err)
	}
	if got == nil {
		t.Fatal("GetByIssue() = nil, want session")
	}
	if got.ID != sess.ID {
		t.Errorf("ID = %q, want %q", got.ID, sess.ID)
	}

	// Non-existent
	got, err = store.GetByIssue(ctx, "owner", "repo", 999)
	if err != nil {
		t.Fatalf("GetByIssue() non-existent = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("GetByIssue() non-existent = %v, want nil", got)
	}
}

func TestSQLiteStoreUpdate(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)
	store.Create(ctx, sess)

	// Update phase and PR number
	sess.Phase = PhaseImplementing
	prNum := 123
	sess.PRNumber = &prNum
	plan := "Implementation plan"
	sess.PlanContent = &plan

	err := store.Update(ctx, sess)
	if err != nil {
		t.Fatalf("Update() = %v, want nil", err)
	}

	// Verify update
	got, _ := store.Get(ctx, sess.ID)
	if got.Phase != PhaseImplementing {
		t.Errorf("Phase = %q, want %q", got.Phase, PhaseImplementing)
	}
	if got.PRNumber == nil || *got.PRNumber != 123 {
		t.Errorf("PRNumber = %v, want 123", got.PRNumber)
	}
	if got.PlanContent == nil || *got.PlanContent != plan {
		t.Errorf("PlanContent = %v, want %q", got.PlanContent, plan)
	}
}

func TestSQLiteStoreDelete(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)
	store.Create(ctx, sess)

	err := store.Delete(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Delete() = %v, want nil", err)
	}

	// Verify deletion
	got, err := store.Get(ctx, sess.ID)
	if err != nil {
		t.Fatalf("Get() after delete = %v, want nil", err)
	}
	if got != nil {
		t.Errorf("Get() after delete = %v, want nil", got)
	}

	// Delete non-existent
	err = store.Delete(ctx, "non-existent")
	if err == nil {
		t.Error("Delete() non-existent = nil, want error")
	}
}

func TestSQLiteStoreList(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple sessions
	sessions := []*Session{
		NewSession("owner1", "repo1", 1),
		NewSession("owner1", "repo1", 2),
		NewSession("owner2", "repo2", 3),
	}

	// Modify phases for variety
	sessions[1].Phase = PhaseImplementing
	sessions[2].Phase = PhaseCompleted

	for _, s := range sessions {
		store.Create(ctx, s)
	}

	// List all
	all, err := store.List(ctx, SessionFilter{})
	if err != nil {
		t.Fatalf("List() all = %v, want nil", err)
	}
	if len(all) != 3 {
		t.Errorf("List() all len = %d, want 3", len(all))
	}

	// Filter by repo
	filtered, err := store.List(ctx, SessionFilter{
		RepoOwner: "owner1",
		RepoName:  "repo1",
	})
	if err != nil {
		t.Fatalf("List() filtered = %v, want nil", err)
	}
	if len(filtered) != 2 {
		t.Errorf("List() filtered len = %d, want 2", len(filtered))
	}

	// Filter by phase
	phase := PhaseImplementing
	byPhase, err := store.List(ctx, SessionFilter{Phase: &phase})
	if err != nil {
		t.Fatalf("List() by phase = %v, want nil", err)
	}
	if len(byPhase) != 1 {
		t.Errorf("List() by phase len = %d, want 1", len(byPhase))
	}

	// Active only
	active, err := store.List(ctx, SessionFilter{ActiveOnly: true})
	if err != nil {
		t.Fatalf("List() active = %v, want nil", err)
	}
	if len(active) != 2 {
		t.Errorf("List() active len = %d, want 2", len(active))
	}

	// With limit
	limited, err := store.List(ctx, SessionFilter{Limit: 1})
	if err != nil {
		t.Fatalf("List() limited = %v, want nil", err)
	}
	if len(limited) != 1 {
		t.Errorf("List() limited len = %d, want 1", len(limited))
	}
}

func TestSQLiteStoreCount(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()

	// Create sessions
	sessions := []*Session{
		NewSession("owner", "repo", 1),
		NewSession("owner", "repo", 2),
		NewSession("owner", "repo", 3),
	}
	sessions[2].Phase = PhaseCompleted

	for _, s := range sessions {
		store.Create(ctx, s)
	}

	// Count all
	count, err := store.Count(ctx, SessionFilter{})
	if err != nil {
		t.Fatalf("Count() = %v, want nil", err)
	}
	if count != 3 {
		t.Errorf("Count() = %d, want 3", count)
	}

	// Count active
	activeCount, err := store.Count(ctx, SessionFilter{ActiveOnly: true})
	if err != nil {
		t.Fatalf("Count() active = %v, want nil", err)
	}
	if activeCount != 2 {
		t.Errorf("Count() active = %d, want 2", activeCount)
	}
}

func TestSQLiteStoreRecordEvent(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)
	store.Create(ctx, sess)

	// Record events
	err := store.RecordEvent(ctx, sess.ID, EventTypePhaseChange, map[string]string{
		"from": "planning",
		"to":   "awaiting_approval",
	})
	if err != nil {
		t.Fatalf("RecordEvent() = %v, want nil", err)
	}

	err = store.RecordEvent(ctx, sess.ID, EventTypeCommentPosted, "Posted plan comment")
	if err != nil {
		t.Fatalf("RecordEvent() = %v, want nil", err)
	}

	// Get events
	events, err := store.GetEvents(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetEvents() = %v, want nil", err)
	}
	if len(events) != 2 {
		t.Errorf("GetEvents() len = %d, want 2", len(events))
	}

	if events[0].EventType != EventTypePhaseChange {
		t.Errorf("events[0].EventType = %q, want %q", events[0].EventType, EventTypePhaseChange)
	}
	if events[1].EventType != EventTypeCommentPosted {
		t.Errorf("events[1].EventType = %q, want %q", events[1].EventType, EventTypeCommentPosted)
	}
}

func TestSQLiteStoreEventsDeletedWithSession(t *testing.T) {
	store, cleanup := setupTestStore(t)
	defer cleanup()

	ctx := context.Background()
	sess := NewSession("owner", "repo", 42)
	store.Create(ctx, sess)

	// Record event
	store.RecordEvent(ctx, sess.ID, EventTypePhaseChange, nil)

	// Delete session
	store.Delete(ctx, sess.ID)

	// Events should be gone too (cascade delete)
	events, err := store.GetEvents(ctx, sess.ID)
	if err != nil {
		t.Fatalf("GetEvents() after delete = %v, want nil", err)
	}
	if len(events) != 0 {
		t.Errorf("GetEvents() after delete len = %d, want 0", len(events))
	}
}
