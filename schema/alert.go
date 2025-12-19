package schema

import "time"

type AlertType string

const (
	AlertTypeParent  AlertType = "parent"
	AlertTypeGrouped AlertType = "grouped"
)

// ESQueryAlertRule represents the document structure for the "esquery_alert" index.
type ESQueryAlertRule struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Index      string `json:"index"`       // The target index to search against
	Query      string `json:"query"`       // The raw ES query DSL (stored as a string)
	TimeWindow string `json:"time_window"` // e.g., "5m", "1h"
	Threshold  int    `json:"threshold"`   // Number of hits to trigger the alert
	Alert      Alert  `json:"alert"`
}

type AlertMetadata struct {
	Dependencies []string `json:"dependencies,omitempty"`
	Host         string   `json:"host,omitempty"`
	RuleID       string   `json:"rule_id,omitempty"`
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

/*
// Mapping for "metrics" index
PUT metrics
{
  "mappings": {
    "properties": {
      "timestamp": {
        "type": "date"
      },
      "service": {
        "type": "keyword"
      },
      "host": {
        "type": "keyword"
      },
      "cpu_usage": {
        "type": "double"
      }
    }
  }
}

// Mapping for "esquery_alert" index
PUT /esquery_alert
{
  "mappings": {
    "properties": {
      "id":          { "type": "keyword" },
      "name":        { "type": "text" },
      "type":        { "type": "keyword" },
      "index":       { "type": "keyword" },
      "query":       { "type": "text" },
      "time_window": { "type": "keyword" },
      "threshold":   { "type": "integer" },
      "alert": {
        "properties": {
          "summary":        { "type": "text" },
          "severity":       { "type": "keyword" },
          "status":         { "type": "keyword" },
          "timestamp":      { "type": "date" },
          "dedup_key":      { "type": "keyword" },
          "grouped_alerts": { "type": "keyword" },
          "metadata": {
            "properties": {
              "dependencies": { "type": "keyword" },
              "host":         { "type": "keyword" },
              "rule_id":      { "type": "keyword" }
            }
          }
        }
      }
    }
  }
}


// Mapping for "argusgo-alerts" index
PUT /argusgo-alerts
{
  "mappings": {
    "properties": {
      "summary":        { "type": "text" },
      "severity":       { "type": "keyword" },
      "status":         { "type": "keyword" },
      "timestamp":      { "type": "date" },
      "dedup_key":      { "type": "keyword" },
      "grouped_alerts": { "type": "keyword" },
      "metadata": {
        "properties": {
          "dependencies": { "type": "keyword" },
          "host":         { "type": "keyword" },
          "rule_id":      { "type": "keyword" }
        }
      }
    }
  }
}
*/

/*
// Sample ESQuery alert rule for Kibana Dev Tools:
POST esquery_alert/_doc
{
  "id": "high_cpu_usage",
  "name": "High CPU Usage",
  "type": "esquery",
  "index": "metrics",
  "query": "{ \"query\": { \"range\": { \"cpu_usage\": { \"gte\": 90 } } } }",
  "time_window": "5m",
  "threshold": 1,
  "alert": {
    "summary": "CPU usage above 90%",
    "severity": "high",
    "dedup_key": "cpu-usage"
  }
}
*/
