package ingest

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"argus-go/internal/domain"
	"argus-go/internal/queue"
	"argus-go/internal/queue/memory"
	storemem "argus-go/internal/store/memory"
)

func TestService_IngestEvent(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	msgQueue := memory.NewQueue(100)
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()

	service := NewService(msgQueue, eventManagerRepo, groupingRuleRepo, logger)

	// Create test data
	ctx := context.Background()

	groupingRule := &domain.GroupingRule{
		ID:                "rule-1",
		Name:              "Test Rule",
		GroupingKey:       "class",
		TimeWindowMinutes: 5,
		CreatedAt:         time.Now(),
	}
	_ = groupingRuleRepo.Create(ctx, groupingRule)

	eventManager := &domain.EventManager{
		ID:             "em-1",
		Name:           "Test EM",
		GroupingRuleID: "rule-1",
		CreatedAt:      time.Now(),
	}
	_ = eventManagerRepo.Create(ctx, eventManager)

	// Test successful ingestion
	event := &domain.Event{
		EventManagerID: "em-1",
		Summary:        "Test alert",
		Severity:       domain.SeverityHigh,
		Action:         domain.ActionTrigger,
		Class:          "database",
		DedupKey:       "alert-1",
	}

	err := service.IngestEvent(ctx, event)
	if err != nil {
		t.Errorf("IngestEvent() error = %v", err)
	}

	// Verify message was published
	if msgQueue.Len() != 1 {
		t.Errorf("Queue should have 1 message, got %d", msgQueue.Len())
	}
}

func TestService_IngestEvent_EventManagerNotFound(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	msgQueue := memory.NewQueue(100)
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()

	service := NewService(msgQueue, eventManagerRepo, groupingRuleRepo, logger)

	// Test with non-existent event manager
	event := &domain.Event{
		EventManagerID: "non-existent",
		Summary:        "Test alert",
		Severity:       domain.SeverityHigh,
		Action:         domain.ActionTrigger,
		Class:          "database",
		DedupKey:       "alert-1",
	}

	err := service.IngestEvent(context.Background(), event)
	if err != ErrEventManagerNotFound {
		t.Errorf("Expected ErrEventManagerNotFound, got %v", err)
	}
}

func TestService_IngestEvent_GroupingRuleNotFound(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	msgQueue := memory.NewQueue(100)
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()

	service := NewService(msgQueue, eventManagerRepo, groupingRuleRepo, logger)

	ctx := context.Background()

	// Create event manager pointing to non-existent grouping rule
	eventManager := &domain.EventManager{
		ID:             "em-1",
		Name:           "Test EM",
		GroupingRuleID: "non-existent-rule",
		CreatedAt:      time.Now(),
	}
	_ = eventManagerRepo.Create(ctx, eventManager)

	event := &domain.Event{
		EventManagerID: "em-1",
		Summary:        "Test alert",
		Severity:       domain.SeverityHigh,
		Action:         domain.ActionTrigger,
		Class:          "database",
		DedupKey:       "alert-1",
	}

	err := service.IngestEvent(ctx, event)
	if err != ErrGroupingRuleNotFound {
		t.Errorf("Expected ErrGroupingRuleNotFound, got %v", err)
	}
}

func TestService_IngestEvent_MessageFormat(t *testing.T) {
	// Setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	msgQueue := memory.NewQueue(100)
	eventManagerRepo := storemem.NewEventManagerRepository()
	groupingRuleRepo := storemem.NewGroupingRuleRepository()

	service := NewService(msgQueue, eventManagerRepo, groupingRuleRepo, logger)

	ctx := context.Background()

	groupingRule := &domain.GroupingRule{
		ID:                "rule-1",
		Name:              "Test Rule",
		GroupingKey:       "class",
		TimeWindowMinutes: 5,
		CreatedAt:         time.Now(),
	}
	_ = groupingRuleRepo.Create(ctx, groupingRule)

	eventManager := &domain.EventManager{
		ID:             "em-1",
		Name:           "Test EM",
		GroupingRuleID: "rule-1",
		CreatedAt:      time.Now(),
	}
	_ = eventManagerRepo.Create(ctx, eventManager)

	event := &domain.Event{
		EventManagerID: "em-1",
		Summary:        "Test alert",
		Severity:       domain.SeverityHigh,
		Action:         domain.ActionTrigger,
		Class:          "database",
		DedupKey:       "alert-1",
	}

	_ = service.IngestEvent(ctx, event)

	// Start a consumer to read the message
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var receivedEvent domain.InternalEvent
	_ = msgQueue.Start(ctx, func(ctx context.Context, msg *queue.Message) error {
		_ = json.Unmarshal(msg.Value, &receivedEvent)
		return nil
	})

	// Verify internal event format
	if receivedEvent.DedupKey != event.DedupKey {
		t.Errorf("DedupKey = %v, want %v", receivedEvent.DedupKey, event.DedupKey)
	}
	if receivedEvent.GroupingValue != "database" {
		t.Errorf("GroupingValue = %v, want 'database'", receivedEvent.GroupingValue)
	}
	if receivedEvent.PartitionKey == "" {
		t.Error("PartitionKey should be set")
	}
	if receivedEvent.ReceivedAt.IsZero() {
		t.Error("ReceivedAt should be set")
	}
}

func TestComputePartitionKey(t *testing.T) {
	// Same inputs should produce same output
	key1 := computePartitionKey("em-1", "database")
	key2 := computePartitionKey("em-1", "database")

	if key1 != key2 {
		t.Error("Same inputs should produce same partition key")
	}

	// Different inputs should produce different output
	key3 := computePartitionKey("em-1", "web")
	if key1 == key3 {
		t.Error("Different inputs should produce different partition keys")
	}

	key4 := computePartitionKey("em-2", "database")
	if key1 == key4 {
		t.Error("Different event manager should produce different partition key")
	}

	// Key should not be empty
	if key1 == "" {
		t.Error("Partition key should not be empty")
	}
}
