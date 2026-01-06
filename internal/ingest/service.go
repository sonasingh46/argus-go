// Package ingest provides the event ingestion service.
// It handles receiving events, validating them, computing routing keys,
// and publishing to the message queue for asynchronous processing.
package ingest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"argus-go/internal/domain"
	"argus-go/internal/metrics"
	"argus-go/internal/queue"
	"argus-go/internal/store"
)

// Service handles event ingestion logic.
// It is responsible for:
// - Looking up event manager configuration
// - Extracting grouping values based on rules
// - Computing partition keys for ordered processing
// - Publishing events to the message queue
type Service struct {
	producer         queue.Producer
	eventManagerRepo store.EventManagerRepository
	groupingRuleRepo store.GroupingRuleRepository
	logger           *slog.Logger

	// eventManagerCache provides fast lookups for event managers.
	// In production, this would be a proper cache with TTL and invalidation.
	// For MVP, we fetch from repo on each request.
}

// NewService creates a new ingest service.
func NewService(
	producer queue.Producer,
	eventManagerRepo store.EventManagerRepository,
	groupingRuleRepo store.GroupingRuleRepository,
	logger *slog.Logger,
) *Service {
	return &Service{
		producer:         producer,
		eventManagerRepo: eventManagerRepo,
		groupingRuleRepo: groupingRuleRepo,
		logger:           logger,
	}
}

// Errors returned by the ingest service.
var (
	ErrEventManagerNotFound = errors.New("event manager not found")
	ErrGroupingRuleNotFound = errors.New("grouping rule not found")
	ErrPublishFailed        = errors.New("failed to publish event to queue")
)

// IngestEvent processes an incoming event and publishes it to the message queue.
// This is the main entry point for event ingestion.
//
// The processing flow:
// 1. Look up the event manager by ID
// 2. Look up the associated grouping rule
// 3. Extract the grouping value from the event
// 4. Compute the partition key for ordering
// 5. Publish to the message queue
func (s *Service) IngestEvent(ctx context.Context, event *domain.Event) error {
	ingestStart := time.Now()

	// Track event received
	metrics.EventsReceivedTotal.WithLabelValues(event.EventManagerID, string(event.Action)).Inc()

	// Step 1: Look up event manager
	em, err := s.eventManagerRepo.GetByID(ctx, event.EventManagerID)
	if err != nil {
		if errors.Is(err, domain.ErrEventManagerNotFound) {
			s.logger.Warn("event manager not found", "event_manager_id", event.EventManagerID)
			return ErrEventManagerNotFound
		}
		s.logger.Error("failed to fetch event manager", "error", err)
		return fmt.Errorf("failed to fetch event manager: %w", err)
	}

	// Step 2: Look up the grouping rule
	groupingRule, err := s.groupingRuleRepo.GetByID(ctx, em.GroupingRuleID)
	if err != nil {
		if errors.Is(err, domain.ErrGroupingRuleNotFound) {
			s.logger.Warn("grouping rule not found", "grouping_rule_id", em.GroupingRuleID)
			return ErrGroupingRuleNotFound
		}
		s.logger.Error("failed to fetch grouping rule", "error", err)
		return fmt.Errorf("failed to fetch grouping rule: %w", err)
	}

	// Step 3: Extract the grouping value from the event
	groupingValue := groupingRule.ExtractGroupingValue(event)

	// Step 4: Compute partition key
	// Events with the same partition key go to the same partition,
	// ensuring they are processed in order by a single consumer.
	partitionKey := computePartitionKey(event.EventManagerID, groupingValue)

	// Step 5: Create internal event with enriched data
	internalEvent := &domain.InternalEvent{
		Event:         *event,
		PartitionKey:  partitionKey,
		GroupingValue: groupingValue,
		ReceivedAt:    time.Now().UTC(),
	}

	// Serialize the internal event
	payload, err := json.Marshal(internalEvent)
	if err != nil {
		s.logger.Error("failed to serialize event", "error", err)
		return fmt.Errorf("failed to serialize event: %w", err)
	}

	// Step 6: Publish to message queue
	msg := &queue.Message{
		Key:   []byte(partitionKey),
		Value: payload,
		Headers: map[string]string{
			"event_manager_id": event.EventManagerID,
			"action":           string(event.Action),
			"dedupKey":         event.DedupKey,
		},
	}

	publishStart := time.Now()
	if err := s.producer.Publish(ctx, msg); err != nil {
		s.logger.Error("failed to publish event", "error", err, "dedupKey", event.DedupKey)
		return ErrPublishFailed
	}
	metrics.QueuePublishLatency.Observe(time.Since(publishStart).Seconds())

	// Track successful publish
	metrics.EventsPublishedTotal.WithLabelValues(event.EventManagerID).Inc()
	metrics.EventIngestLatency.Observe(time.Since(ingestStart).Seconds())

	s.logger.Debug("event published to queue",
		"dedupKey", event.DedupKey,
		"partitionKey", partitionKey,
		"groupingValue", groupingValue,
	)

	return nil
}

// computePartitionKey generates a deterministic partition key for an event.
// Events with the same event_manager_id and grouping_value will always
// get the same partition key, ensuring they go to the same partition.
//
// Format: hash(event_manager_id + grouping_value)
func computePartitionKey(eventManagerID, groupingValue string) string {
	input := eventManagerID + ":" + groupingValue
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars) for brevity
}
