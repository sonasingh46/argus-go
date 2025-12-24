package alert

import (
	"bytes"
	"context"
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

	var alerts []schema.Alert

	// 1. Group hits by dedup key
	groupedHits := make(map[string][]map[string]interface{})
	if rule.DedupRules != nil {
		for _, hit := range hits {
			fullKey := calculateDedupKey(rule.ID, rule.DedupRules, hit)
			if fullKey != "" {
				groupedHits[fullKey] = append(groupedHits[fullKey], hit)
			}
		}
	} else {
		// No dedup rules, treat all hits as one group
		if hitCount > 0 {
			groupedHits[rule.ID] = hits
		}
	}

	// 2. Fetch existing active alerts for this rule to handle resolution
	existingAlertsMap := make(map[string]schema.Alert)
	activeAlerts, err := fetchActiveAlertsForRule(esClient, rule.ID)
	if err == nil {
		for _, a := range activeAlerts {
			existingAlertsMap[a.DedupKey] = a
		}
	}

	// Fetch grouping rules once
	groupingRules, err := fetchGroupingRules(esClient)
	if err != nil {
		fmt.Printf("Error fetching grouping rules: %v\n", err)
	}

	// Cache for newly created parents in this batch.
	// Map: GroupingRuleID -> Map: GroupValue -> ParentDedupKey
	newParents := make(map[string]map[string]string)

	// 3. Process groups and generate ACTIVE alerts
	processedKeys := make(map[string]bool)

	for dedupKey, groupHits := range groupedHits {
		// Check threshold per group
		if len(groupHits) < rule.Threshold {
			continue
		}

		processedKeys[dedupKey] = true

		// Build the alert object
		alert := buildAlertFromRule(rule)
		alert.DedupKey = dedupKey
		alert.Timestamp = time.Now().UTC()
		alert.Status = "ACTIVE"

		// Populate metadata from first hit in group
		if len(groupHits) > 0 {
			if host, ok := groupHits[0]["host"].(string); ok {
				alert.Metadata.Host = host
			}
		}

		// Check if alert already exists
		if existingAlert, found := existingAlertsMap[dedupKey]; found {
			alert.Metadata.TriggerCount = existingAlert.Metadata.TriggerCount + 1
			alert.AlertType = existingAlert.AlertType
			alert.GroupedAlerts = existingAlert.GroupedAlerts
		} else {
			alert.Metadata.TriggerCount = 1
			// New alert, check if it should be grouped

			isGrouped := false
			var parentID string
			var parentIsNew bool

			for _, gr := range groupingRules {
				val := getFieldValue(alert, gr.GroupByField)
				if val == "" {
					continue
				}

				// Check cache (newly created parents)
				if parents, ok := newParents[gr.ID]; ok {
					if pid, ok := parents[val]; ok {
						parentID = pid
						isGrouped = true
						parentIsNew = true
						break
					}
				}

				// Check ES (existing parents)
				pid, found := findMatchingParentAlert(esClient, alert, gr)
				if found {
					parentID = pid
					isGrouped = true
					parentIsNew = false
					break
				}
			}

			if isGrouped {
				alert.AlertType = schema.AlertTypeGrouped
				// Update parent alert
				if parentIsNew {
					// Update parent in local alerts slice
					for i := range alerts {
						if alerts[i].DedupKey == parentID {
							alerts[i].GroupedAlerts = append(alerts[i].GroupedAlerts, alert.DedupKey)
							break
						}
					}
				} else {
					if err := updateParentAlert(esClient, parentID, alert.DedupKey); err != nil {
						fmt.Printf("Error updating parent alert: %v\n", err)
					}
				}
			} else {
				alert.AlertType = schema.AlertTypeParent
				// Register as potential parent
				for _, gr := range groupingRules {
					val := getFieldValue(alert, gr.GroupByField)
					if val != "" {
						if newParents[gr.ID] == nil {
							newParents[gr.ID] = make(map[string]string)
						}
						newParents[gr.ID][val] = alert.DedupKey
					}
				}
			}
		}

		alerts = append(alerts, alert)
		printAlertStatus(alert, rule.ID)
	}

	// 4. Resolve alerts that are no longer active
	for key, existingAlert := range existingAlertsMap {
		if !processedKeys[key] {
			// Check if it is a parent alert
			if existingAlert.AlertType == schema.AlertTypeParent {
				hasActiveChild := false
				for _, childKey := range existingAlert.GroupedAlerts {
					// 1. Check if active in current batch (same rule)
					if processedKeys[childKey] {
						hasActiveChild = true
						break
					}

					// 2. Check if it is known to be resolving (same rule)
					if _, known := existingAlertsMap[childKey]; known {
						// It is in existingAlertsMap but not processedKeys, so it is resolving.
						// So it is NOT active.
						continue
					}

					// 3. Unknown (different rule or old). Check ES.
					active, _ := fetchExistingActiveAlert(esClient, childKey)
					if active {
						hasActiveChild = true
						break
					}
				}

				if hasActiveChild {
					continue
				}
			}

			existingAlert.Status = "RESOLVED"
			existingAlert.Timestamp = time.Now().UTC()
			alerts = append(alerts, existingAlert)
			printAlertStatus(existingAlert, rule.ID)
		}
	}

	return alerts, nil
}

