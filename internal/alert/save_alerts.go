package alert

import (
	"argus-go/internal/es"
	"argus-go/schema"
	"bytes"
	"context"
	"encoding/json"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

// SaveAlert saves or updates an alert in the "argusgo-alerts" index.
// If the alert exists, it is updated. If not, a RESOLVED alert is not created.
func SaveAlert(esClient *es.Client, alert schema.Alert) error {
	alertID := alert.DedupKey
	indexName := ArgusAlertsIndex

	exists := alertExists(esClient, indexName, alertID)
	if exists {
		return updateAlert(esClient, indexName, alertID, alert)
	}

	// Only create if not RESOLVED
	if alert.Status == "RESOLVED" {
		return nil
	}
	return createAlert(esClient, indexName, alertID, alert)
}

// updateAlert updates an existing alert document in ES.
func updateAlert(esClient *es.Client, indexName, alertID string, alert schema.Alert) error {
	doc := map[string]interface{}{
		"doc": alert,
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(doc); err != nil {
		return err
	}
	req := esapi.UpdateRequest{
		Index:      indexName,
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), esClient.ES)
	if err != nil {
		return err
	}
	defer func() {
		_ = res.Body.Close()
	}()
	return nil
}

// createAlert creates a new alert document in ES.
func createAlert(esClient *es.Client, indexName, alertID string, alert schema.Alert) error {
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(alert)
	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), esClient.ES)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return nil
}
