package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"argus-go/internal/es"
	"argus-go/schema"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	Brand           = "ArgusGo"
	MetricsIndex    = "metrics"
)

// ThresholdRule defines a simple threshold rule for metrics.
type ThresholdRule struct {
	RuleName      string
	Threshold     float64
	WindowMinutes float64
}

// AlertEngine provides methods to check threshold rules and update alert state.
type AlertEngine struct {
	ES *es.Client
}

// New creates a new AlertEngine.
func New(esClient *es.Client) *AlertEngine {
	return &AlertEngine{ES: esClient}
}

// CheckThreshold checks a threshold rule and updates alert state accordingly.
func (a *AlertEngine) CheckThreshold(rule ThresholdRule) {
	fmt.Println("Checking threshold rule:", rule.RuleName)
	threshold := rule.Threshold
	window := rule.WindowMinutes
	ruleName := rule.RuleName

	if window == 0 {
		window = 5
	}

	query := buildThresholdQuery(window)
	r, err := a.ES.Search(MetricsIndex, query)
	if err != nil {
		return
	}

	buckets := extractBuckets(r)
	for _, b := range buckets {
		hostName, avgValue := extractHostAndValue(b)
		if hostName == "" {
			continue
		}
		if avgValue > threshold {
			fmt.Printf("[%s] 🚨 BREACH: %s | Host: %s | Val: %.2f\n", Brand, ruleName, hostName, avgValue)
			a.UpdateAlertState(ruleName, hostName, avgValue, "ACTIVE")
		} else {
			a.UpdateAlertState(ruleName, hostName, avgValue, "RESOLVED")
		}
	}
}

// buildThresholdQuery constructs the ES aggregation query for threshold checks.
func buildThresholdQuery(window float64) map[string]interface{} {
	return map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{"gte": fmt.Sprintf("now-%dm", int(window))},
			},
		},
		"aggs": map[string]interface{}{
			"hosts": map[string]interface{}{
				"terms": map[string]interface{}{"field": "host"},
				"aggs": map[string]interface{}{
					"avg_metric": map[string]interface{}{
						"avg": map[string]interface{}{"field": "cpu_usage"},
					},
				},
			},
		},
	}
}

// extractBuckets extracts aggregation buckets from the ES response.
func extractBuckets(r map[string]interface{}) []interface{} {
	aggs, ok := r["aggregations"].(map[string]interface{})
	if !ok {
		return nil
	}
	buckets, ok := aggs["hosts"].(map[string]interface{})["buckets"].([]interface{})
	if !ok {
		return nil
	}
	return buckets
}

// extractHostAndValue extracts the host name and average value from a bucket.
func extractHostAndValue(b interface{}) (string, float64) {
	bucket, ok := b.(map[string]interface{})
	if !ok {
		return "", 0
	}
	hostName, _ := bucket["key"].(string)
	val := bucket["avg_metric"].(map[string]interface{})["value"]
	if val == nil {
		return hostName, 0
	}
	avgValue, _ := val.(float64)
	return hostName, avgValue
}

// UpdateAlertState updates or creates an alert in ES for the given rule/host.
func (a *AlertEngine) UpdateAlertState(ruleName, host string, val float64, status string) {
	alertID := fmt.Sprintf("%s_%s", ruleName, host)
	indexName := ArgusAlertsIndex

	if status == "RESOLVED" && !alertExists(a.ES, indexName, alertID) {
		return
	}

	severity := "info"
	if status == "ACTIVE" {
		severity = "high"
	}

	doc := schema.Alert{
		Summary:   fmt.Sprintf("Rule %s triggered. Value: %.2f", ruleName, val),
		Severity:  severity,
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metadata: schema.AlertMetadata{
			Host:   host,
			RuleID: ruleName,
		},
	}

	saveOrUpdateAlert(a.ES, indexName, alertID, doc)
}

// alertExists checks if an alert exists in ES by ID.
func alertExists(esClient *es.Client, indexName, alertID string) bool {
	getRes, err := esClient.ES.Get(indexName, alertID)
	if err != nil {
		return false
	}
	defer getRes.Body.Close()

	return getRes.StatusCode == 200
}

// saveOrUpdateAlert saves or updates an alert in ES.
func saveOrUpdateAlert(esClient *es.Client, indexName, alertID string, alert schema.Alert) {
	if alertExists(esClient, indexName, alertID) {
		updateAlertDoc(esClient, indexName, alertID, alert)
	} else {
		createAlertDoc(esClient, indexName, alertID, alert)
	}
}

// updateAlertDoc updates an existing alert document in ES.
func updateAlertDoc(esClient *es.Client, indexName, alertID string, alert schema.Alert) {
	doc := map[string]interface{}{
		"doc": alert,
	}
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(doc)
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}
	res, _ := req.Do(context.Background(), esClient.ES)
	defer res.Body.Close()
}

// createAlertDoc creates a new alert document in ES.
func createAlertDoc(esClient *es.Client, indexName, alertID string, alert schema.Alert) {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(alert)
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}
	res, _ := req.Do(context.Background(), esClient.ES)
	defer res.Body.Close()
}
