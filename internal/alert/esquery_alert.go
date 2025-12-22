package alert

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"argus-go/internal/es"
	"argus-go/schema"
)

const (
	ArgusAlertsIndex   = "argusgo-alerts"
	GroupingRulesIndex = "grouping_rules"
)

// ExecuteESQueryAlertRule runs the ESQuery alert rule and generates alerts based on the threshold.
func ExecuteESQueryAlertRule(esClient *es.Client, rule schema.ESQueryAlertRule) ([]schema.Alert, error) {
	// Run the ES query and get the hit count
	hitCount, hits, err := runESQueryForRule(esClient, rule)
	if err != nil {
		return nil, err
	}

	// CRITICAL FIX: Handle missing data for rules with dynamic keys.
	// If there are no hits, we cannot calculate the dynamic DedupKey from data.
	// We must fetch ALL currently ACTIVE alerts for this rule and resolve them.
	if len(hits) == 0 && rule.DedupRules != nil {
		return resolveActiveAlertsForRule(esClient, rule)
	}

	var alerts []schema.Alert

	// Build the alert object
	alert := buildAlertFromRule(rule)

	// Populate metadata from hits
	if len(hits) > 0 {
		if host, ok := hits[0]["host"].(string); ok {
			alert.Metadata.Host = host
		}
	}

	// Calculate dynamic dedup key if rules are present
	if rule.DedupRules != nil {
		dynamicKey := calculateDedupKey(rule.DedupRules, hits)
		if dynamicKey != "" {
			alert.DedupKey = dynamicKey
		} else {
			alert.DedupKey = generateRandomString()
		}
	}

	alert.Status = determineAlertStatus(hitCount, rule.Threshold)
	alert.Timestamp = time.Now().UTC()

	// Check if alert already exists
	found, existingAlert := fetchExistingAlert(esClient, alert.DedupKey)
	if found {
		// Optionally, merge fields from existingAlert if needed
		alert.Timestamp = time.Now().UTC()

		if alert.Status == "RESOLVED" {
			alert.Metadata.TriggerCount = existingAlert.Metadata.TriggerCount
		} else {
			alert.Metadata.TriggerCount = existingAlert.Metadata.TriggerCount + 1
		}

		alert.AlertType = existingAlert.AlertType
		alert.GroupedAlerts = existingAlert.GroupedAlerts
	} else {
		alert.Metadata.TriggerCount = 1
		// New alert, check if it should be grouped
		parentAlertID, shouldGroup := checkGroupingRules(esClient, alert)
		if shouldGroup {
			alert.AlertType = schema.AlertTypeGrouped
			// Update parent alert
			if err := updateParentAlert(esClient, parentAlertID, alert.DedupKey); err != nil {
				fmt.Printf("Error updating parent alert: %v\n", err)
			}
		} else {
			alert.AlertType = schema.AlertTypeParent
		}
	}

	alerts = append(alerts, alert)
	printAlertStatus(alert, rule.ID)
	return alerts, nil
}

// resolveActiveAlertsForRule finds all ACTIVE alerts for a rule and returns them as RESOLVED.
func resolveActiveAlertsForRule(esClient *es.Client, rule schema.ESQueryAlertRule) ([]schema.Alert, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"metadata.rule_id": rule.ID}},
					map[string]interface{}{"term": map[string]interface{}{"status": "ACTIVE"}},
				},
			},
		},
	}

	res, err := esClient.Search(ArgusAlertsIndex, query)
	if err != nil {
		return nil, err
	}

	var resolvedAlerts []schema.Alert
	hitsObj, ok := res["hits"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	hits, ok := hitsObj["hits"].([]interface{})
	if !ok {
		return nil, nil
	}

	for _, h := range hits {
		hitMap, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}

		b, _ := json.Marshal(source)
		var existingAlert schema.Alert
		if err := json.Unmarshal(b, &existingAlert); err != nil {
			continue
		}

		// Mark as resolved
		existingAlert.Status = "RESOLVED"
		existingAlert.Timestamp = time.Now().UTC()
		printAlertStatus(existingAlert, rule.ID)
		resolvedAlerts = append(resolvedAlerts, existingAlert)
	}

	return resolvedAlerts, nil
}

