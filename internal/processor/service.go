// Package processor handles the core alert processing logic.
// It consumes events from the message queue and applies grouping rules,
// manages alert state, and handles the alert lifecycle.
package processor

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"argus-go/internal/domain"
	"argus-go/internal/notification"
	"argus-go/internal/queue"
	"argus-go/internal/store"
)

// Service processes events from the queue and manages alert lifecycle.
// It is responsible for:
// - Consuming events from the message queue
// - Determining if events should create new alerts or group with existing ones
// - Managing parent-child alert relationships
// - Handling alert resolution logic
// - Triggering notifications for parent alerts
type Service struct {
	consumer         queue.Consumer
	stateStore       store.StateStore
	alertRepo        store.AlertRepository
	eventManagerRepo store.EventManagerRepository
	groupingRuleRepo store.GroupingRuleRepository
	notifier         notification.Notifier
	logger           *slog.Logger
}

// NewService creates a new processor service.
func NewService(
	consumer queue.Consumer,
	stateStore store.StateStore,
	alertRepo store.AlertRepository,
	eventManagerRepo store.EventManagerRepository,
	groupingRuleRepo store.GroupingRuleRepository,
	notifier notification.Notifier,
	logger *slog.Logger,
) *Service {
	return &Service{
		consumer:         consumer,
		stateStore:       stateStore,
		alertRepo:        alertRepo,
		eventManagerRepo: eventManagerRepo,
		groupingRuleRepo: groupingRuleRepo,
		notifier:         notifier,
		logger:           logger,
	}
}

// Start begins consuming events from the queue and processing them.
// This is a blocking call that runs until the context is canceled.
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("starting processor service")
	return s.consumer.Start(ctx, s.handleMessage)
}

// handleMessage is the callback for processing each message from the queue.
func (s *Service) handleMessage(ctx context.Context, msg *queue.Message) error {
	// Deserialize the internal event
	var event domain.InternalEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		s.logger.Error("failed to deserialize event", "error", err)
		// Return nil to avoid reprocessing malformed messages
		return nil
	}

	s.logger.Debug("processing event",
		"dedupKey", event.DedupKey,
		"action", event.Action,
		"groupingValue", event.GroupingValue,
	)

	// Route to appropriate handler based on action
	switch event.Action {
	case domain.ActionTrigger:
		return s.handleTrigger(ctx, &event)
	case domain.ActionResolve:
		return s.handleResolve(ctx, &event)
	default:
		s.logger.Warn("unknown action", "action", event.Action, "dedupKey", event.DedupKey)
		return nil
	}
}

// handleTrigger processes a trigger action event.
// It determines whether to create a new parent alert or link as a child.
func (s *Service) handleTrigger(ctx context.Context, event *domain.InternalEvent) error {
	// Check if alert already exists (deduplication)
	existingAlert, err := s.stateStore.GetAlert(ctx, event.DedupKey)
	if err != nil {
		s.logger.Error("failed to check existing alert", "error", err)
		return err
	}

	if existingAlert != nil {
		// Alert already exists - update it if needed
		s.logger.Debug("alert already exists", "dedupKey", event.DedupKey, "status", existingAlert.Status)

		// If it was resolved and we get a new trigger, reactivate it
		if existingAlert.Status == string(domain.AlertStatusResolved) {
			return s.reactivateAlert(ctx, event, existingAlert)
		}

		// Already active, nothing to do
		return nil
	}

	// Look up event manager to get grouping rule
	em, err := s.eventManagerRepo.GetByID(ctx, event.EventManagerID)
	if err != nil {
		s.logger.Error("failed to fetch event manager", "error", err)
		return err
	}

	groupingRule, err := s.groupingRuleRepo.GetByID(ctx, em.GroupingRuleID)
	if err != nil {
		s.logger.Error("failed to fetch grouping rule", "error", err)
		return err
	}

	// Check for existing parent in the time window
	parentState, err := s.stateStore.GetParent(
		ctx,
		event.EventManagerID,
		groupingRule.GroupingKey,
		event.GroupingValue,
	)
	if err != nil {
		s.logger.Error("failed to check for parent", "error", err)
		return err
	}

	if parentState != nil {
		// Parent exists - create as child
		return s.createChildAlert(ctx, event, parentState, groupingRule)
	}

	// No parent exists - create as new parent
	return s.createParentAlert(ctx, event, groupingRule, em)
}

