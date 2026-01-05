package processor

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"argus-go/internal/domain"
	"argus-go/internal/notification"
	"argus-go/internal/queue"
	"argus-go/internal/queue/memory"
	"argus-go/internal/store"
	storemem "argus-go/internal/store/memory"
)

// testSetup creates all dependencies needed for processor tests.
func testSetup() (*Service, *memory.Queue, *storemem.StateStore, *storemem.AlertRepository, *storemem.EventManagerRepository, *storemem.GroupingRuleRepository) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	msgQueue := memory.NewQueue(100)
	stateStore := storemem.NewStateStore()
	alertRepo := storemem.NewAlertRepository()
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()
	notifier := notification.NewStubNotifier(logger)

	service := NewService(
		msgQueue,
		stateStore,
		alertRepo,
		eventManagerRepo,
		groupingRuleRepo,
		notifier,
		logger,
	)

	return service, msgQueue, stateStore, alertRepo, eventManagerRepo, groupingRuleRepo
}

// setupTestData creates event manager and grouping rule for tests.
func setupTestData(ctx context.Context, emRepo *storemem.EventManagerRepository, grRepo *storemem.GroupingRuleRepository) {
	groupingRule := &domain.GroupingRule{
		ID:                "rule-1",
		Name:              "Test Rule",
		GroupingKey:       "class",
		TimeWindowMinutes: 5,
		CreatedAt:         time.Now(),
	}
	_ = grRepo.Create(ctx, groupingRule)

	eventManager := &domain.EventManager{
		ID:             "em-1",
		Name:           "Test EM",
		GroupingRuleID: "rule-1",
		CreatedAt:      time.Now(),
	}
	_ = emRepo.Create(ctx, eventManager)
}

func TestProcessor_HandleTrigger_CreatesParentAlert(t *testing.T) {
	service, _, stateStore, alertRepo, emRepo, grRepo := testSetup()
	ctx := context.Background()

	setupTestData(ctx, emRepo, grRepo)

	// Create internal event
	event := &domain.InternalEvent{
		Event: domain.Event{
			EventManagerID: "em-1",
			Summary:        "Test alert",
			Severity:       domain.SeverityHigh,
			Action:         domain.ActionTrigger,
			Class:          "database",
			DedupKey:       "alert-1",
		},
		PartitionKey:  "partition-1",
		GroupingValue: "database",
		ReceivedAt:    time.Now(),
	}

	// Create message
	payload, _ := json.Marshal(event)
	msg := &queue.Message{
		Key:   []byte(event.PartitionKey),
		Value: payload,
	}

	// Process the message
	err := service.handleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("handleMessage error: %v", err)
	}

	// Verify alert was created
	alert, err := alertRepo.GetByDedupKey(ctx, "alert-1")
	if err != nil {
		t.Fatalf("GetByDedupKey error: %v", err)
	}
	if alert == nil {
		t.Fatal("Alert should be created")
	}
	if alert.Type != domain.AlertTypeParent {
		t.Errorf("Alert type = %v, want parent", alert.Type)
	}
	if alert.Status != domain.AlertStatusActive {
		t.Errorf("Alert status = %v, want active", alert.Status)
	}

	// Verify state store was updated
	alertState, _ := stateStore.GetAlert(ctx, "alert-1")
	if alertState == nil {
		t.Error("Alert state should be saved")
	}
}

func TestProcessor_HandleTrigger_CreatesChildAlert(t *testing.T) {
	service, _, stateStore, alertRepo, emRepo, grRepo := testSetup()
	ctx := context.Background()

	setupTestData(ctx, emRepo, grRepo)

	// First, create a parent alert
	parentState := &store.ParentState{
		DedupKey:   "parent-alert",
		CreatedAt:  time.Now(),
		ChildCount: 0,
	}
	_ = stateStore.SetParent(ctx, "em-1", "class", "database", parentState, 5*time.Minute)

	// Also create the parent alert in the repo
	parentAlert := &domain.Alert{
		ID:             "parent-id",
		DedupKey:       "parent-alert",
		EventManagerID: "em-1",
		Type:           domain.AlertTypeParent,
		Status:         domain.AlertStatusActive,
		CreatedAt:      time.Now(),
	}
	_ = alertRepo.Create(ctx, parentAlert)

	// Create child event
	event := &domain.InternalEvent{
		Event: domain.Event{
			EventManagerID: "em-1",
			Summary:        "Child alert",
			Severity:       domain.SeverityHigh,
			Action:         domain.ActionTrigger,
			Class:          "database",
			DedupKey:       "child-alert",
		},
		PartitionKey:  "partition-1",
		GroupingValue: "database",
		ReceivedAt:    time.Now(),
	}

	// Create message
	payload, _ := json.Marshal(event)
	msg := &queue.Message{
		Key:   []byte(event.PartitionKey),
		Value: payload,
	}

	// Process the message
	err := service.handleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("handleMessage error: %v", err)
	}

	// Verify child alert was created
	childAlert, err := alertRepo.GetByDedupKey(ctx, "child-alert")
	if err != nil {
		t.Fatalf("GetByDedupKey error: %v", err)
	}
	if childAlert.Type != domain.AlertTypeChild {
		t.Errorf("Alert type = %v, want child", childAlert.Type)
	}
	if childAlert.ParentDedupKey != "parent-alert" {
		t.Errorf("ParentDedupKey = %v, want parent-alert", childAlert.ParentDedupKey)
	}

	// Verify child was added to parent's children
	children, _ := stateStore.GetChildren(ctx, "parent-alert")
	if len(children) != 1 {
		t.Errorf("Parent should have 1 child, got %d", len(children))
	}
}

