package main

import (
	"fmt"
	"time"

	"argus-go/internal/alert"
	"argus-go/internal/banner"
	"argus-go/internal/es"
)

func main() {
	banner.Print()

	esClient := es.New([]string{"http://localhost:9200"})
	engine := alert.New(esClient)

	for {
		now := time.Now().Format("15:04:05")
		fmt.Printf("\n[%s] 👁️  Scan Cycle Started at %s\n", alert.Brand, now)

		rules, err := esClient.FetchThresholdRules()
		if err != nil || len(rules) == 0 {
			fmt.Printf("[%s] ℹ️  Waiting for rules in 'alert_rules' index...\n", alert.Brand)
			time.Sleep(30 * time.Second)
			continue
		}

		for _, r := range rules {
			// Safely assert types or handle defaults if necessary
			name, _ := r["rule_name"].(string)
			threshold, _ := r["threshold"].(float64)
			window, _ := r["window_minutes"].(float64)

			rule := alert.ThresholdRule{
				RuleName:      name,
				Threshold:     threshold,
				WindowMinutes: window,
			}
			engine.CheckThreshold(rule)
		}

		time.Sleep(5 * time.Second)
	}
}
