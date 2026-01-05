package memory

import (
	"context"
	"testing"
	"time"

	"argus-go/internal/store"
)

func TestStateStore_ParentOperations(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Test GetParent on empty store
	parent, err := s.GetParent(ctx, "em-1", "class", "database")
	if err != nil {
		t.Fatalf("GetParent error: %v", err)
	}
	if parent != nil {
		t.Error("Expected nil for non-existent parent")
	}

	// Test SetParent
	parentState := &store.ParentState{
		DedupKey:   "alert-1",
		CreatedAt:  time.Now(),
		ChildCount: 0,
	}
	err = s.SetParent(ctx, "em-1", "class", "database", parentState, 5*time.Minute)
	if err != nil {
		t.Fatalf("SetParent error: %v", err)
	}

	// Test GetParent returns the stored value
	retrieved, err := s.GetParent(ctx, "em-1", "class", "database")
	if err != nil {
		t.Fatalf("GetParent error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected parent to be found")
	}
	if retrieved.DedupKey != parentState.DedupKey {
		t.Errorf("DedupKey = %v, want %v", retrieved.DedupKey, parentState.DedupKey)
	}

	// Test DeleteParent
	err = s.DeleteParent(ctx, "em-1", "class", "database")
	if err != nil {
		t.Fatalf("DeleteParent error: %v", err)
	}
	retrieved, _ = s.GetParent(ctx, "em-1", "class", "database")
	if retrieved != nil {
		t.Error("Parent should be deleted")
	}
}

func TestStateStore_ParentExpiration(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Set parent with very short TTL
	parentState := &store.ParentState{
		DedupKey:   "alert-1",
		CreatedAt:  time.Now(),
		ChildCount: 0,
	}
	err := s.SetParent(ctx, "em-1", "class", "database", parentState, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("SetParent error: %v", err)
	}

	// Wait for expiration
	time.Sleep(5 * time.Millisecond)

	// Should return nil due to expiration
	retrieved, err := s.GetParent(ctx, "em-1", "class", "database")
	if err != nil {
		t.Fatalf("GetParent error: %v", err)
	}
	if retrieved != nil {
		t.Error("Parent should be expired")
	}
}

func TestStateStore_AlertOperations(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Test GetAlert on empty store
	alert, err := s.GetAlert(ctx, "alert-1")
	if err != nil {
		t.Fatalf("GetAlert error: %v", err)
	}
	if alert != nil {
		t.Error("Expected nil for non-existent alert")
	}

	// Test SetAlert
	alertState := &store.AlertState{
		DedupKey:       "alert-1",
		EventManagerID: "em-1",
		Type:           "parent",
		Status:         "active",
	}
	err = s.SetAlert(ctx, alertState)
	if err != nil {
		t.Fatalf("SetAlert error: %v", err)
	}

	// Test GetAlert returns the stored value
	retrieved, err := s.GetAlert(ctx, "alert-1")
	if err != nil {
		t.Fatalf("GetAlert error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected alert to be found")
	}
	if retrieved.DedupKey != alertState.DedupKey {
		t.Errorf("DedupKey = %v, want %v", retrieved.DedupKey, alertState.DedupKey)
	}

	// Test DeleteAlert
	err = s.DeleteAlert(ctx, "alert-1")
	if err != nil {
		t.Fatalf("DeleteAlert error: %v", err)
	}
	retrieved, _ = s.GetAlert(ctx, "alert-1")
	if retrieved != nil {
		t.Error("Alert should be deleted")
	}
}

func TestStateStore_ChildOperations(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Test GetChildren on empty store
	children, err := s.GetChildren(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetChildren error: %v", err)
	}
	if len(children) != 0 {
		t.Error("Expected empty children list")
	}

	// Test AddChild
	err = s.AddChild(ctx, "parent-1", "child-1")
	if err != nil {
		t.Fatalf("AddChild error: %v", err)
	}
	err = s.AddChild(ctx, "parent-1", "child-2")
	if err != nil {
		t.Fatalf("AddChild error: %v", err)
	}

	// Test GetChildren
	children, err = s.GetChildren(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetChildren error: %v", err)
	}
	if len(children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(children))
	}

	// Test GetChildCount
	count, err := s.GetChildCount(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetChildCount error: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected count 2, got %d", count)
	}

	// Test RemoveChild
	err = s.RemoveChild(ctx, "parent-1", "child-1")
	if err != nil {
		t.Fatalf("RemoveChild error: %v", err)
	}
	count, _ = s.GetChildCount(ctx, "parent-1")
	if count != 1 {
		t.Errorf("Expected count 1, got %d", count)
	}
}

func TestStateStore_PendingResolveOperations(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Test GetPendingResolve on empty store
	pending, err := s.GetPendingResolve(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetPendingResolve error: %v", err)
	}
	if pending != nil {
		t.Error("Expected nil for non-existent pending resolve")
	}

	// Test SetPendingResolve
	pendingResolve := &store.PendingResolve{
		RequestedAt:       time.Now(),
		RemainingChildren: 3,
	}
	err = s.SetPendingResolve(ctx, "parent-1", pendingResolve)
	if err != nil {
		t.Fatalf("SetPendingResolve error: %v", err)
	}

	// Test GetPendingResolve returns the stored value
	retrieved, err := s.GetPendingResolve(ctx, "parent-1")
	if err != nil {
		t.Fatalf("GetPendingResolve error: %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected pending resolve to be found")
	}
	if retrieved.RemainingChildren != 3 {
		t.Errorf("RemainingChildren = %v, want 3", retrieved.RemainingChildren)
	}

	// Test DeletePendingResolve
	err = s.DeletePendingResolve(ctx, "parent-1")
	if err != nil {
		t.Fatalf("DeletePendingResolve error: %v", err)
	}
	retrieved, _ = s.GetPendingResolve(ctx, "parent-1")
	if retrieved != nil {
		t.Error("Pending resolve should be deleted")
	}
}

func TestStateStore_Clear(t *testing.T) {
	s := NewStateStore()
	ctx := context.Background()

	// Add some data
	_ = s.SetAlert(ctx, &store.AlertState{DedupKey: "alert-1", Status: "active"})
	_ = s.AddChild(ctx, "parent-1", "child-1")

	// Clear
	s.Clear()

	// Verify data is cleared
	alert, _ := s.GetAlert(ctx, "alert-1")
	if alert != nil {
		t.Error("Alert should be cleared")
	}

	count, _ := s.GetChildCount(ctx, "parent-1")
	if count != 0 {
		t.Error("Children should be cleared")
	}
}
