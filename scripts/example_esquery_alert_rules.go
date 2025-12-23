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
			"id":          "high_cpu_usage_svc_1",
			"name":        "High CPU Usage Service 1",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "bool": { "must": [ { "range": { "cpu_usage": { "gte": 90 } } }, { "term": { "service": "service-1" } } ] } } }`,
			"time_window": "30m",
			"threshold":   1,
			"dedup_rules": map[string]interface{}{
				"fields": []string{"service"},
			},
			"alert": map[string]interface{}{
				"summary":  "CPU usage above 90%",
				"severity": "high",
				// "dedup_key": "cpu-usage", // Optional now, dynamic key takes precedence
			},
		},
		{
			"id":          "high_cpu_usage_svc_2",
			"name":        "High CPU Usage Service 2",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "bool": { "must": [ { "range": { "cpu_usage": { "gte": 90 } } }, { "term": { "service": "service-2" } } ] } } }`,
			"time_window": "5m",
			"threshold":   1,
			"dedup_rules": map[string]interface{}{
				"fields": []string{"service"},
			},
			"alert": map[string]interface{}{
				"summary":  "CPU usage above 90%",
				"severity": "high",
				// "dedup_key": "cpu-usage-service-2",
			},
		},
		{
			"id":          "high_cpu_usage_svc_3",
			"name":        "High CPU Usage Service 3",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "bool": { "must": [ { "range": { "cpu_usage": { "gte": 90 } } }, { "term": { "service": "service-3" } } ] } } }`,
			"time_window": "5m",
			"threshold":   1,
			"dedup_rules": map[string]interface{}{
				"fields": []string{"service"},
			},
			"alert": map[string]interface{}{
				"summary":  "CPU usage above 90%",
				"severity": "high",
				// "dedup_key": "cpu-usage-service-3",
			},
		},
		{
			"id":          "high_cpu_usage_svc_4",
			"name":        "High CPU Usage Service 4",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "bool": { "must": [ { "range": { "cpu_usage": { "gte": 90 } } }, { "term": { "service": "service-4" } } ] } } }`,
			"time_window": "5m",
			"threshold":   1,
			"dedup_rules": map[string]interface{}{
				"fields": []string{"service"},
			},
			"alert": map[string]interface{}{
				"summary":  "CPU usage above 90%",
				"severity": "high",
				// "dedup_key": "cpu-usage-service-4",
			},
		},
		{
			"id":          "high_cpu_usage_svc_5",
			"name":        "High CPU Usage Service 5",
			"type":        "esquery",
			"index":       "metrics",
			"query":       `{ "query": { "bool": { "must": [ { "range": { "cpu_usage": { "gte": 90 } } }, { "term": { "service": "service-5" } } ] } } }`,
			"time_window": "5m",
			"threshold":   1,
			"dedup_rules": map[string]interface{}{
				"fields": []string{"service"},
			},
			"alert": map[string]interface{}{
				"summary":  "CPU usage above 90%",
				"severity": "high",
				// "dedup_key": "cpu-usage-service-5",
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
