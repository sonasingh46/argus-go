package schema

import "time"

type AlertMetadata struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Host         string   `json:"host,omitempty"`
	RuleID       string   `json:"rule_id,omitempty"`
}

type Alert struct {
	Summary   string        `json:"summary"`
	Severity  string        `json:"severity"`
	Status    string        `json:"status"`
	Timestamp time.Time     `json:"timestamp"`
	Metadata  AlertMetadata `json:"metadata"`
}
