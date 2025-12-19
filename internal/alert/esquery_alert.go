package alert

import (
	"encoding/json"
	"fmt"
	"time"

	"argus-go/internal/es"
	"argus-go/schema"
)

const (
	ArgusAlertsIndex = "argusgo-alerts"
)

// ExecuteESQueryAlertRule runs the ESQuery alert rule and generates alerts based on the threshold.
func ExecuteESQueryAlertRule(esClient *es.Client, rule schema.ESQueryAlertRule) ([]schema.Alert, error) {
	var alerts []schema.Alert

	// Run the ES query and get the hit count
	hitCount, err := runESQueryForRule(esClient, rule)
	if err != nil {
		return nil, err
	}

	// Build the alert object
	alert := buildAlertFromRule(rule)
	alert.Status = determineAlertStatus(hitCount, rule.Threshold)
	alert.Timestamp = time.Now().UTC()

	// Check if alert already exists
	found, _ := fetchExistingAlert(esClient, alert.DedupKey)
	if found {
		// Optionally, merge fields from existingAlert if needed
		alert.Timestamp = time.Now().UTC()
	}

	alerts = append(alerts, alert)
	printAlertStatus(alert, rule.ID)
	return alerts, nil
}

// runESQueryForRule executes the ES query for the given rule and returns the hit count.
// It injects a time window filter on the "timestamp" field.
func runESQueryForRule(esClient *es.Client, rule schema.ESQueryAlertRule) (int, error) {
	query, err := parseQuery(rule.Query)
	if err != nil {
		return 0, err
	}
	injectTimeWindowFilter(query, rule.TimeWindow)
	return getHitCount(esClient, rule.Index, query)
}

// parseQuery parses the raw query string into a map.
func parseQuery(raw string) (map[string]interface{}, error) {
	var query map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &query); err != nil {
		return nil, fmt.Errorf("invalid query DSL: %w", err)
	}
	return query, nil
}

// injectTimeWindowFilter adds a time window filter to the query.
func injectTimeWindowFilter(query map[string]interface{}, timeWindow string) {
	if timeWindow == "" {
		timeWindow = "5m"
	}
	rangeFilter := map[string]interface{}{
		"range": map[string]interface{}{
			"timestamp": map[string]interface{}{
				"gte": fmt.Sprintf("now-%s", timeWindow),
			},
		},
	}

	// Ensure the query is a bool/filter or add it as a filter
	if q, ok := query["query"].(map[string]interface{}); ok {
		if boolQ, ok := q["bool"].(map[string]interface{}); ok {
			if filters, ok := boolQ["filter"].([]interface{}); ok {
				boolQ["filter"] = append(filters, rangeFilter)
			} else {
				boolQ["filter"] = []interface{}{rangeFilter}
			}
		} else {
			query["query"] = map[string]interface{}{
				"bool": map[string]interface{}{
					"must":   q,
					"filter": []interface{}{rangeFilter},
				},
			}
		}
	} else {
		query["query"] = rangeFilter
	}
}

// getHitCount executes the query and returns the hit count.
func getHitCount(esClient *es.Client, index string, query map[string]interface{}) (int, error) {
	res, err := esClient.Search(index, query)
	if err != nil {
		return 0, err
	}
	hitsObj, ok := res["hits"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected ES response format: missing hits")
	}
	total, ok := hitsObj["total"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected ES response format: missing total")
	}
	value, ok := total["value"].(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected ES response format: total value not float64")
	}
	return int(value), nil
}

// buildAlertFromRule constructs an Alert from the rule definition.
func buildAlertFromRule(rule schema.ESQueryAlertRule) schema.Alert {
	alert := schema.Alert{
		Summary:  rule.Alert.Summary,
		Severity: rule.Alert.Severity,
		Metadata: rule.Alert.Metadata,
		DedupKey: rule.Alert.DedupKey,
	}
	alert.Metadata.RuleID = rule.ID
	return alert
}

// determineAlertStatus returns "ACTIVE" or "RESOLVED" based on hit count and threshold.
func determineAlertStatus(hitCount, threshold int) string {
	if hitCount >= threshold {
		return "ACTIVE"
	}
	return "RESOLVED"
}

// fetchExistingAlert tries to fetch an alert by dedupKey from ES.
func fetchExistingAlert(esClient *es.Client, dedupKey string) (bool, schema.Alert) {
	getRes, err := esClient.ES.Get(ArgusAlertsIndex, dedupKey)
	if err != nil || getRes.StatusCode != 200 {
		return false, schema.Alert{}
	}
	defer getRes.Body.Close()
	var getResp map[string]interface{}
	json.NewDecoder(getRes.Body).Decode(&getResp)
	src, ok := getResp["_source"]
	if !ok {
		return false, schema.Alert{}
	}
	b, _ := json.Marshal(src)
	var alert schema.Alert
	json.Unmarshal(b, &alert)
	return true, alert
}

// printAlertStatus prints the alert status to the console.
func printAlertStatus(alert schema.Alert, ruleID string) {
	if alert.Status == "ACTIVE" {
		fmt.Println("[ArgusGo] 🚨 Alert Triggered!", ruleID)
	} else {
		fmt.Println("[ArgusGo] Alert Resolved", ruleID)
	}
}