func TestProcessor_HandleResolve_ResolvesChildAlert(t *testing.T) {
	service, _, stateStore, alertRepo, _, _ := testSetup()
	ctx := context.Background()

	// Create child alert in state store
	alertState := &store.AlertState{
		DedupKey:       "child-alert",
		EventManagerID: "em-1",
		Type:           string(domain.AlertTypeChild),
		Status:         string(domain.AlertStatusActive),
		ParentDedupKey: "parent-alert",
	}
	_ = stateStore.SetAlert(ctx, alertState)

	// Create child alert in repo
	childAlert := &domain.Alert{
		ID:             "child-id",
		DedupKey:       "child-alert",
		EventManagerID: "em-1",
		Type:           domain.AlertTypeChild,
		Status:         domain.AlertStatusActive,
		ParentDedupKey: "parent-alert",
		CreatedAt:      time.Now(),
	}
	_ = alertRepo.Create(ctx, childAlert)

	// Create resolve event
	event := &domain.InternalEvent{
		Event: domain.Event{
			EventManagerID: "em-1",
			Action:         domain.ActionResolve,
			DedupKey:       "child-alert",
		},
		ReceivedAt: time.Now(),
	}

	// Create message
	payload, _ := json.Marshal(event)
	msg := &queue.Message{
		Value: payload,
	}

	// Process the message
	err := service.handleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("handleMessage error: %v", err)
	}

	// Verify alert was resolved
	resolved, _ := alertRepo.GetByDedupKey(ctx, "child-alert")
	if resolved.Status != domain.AlertStatusResolved {
		t.Errorf("Alert status = %v, want resolved", resolved.Status)
	}
}

func TestProcessor_HandleResolve_ParentWaitsForChildren(t *testing.T) {
	service, _, stateStore, alertRepo, emRepo, grRepo := testSetup()
	ctx := context.Background()

	setupTestData(ctx, emRepo, grRepo)

	// Create parent alert with active child
	parentAlertState := &store.AlertState{
		DedupKey:       "parent-alert",
		EventManagerID: "em-1",
		Type:           string(domain.AlertTypeParent),
		Status:         string(domain.AlertStatusActive),
	}
	_ = stateStore.SetAlert(ctx, parentAlertState)

	parentAlert := &domain.Alert{
		ID:             "parent-id",
		DedupKey:       "parent-alert",
		EventManagerID: "em-1",
		Type:           domain.AlertTypeParent,
		Status:         domain.AlertStatusActive,
		ChildCount:     1,
		CreatedAt:      time.Now(),
	}
	_ = alertRepo.Create(ctx, parentAlert)

	// Create child alert (still active)
	childAlert := &domain.Alert{
		ID:             "child-id",
		DedupKey:       "child-alert",
		EventManagerID: "em-1",
		Type:           domain.AlertTypeChild,
		Status:         domain.AlertStatusActive,
		ParentDedupKey: "parent-alert",
		CreatedAt:      time.Now(),
	}
	_ = alertRepo.Create(ctx, childAlert)
	_ = stateStore.AddChild(ctx, "parent-alert", "child-alert")

	// Try to resolve parent
	event := &domain.InternalEvent{
		Event: domain.Event{
			EventManagerID: "em-1",
			Action:         domain.ActionResolve,
			DedupKey:       "parent-alert",
		},
		ReceivedAt: time.Now(),
	}

	payload, _ := json.Marshal(event)
	msg := &queue.Message{Value: payload}

	err := service.handleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("handleMessage error: %v", err)
	}

	// Parent should NOT be resolved yet
	parent, _ := alertRepo.GetByDedupKey(ctx, "parent-alert")
	if parent.Status == domain.AlertStatusResolved {
		t.Error("Parent should not be resolved while children are active")
	}
	if !parent.ResolveRequested {
		t.Error("Parent should have ResolveRequested=true")
	}

	// Verify pending resolve was set
	pending, _ := stateStore.GetPendingResolve(ctx, "parent-alert")
	if pending == nil {
		t.Error("Pending resolve should be set")
	}
}

func TestProcessor_DuplicateEvent_Ignored(t *testing.T) {
	service, _, stateStore, alertRepo, emRepo, grRepo := testSetup()
	ctx := context.Background()

	setupTestData(ctx, emRepo, grRepo)

	// Create existing alert
	alertState := &store.AlertState{
		DedupKey:       "alert-1",
		EventManagerID: "em-1",
		Type:           string(domain.AlertTypeParent),
		Status:         string(domain.AlertStatusActive),
	}
	_ = stateStore.SetAlert(ctx, alertState)

	existingAlert := &domain.Alert{
		ID:             "existing-id",
		DedupKey:       "alert-1",
		EventManagerID: "em-1",
		Type:           domain.AlertTypeParent,
		Status:         domain.AlertStatusActive,
		Summary:        "Original summary",
		CreatedAt:      time.Now(),
	}
	_ = alertRepo.Create(ctx, existingAlert)

	// Send duplicate trigger event
	event := &domain.InternalEvent{
		Event: domain.Event{
			EventManagerID: "em-1",
			Summary:        "New summary", // Different summary
			Action:         domain.ActionTrigger,
			DedupKey:       "alert-1",
		},
		ReceivedAt: time.Now(),
	}

	payload, _ := json.Marshal(event)
	msg := &queue.Message{Value: payload}

	err := service.handleMessage(ctx, msg)
	if err != nil {
		t.Fatalf("handleMessage error: %v", err)
	}

	// Alert should not be modified (summary should remain original)
	alert, _ := alertRepo.GetByDedupKey(ctx, "alert-1")
	if alert.Summary != "Original summary" {
		t.Errorf("Alert summary should not change, got %v", alert.Summary)
	}
}
