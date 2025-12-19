package alert

import (
	"argus-go/internal/es"
	"argus-go/schema"
	"encoding/json"
)

const (
	ESQueryAlertIndex = "esquery_alert"
)

// FetchESQueryAlertRules retrieves all ESQuery alert rules from the "esquery_alert" index.
func FetchESQueryAlertRules(esClient *es.Client) ([]schema.ESQueryAlertRule, error) {
	res, err := esClient.ES.Search(esClient.ES.Search.WithIndex(ESQueryAlertIndex))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var r map[string]interface{}
	json.NewDecoder(res.Body).Decode(&r)

	return parseAlertRulesFromHits(r), nil
}

// parseAlertRulesFromHits extracts rules from the ES search response.
func parseAlertRulesFromHits(r map[string]interface{}) []schema.ESQueryAlertRule {
	var rules []schema.ESQueryAlertRule
	hitsObj, ok := r["hits"].(map[string]interface{})
	if !ok {
		return rules
	}

	for _, hit := range hitsObj["hits"].([]interface{}) {
		source := hit.(map[string]interface{})["_source"]
		b, _ := json.Marshal(source)
		var rule schema.ESQueryAlertRule
		json.Unmarshal(b, &rule)
		rules = append(rules, rule)
	}
	return rules
}
