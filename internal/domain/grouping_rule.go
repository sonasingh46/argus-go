package domain

import (
	"errors"
	"time"
)

// GroupingRule defines how incoming events are grouped into parent/child alerts.
// For MVP, this supports single-field grouping with a fixed time window.
type GroupingRule struct {
	// ID is the unique identifier for this grouping rule.
	ID string `json:"id"`

	// Name is a human-readable name for the grouping rule.
	Name string `json:"name"`

	// GroupingKey is the field name from the event to use for grouping.
	// For example, "class" would group events by their class field value.
	GroupingKey string `json:"grouping_key"`

	// TimeWindowMinutes defines how long a parent alert remains "open" for grouping.
	// New events with the same grouping key value within this window become children.
	TimeWindowMinutes int `json:"time_window_minutes"`

	// CreatedAt is when the grouping rule was created.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the grouping rule was last modified.
	UpdatedAt time.Time `json:"updated_at"`
}

// Validation errors for GroupingRule.
var (
	ErrEmptyGroupingRuleName = errors.New("name is required")
	ErrEmptyGroupingKey      = errors.New("grouping_key is required")
	ErrInvalidTimeWindow     = errors.New("time_window_minutes must be positive")
	ErrGroupingRuleNotFound  = errors.New("grouping rule not found")
)

// Validate checks if the grouping rule has all required fields with valid values.
func (gr *GroupingRule) Validate() error {
	if gr.Name == "" {
		return ErrEmptyGroupingRuleName
	}
	if gr.GroupingKey == "" {
		return ErrEmptyGroupingKey
	}
	if gr.TimeWindowMinutes <= 0 {
		return ErrInvalidTimeWindow
	}
	return nil
}

// TimeWindow returns the time window as a time.Duration.
func (gr *GroupingRule) TimeWindow() time.Duration {
	return time.Duration(gr.TimeWindowMinutes) * time.Minute
}

// ExtractGroupingValue extracts the value of the grouping key from an event.
// For MVP, we support extracting from known fields: class.
// Returns empty string if the field is not found or not supported.
func (gr *GroupingRule) ExtractGroupingValue(event *Event) string {
	switch gr.GroupingKey {
	case "class":
		return event.Class
	case "severity":
		return string(event.Severity)
	case "event_manager_id":
		return event.EventManagerID
	default:
		// For MVP, only support known fields
		// Future: support arbitrary fields via reflection or map-based events
		return ""
	}
}

// CreateGroupingRuleRequest represents the input for creating a new grouping rule.
type CreateGroupingRuleRequest struct {
	Name              string `json:"name"`
	GroupingKey       string `json:"grouping_key"`
	TimeWindowMinutes int    `json:"time_window_minutes"`
}

// Validate checks the create request has required fields.
func (r *CreateGroupingRuleRequest) Validate() error {
	if r.Name == "" {
		return ErrEmptyGroupingRuleName
	}
	if r.GroupingKey == "" {
		return ErrEmptyGroupingKey
	}
	if r.TimeWindowMinutes <= 0 {
		return ErrInvalidTimeWindow
	}
	return nil
}

// ToGroupingRule converts the request to a GroupingRule entity.
func (r *CreateGroupingRuleRequest) ToGroupingRule(id string) *GroupingRule {
	now := time.Now().UTC()
	return &GroupingRule{
		ID:                id,
		Name:              r.Name,
		GroupingKey:       r.GroupingKey,
		TimeWindowMinutes: r.TimeWindowMinutes,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// UpdateGroupingRuleRequest represents the input for updating a grouping rule.
type UpdateGroupingRuleRequest struct {
	Name              string `json:"name"`
	GroupingKey       string `json:"grouping_key"`
	TimeWindowMinutes int    `json:"time_window_minutes"`
}

// Validate checks the update request has required fields.
func (r *UpdateGroupingRuleRequest) Validate() error {
	if r.Name == "" {
		return ErrEmptyGroupingRuleName
	}
	if r.GroupingKey == "" {
		return ErrEmptyGroupingKey
	}
	if r.TimeWindowMinutes <= 0 {
		return ErrInvalidTimeWindow
	}
	return nil
}

// ApplyTo updates an existing GroupingRule with the request values.
func (r *UpdateGroupingRuleRequest) ApplyTo(gr *GroupingRule) {
	gr.Name = r.Name
	gr.GroupingKey = r.GroupingKey
	gr.TimeWindowMinutes = r.TimeWindowMinutes
	gr.UpdatedAt = time.Now().UTC()
}
