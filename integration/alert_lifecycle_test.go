package integration

import (
	"argus-go/internal/alert"
	"argus-go/internal/es"
	"argus-go/schema"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	metricsIndex       = "metrics"
	esqueryAlertIndex  = "esquery_alert"
	alertsIndex        = "argusgo-alerts"
	groupingRulesIndex = "grouping_rules"
)

func TestIT(t *testing.T) {
	fmt.Println("Starting Integration Test Suite")
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Test Suite")
}

var _ = Describe("Alert Lifecycle Integration", func() {
	var esClient *es.Client

	BeforeEach(func() {
		esClient = es.New([]string{"http://localhost:9200"})
		setupIndices(esClient)
	})

	AfterEach(func() {
		cleanupIndices(esClient)
	})

	// Just create a single alert rule and ingest a singe metric
	// Assert that alert is created when threshold is breached
	// Then delete that metric that resolves the alert and assert alert is resolved
	Context("When a simple threshold rule is configured", func() {
		It("should create an alert when threshold is breached and resolve it when metric drops", func() {
			// 1. Create Alert Rule
			rule := schema.ESQueryAlertRule{
				ID:         "high_cpu_test",
				Name:       "High CPU Test",
				Type:       "esquery",
				Index:      metricsIndex,
				Query:      `{ "query": { "range": { "cpu_usage": { "gte": 90 } } } }`,
				TimeWindow: "5m",
				Threshold:  1,
				DedupRules: &schema.DedupRules{
					Key:    "cpu-alert",
					Fields: []string{"host"},
				},
				Alert: schema.Alert{
					Summary:  "High CPU detected",
					Severity: "high",
				},
			}
			createAlertRule(esClient, rule)

			// 2. Ingest High CPU Metric
			ingestMetric(esClient, map[string]interface{}{
				"timestamp": time.Now().UTC().Format(time.RFC3339),
				"host":      "test-host-1",
				"cpu_usage": 95.0,
			})

			// 3. Execute Rule
			executeRuleAndSaveAlerts(esClient, rule)

			// 4. Verify Alert is ACTIVE
			activeAlerts := fetchActiveAlerts(esClient, "high_cpu_test_cpu-alert-test-host-1")
			Expect(activeAlerts).To(HaveLen(1))
			Expect(activeAlerts[0].Status).To(Equal("ACTIVE"))
			Expect(activeAlerts[0].Metadata.Host).To(Equal("test-host-1"))

			// 5. Simulate Resolution (delete old metrics)
			deleteMetrics(esClient)

			// 6. Execute Rule Again
			executeRuleAndSaveAlerts(esClient, rule)

			// 7. Verify Alert is RESOLVED
			resolvedAlerts := fetchResolvedAlerts(esClient, "high_cpu_test_cpu-alert-test-host-1")
			Expect(resolvedAlerts).To(HaveLen(1))
			Expect(resolvedAlerts[0].Status).To(Equal("RESOLVED"))
		})
	})

	Context("When multiple services trigger alerts with deduplication based on service name", func() {
		It("should deduplicate alerts per service and resolve them independently", func() {
			services := []string{"service-1", "service-2", "service-3"}
			var rules []schema.ESQueryAlertRule

			// 1. Create 3 Alert Rules (one for each service)
			for _, svc := range services {
				rule := schema.ESQueryAlertRule{
					ID:         "rule_" + svc,
					Name:       "High CPU " + svc,
					Type:       "esquery",
					Index:      metricsIndex,
					Query:      fmt.Sprintf(`{ "query": { "bool": { "must": [ { "term": { "service": "%s" } }, { "range": { "cpu_usage": { "gte": 90 } } } ] } } }`, svc),
					TimeWindow: "5m",
					Threshold:  1,
					DedupRules: &schema.DedupRules{
						Fields: []string{"service"},
					},
					Alert: schema.Alert{
						Summary:  "High CPU detected for " + svc,
						Severity: "high",
					},
				}
				createAlertRule(esClient, rule)
				rules = append(rules, rule)
			}

			// 2. Ingest 5 metrics for each service breaching the threshold
			// Since the dedup key will be based on svc name 3 alerts should be created.
			for _, svc := range services {
				for i := 0; i < 5; i++ {
					ingestMetric(esClient, map[string]interface{}{
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"host":      "prod-server-01",
						"service":   svc,
						"cpu_usage": 95.0 + float64(i),
					})
				}
			}

			// 3. Execute Rules and Assert only 3 alerts are created (one per service)
			for _, rule := range rules {
				executeRuleAndSaveAlerts(esClient, rule)
			}

			for _, svc := range services {
				dedupKey := "rule_" + svc + "_" + svc
				activeAlerts := fetchActiveAlerts(esClient, dedupKey)
				Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for %s", svc))
				Expect(activeAlerts[0].Metadata.TriggerCount).To(BeNumerically(">=", 1))
			}

			resolvedServices := make(map[string]bool)
			// Delete metrics for services in a loop and asert alerts resolving independently
			for _, svc := range services {
				deleteMetricsForService(esClient, svc)
				resolvedServices[svc] = true

				// Re-execute rules
				for _, rule := range rules {
					executeRuleAndSaveAlerts(esClient, rule)
				}

				// Assert alert for this service is RESOLVED
				dedupKey := "rule_" + svc + "_" + svc
				resolvedAlerts := fetchResolvedAlerts(esClient, dedupKey)
				Expect(resolvedAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 resolved alert for %s", svc))

				// Assert other alerts remain ACTIVE
				for _, otherSvc := range services {
					if resolvedServices[otherSvc] {
						continue
					}
					otherDedupKey := "rule_" + otherSvc + "_" + otherSvc
					activeAlerts := fetchActiveAlerts(esClient, otherDedupKey)
					Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for %s", otherSvc))
				}
			}
		})
	})

	Context("When multiple services trigger alerts with deduplication based on host name", func() {
		It("should deduplicate alerts per service and resolve them independently", func() {
			services := []string{"service-1", "service-2", "service-3"}
			var rules []schema.ESQueryAlertRule

			// Create 1 Alert Rule for ALL services
			rule := schema.ESQueryAlertRule{
				ID:    "rule_high_cpu_all_services",
				Name:  "High CPU Usage - All Services",
				Type:  "esquery",
				Index: metricsIndex,
				// The query now only looks for the threshold, irrespective of service name
				Query: `{ "query": { "range": { "cpu_usage": { "gte": 90 } } } }`,

				TimeWindow: "5m",
				Threshold:  1,

				DedupRules: &schema.DedupRules{
					// By adding "service" here, ES will create a unique alert
					// for every unique combination of host + service
					Fields: []string{"host"},
				},

				Alert: schema.Alert{
					Summary:  "High CPU detected on service",
					Severity: "high",
				},
			}
			createAlertRule(esClient, rule)
			rules = append(rules, rule)

			// 2. Ingest 5 metrics for each service breaching the threshold
			// Since the dedup key will be based on host name 1 alert should be created.
			for _, svc := range services {
				for i := 0; i < 5; i++ {
					ingestMetric(esClient, map[string]interface{}{
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"host":      "prod-server-01",
						"service":   svc,
						"cpu_usage": 95.0 + float64(i),
					})
				}
			}

			// 3. Execute Rules and Assert only 1 alert is created (for the host)
			for _, rule := range rules {
				executeRuleAndSaveAlerts(esClient, rule)
			}

			// Fetch all the active alerts to ensure only 1 alert exists
			activeAlerts := fetchOnlyActiveAlerts(esClient)
			Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for host %s", "prod-server-01"))
			Expect(activeAlerts[0].Metadata.TriggerCount).To(BeNumerically(">=", 1))

			// Fetch by dedup key
			dedupKey := "rule_high_cpu_all_services_prod-server-01"
			activeAlerts = fetchActiveAlerts(esClient, dedupKey)
			Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for host %s", "prod-server-01"))
			Expect(activeAlerts[0].Metadata.TriggerCount).To(BeNumerically(">=", 1))

			// delete all the metrics for service-1 only
			// alert should still be active as other services are breaching threshold on same host
			deleteMetricsForService(esClient, "service-1")

			// Re-execute rules
			for _, rule := range rules {
				executeRuleAndSaveAlerts(esClient, rule)
			}

			// the alert will stil be active as there are still metrics breaching threshold for service-1
			// and other services.

			// Fetch all the active alerts to ensure only 1 alert exists
			activeAlerts = fetchOnlyActiveAlerts(esClient)
			Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for host %s", "prod-server-01"))
			Expect(activeAlerts[0].Metadata.TriggerCount).To(BeNumerically(">=", 1))

			// Fetch by dedup key
			dedupKey = "rule_high_cpu_all_services_prod-server-01"
			activeAlerts = fetchActiveAlerts(esClient, dedupKey)
			Expect(activeAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 active alert for host %s", "prod-server-01"))
			Expect(activeAlerts[0].Metadata.TriggerCount).To(BeNumerically(">=", 1))

			// Now delete all metrics for all services on that host to resolve the alert
			deleteMetricsForService(esClient, "service-2")
			deleteMetricsForService(esClient, "service-3")
			// Re-execute rules
			for _, rule := range rules {
				executeRuleAndSaveAlerts(esClient, rule)
			}

			// Fetch all the active alerts to ensure only 0 alert exists
			activeAlerts = fetchOnlyActiveAlerts(esClient)
			Expect(activeAlerts).To(HaveLen(0), fmt.Sprintf("Expected 1 active alert for host %s", "prod-server-01"))

			// Fetch by dedup key
			dedupKey = "rule_high_cpu_all_services_prod-server-01"
			resolvedAlerts := fetchResolvedAlerts(esClient, dedupKey)
			Expect(resolvedAlerts).To(HaveLen(1), fmt.Sprintf("Expected 1 resolved alert for host %s", "prod-server-01"))
		})
	})

	Context("When grouping rules are configured", func() {
		It("should group alerts based on grouping rules", func() {
			// 1. Create Grouping Rule
			// We group by rule_id so that all alerts from the same rule are grouped together
			groupingRule := schema.GroupingRule{
				ID:           "group_by_host",
				Name:         "Group by Host",
				GroupByField: "metadata.host",
				TimeWindow:   "10m",
			}

			createGroupingRule(esClient, groupingRule)

			// 2. Create Alert Rule
			rule := schema.ESQueryAlertRule{
				ID:         "cpu_breach_grouping_test",
				Name:       "CPU Breach Grouping Test",
				Type:       "esquery",
				Index:      metricsIndex,
				Query:      `{ "query": { "range": { "cpu_usage": { "gte": 90 } } } }`,
				TimeWindow: "5m",
				Threshold:  1,
				DedupRules: &schema.DedupRules{
					// so that each host service alert are unique and does
					// not get deduped
					Fields: []string{"service", "host"},
				},
				Alert: schema.Alert{
					Summary:  "High CPU detected",
					Severity: "high",
				},
			}
			createAlertRule(esClient, rule)
			services := []string{"service-1", "service-2", "service-3"}
			hosts := []string{"host-1", "host-2", "host-3"}

			// 3. Ingest metrics and execute rule sequentially to generate multiple alerts
			// Total 9 metrics (3 per host) will be ingested
			for _, host := range hosts {
				for _, svc := range services {
					ingestMetric(esClient, map[string]interface{}{
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"host":      host,
						"cpu_usage": 97.0,
						"service":   svc,
					})
				}
			}

			executeRuleAndSaveAlerts(esClient, rule)

			// 4. Assert that there are 9 active alerts
			activeAlerts := fetchOnlyActiveAlerts(esClient)
			Expect(activeAlerts).To(HaveLen(9))

			// 5. Assert parent/grouped status
			parentCount := 0
			groupedCount := 0
			parentsByHost := make(map[string]schema.Alert)

			for _, a := range activeAlerts {
				if a.AlertType == "parent" {
					parentCount++
					parentsByHost[a.Metadata.Host] = a
				} else if a.AlertType == "grouped" {
					groupedCount++
				}
			}

			Expect(parentCount).To(Equal(3), "Expected exactly 3 parent alerts (1 per host)")
			Expect(groupedCount).To(Equal(6), "Expected exactly 6 grouped alerts (2 per host)")
			Expect(parentsByHost).To(HaveLen(3))

			// 6. Assert parent has grouped alerts IDs
			refreshIndex(esClient, alertsIndex)

			for host, parent := range parentsByHost {
				// Fetch parent again to get updated grouped_alerts
				activeParent := fetchActiveAlerts(esClient, parent.DedupKey)
				Expect(activeParent).To(HaveLen(1))
				freshParent := activeParent[0]

				Expect(freshParent.GroupedAlerts).To(HaveLen(2), fmt.Sprintf("Host %s parent should have 2 grouped alerts", host))
			}
		})
	})

	Context("When multiple alerts are grouped together", func() {
		It("all the alerts including the parent and the grouped alert should be resolved for the parent to resolve", func() {
			// 1. Create Grouping Rule
			// We group by rule_id so that all alerts from the same rule are grouped together
			groupingRule := schema.GroupingRule{
				ID:           "group_by_host",
				Name:         "Group by Host",
				GroupByField: "metadata.host",
				TimeWindow:   "10m",
			}

			createGroupingRule(esClient, groupingRule)

			// 2. Create Alert Rule
			rule := schema.ESQueryAlertRule{
				ID:         "cpu_breach_grouping_test",
				Name:       "CPU Breach Grouping Test",
				Type:       "esquery",
				Index:      metricsIndex,
				Query:      `{ "query": { "range": { "cpu_usage": { "gte": 90 } } } }`,
				TimeWindow: "5m",
				Threshold:  1,
				DedupRules: &schema.DedupRules{
					// so that each host service alert are unique and does
					// not get deduped
					Fields: []string{"service", "host"},
				},
				Alert: schema.Alert{
					Summary:  "High CPU detected",
					Severity: "high",
				},
			}
			createAlertRule(esClient, rule)
			services := []string{"service-1", "service-2", "service-3"}
			hosts := []string{"host-1", "host-2", "host-3"}

			// 3. Ingest metrics and execute rule sequentially to generate multiple alerts
			// Total 9 metrics (3 per host) will be ingested
			for _, host := range hosts {
				for _, svc := range services {
					ingestMetric(esClient, map[string]interface{}{
						"timestamp": time.Now().UTC().Format(time.RFC3339),
						"host":      host,
						"cpu_usage": 97.0,
						"service":   svc,
					})
				}
			}

			executeRuleAndSaveAlerts(esClient, rule)

			// 4. Assert that there are 9 active alerts
			activeAlerts := fetchOnlyActiveAlerts(esClient)
			Expect(activeAlerts).To(HaveLen(9))

			// 5. Assert parent/grouped status
			parentCount := 0
			groupedCount := 0
			parentsByHost := make(map[string]schema.Alert)

			for _, a := range activeAlerts {
				if a.AlertType == "parent" {
					parentCount++
					parentsByHost[a.Metadata.Host] = a
				} else if a.AlertType == "grouped" {
					groupedCount++
				}
			}

			Expect(parentCount).To(Equal(3), "Expected exactly 3 parent alerts (1 per host)")
			Expect(groupedCount).To(Equal(6), "Expected exactly 6 grouped alerts (2 per host)")
			Expect(parentsByHost).To(HaveLen(3))

			// 6. Assert parent has grouped alerts IDs
			refreshIndex(esClient, alertsIndex)

			for host, parent := range parentsByHost {
				// Fetch parent again to get updated grouped_alerts
				activeParent := fetchActiveAlerts(esClient, parent.DedupKey)
				Expect(activeParent).To(HaveLen(1))
				freshParent := activeParent[0]

				Expect(freshParent.GroupedAlerts).To(HaveLen(2), fmt.Sprintf("Host %s parent should have 2 grouped alerts", host))
			}

			// search all the metrics for service-1 and delete them.
			deleteMetricsForService(esClient, "service-1")
			// re execute rule
			executeRuleAndSaveAlerts(esClient, rule)
			// Assert that service-1 behavior depends on whether it is parent or grouped
			for host := range parentsByHost {
				parent := parentsByHost[host]
				service1Key := fmt.Sprintf("cpu_breach_grouping_test_service-1-%s", host)

				if parent.DedupKey == service1Key {
					// Service-1 is Parent. It should stay ACTIVE because other children are active.
					activeParents := fetchActiveAlerts(esClient, service1Key)
					Expect(activeParents).To(HaveLen(1), fmt.Sprintf("Expected Parent Service-1 to remain active on host %s", host))
				} else {
					// Service-1 is Grouped. It should RESOLVE.
					resolvedAlerts := fetchResolvedAlerts(esClient, service1Key)
					Expect(resolvedAlerts).To(HaveLen(1), fmt.Sprintf("Expected resolved alert for service-1 on host %s", host))

					// And the Parent (whoever it is) should stay ACTIVE.
					activeParents := fetchActiveAlerts(esClient, parent.DedupKey)
					Expect(activeParents).To(HaveLen(1), fmt.Sprintf("Expected Parent %s to remain active on host %s", parent.DedupKey, host))
				}
			}

			// Now delete Service-2 (Grouped)
			deleteMetricsForService(esClient, "service-2")
			executeRuleAndSaveAlerts(esClient, rule)

			for host := range parentsByHost {
				parent := parentsByHost[host]
				service2Key := fmt.Sprintf("cpu_breach_grouping_test_service-2-%s", host)

				if parent.DedupKey == service2Key {
					// Service-2 is Parent. It should stay ACTIVE.
					activeParents := fetchActiveAlerts(esClient, service2Key)
					Expect(activeParents).To(HaveLen(1), fmt.Sprintf("Expected Parent Service-2 to remain active on host %s", host))
				} else {
					// Service-2 is Grouped. It should RESOLVE.
					resolvedAlerts := fetchResolvedAlerts(esClient, service2Key)
					Expect(resolvedAlerts).To(HaveLen(1), fmt.Sprintf("Expected resolved alert for service-2 on host %s", host))
				}
			}

			// Finally delete Service-3 (Grouped)
			deleteMetricsForService(esClient, "service-3")
			executeRuleAndSaveAlerts(esClient, rule)
			// assert that all alerts incuding parents are resolved
			for host := range parentsByHost {
				parent := parentsByHost[host]
				service3Key := fmt.Sprintf("cpu_breach_grouping_test_service-3-%s", host)

				if parent.DedupKey == service3Key {
					// Service-3 is Parent. It should now RESOLVE.
					resolvedParents := fetchResolvedAlerts(esClient, service3Key)
					Expect(resolvedParents).To(HaveLen(1), fmt.Sprintf("Expected Parent Service-3 to be resolved on host %s", host))
				} else {
					// Service-3 is Grouped. It should RESOLVE.
					resolvedAlerts := fetchResolvedAlerts(esClient, service3Key)
					Expect(resolvedAlerts).To(HaveLen(1), fmt.Sprintf("Expected resolved alert for service-3 on host %s", host))
				}
			}
		})
	})
})

// --- Helper Functions ---

func setupIndices(client *es.Client) {
	cleanupIndices(client)

	createIndex(client, metricsIndex, `{
		"mappings": {
			"properties": {
				"timestamp": { "type": "date" },
				"service":   { "type": "keyword" },
				"host":      { "type": "keyword" },
				"cpu_usage": { "type": "double" }
			}
		}
	}`)

	createIndex(client, esqueryAlertIndex, `{
		"mappings": {
			"properties": {
				"id":          { "type": "keyword" },
				"name":        { "type": "text" },
				"type":        { "type": "keyword" },
				"index":       { "type": "keyword" },
				"query":       { "type": "text" },
				"time_window": { "type": "keyword" },
				"threshold":   { "type": "integer" },
				"dedup_rules": {
					"properties": {
						"key":    { "type": "keyword" },
						"fields": { "type": "keyword" }
					}
				},
				"alert": {
					"properties": {
						"summary":        { "type": "text" },
						"severity":       { "type": "keyword" },
						"status":         { "type": "keyword" },
						"alert_type":     { "type": "keyword" },
						"timestamp":      { "type": "date" },
						"dedup_key":      { "type": "keyword" },
						"grouped_alerts": { "type": "keyword" },
						"metadata": {
							"properties": {
								"dependencies": { "type": "keyword" },
								"host":         { "type": "keyword" },
								"rule_id":      { "type": "keyword" },
								"trigger_count": { "type": "integer" }
							}
						}
					}
				}
			}
		}
	}`)

	createIndex(client, alertsIndex, `{
		"mappings": {
			"properties": {
				"summary":        { "type": "text" },
				"severity":       { "type": "keyword" },
				"status":         { "type": "keyword" },
				"alert_type":     { "type": "keyword" },
				"timestamp":      { "type": "date" },
				"dedup_key":      { "type": "keyword" },
				"grouped_alerts": { "type": "keyword" },
				"metadata": {
					"properties": {
						"dependencies": { "type": "keyword" },
						"host":         { "type": "keyword" },
						"rule_id":      { "type": "keyword" },
						"trigger_count": { "type": "integer" }
					}
				}
			}
		}
	}`)

	createIndex(client, groupingRulesIndex, `{
		"mappings": {
			"properties": {
				"id":             { "type": "keyword" },
				"name":           { "type": "text" },
				"group_by_field": { "type": "keyword" },
				"time_window":    { "type": "keyword" }
			}
		}
	}`)
}

func cleanupIndices(client *es.Client) {
	indices := []string{metricsIndex, esqueryAlertIndex, alertsIndex, groupingRulesIndex}
	for _, idx := range indices {
		req := esapi.IndicesDeleteRequest{Index: []string{idx}}
		req.Do(context.Background(), client.ES)
	}
}

func createIndex(client *es.Client, index, mapping string) {
	req := esapi.IndicesCreateRequest{
		Index: index,
		Body:  bytes.NewReader([]byte(mapping)),
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse(), fmt.Sprintf("Failed to create index %s: %s", index, res.String()))
}

func createAlertRule(client *es.Client, rule schema.ESQueryAlertRule) {
	data, err := json.Marshal(rule)
	Expect(err).NotTo(HaveOccurred())

	req := esapi.IndexRequest{
		Index:      esqueryAlertIndex,
		DocumentID: rule.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse())
}

func createGroupingRule(client *es.Client, rule schema.GroupingRule) {
	data, err := json.Marshal(rule)
	Expect(err).NotTo(HaveOccurred())

	req := esapi.IndexRequest{
		Index:      groupingRulesIndex,
		DocumentID: rule.ID,
		Body:       bytes.NewReader(data),
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse())
}

func ingestMetric(client *es.Client, metric map[string]interface{}) {
	data, err := json.Marshal(metric)
	Expect(err).NotTo(HaveOccurred())

	req := esapi.IndexRequest{
		Index:   metricsIndex,
		Body:    bytes.NewReader(data),
		Refresh: "true",
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse())
}

func executeRuleAndSaveAlerts(client *es.Client, rule schema.ESQueryAlertRule) {
	alerts, err := alert.ExecuteESQueryAlertRule(client, rule)
	Expect(err).NotTo(HaveOccurred())
	for _, a := range alerts {
		err := alert.SaveAlert(client, a)
		Expect(err).NotTo(HaveOccurred())
	}
}

func refreshIndex(client *es.Client, index string) {
	req := esapi.IndicesRefreshRequest{
		Index: []string{index},
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
}

func fetchActiveAlerts(client *es.Client, dedupKey string) []schema.Alert {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"dedup_key": dedupKey}},
					map[string]interface{}{"term": map[string]interface{}{"status": "ACTIVE"}},
				},
			},
		},
	}
	return searchAlerts(client, query)
}

