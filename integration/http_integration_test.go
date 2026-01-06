package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// getBaseURL returns the base URL for API calls.
// Uses ARGUS_BASE_URL env var if set (for container tests),
// otherwise defaults to localhost:8080.
func getBaseURL() string {
	if url := os.Getenv("ARGUS_BASE_URL"); url != "" {
		return url
	}
	return "http://localhost:8080"
}

// httpClient creates an HTTP client with sensible defaults.
func httpClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

// doRequest performs an HTTP request and returns the response.
func doRequest(method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	url := getBaseURL() + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return httpClient().Do(req)
}

// parseResponse parses JSON response into target.
func parseResponse(resp *http.Response, target interface{}) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(target)
}

// cleanupTestData removes test data by making DELETE requests.
func cleanupTestData(groupingRuleIDs, eventManagerIDs []string) {
	for _, id := range eventManagerIDs {
		_, _ = doRequest("DELETE", "/v1/event-managers/"+id, nil)
	}
	for _, id := range groupingRuleIDs {
		_, _ = doRequest("DELETE", "/v1/grouping-rules/"+id, nil)
	}
}

var _ = Describe("HTTP Integration Tests", Ordered, func() {
	var (
		groupingRuleID string
		eventManagerID string
		createdRuleIDs []string
		createdEMIDs   []string
	)

	BeforeAll(func() {
		// Check if the server is reachable
		resp, err := doRequest("GET", "/healthz", nil)
		if err != nil {
			Skip(fmt.Sprintf("Server not reachable at %s: %v", getBaseURL(), err))
		}
		resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	AfterAll(func() {
		cleanupTestData(createdRuleIDs, createdEMIDs)
	})

	Describe("Health Check", func() {
		It("should return healthy status", func() {
			resp, err := doRequest("GET", "/healthz", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Grouping Rules API", func() {
		It("should create a grouping rule", func() {
			payload := map[string]interface{}{
				"name":                "HTTP Test Rule",
				"grouping_key":        "class",
				"time_window_minutes": 5,
			}

			resp, err := doRequest("POST", "/v1/grouping-rules", payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusCreated))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data, ok := result["data"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			groupingRuleID = data["id"].(string)
			createdRuleIDs = append(createdRuleIDs, groupingRuleID)

			Expect(data["name"]).To(Equal("HTTP Test Rule"))
			Expect(data["grouping_key"]).To(Equal("class"))
		})

		It("should get the created grouping rule", func() {
			resp, err := doRequest("GET", "/v1/grouping-rules/"+groupingRuleID, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data := result["data"].(map[string]interface{})
			Expect(data["name"]).To(Equal("HTTP Test Rule"))
		})

		It("should list grouping rules", func() {
			resp, err := doRequest("GET", "/v1/grouping-rules", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data, ok := result["data"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(data)).To(BeNumerically(">=", 1))
		})
	})

	Describe("Event Managers API", func() {
		It("should create an event manager", func() {
			payload := map[string]interface{}{
				"name":             "HTTP Test EM",
				"description":      "Test event manager",
				"grouping_rule_id": groupingRuleID,
			}

			resp, err := doRequest("POST", "/v1/event-managers", payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusCreated))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data := result["data"].(map[string]interface{})
			eventManagerID = data["id"].(string)
			createdEMIDs = append(createdEMIDs, eventManagerID)

			Expect(data["name"]).To(Equal("HTTP Test EM"))
		})

		It("should get the created event manager", func() {
			resp, err := doRequest("GET", "/v1/event-managers/"+eventManagerID, nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data := result["data"].(map[string]interface{})
			Expect(data["name"]).To(Equal("HTTP Test EM"))
		})
	})

	Describe("Event Ingestion and Alert Creation", func() {
		It("should ingest an event and create an alert", func() {
			payload := map[string]interface{}{
				"event_manager_id": eventManagerID,
				"summary":          "HTTP Test Alert",
				"severity":         "high",
				"action":           "trigger",
				"class":            "http-test",
				"dedupKey":         "http-test-alert-1",
			}

			resp, err := doRequest("POST", "/v1/events", payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			// Wait for event to be processed with retry
			var data map[string]interface{}
			Eventually(func() int {
				alertResp, err := doRequest("GET", "/v1/alerts/http-test-alert-1", nil)
				if err != nil {
					return 0
				}
				defer alertResp.Body.Close()

				if alertResp.StatusCode != http.StatusOK {
					return alertResp.StatusCode
				}

				var result map[string]interface{}
				if parseResponse(alertResp, &result) != nil {
					return 0
				}

				data = result["data"].(map[string]interface{})
				return http.StatusOK
			}, 5*time.Second, 200*time.Millisecond).Should(Equal(http.StatusOK))

			Expect(data["summary"]).To(Equal("HTTP Test Alert"))
			Expect(data["type"]).To(Equal("parent"))
			Expect(data["status"]).To(Equal("active"))
		})

		It("should group subsequent events as children", func() {
			// Send another event with same class
			payload := map[string]interface{}{
				"event_manager_id": eventManagerID,
				"summary":          "HTTP Test Child Alert",
				"severity":         "medium",
				"action":           "trigger",
				"class":            "http-test", // Same class as parent
				"dedupKey":         "http-test-alert-2",
			}

			resp, err := doRequest("POST", "/v1/events", payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			// Wait for processing and retry to handle async processing
			var data map[string]interface{}
			Eventually(func() string {
				alertResp, err := doRequest("GET", "/v1/alerts/http-test-alert-2", nil)
				if err != nil {
					return ""
				}
				defer alertResp.Body.Close()

				if alertResp.StatusCode != http.StatusOK {
					return ""
				}

				var result map[string]interface{}
				if parseResponse(alertResp, &result) != nil {
					return ""
				}

				data = result["data"].(map[string]interface{})
				if data["type"] == nil {
					return ""
				}
				return data["type"].(string)
			}, 5*time.Second, 200*time.Millisecond).Should(Equal("child"))

			Expect(data["parent_dedup_key"]).To(Equal("http-test-alert-1"))
		})

		It("should list alerts", func() {
			resp, err := doRequest("GET", "/v1/alerts", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())
		})

		It("should get children of parent alert", func() {
			resp, err := doRequest("GET", "/v1/alerts/http-test-alert-1/children", nil)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var result map[string]interface{}
			Expect(parseResponse(resp, &result)).To(Succeed())

			data, ok := result["data"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(data)).To(BeNumerically(">=", 1))
		})
	})

	Describe("Alert Resolution via Events", func() {
		It("should resolve a child alert", func() {
			payload := map[string]interface{}{
				"event_manager_id": eventManagerID,
				"action":           "resolve",
				"dedupKey":         "http-test-alert-2",
			}

			resp, err := doRequest("POST", "/v1/events", payload)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusAccepted))

			// Wait for processing with retry
			Eventually(func() string {
				alertResp, err := doRequest("GET", "/v1/alerts/http-test-alert-2", nil)
				if err != nil {
					return ""
				}
				defer alertResp.Body.Close()

				if alertResp.StatusCode != http.StatusOK {
					return ""
				}

				var result map[string]interface{}
				if parseResponse(alertResp, &result) != nil {
					return ""
				}

				data := result["data"].(map[string]interface{})
				if data["status"] == nil {
					return ""
				}
				return data["status"].(string)
			}, 5*time.Second, 200*time.Millisecond).Should(Equal("resolved"))
		})
	})
})