func fetchActiveAlertsForRule(esClient *es.Client, ruleID string) ([]schema.Alert, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"metadata.rule_id": ruleID}},
					map[string]interface{}{"term": map[string]interface{}{"status": "ACTIVE"}},
				},
			},
		},
		"size": 1000,
	}

	res, err := esClient.Search(ArgusAlertsIndex, query)
	if err != nil {
		return nil, err
	}

	var alerts []schema.Alert
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
		var a schema.Alert
		if err := json.Unmarshal(b, &a); err == nil {
			alerts = append(alerts, a)
		}
	}
	return alerts, nil
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

// fetchExistingActiveAlert searches for a document that matches the dedupKey AND has an ACTIVE status.
func fetchExistingActiveAlert(esClient *es.Client, dedupKey string) (bool, schema.Alert) {
	// 1. Construct the Search Query with multiple criteria
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{
						"term": map[string]interface{}{
							"dedup_key": dedupKey,
						},
					},
					{
						"term": map[string]interface{}{
							"status": "ACTIVE", // Only fetch if the alert is currently active
						},
					},
				},
			},
		},
		"size": 1,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(query); err != nil {
		return false, schema.Alert{}
	}

	// 2. Execute the Search request
	res, err := esClient.ES.Search(
		esClient.ES.Search.WithIndex(ArgusAlertsIndex),
		esClient.ES.Search.WithBody(&buf),
		esClient.ES.Search.WithContext(context.Background()),
	)
	if err != nil {
		return false, schema.Alert{}
	}
	defer res.Body.Close()

	if res.IsError() {
		return false, schema.Alert{}
	}

	// 3. Define the Search Response Structure
	var searchResult struct {
		Hits struct {
			Hits []struct {
				Source schema.Alert `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}

	if err := json.NewDecoder(res.Body).Decode(&searchResult); err != nil {
		return false, schema.Alert{}
	}

	// 4. Return the first hit if it exists
	if len(searchResult.Hits.Hits) > 0 {
		return true, searchResult.Hits.Hits[0].Source
	}

	return false, schema.Alert{}
}

// printAlertStatus prints the alert status to the console.
func printAlertStatus(alert schema.Alert, ruleID string) {
	if alert.Status == "ACTIVE" {
		fmt.Println("[ArgusGo] 🚨 Alert Triggered!", ruleID)
	} else {
		fmt.Println("[ArgusGo] Alert Resolved", ruleID)
	}
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

func calculateDedupKey(ruleID string, rules *schema.DedupRules, hit map[string]interface{}) string {
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
	return ruleID + "_" + strings.Join(parts, "-")
}
