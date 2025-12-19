package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"argus-go/internal/es"
	"argus-go/schema"

	"github.com/elastic/go-elasticsearch/v8/esapi"
)

const (
	Brand = "ArgusGo"
)

type ThresholdRule struct {
	RuleName      string
	Threshold     float64
	WindowMinutes float64
}

type AlertEngine struct {
	ES *es.Client
}

func New(esClient *es.Client) *AlertEngine {
	return &AlertEngine{ES: esClient}
}

func (a *AlertEngine) CheckThreshold(rule ThresholdRule) {
	fmt.Println("Checking threshold rule:", rule.RuleName)
	threshold := rule.Threshold
	window := rule.WindowMinutes
	ruleName := rule.RuleName

	if window == 0 {
		window = 5
	}

	/* Equivalent Elasticsearch query for Kibana Dev Tools:
	GET metrics/_search
	{
	  "size": 0,
	  "query": {
	    "range": {
	      "@timestamp": { "gte": "now-5m" }
	    }
	  },
	  "aggs": {
	    "hosts": {
	      "terms": { "field": "host" },
	      "aggs": {
	        "avg_metric": {
	          "avg": { "field": "cpu_usage" }
	        }
	      }
	    }
	  }
	}
	*/

	query := map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"range": map[string]interface{}{
				"timestamp": map[string]interface{}{"gte": fmt.Sprintf("now-%dm", int(window))},
			},
		},
		"aggs": map[string]interface{}{
			"hosts": map[string]interface{}{
				"terms": map[string]interface{}{"field": "host"},
				"aggs": map[string]interface{}{
					"avg_metric": map[string]interface{}{
						"avg": map[string]interface{}{"field": "cpu_usage"},
					},
				},
			},
		},
	}

	r, err := a.ES.Search("metrics", query)
	if err != nil {
		return
	}

	fmt.Println("Got response from ES for rule:", ruleName, r)
	aggs, ok := r["aggregations"].(map[string]interface{})
	if !ok {
		return
	}

	buckets := aggs["hosts"].(map[string]interface{})["buckets"].([]interface{})

	for _, b := range buckets {
		bucket := b.(map[string]interface{})
		hostName := bucket["key"].(string)
		val := bucket["avg_metric"].(map[string]interface{})["value"]

		if val == nil {
			continue
		}
		avgValue := val.(float64)

		if avgValue > threshold {
			fmt.Printf("[%s] 🚨 BREACH: %s | Host: %s | Val: %.2f\n", Brand, ruleName, hostName, avgValue)
			a.UpdateAlertState(ruleName, hostName, avgValue, "ACTIVE")
		} else {
			a.UpdateAlertState(ruleName, hostName, avgValue, "RESOLVED")
		}
	}
}

func (a *AlertEngine) UpdateAlertState(ruleName, host string, val float64, status string) {
	alertID := fmt.Sprintf("%s_%s", ruleName, host)
	indexName := "argusgo-alerts"

	if status == "RESOLVED" {
		res, _ := a.ES.ES.Get(indexName, alertID)
		if res.IsError() {
			return
		}
		fmt.Printf("[%s] ✅ RESOLVED: %s on %s\n", Brand, ruleName, host)
	}

	severity := "info"
	if status == "ACTIVE" {
		severity = "high"
	}

	doc := schema.Alert{
		Summary:   fmt.Sprintf("Rule %s triggered. Value: %.2f", ruleName, val),
		Severity:  severity,
		Status:    status,
		Timestamp: time.Now().UTC(),
		Metadata: schema.AlertMetadata{
			Host:   host,
			RuleID: ruleName,
		},
	}

	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(doc)

	req := esapi.IndexRequest{
		Index:      indexName,
		DocumentID: alertID,
		Body:       &buf,
		Refresh:    "true",
	}
	res, _ := req.Do(context.Background(), a.ES.ES)
	defer res.Body.Close()
}
