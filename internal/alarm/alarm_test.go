package alarm

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create .runtime/alarms/ structure
	if err := os.MkdirAll(filepath.Join(dir, ".runtime", "alarms"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSaveLoadCancel(t *testing.T) {
	townRoot := setupTestDir(t)

	a := &Alarm{
		ID:         "test-001",
		Schedule:   "repeat:1m@m",
		Target:     "gastown/witness",
		Message:    "status check",
		Recurring:  true,
		Interval:   time.Minute,
		SnapUnit:   "m",
		NextFireAt: time.Now().UTC().Add(time.Minute),
		CreatedAt:  time.Now().UTC(),
	}

	// Save
	if err := Save(townRoot, a); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Load
	loaded, err := Load(townRoot, "test-001")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != a.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, a.ID)
	}
	if loaded.Target != a.Target {
		t.Errorf("Target = %q, want %q", loaded.Target, a.Target)
	}
	if loaded.Message != a.Message {
		t.Errorf("Message = %q, want %q", loaded.Message, a.Message)
	}

	// List
	alarms, err := List(townRoot)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(alarms) != 1 {
		t.Fatalf("List: got %d alarms, want 1", len(alarms))
	}

	// Cancel
	if err := Cancel(townRoot, "test-001"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	// Verify gone
	alarms, err = List(townRoot)
	if err != nil {
		t.Fatalf("List after cancel: %v", err)
	}
	if len(alarms) != 0 {
		t.Errorf("List after cancel: got %d alarms, want 0", len(alarms))
	}
}

func TestCancelNotFound(t *testing.T) {
	townRoot := setupTestDir(t)
	err := Cancel(townRoot, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alarm")
	}
}

func TestDue(t *testing.T) {
	townRoot := setupTestDir(t)
	now := time.Now().UTC()

	// Create one due and one not-due alarm
	due := &Alarm{
		ID:         "due-001",
		Schedule:   "repeat:1m",
		Target:     "mayor/",
		Message:    "overdue",
		Recurring:  true,
		Interval:   time.Minute,
		NextFireAt: now.Add(-time.Minute), // past
		CreatedAt:  now,
	}
	notDue := &Alarm{
		ID:         "future-001",
		Schedule:   "in:1h",
		Target:     "mayor/",
		Message:    "future",
		NextFireAt: now.Add(time.Hour), // future
		CreatedAt:  now,
	}

	Save(townRoot, due)
	Save(townRoot, notDue)

	result, err := Due(townRoot, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("Due: got %d, want 1", len(result))
	}
	if result[0].ID != "due-001" {
		t.Errorf("Due alarm ID = %q, want due-001", result[0].ID)
	}
}

func TestAdvanceRecurring(t *testing.T) {
	townRoot := setupTestDir(t)
	now := time.Now().UTC()

	a := &Alarm{
		ID:         "recur-001",
		Schedule:   "repeat:5m",
		Target:     "mayor/",
		Message:    "ping",
		Recurring:  true,
		Interval:   5 * time.Minute,
		NextFireAt: now.Add(-time.Second),
		CreatedAt:  now,
	}
	Save(townRoot, a)

	if err := Advance(townRoot, a, now); err != nil {
		t.Fatal(err)
	}

	// Should still exist with updated fire time
	reloaded, err := Load(townRoot, "recur-001")
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.FireCount != 1 {
		t.Errorf("FireCount = %d, want 1", reloaded.FireCount)
	}
	if !reloaded.NextFireAt.After(now) {
		t.Errorf("NextFireAt %v should be after %v", reloaded.NextFireAt, now)
	}
}

func TestAdvanceOneShot(t *testing.T) {
	townRoot := setupTestDir(t)
	now := time.Now().UTC()

	a := &Alarm{
		ID:         "oneshot-001",
		Schedule:   "in:5m",
		Target:     "mayor/",
		Message:    "ping",
		Recurring:  false,
		NextFireAt: now.Add(-time.Second),
		CreatedAt:  now,
	}
	Save(townRoot, a)

	if err := Advance(townRoot, a, now); err != nil {
		t.Fatal(err)
	}

	// One-shot should be removed
	_, err := Load(townRoot, "oneshot-001")
	if err == nil {
		t.Error("expected one-shot alarm to be removed after advance")
	}
}

func TestRecordFailure(t *testing.T) {
	townRoot := setupTestDir(t)
	now := time.Now().UTC()

	a := &Alarm{
		ID:         "fail-001",
		Schedule:   "repeat:1m",
		Target:     "mayor/",
		Message:    "ping",
		Recurring:  true,
		Interval:   time.Minute,
		NextFireAt: now,
		CreatedAt:  now,
	}
	Save(townRoot, a)

	// First failure: 10s backoff
	RecordFailure(townRoot, a, fmt.Errorf("connection refused"))
	if a.FailCount != 1 {
		t.Errorf("FailCount = %d, want 1", a.FailCount)
	}
	if a.LastError != "connection refused" {
		t.Errorf("LastError = %q, want 'connection refused'", a.LastError)
	}

	// Second failure: 30s backoff
	RecordFailure(townRoot, a, fmt.Errorf("timeout"))
	if a.FailCount != 2 {
		t.Errorf("FailCount = %d, want 2", a.FailCount)
	}
}
