package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func fetchMetrics() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "payment-service",
			"host":      "prod-server-01",
			"cpu_usage": 95.5,
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

	metrics := fetchMetrics()

	for _, metric := range metrics {
		// Serialize the document to JSON
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(metric); err != nil {
			log.Fatalf("Error encoding document: %s", err)
		}

		// Index the document
		req := esapi.IndexRequest{
			Index:   "metrics",
			Body:    &buf,
			Refresh: "true",
		}

		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Fatalf("Error getting response: %s", err)
		}

		if res.IsError() {
			log.Printf("[%s] Error indexing metric", res.Status())
		} else {
			log.Printf("[%s] Metric indexed successfully", res.Status())
		}

		if err := res.Body.Close(); err != nil {
			log.Printf("Error closing response body: %s", err)
		}
	}
}
