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
		// call this alert 1
		{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "service-1",
			"host":      "prod-server-01",
			"cpu_usage": 95.5,
		},
		// call this alert 2
		{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "service-1",
			"host":      "prod-server-01",
			"cpu_usage": 96.5,
		},

		// call this alert 3
		{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "service-2",
			"host":      "prod-server-01",
			"cpu_usage": 96.5,
		},
		//
		//// call this alert 4
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-2",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 98.5,
		//},
		//
		//// call this alert 5
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-2",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 99.5,
		//},
		//
		//// call this alert 6
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-3",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 97,
		//},
		//
		//// call this alert 7
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-3",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 97.5,
		//},
		//
		//// call this alert 8
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-4",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 96,
		//},
		//
		//// call this alert 9
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-4",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 96.5,
		//},
		//
		//// call this alert 10
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-5",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 98,
		//},
		//// call this alert 11
		//{
		//	"timestamp": time.Now().UTC().Format(time.RFC3339),
		//	"service":   "service-5",
		//	"host":      "prod-server-01",
		//	"cpu_usage": 98.5,
		//},

		/*
			- since the grouping is bases on host and host is same for all above alerts.
			- There will be only 1 parent alert and rest all will be grouped under it.
			- Note that the total alerts that will be generated wiil be:
			   - 5 alerts for service-1 to service-5 (1 each) as the dedup key is based on service value.
			   - Know that if two alerts have the same dedup key, they just get updated.
			   - Out of 5 1 will be parent type and 4 will be grouped type.
		*/
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
