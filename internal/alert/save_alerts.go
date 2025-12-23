package alert

import (
	"argus-go/internal/es"
	"argus-go/schema"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// SaveAlert saves or updates an alert in the "argusgo-alerts" index.
// If the alert exists, it is updated. If not, a RESOLVED alert is not created.
func SaveAlert(esClient *es.Client, alert schema.Alert) error {
	alertID := alert.DedupKey
	indexName := ArgusAlertsIndex

	found, _ := fetchExistingActiveAlert(esClient, alertID)

	if found {
		return updateAlert(esClient, indexName, alertID, alert)
	}

	// If alert does not exist:
	//   - If new alert is RESOLVED, do not create.
	if alert.Status == "RESOLVED" {
		return nil
	}
	//   - If new alert is ACTIVE, create new alert.
	return createAlert(esClient, indexName, alert)
}

func updateAlert(esClient *es.Client, indexName, dedupKey string, alert schema.Alert) error {
	// 1. Prepare the script
	// Since we are updating the whole document, we replace the source
	// Note: 'params.new_alert' is passed via the 'params' map below
	script := map[string]interface{}{
		"script": map[string]interface{}{
			"source": "ctx._source = params.new_alert",
			"lang":   "painless",
			"params": map[string]interface{}{
				"new_alert": alert,
			},
		},
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"dedup_key": dedupKey,
			},
		},
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(script); err != nil {
		return err
	}

	// 2. Use UpdateByQuery instead of UpdateRequest
	res, err := esClient.ES.UpdateByQuery(
		[]string{indexName},
		esClient.ES.UpdateByQuery.WithBody(&buf),
		esClient.ES.UpdateByQuery.WithContext(context.Background()),
		esClient.ES.UpdateByQuery.WithRefresh(true),
	)

	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.IsError() {
		return fmt.Errorf("failed to update alert: %s", res.String())
	}

	return nil
}

// createAlert creates a new alert document in ES.
func createAlert(esClient *es.Client, indexName string, alert schema.Alert) error {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(alert)
	req := esapi.IndexRequest{
		Index:   indexName,
		Body:    &buf,
		Refresh: "true",
	}
	res, err := req.Do(context.Background(), esClient.ES)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
