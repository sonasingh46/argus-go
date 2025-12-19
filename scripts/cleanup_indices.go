package main

import (
	"fmt"
	"net/http"
)

const (
	metricsIndex      = "metrics"
	esqueryAlertIndex = "esquery_alert"
	alertsIndex       = "argusgo-alerts"
)

func main() {
	indices := []string{metricsIndex, esqueryAlertIndex, alertsIndex}

	for _, idx := range indices {
		url := fmt.Sprintf("http://localhost:9200/%s", idx)
		req, _ := http.NewRequest("DELETE", url, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Printf("Error deleting %s: %v\n", idx, err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == 200 {
			fmt.Printf("Deleted index: %s\n", idx)
		} else {
			fmt.Printf("Failed to delete index %s: %s\n", idx, resp.Status)
		}
	}
}
