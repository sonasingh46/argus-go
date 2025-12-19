package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

var es *elasticsearch.Client

func main() {
	// Initialize ES Client
	client, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses: []string{"http://localhost:9200"},
	})
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	es = client

	for {
		fmt.Printf("\n⏲️  [%s] Starting rule check cycle...\n", time.Now().Format("15:04:05"))

		rules := fetchThresholdRules()
		for _, rule := range rules {
			checkThreshold(rule)
		}

		time.Sleep(30 * time.Second)
	}
}

func checkThreshold(rule map[string]interface{}) {
	threshold, _ := rule["threshold"].(float64)
	window, _ := rule["window_minutes"].(float64)
	ruleName, _ := rule["rule_name"].(string)

	if window == 0 {
		window = 5
	}

	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": fmt.Sprintf("now-%dm", int(window)),
				},
			},
		},
		"aggs": map[string]interface{}{
			"hosts": map[string]interface{}{
				"terms": map[string]interface{}{"field": "host"}, // Ensure host is keyword in ES
				"aggs": map[string]interface{}{
					"avg_cpu": map[string]interface{}{
						"avg": map[string]interface{}{"field": "cpu_usage"},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(query)

	res, err := es.Search(es.Search.WithIndex("metrics"), es.Search.WithBody(&buf))
	if err != nil {
		return
	}
	defer res.Body.Close()

	var r map[string]interface{}
	json.NewDecoder(res.Body).Decode(&r)

	aggs, ok := r["aggregations"].(map[string]interface{})
	if !ok {
		fmt.Printf("ℹ️  [%s] No metrics found in window.\n", ruleName)
		return
	}

	buckets := aggs["hosts"].(map[string]interface{})["buckets"].([]interface{})
	if len(buckets) == 0 {
		fmt.Printf("ℹ️  [%s] No host data available.\n", ruleName)
		return
	}

	for _, b := range buckets {
		bucket := b.(map[string]interface{})
		hostName := bucket["key"].(string)
		val := bucket["avg_cpu"].(map[string]interface{})["value"]

		if val == nil {
			continue
		}
		avgValue := val.(float64)

		if avgValue > threshold {
			fmt.Printf("🚨 ALERT: %s | Host: %s | Avg: %.2f | Limit: %.2f\n", ruleName, hostName, avgValue, threshold)
			updateAlertState(ruleName, hostName, avgValue, "ACTIVE")
		} else {
			fmt.Printf("✅ Healthy: %s | Host: %s | Avg: %.2f\n", ruleName, hostName, avgValue)
			updateAlertState(ruleName, hostName, avgValue, "RESOLVED")
		}
	}
}

func updateAlertState(ruleName, host string, val float64, status string) {
	alertID := fmt.Sprintf("%s_%s", ruleName, host)

	// If resolving, we check if an ACTIVE one exists first to avoid cluttering RESOLVED docs
	if status == "RESOLVED" {
		res, _ := es.Get("active_alerts", alertID)
		if res.IsError() {
			return
		} // No active alert found, so nothing to resolve
	}

	doc := map[string]interface{}{
		"rule_name":  ruleName,
		"host":       host,
		"value":      val,
		"status":     status,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(doc)

	req := esapi.IndexRequest{
		Index:      "active_alerts",
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}

	res, _ := req.Do(context.Background(), es)
	defer res.Body.Close()
}

func fetchThresholdRules() []map[string]interface{} {
	res, err := es.Search(es.Search.WithIndex("alert_rules"))
	if err != nil {
		return nil
	}
	defer res.Body.Close()

	var r map[string]interface{}
	json.NewDecoder(res.Body).Decode(&r)

	var rules []map[string]interface{}
	hitsObj, ok := r["hits"].(map[string]interface{})
	if !ok {
		return nil
	}

	for _, hit := range hitsObj["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"].(map[string]interface{})
		rules = append(rules, source)
	}
	return rules
}
