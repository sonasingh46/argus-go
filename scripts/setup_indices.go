package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

const (
	metricsIndex       = "metrics"
	esqueryAlertIndex  = "esquery_alert"
	alertsIndex        = "argusgo-alerts"
	groupingRulesIndex = "grouping_rules"
)

func main() {
	esURL := "http://localhost:9200"

	// Metrics index mapping
	metricsMapping := []byte(`{
	  "mappings": {
	    "properties": {
	      "timestamp": { "type": "date" },
	      "service":   { "type": "keyword" },
	      "host":      { "type": "keyword" },
	      "cpu_usage": { "type": "double" }
	    }
	  }
	}`)

	// esquery_alert index mapping
	esqueryAlertMapping := []byte(`{
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
	          "alert_type":     { "type": "keyword" },
	          "timestamp":      { "type": "date" },
	          "dedup_key":      { "type": "keyword" },
	          "grouped_alerts": { "type": "keyword" },
	          "metadata": {
	            "properties": {
	              "dependencies": { "type": "keyword" },
	              "host":         { "type": "keyword" },
	              "rule_id":      { "type": "keyword" },
	              "trigger_count": { "type": "integer" }
	            }
	          }
	        }
	      }
	    }
	  }
	}`)

	// argusgo-alerts index mapping
	alertsMapping := []byte(`{
	  "mappings": {
	    "properties": {
	      "summary":        { "type": "text" },
	      "severity":       { "type": "keyword" },
	      "status":         { "type": "keyword" },
	      "alert_type":     { "type": "keyword" },
	      "timestamp":      { "type": "date" },
	      "dedup_key":      { "type": "keyword" },
	      "grouped_alerts": { "type": "keyword" },
	      "metadata": {
	        "properties": {
	          "dependencies": { "type": "keyword" },
	          "host":         { "type": "keyword" },
	          "rule_id":      { "type": "keyword" },
	          "trigger_count": { "type": "integer" }
	        }
	      }
	    }
	  }
	}`)

	// grouping_rules index mapping
	groupingRulesMapping := []byte(`{
	  "mappings": {
	    "properties": {
	      "id":             { "type": "keyword" },
	      "name":           { "type": "text" },
	      "group_by_field": { "type": "keyword" },
	      "time_window":    { "type": "keyword" }
	    }
	  }
	}`)

	createIndex(esURL, metricsIndex, metricsMapping)
	createIndex(esURL, esqueryAlertIndex, esqueryAlertMapping)
	createIndex(esURL, alertsIndex, alertsMapping)
	createIndex(esURL, groupingRulesIndex, groupingRulesMapping)
}

func createIndex(esURL, index string, mapping []byte) {
	url := fmt.Sprintf("%s/%s", esURL, index)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(mapping))
	if err != nil {
		fmt.Printf("Failed to create request for %s: %v\n", index, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Printf("Failed to create index %s: %v\n", index, err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Index %s: %s\n", index, string(body))
}
