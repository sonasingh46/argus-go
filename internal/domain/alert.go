package domain

import (
	"errors"
	"time"
)

// ErrAlertNotFound is returned when an alert cannot be found.
var ErrAlertNotFound = errors.New("alert not found")

// AlertType indicates whether an alert is a parent or child in the grouping hierarchy.
type AlertType string

const (
	// AlertTypeParent indicates this is a parent alert that may have children grouped under it.
	AlertTypeParent AlertType = "parent"
	// AlertTypeChild indicates this alert is grouped under a parent alert.
	AlertTypeChild AlertType = "child"
)

// AlertStatus represents the current state of an alert.
type AlertStatus string

const (
	// AlertStatusActive indicates the alert condition is currently active.
	AlertStatusActive AlertStatus = "active"
	// AlertStatusResolved indicates the alert has been resolved.
	AlertStatusResolved AlertStatus = "resolved"
)

// Alert represents a processed alert in the system.
// Alerts are created from incoming events after applying grouping logic.
type Alert struct {
	// ID is the unique database identifier for this alert.
	ID string `json:"id"`

	// DedupKey is the deduplication key from the original event.
	// This is the primary business identifier for the alert.
	DedupKey string `json:"dedupKey"`

	// EventManagerID identifies the namespace/tenant this alert belongs to.
	EventManagerID string `json:"event_manager_id"`

	// Summary is a human-readable description of the alert.
	Summary string `json:"summary"`

	// Severity indicates the alert severity level.
	Severity Severity `json:"severity"`

	// Class is the classification/category of the alert.
	Class string `json:"class"`

	// Type indicates whether this is a parent or child alert.
	Type AlertType `json:"type"`

	// Status indicates the current state (active or resolved).
	Status AlertStatus `json:"status"`

	// ParentDedupKey is set for child alerts to reference their parent.
	// Empty for parent alerts.
	ParentDedupKey string `json:"parent_dedupKey,omitempty"`

	// ChildCount tracks the number of child alerts grouped under this parent.
	// Only meaningful for parent alerts.
	ChildCount int `json:"child_count"`

	// ResolveRequested indicates a resolve action was received but the alert
	// cannot be resolved yet (e.g., parent waiting for children to resolve).
	ResolveRequested bool `json:"resolve_requested"`

	// CreatedAt is when the alert was first created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the alert was last modified.
	UpdatedAt time.Time `json:"updated_at"`

	// ResolvedAt is when the alert was resolved. Zero value if still active.
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}

// NewParentAlert creates a new parent alert from an event.
func NewParentAlert(event *Event) *Alert {
	now := time.Now().UTC()
	return &Alert{
		DedupKey:       event.DedupKey,
		EventManagerID: event.EventManagerID,
		Summary:        event.Summary,
		Severity:       event.Severity,
		Class:          event.Class,
		Type:           AlertTypeParent,
		Status:         AlertStatusActive,
		ChildCount:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// NewChildAlert creates a new child alert from an event, linked to a parent.
func NewChildAlert(event *Event, parentDedupKey string) *Alert {
	now := time.Now().UTC()
	return &Alert{
		DedupKey:       event.DedupKey,
		EventManagerID: event.EventManagerID,
		Summary:        event.Summary,
		Severity:       event.Severity,
		Class:          event.Class,
		Type:           AlertTypeChild,
		Status:         AlertStatusActive,
		ParentDedupKey: parentDedupKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// IsParent returns true if this is a parent alert.
func (a *Alert) IsParent() bool {
	return a.Type == AlertTypeParent
}

// IsChild returns true if this is a child alert.
func (a *Alert) IsChild() bool {
	return a.Type == AlertTypeChild
}

// IsActive returns true if the alert is currently active.
func (a *Alert) IsActive() bool {
	return a.Status == AlertStatusActive
}

// IsResolved returns true if the alert has been resolved.
func (a *Alert) IsResolved() bool {
	return a.Status == AlertStatusResolved
}

// Resolve marks the alert as resolved.
func (a *Alert) Resolve() {
	now := time.Now().UTC()
	a.Status = AlertStatusResolved
	a.UpdatedAt = now
	a.ResolvedAt = &now
	a.ResolveRequested = false
}

// MarkResolveRequested marks that a resolve was requested but cannot be completed yet.
// This is used for parent alerts waiting for children to resolve.
func (a *Alert) MarkResolveRequested() {
	a.ResolveRequested = true
	a.UpdatedAt = time.Now().UTC()
}

// IncrementChildCount increases the child counter for a parent alert.
func (a *Alert) IncrementChildCount() {
	a.ChildCount++
	a.UpdatedAt = time.Now().UTC()
}

// AlertFilter provides filtering options for querying alerts.
type AlertFilter struct {
	EventManagerID string
	Status         AlertStatus
	Type           AlertType
	Limit          int
	Offset         int
}
