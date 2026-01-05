package domain

import (
	"testing"
)

func TestEvent_Validate(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr error
	}{
		{
			name: "valid event",
			event: Event{
				EventManagerID: "em-1",
				Summary:        "Test alert",
				Severity:       SeverityHigh,
				Action:         ActionTrigger,
				Class:          "database",
				DedupKey:       "db-alert-1",
			},
			wantErr: nil,
		},
		{
			name: "missing event_manager_id",
			event: Event{
				Summary:  "Test alert",
				Severity: SeverityHigh,
				Action:   ActionTrigger,
				DedupKey: "db-alert-1",
			},
			wantErr: ErrEmptyEventManagerID,
		},
		{
			name: "missing summary",
			event: Event{
				EventManagerID: "em-1",
				Severity:       SeverityHigh,
				Action:         ActionTrigger,
				DedupKey:       "db-alert-1",
			},
			wantErr: ErrEmptySummary,
		},
		{
			name: "invalid severity",
			event: Event{
				EventManagerID: "em-1",
				Summary:        "Test alert",
				Severity:       "critical", // invalid
				Action:         ActionTrigger,
				DedupKey:       "db-alert-1",
			},
			wantErr: ErrInvalidSeverity,
		},
		{
			name: "invalid action",
			event: Event{
				EventManagerID: "em-1",
				Summary:        "Test alert",
				Severity:       SeverityHigh,
				Action:         "unknown", // invalid
				DedupKey:       "db-alert-1",
			},
			wantErr: ErrInvalidAction,
		},
		{
			name: "missing dedupKey",
			event: Event{
				EventManagerID: "em-1",
				Summary:        "Test alert",
				Severity:       SeverityHigh,
				Action:         ActionTrigger,
			},
			wantErr: ErrEmptyDedupKey,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if err != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSeverity_IsValid(t *testing.T) {
	tests := []struct {
		severity Severity
		want     bool
	}{
		{SeverityHigh, true},
		{SeverityMedium, true},
		{SeverityLow, true},
		{"critical", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			if got := tt.severity.IsValid(); got != tt.want {
				t.Errorf("Severity.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAction_IsValid(t *testing.T) {
	tests := []struct {
		action Action
		want   bool
	}{
		{ActionTrigger, true},
		{ActionResolve, true},
		{"unknown", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			if got := tt.action.IsValid(); got != tt.want {
				t.Errorf("Action.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}
