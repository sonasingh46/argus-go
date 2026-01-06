// Package notification provides alert notification functionality.
// For MVP, this is a stubbed implementation that logs notifications.
// Future implementations will support webhook delivery with retry logic.
package notification

import (
	"context"
	"log/slog"
	"time"

	"argus-go/internal/domain"
	"argus-go/internal/metrics"
)

// NotificationPayload represents the data sent in webhook notifications.
type NotificationPayload struct {
	AlertID        string    `json:"alert_id"`
	DedupKey       string    `json:"dedupKey"`
	EventManagerID string    `json:"event_manager_id"`
	Summary        string    `json:"summary"`
	Severity       string    `json:"severity"`
	Status         string    `json:"status"`
	Type           string    `json:"type"`
	ChildCount     int       `json:"child_count"`
	Timestamp      time.Time `json:"timestamp"`
}

// Notifier defines the interface for sending alert notifications.
type Notifier interface {
	// NotifyNewParent sends a notification when a new parent alert is created.
	NotifyNewParent(ctx context.Context, alert *domain.Alert, em *domain.EventManager)

	// NotifyResolved sends a notification when a parent alert is resolved.
	NotifyResolved(ctx context.Context, alert *domain.Alert, em *domain.EventManager)
}

// StubNotifier is a no-op implementation that logs notifications.
// This is used for MVP until webhook delivery is implemented.
type StubNotifier struct {
	logger *slog.Logger
}

// NewStubNotifier creates a new stub notifier.
func NewStubNotifier(logger *slog.Logger) *StubNotifier {
	return &StubNotifier{
		logger: logger,
	}
}

// NotifyNewParent logs a notification for a new parent alert.
func (n *StubNotifier) NotifyNewParent(ctx context.Context, alert *domain.Alert, em *domain.EventManager) {
	payload := buildPayload(alert)

	n.logger.Info("STUB: would send new parent notification",
		"webhookURL", em.NotificationConfig.WebhookURL,
		"alertID", payload.AlertID,
		"dedupKey", payload.DedupKey,
		"summary", payload.Summary,
		"severity", payload.Severity,
	)

	// Track notification metrics
	metrics.NotificationsSentTotal.WithLabelValues(alert.EventManagerID, "success").Inc()

	// Track notification latency (time from alert creation to notification dispatch)
	if !alert.CreatedAt.IsZero() {
		metrics.NotificationLatency.Observe(time.Since(alert.CreatedAt).Seconds())
	}
}

// NotifyResolved logs a notification for a resolved parent alert.
func (n *StubNotifier) NotifyResolved(ctx context.Context, alert *domain.Alert, em *domain.EventManager) {
	payload := buildPayload(alert)

	n.logger.Info("STUB: would send resolved notification",
		"webhookURL", em.NotificationConfig.WebhookURL,
		"alertID", payload.AlertID,
		"dedupKey", payload.DedupKey,
		"summary", payload.Summary,
		"childCount", payload.ChildCount,
	)

	// Track notification metrics
	metrics.NotificationsSentTotal.WithLabelValues(alert.EventManagerID, "success").Inc()

	// Track notification latency (time from resolution to notification dispatch)
	// For resolved alerts, we use UpdatedAt as that's when the resolution happened
	if alert.ResolvedAt != nil {
		metrics.NotificationLatency.Observe(time.Since(*alert.ResolvedAt).Seconds())
	}
}

// buildPayload creates a notification payload from an alert.
func buildPayload(alert *domain.Alert) *NotificationPayload {
	return &NotificationPayload{
		AlertID:        alert.ID,
		DedupKey:       alert.DedupKey,
		EventManagerID: alert.EventManagerID,
		Summary:        alert.Summary,
		Severity:       string(alert.Severity),
		Status:         string(alert.Status),
		Type:           string(alert.Type),
		ChildCount:     alert.ChildCount,
		Timestamp:      time.Now().UTC(),
	}
}