// createParentAlert creates a new parent alert.
func (s *Service) createParentAlert(
	ctx context.Context,
	event *domain.InternalEvent,
	rule *domain.GroupingRule,
	em *domain.EventManager,
) error {
	// Create the alert
	alert := domain.NewParentAlert(&event.Event)
	alert.ID = uuid.New().String()

	// Save to state store
	alertState := &store.AlertState{
		DedupKey:       alert.DedupKey,
		EventManagerID: alert.EventManagerID,
		Type:           string(alert.Type),
		Status:         string(alert.Status),
	}
	if err := s.stateStore.SetAlert(ctx, alertState); err != nil {
		s.logger.Error("failed to save alert state", "error", err)
		return err
	}

	// Save parent lookup with TTL based on grouping rule time window
	parentState := &store.ParentState{
		DedupKey:   alert.DedupKey,
		CreatedAt:  alert.CreatedAt,
		ChildCount: 0,
	}
	if err := s.stateStore.SetParent(
		ctx,
		event.EventManagerID,
		rule.GroupingKey,
		event.GroupingValue,
		parentState,
		rule.TimeWindow(),
	); err != nil {
		s.logger.Error("failed to save parent state", "error", err)
		return err
	}

	// Persist to database
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		s.logger.Error("failed to persist alert", "error", err)
		return err
	}

	s.logger.Info("created parent alert",
		"dedupKey", alert.DedupKey,
		"eventManagerID", alert.EventManagerID,
	)

	// Send notification for new parent alert
	s.notifier.NotifyNewParent(ctx, alert, em)

	return nil
}

// createChildAlert creates a child alert linked to an existing parent.
func (s *Service) createChildAlert(
	ctx context.Context,
	event *domain.InternalEvent,
	parentState *store.ParentState,
	rule *domain.GroupingRule,
) error {
	// Create the child alert
	alert := domain.NewChildAlert(&event.Event, parentState.DedupKey)
	alert.ID = uuid.New().String()

	// Save to state store
	alertState := &store.AlertState{
		DedupKey:       alert.DedupKey,
		EventManagerID: alert.EventManagerID,
		Type:           string(alert.Type),
		Status:         string(alert.Status),
		ParentDedupKey: alert.ParentDedupKey,
	}
	if err := s.stateStore.SetAlert(ctx, alertState); err != nil {
		s.logger.Error("failed to save alert state", "error", err)
		return err
	}

	// Add to parent's children set
	if err := s.stateStore.AddChild(ctx, parentState.DedupKey, alert.DedupKey); err != nil {
		s.logger.Error("failed to add child to parent", "error", err)
		return err
	}

	// Persist to database
	if err := s.alertRepo.Create(ctx, alert); err != nil {
		s.logger.Error("failed to persist alert", "error", err)
		return err
	}

	// Update parent's child count in database
	parentAlert, err := s.alertRepo.GetByDedupKey(ctx, parentState.DedupKey)
	if err == nil {
		parentAlert.IncrementChildCount()
		if updateErr := s.alertRepo.Update(ctx, parentAlert); updateErr != nil {
			s.logger.Warn("failed to update parent child count", "error", updateErr)
		}
	}

	s.logger.Info("created child alert",
		"dedupKey", alert.DedupKey,
		"parentDedupKey", parentState.DedupKey,
	)

	return nil
}

// reactivateAlert reactivates a previously resolved alert.
func (s *Service) reactivateAlert(
	ctx context.Context,
	event *domain.InternalEvent,
	existingState *store.AlertState,
) error {
	// Update state store
	existingState.Status = string(domain.AlertStatusActive)
	existingState.ResolveRequested = false
	if err := s.stateStore.SetAlert(ctx, existingState); err != nil {
		return err
	}

	// Update database
	alert, err := s.alertRepo.GetByDedupKey(ctx, event.DedupKey)
	if err != nil {
		return err
	}

	alert.Status = domain.AlertStatusActive
	alert.ResolveRequested = false
	alert.ResolvedAt = nil
	alert.UpdatedAt = time.Now().UTC()

	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return err
	}

	s.logger.Info("reactivated alert", "dedupKey", event.DedupKey)
	return nil
}

// handleResolve processes a resolve action event.
func (s *Service) handleResolve(ctx context.Context, event *domain.InternalEvent) error {
	// Look up existing alert state
	alertState, err := s.stateStore.GetAlert(ctx, event.DedupKey)
	if err != nil {
		s.logger.Error("failed to get alert state", "error", err)
		return err
	}

	if alertState == nil {
		s.logger.Warn("resolve requested for unknown alert", "dedupKey", event.DedupKey)
		return nil
	}

	// Already resolved - nothing to do
	if alertState.Status == string(domain.AlertStatusResolved) {
		s.logger.Debug("alert already resolved", "dedupKey", event.DedupKey)
		return nil
	}

	if alertState.Type == string(domain.AlertTypeChild) {
		return s.resolveChildAlert(ctx, event, alertState)
	}

	return s.resolveParentAlert(ctx, event, alertState)
}

