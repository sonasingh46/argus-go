package domain

import (
	"errors"
	"time"
)

// EventManager represents a logical namespace/tenant abstraction for teams.
// Each team creates their own Event Manager to route and configure their alerts.
type EventManager struct {
	// ID is the unique identifier for this event manager.
	ID string `json:"id"`

	// Name is a human-readable name for the event manager.
	Name string `json:"name"`

	// Description provides additional context about this event manager.
	Description string `json:"description"`

	// GroupingRuleID links to the grouping rule that applies to this event manager.
	// For MVP, this is a 1:1 relationship.
	GroupingRuleID string `json:"grouping_rule_id"`

	// NotificationConfig contains webhook configuration for alert notifications.
	NotificationConfig NotificationConfig `json:"notification_config"`

	// CreatedAt is when the event manager was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the event manager was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// NotificationConfig holds webhook settings for sending alert notifications.
type NotificationConfig struct {
	// WebhookURL is the endpoint to send notifications to.
	WebhookURL string `json:"webhook_url"`
}

// Validation errors for EventManager.
var (
	ErrEmptyEventManagerName    = errors.New("name is required")
	ErrEmptyGroupingRuleID      = errors.New("grouping_rule_id is required")
	ErrEventManagerNotFound     = errors.New("event manager not found")
	ErrEventManagerAlreadyExists = errors.New("event manager already exists")
)

// Validate checks if the event manager has all required fields.
func (em *EventManager) Validate() error {
	if em.Name == "" {
		return ErrEmptyEventManagerName
	}
	if em.GroupingRuleID == "" {
		return ErrEmptyGroupingRuleID
	}
	return nil
}

// CreateEventManagerRequest represents the input for creating a new event manager.
type CreateEventManagerRequest struct {
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	GroupingRuleID     string             `json:"grouping_rule_id"`
	NotificationConfig NotificationConfig `json:"notification_config"`
}

// Validate checks the create request has required fields.
func (r *CreateEventManagerRequest) Validate() error {
	if r.Name == "" {
		return ErrEmptyEventManagerName
	}
	if r.GroupingRuleID == "" {
		return ErrEmptyGroupingRuleID
	}
	return nil
}

// ToEventManager converts the request to an EventManager entity.
func (r *CreateEventManagerRequest) ToEventManager(id string) *EventManager {
	now := time.Now().UTC()
	return &EventManager{
		ID:                 id,
		Name:               r.Name,
		Description:        r.Description,
		GroupingRuleID:     r.GroupingRuleID,
		NotificationConfig: r.NotificationConfig,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
}

// UpdateEventManagerRequest represents the input for updating an event manager.
type UpdateEventManagerRequest struct {
	Name               string             `json:"name"`
	Description        string             `json:"description"`
	GroupingRuleID     string             `json:"grouping_rule_id"`
	NotificationConfig NotificationConfig `json:"notification_config"`
}

// Validate checks the update request has required fields.
func (r *UpdateEventManagerRequest) Validate() error {
	if r.Name == "" {
		return ErrEmptyEventManagerName
	}
	if r.GroupingRuleID == "" {
		return ErrEmptyGroupingRuleID
	}
	return nil
}

// ApplyTo updates an existing EventManager with the request values.
func (r *UpdateEventManagerRequest) ApplyTo(em *EventManager) {
	em.Name = r.Name
	em.Description = r.Description
	em.GroupingRuleID = r.GroupingRuleID
	em.NotificationConfig = r.NotificationConfig
	em.UpdatedAt = time.Now().UTC()
}