// runESQueryForRule executes the ES query for the given rule and returns the hit count.
// It injects a time window filter on the "timestamp" field.
func runESQueryForRule(esClient *es.Client, rule schema.ESQueryAlertRule) (int, []map[string]interface{}, error) {
	query, err := parseQuery(rule.Query)
	if err != nil {
		return 0, nil, err
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
func getHitCount(esClient *es.Client, index string, query map[string]interface{}) (int, []map[string]interface{}, error) {
	res, err := esClient.Search(index, query)
	if err != nil {
		return 0, nil, err
	}
	hitsObj, ok := res["hits"].(map[string]interface{})
	if !ok {
		return 0, nil, fmt.Errorf("unexpected ES response format: missing hits")
	}
	total, ok := hitsObj["total"].(map[string]interface{})
	if !ok {
		return 0, nil, fmt.Errorf("unexpected ES response format: missing total")
	}
	value, ok := total["value"].(float64)
	if !ok {
		return 0, nil, fmt.Errorf("unexpected ES response format: total value not float64")
	}

	var hits []map[string]interface{}
	if hitsArr, ok := hitsObj["hits"].([]interface{}); ok {
		for _, h := range hitsArr {
			if hitMap, ok := h.(map[string]interface{}); ok {
				if source, ok := hitMap["_source"].(map[string]interface{}); ok {
					hits = append(hits, source)
				}
			}
		}
	}

	return int(value), hits, nil
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
	defer func() {
		_ = getRes.Body.Close()
	}()
	var getResp map[string]interface{}
	if err := json.NewDecoder(getRes.Body).Decode(&getResp); err != nil {
		return false, schema.Alert{}
	}
	src, ok := getResp["_source"]
	if !ok {
		return false, schema.Alert{}
	}
	b, _ := json.Marshal(src)
	var alert schema.Alert
	if err := json.Unmarshal(b, &alert); err != nil {
		return false, schema.Alert{}
	}
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

func checkGroupingRules(esClient *es.Client, alert schema.Alert) (string, bool) {
	// Fetch all grouping rules
	// In a real scenario, we might want to cache these or fetch only relevant ones
	rules, err := fetchGroupingRules(esClient)
	if err != nil {
		fmt.Printf("Error fetching grouping rules: %v\n", err)
		return "", false
	}

	for _, rule := range rules {
		// Check if there is a parent alert that matches the grouping rule
		parentID, found := findMatchingParentAlert(esClient, alert, rule)
		if found {
			return parentID, true
		}
	}

	return "", false
}

func fetchGroupingRules(esClient *es.Client) ([]schema.GroupingRule, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"match_all": map[string]interface{}{},
		},
	}
	res, err := esClient.Search(GroupingRulesIndex, query)
	if err != nil {
		return nil, err
	}

	var rules []schema.GroupingRule
	hitsObj, ok := res["hits"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected ES response format: missing hits")
	}
	hits, ok := hitsObj["hits"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected ES response format: missing hits array")
	}

	for _, hit := range hits {
		hitMap, ok := hit.(map[string]interface{})
		if !ok {
			continue
		}
		source, ok := hitMap["_source"].(map[string]interface{})
		if !ok {
			continue
		}
		b, _ := json.Marshal(source)
		var rule schema.GroupingRule
		if err := json.Unmarshal(b, &rule); err == nil {
			rules = append(rules, rule)
		}
	}
	return rules, nil
}

func findMatchingParentAlert(esClient *es.Client, alert schema.Alert, rule schema.GroupingRule) (string, bool) {
	// Construct query to find parent alert with matching field within time window
	// For simplicity, let's assume GroupByField maps directly to a field in Alert struct or Metadata
	// We need to reflect or map the field name.
	// Example: "metadata.host" -> alert.Metadata.Host

	fieldValue := getFieldValue(alert, rule.GroupByField)
	if fieldValue == "" {
		return "", false
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"term": map[string]interface{}{
							"alert_type": "parent",
						},
					},
					map[string]interface{}{
						"term": map[string]interface{}{
							rule.GroupByField: fieldValue,
						},
					},
					map[string]interface{}{
						"range": map[string]interface{}{
							"timestamp": map[string]interface{}{
								"gte": fmt.Sprintf("now-%s", rule.TimeWindow),
							},
						},
					},
				},
			},
		},
		"size": 1,
		"sort": []interface{}{
			map[string]interface{}{
				"timestamp": map[string]interface{}{
					"order": "desc",
				},
			},
		},
	}

	res, err := esClient.Search(ArgusAlertsIndex, query)
	if err != nil {
		fmt.Printf("Error searching for parent alert: %v\n", err)
		return "", false
	}

	hitsObj, ok := res["hits"].(map[string]interface{})
	if !ok {
		return "", false
	}
	hits, ok := hitsObj["hits"].([]interface{})
	if !ok || len(hits) == 0 {
		return "", false
	}

	hitMap, ok := hits[0].(map[string]interface{})
	if !ok {
		return "", false
	}

	// The ID of the document is usually in "_id"
	id, ok := hitMap["_id"].(string)
	if !ok {
		// Fallback to dedup_key if _id is not available or we use dedup_key as ID
		source, ok := hitMap["_source"].(map[string]interface{})
		if ok {
			if dedupKey, ok := source["dedup_key"].(string); ok {
				return dedupKey, true
			}
		}
		return "", false
	}

	return id, true
}

func getFieldValue(alert schema.Alert, fieldPath string) string {
	// Simple implementation for specific fields
	switch fieldPath {
	case "metadata.host":
		return alert.Metadata.Host
	case "metadata.rule_id":
		return alert.Metadata.RuleID
	// Add more cases as needed
	default:
		return ""
	}
}

func updateParentAlert(esClient *es.Client, parentID string, childAlertID string) error {
	// We need to append childAlertID to grouped_alerts list of parent alert
	// This requires a script update or read-modify-write
	// Using script update for atomicity

	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "if (ctx._source.grouped_alerts == null) { ctx._source.grouped_alerts = [params.child_id] } else { ctx._source.grouped_alerts.add(params.child_id) }",
			"lang":   "painless",
			"params": map[string]interface{}{
				"child_id": childAlertID,
			},
		},
	}

	// Assuming parentID is the document ID in ES
	// If we use dedup_key as ID, this works.
	_, err := esClient.Update(ArgusAlertsIndex, parentID, script)
	return err
}

func generateRandomString() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

func calculateDedupKey(rules *schema.DedupRules, hits []map[string]interface{}) string {
	if len(hits) == 0 {
		return ""
	}
	// Use the first hit to extract fields
	hit := hits[0]

	var parts []string
	if rules.Key != "" {
		parts = append(parts, rules.Key)
	}

	for _, field := range rules.Fields {
		if val, ok := hit[field]; ok {
			parts = append(parts, fmt.Sprintf("%v", val))
		}
	}

	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "-")
}