// resolveChildAlert handles resolution of a child alert.
func (s *Service) resolveChildAlert(
	ctx context.Context,
	event *domain.InternalEvent,
	alertState *store.AlertState,
) error {
	// Update state store
	alertState.Status = string(domain.AlertStatusResolved)
	if err := s.stateStore.SetAlert(ctx, alertState); err != nil {
		return err
	}

	// Update database
	alert, err := s.alertRepo.GetByDedupKey(ctx, event.DedupKey)
	if err != nil {
		return err
	}
	alert.Resolve()
	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return err
	}

	s.logger.Info("resolved child alert", "dedupKey", event.DedupKey)

	// Check if parent has pending resolve and all children are now resolved
	if alertState.ParentDedupKey != "" {
		return s.checkParentResolution(ctx, alertState.ParentDedupKey)
	}

	return nil
}

// resolveParentAlert handles resolution of a parent alert.
func (s *Service) resolveParentAlert(
	ctx context.Context,
	event *domain.InternalEvent,
	alertState *store.AlertState,
) error {
	// Check if there are any active children
	activeChildren, err := s.alertRepo.CountActiveChildren(ctx, event.DedupKey)
	if err != nil {
		s.logger.Error("failed to count active children", "error", err)
		return err
	}

	if activeChildren > 0 {
		// Cannot resolve yet - mark as resolve requested
		alertState.ResolveRequested = true
		if err := s.stateStore.SetAlert(ctx, alertState); err != nil {
			return err
		}

		// Set pending resolve
		pending := &store.PendingResolve{
			RequestedAt:       time.Now().UTC(),
			RemainingChildren: activeChildren,
		}
		if err := s.stateStore.SetPendingResolve(ctx, event.DedupKey, pending); err != nil {
			return err
		}

		// Update database
		alert, err := s.alertRepo.GetByDedupKey(ctx, event.DedupKey)
		if err != nil {
			return err
		}
		alert.MarkResolveRequested()
		if err := s.alertRepo.Update(ctx, alert); err != nil {
			return err
		}

		s.logger.Info("parent resolve requested, waiting for children",
			"dedupKey", event.DedupKey,
			"activeChildren", activeChildren,
		)
		return nil
	}

	// No active children - can resolve immediately
	return s.completeParentResolution(ctx, event.DedupKey, alertState)
}

// checkParentResolution checks if a parent can now be resolved after a child resolution.
func (s *Service) checkParentResolution(ctx context.Context, parentDedupKey string) error {
	// Check if parent has pending resolve
	pending, err := s.stateStore.GetPendingResolve(ctx, parentDedupKey)
	if err != nil {
		return err
	}

	if pending == nil {
		// No pending resolve - nothing to do
		return nil
	}

	// Check if all children are now resolved
	activeChildren, err := s.alertRepo.CountActiveChildren(ctx, parentDedupKey)
	if err != nil {
		return err
	}

	if activeChildren > 0 {
		// Still have active children - update pending count
		pending.RemainingChildren = activeChildren
		return s.stateStore.SetPendingResolve(ctx, parentDedupKey, pending)
	}

	// All children resolved - resolve parent
	parentState, err := s.stateStore.GetAlert(ctx, parentDedupKey)
	if err != nil {
		return err
	}

	if parentState == nil {
		return errors.New("parent alert state not found")
	}

	return s.completeParentResolution(ctx, parentDedupKey, parentState)
}

// completeParentResolution finalizes the resolution of a parent alert.
func (s *Service) completeParentResolution(
	ctx context.Context,
	dedupKey string,
	alertState *store.AlertState,
) error {
	// Update state store
	alertState.Status = string(domain.AlertStatusResolved)
	alertState.ResolveRequested = false
	if err := s.stateStore.SetAlert(ctx, alertState); err != nil {
		return err
	}

	// Clean up pending resolve
	if err := s.stateStore.DeletePendingResolve(ctx, dedupKey); err != nil {
		s.logger.Warn("failed to delete pending resolve", "error", err)
	}

	// Update database
	alert, err := s.alertRepo.GetByDedupKey(ctx, dedupKey)
	if err != nil {
		return err
	}
	alert.Resolve()
	if err := s.alertRepo.Update(ctx, alert); err != nil {
		return err
	}

	s.logger.Info("resolved parent alert", "dedupKey", dedupKey)

	// Get event manager for notification
	em, err := s.eventManagerRepo.GetByID(ctx, alertState.EventManagerID)
	if err != nil {
		s.logger.Warn("failed to get event manager for notification", "error", err)
		return nil // Don't fail the resolution just because notification setup failed
	}

	// Send notification for resolved parent alert
	s.notifier.NotifyResolved(ctx, alert, em)

	return nil
}

// Stop gracefully stops the processor service.
func (s *Service) Stop() error {
	s.logger.Info("stopping processor service")
	return s.consumer.Close()
}
