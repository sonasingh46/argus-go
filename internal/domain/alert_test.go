package domain

import (
	"testing"
	"time"
)

func TestNewParentAlert(t *testing.T) {
	event := &Event{
		EventManagerID: "em-1",
		Summary:        "Test alert",
		Severity:       SeverityHigh,
		Class:          "database",
		DedupKey:       "db-alert-1",
	}

	alert := NewParentAlert(event)

	if alert.DedupKey != event.DedupKey {
		t.Errorf("DedupKey = %v, want %v", alert.DedupKey, event.DedupKey)
	}
	if alert.EventManagerID != event.EventManagerID {
		t.Errorf("EventManagerID = %v, want %v", alert.EventManagerID, event.EventManagerID)
	}
	if alert.Type != AlertTypeParent {
		t.Errorf("Type = %v, want %v", alert.Type, AlertTypeParent)
	}
	if alert.Status != AlertStatusActive {
		t.Errorf("Status = %v, want %v", alert.Status, AlertStatusActive)
	}
	if alert.ChildCount != 0 {
		t.Errorf("ChildCount = %v, want 0", alert.ChildCount)
	}
	if alert.ParentDedupKey != "" {
		t.Errorf("ParentDedupKey = %v, want empty", alert.ParentDedupKey)
	}
}

func TestNewChildAlert(t *testing.T) {
	event := &Event{
		EventManagerID: "em-1",
		Summary:        "Test alert",
		Severity:       SeverityHigh,
		Class:          "database",
		DedupKey:       "db-alert-2",
	}
	parentDedupKey := "db-alert-1"

	alert := NewChildAlert(event, parentDedupKey)

	if alert.DedupKey != event.DedupKey {
		t.Errorf("DedupKey = %v, want %v", alert.DedupKey, event.DedupKey)
	}
	if alert.Type != AlertTypeChild {
		t.Errorf("Type = %v, want %v", alert.Type, AlertTypeChild)
	}
	if alert.ParentDedupKey != parentDedupKey {
		t.Errorf("ParentDedupKey = %v, want %v", alert.ParentDedupKey, parentDedupKey)
	}
}

func TestAlert_IsParent(t *testing.T) {
	parent := &Alert{Type: AlertTypeParent}
	child := &Alert{Type: AlertTypeChild}

	if !parent.IsParent() {
		t.Error("IsParent() should return true for parent alert")
	}
	if parent.IsChild() {
		t.Error("IsChild() should return false for parent alert")
	}
	if child.IsParent() {
		t.Error("IsParent() should return false for child alert")
	}
	if !child.IsChild() {
		t.Error("IsChild() should return true for child alert")
	}
}

func TestAlert_IsActive(t *testing.T) {
	active := &Alert{Status: AlertStatusActive}
	resolved := &Alert{Status: AlertStatusResolved}

	if !active.IsActive() {
		t.Error("IsActive() should return true for active alert")
	}
	if active.IsResolved() {
		t.Error("IsResolved() should return false for active alert")
	}
	if resolved.IsActive() {
		t.Error("IsActive() should return false for resolved alert")
	}
	if !resolved.IsResolved() {
		t.Error("IsResolved() should return true for resolved alert")
	}
}

func TestAlert_Resolve(t *testing.T) {
	alert := &Alert{
		Status:           AlertStatusActive,
		ResolveRequested: true,
	}

	beforeResolve := time.Now()
	alert.Resolve()
	afterResolve := time.Now()

	if alert.Status != AlertStatusResolved {
		t.Errorf("Status = %v, want %v", alert.Status, AlertStatusResolved)
	}
	if alert.ResolveRequested {
		t.Error("ResolveRequested should be false after Resolve()")
	}
	if alert.ResolvedAt == nil {
		t.Error("ResolvedAt should be set after Resolve()")
	}
	if alert.ResolvedAt.Before(beforeResolve) || alert.ResolvedAt.After(afterResolve) {
		t.Error("ResolvedAt should be set to current time")
	}
}

func TestAlert_MarkResolveRequested(t *testing.T) {
	alert := &Alert{
		Status:           AlertStatusActive,
		ResolveRequested: false,
	}

	alert.MarkResolveRequested()

	if !alert.ResolveRequested {
		t.Error("ResolveRequested should be true after MarkResolveRequested()")
	}
}

func TestAlert_IncrementChildCount(t *testing.T) {
	alert := &Alert{ChildCount: 0}

	alert.IncrementChildCount()
	if alert.ChildCount != 1 {
		t.Errorf("ChildCount = %v, want 1", alert.ChildCount)
	}

	alert.IncrementChildCount()
	if alert.ChildCount != 2 {
		t.Errorf("ChildCount = %v, want 2", alert.ChildCount)
	}
}
