// Package domain contains the core business entities and value objects for ArgusGo.
// These models represent the ubiquitous language of the alert management domain.
package domain

import (
	"errors"
	"time"
)

// Action represents the intent of an incoming event.
type Action string

const (
	// ActionTrigger indicates the event should create or activate an alert.
	ActionTrigger Action = "trigger"
	// ActionResolve indicates the event requests resolution of an alert.
	ActionResolve Action = "resolve"
)

// Severity represents the severity level of an alert.
type Severity string

const (
	SeverityHigh   Severity = "high"
	SeverityMedium Severity = "medium"
	SeverityLow    Severity = "low"
)

// Event represents an incoming alert event from a client.
// This is the input payload received at the ingestion endpoint.
type Event struct {
	// EventManagerID identifies the namespace/tenant this event belongs to.
	EventManagerID string `json:"event_manager_id"`

	// Summary is a human-readable description of the alert.
	Summary string `json:"summary"`

	// Severity indicates the alert severity level.
	Severity Severity `json:"severity"`

	// Action indicates client intent: trigger to create/activate, resolve to close.
	Action Action `json:"action"`

	// Class is the classification/category of the alert.
	Class string `json:"class"`

	// DedupKey is the unique identifier for deduplication.
	DedupKey string `json:"dedupKey"`
}

// Validation errors for Event.
var (
	ErrEmptyEventManagerID = errors.New("event_manager_id is required")
	ErrEmptySummary        = errors.New("summary is required")
	ErrInvalidSeverity     = errors.New("severity must be 'high', 'medium', or 'low'")
	ErrInvalidAction       = errors.New("action must be 'trigger' or 'resolve'")
	ErrEmptyDedupKey       = errors.New("dedupKey is required")
)

// Validate checks if the event has all required fields with valid values.
// Returns an error describing the first validation failure, or nil if valid.
func (e *Event) Validate() error {
	if e.EventManagerID == "" {
		return ErrEmptyEventManagerID
	}
	if e.Summary == "" {
		return ErrEmptySummary
	}
	if !e.Severity.IsValid() {
		return ErrInvalidSeverity
	}
	if !e.Action.IsValid() {
		return ErrInvalidAction
	}
	if e.DedupKey == "" {
		return ErrEmptyDedupKey
	}
	return nil
}

// IsValid returns true if the severity is a known valid value.
func (s Severity) IsValid() bool {
	switch s {
	case SeverityHigh, SeverityMedium, SeverityLow:
		return true
	default:
		return false
	}
}

// IsValid returns true if the action is a known valid value.
func (a Action) IsValid() bool {
	switch a {
	case ActionTrigger, ActionResolve:
		return true
	default:
		return false
	}
}

// InternalEvent is an enriched event used for internal processing.
// It contains the original event plus computed routing information.
type InternalEvent struct {
	Event

	// PartitionKey is the computed key for message queue partitioning.
	// Format: hash(event_manager_id + grouping_key_value)
	PartitionKey string `json:"partition_key"`

	// GroupingValue is the value extracted from the event based on the grouping rule.
	// For example, if grouping_key is "class", this would be the event's class value.
	GroupingValue string `json:"grouping_value"`

	// ReceivedAt is the timestamp when the event was received by the ingest service.
	ReceivedAt time.Time `json:"received_at"`
}