func fetchOnlyActiveAlerts(client *es.Client) []schema.Alert {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"status": "ACTIVE"}},
				},
			},
		},
	}
	return searchAlerts(client, query)
}

func fetchResolvedAlerts(client *es.Client, dedupKey string) []schema.Alert {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"term": map[string]interface{}{"dedup_key": dedupKey}},
					map[string]interface{}{"term": map[string]interface{}{"status": "RESOLVED"}},
				},
			},
		},
	}
	return searchAlerts(client, query)
}

func searchAlerts(client *es.Client, query map[string]interface{}) []schema.Alert {
	res, err := client.Search(alertsIndex, query)
	Expect(err).NotTo(HaveOccurred())

	var alerts []schema.Alert
	hitsObj := res["hits"].(map[string]interface{})
	hits := hitsObj["hits"].([]interface{})

	for _, h := range hits {
		source := h.(map[string]interface{})["_source"].(map[string]interface{})
		b, _ := json.Marshal(source)
		var a schema.Alert
		json.Unmarshal(b, &a)
		alerts = append(alerts, a)
	}
	return alerts
}

func deleteMetrics(client *es.Client) {
	refresh := true
	req := esapi.DeleteByQueryRequest{
		Index:   []string{metricsIndex},
		Body:    bytes.NewReader([]byte(`{"query": {"match_all": {}}}`)),
		Refresh: &refresh,
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse())
}

func deleteMetricsForService(client *es.Client, service string) {
	refresh := true
	query := fmt.Sprintf(`{ "query": { "term": { "service": "%s" } } }`, service)
	req := esapi.DeleteByQueryRequest{
		Index:   []string{metricsIndex},
		Body:    bytes.NewReader([]byte(query)),
		Refresh: &refresh,
	}
	res, err := req.Do(context.Background(), client.ES)
	Expect(err).NotTo(HaveOccurred())
	defer res.Body.Close()
	Expect(res.IsError()).To(BeFalse())
}
