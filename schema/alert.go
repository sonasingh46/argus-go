package schema

import "time"

type AlertType string

const (
	AlertTypeParent  AlertType = "parent"
	AlertTypeGrouped AlertType = "grouped"
)

type GroupingRule struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	GroupByField string `json:"group_by_field"` // e.g., "metadata.host"
	TimeWindow   string `json:"time_window"`    // e.g., "10m"
}

type DedupRules struct {
	Key    string   `json:"key"`
	Fields []string `json:"fields"`
}

// ESQueryAlertRule represents the document structure for the "esquery_alert" index.
type ESQueryAlertRule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Index      string      `json:"index"`       // The target index to search against
	Query      string      `json:"query"`       // The raw ES query DSL (stored as a string)
	TimeWindow string      `json:"time_window"` // e.g., "5m", "1h"
	Threshold  int         `json:"threshold"`   // Number of hits to trigger the alert
	DedupRules *DedupRules `json:"dedup_rules,omitempty"`
	Alert      Alert       `json:"alert"`
}

type AlertMetadata struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Host         string   `json:"host,omitempty"`
	RuleID       string   `json:"rule_id,omitempty"`
	TriggerCount int      `json:"trigger_count,omitempty"`
}

type Alert struct {
	Summary       string        `json:"summary"`
	Severity      string        `json:"severity"`
	Status        string        `json:"status"`
	AlertType     AlertType     `json:"alert_type"`
	Timestamp     time.Time     `json:"timestamp"`
	Metadata      AlertMetadata `json:"metadata"`
	DedupKey      string        `json:"dedup_key"`      // Unique key for deduplication
	GroupedAlerts []string      `json:"grouped_alerts"` // List of alert IDs grouped together
}
