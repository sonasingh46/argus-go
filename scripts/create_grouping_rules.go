package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

func fetchGroupingRules() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"id":             "group_by_host",
			"name":           "Group by Host",
			"group_by_field": "metadata.host",
			"time_window":    "10m",
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

	rules := fetchGroupingRules()

	for _, rule := range rules {
		// Serialize the document to JSON
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(rule); err != nil {
			log.Fatalf("Error encoding document: %s", err)
		}

		ruleID, ok := rule["id"].(string)
		if !ok {
			log.Fatalf("Rule ID is missing or not a string")
		}

		// Index the document
		req := esapi.IndexRequest{
			Index:      "grouping_rules",
			DocumentID: ruleID,
			Body:       &buf,
			Refresh:    "true",
		}

		res, err := req.Do(context.Background(), es)
		if err != nil {
			log.Fatalf("Error getting response: %s", err)
		}

		if res.IsError() {
			log.Printf("[%s] Error indexing grouping rule %s", res.Status(), ruleID)
		} else {
			log.Printf("[%s] Grouping rule %s indexed successfully", res.Status(), ruleID)
		}
		if err := res.Body.Close(); err != nil {
			log.Printf("Error closing response body: %s", err)
		}
	}
}
