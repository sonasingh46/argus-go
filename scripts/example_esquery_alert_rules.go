package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func fetchDocList() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":          "high_cpu_usage",
			"name":        "High CPU Usage",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "range": { "cpu_usage": { "gte": 90 } } } }`,
			"time_window": "5m",
			"threshold":   1,
			"alert": map[string]interface{}{
				"summary":   "CPU usage above 90%",
				"severity":  "high",
				"dedup_key": "cpu-usage",
			},
		},
	}
}

func main() {
	// Initialize the client
	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://localhost:9200",
		},
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}

	docs := fetchDocList()

	for _, doc := range docs {
		// Serialize the document to JSON
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(doc); err != nil {
			log.Fatalf("Error encoding document: %s", err)
		}

		docID, ok := doc["id"].(string)
		if !ok {
			log.Fatalf("Document ID is missing or not a string")
		}

		// Index the document
		req := esapi.IndexRequest{
			Index:      "esquery_alert",
			DocumentID: docID,
			Body:       &buf,
			Refresh:    "true",
		}

		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Fatalf("Error getting response: %s", err)
		}

		if res.IsError() {
			log.Printf("[%s] Error indexing document %s", res.Status(), docID)
		} else {
			log.Printf("[%s] Document %s indexed successfully", res.Status(), docID)
		}
		if err := res.Body.Close(); err != nil {
			log.Printf("Error closing response body: %s", err)
		}
	}
}
