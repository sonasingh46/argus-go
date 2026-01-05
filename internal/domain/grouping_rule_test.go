package domain

import (
	"testing"
	"time"
)

func TestGroupingRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    GroupingRule
		wantErr error
	}{
		{
			name: "valid rule",
			rule: GroupingRule{
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
			},
			wantErr: nil,
		},
		{
			name: "missing name",
			rule: GroupingRule{
				GroupingKey:       "class",
				TimeWindowMinutes: 5,
			},
			wantErr: ErrEmptyGroupingRuleName,
		},
		{
			name: "missing grouping key",
			rule: GroupingRule{
				Name:              "Test Rule",
				TimeWindowMinutes: 5,
			},
			wantErr: ErrEmptyGroupingKey,
		},
		{
			name: "zero time window",
			rule: GroupingRule{
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: 0,
			},
			wantErr: ErrInvalidTimeWindow,
		},
		{
			name: "negative time window",
			rule: GroupingRule{
				Name:              "Test Rule",
				GroupingKey:       "class",
				TimeWindowMinutes: -5,
			},
			wantErr: ErrInvalidTimeWindow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if err != tt.wantErr {
				t.Errorf("GroupingRule.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupingRule_TimeWindow(t *testing.T) {
	rule := GroupingRule{TimeWindowMinutes: 5}
	expected := 5 * time.Minute

	if got := rule.TimeWindow(); got != expected {
		t.Errorf("TimeWindow() = %v, want %v", got, expected)
	}
}

func TestGroupingRule_ExtractGroupingValue(t *testing.T) {
	event := &Event{
		EventManagerID: "em-1",
		Severity:       SeverityHigh,
		Class:          "database",
	}

	tests := []struct {
		name        string
		groupingKey string
		want        string
	}{
		{
			name:        "extract class",
			groupingKey: "class",
			want:        "database",
		},
		{
			name:        "extract severity",
			groupingKey: "severity",
			want:        "high",
		},
		{
			name:        "extract event_manager_id",
			groupingKey: "event_manager_id",
			want:        "em-1",
		},
		{
			name:        "unknown field returns empty",
			groupingKey: "unknown_field",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := GroupingRule{GroupingKey: tt.groupingKey}
			if got := rule.ExtractGroupingValue(event); got != tt.want {
				t.Errorf("ExtractGroupingValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateGroupingRuleRequest_ToGroupingRule(t *testing.T) {
	req := CreateGroupingRuleRequest{
		Name:              "Test Rule",
		GroupingKey:       "class",
		TimeWindowMinutes: 10,
	}
	id := "rule-123"

	rule := req.ToGroupingRule(id)

	if rule.ID != id {
		t.Errorf("ID = %v, want %v", rule.ID, id)
	}
	if rule.Name != req.Name {
		t.Errorf("Name = %v, want %v", rule.Name, req.Name)
	}
	if rule.GroupingKey != req.GroupingKey {
		t.Errorf("GroupingKey = %v, want %v", rule.GroupingKey, req.GroupingKey)
	}
	if rule.TimeWindowMinutes != req.TimeWindowMinutes {
		t.Errorf("TimeWindowMinutes = %v, want %v", rule.TimeWindowMinutes, req.TimeWindowMinutes)
	}
	if rule.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
}

func TestUpdateGroupingRuleRequest_ApplyTo(t *testing.T) {
	rule := &GroupingRule{
		ID:                "rule-123",
		Name:              "Old Name",
		GroupingKey:       "class",
		TimeWindowMinutes: 5,
	}
	req := UpdateGroupingRuleRequest{
		Name:              "New Name",
		GroupingKey:       "severity",
		TimeWindowMinutes: 10,
	}

	oldUpdatedAt := rule.UpdatedAt
	req.ApplyTo(rule)

	if rule.Name != req.Name {
		t.Errorf("Name = %v, want %v", rule.Name, req.Name)
	}
	if rule.GroupingKey != req.GroupingKey {
		t.Errorf("GroupingKey = %v, want %v", rule.GroupingKey, req.GroupingKey)
	}
	if rule.TimeWindowMinutes != req.TimeWindowMinutes {
		t.Errorf("TimeWindowMinutes = %v, want %v", rule.TimeWindowMinutes, req.TimeWindowMinutes)
	}
	if !rule.UpdatedAt.After(oldUpdatedAt) {
		t.Error("UpdatedAt should be updated")
	}
	// ID should not change
	if rule.ID != "rule-123" {
		t.Errorf("ID should not change, got %v", rule.ID)
	}
}
