package main

import (
	"fmt"
	"time"

	"argus-go/internal/alert"
	"argus-go/internal/banner"
	"argus-go/internal/es"
	"argus-go/internal/server"
	"argus-go/schema"
)

// main is the entry point of the application.
func main() {
	banner.Print()
	esClient := es.New([]string{"http://localhost:9200"})

	// Start the health check server in a goroutine
	go func() {
		fmt.Println("[ArgusGo] 🌐 Starting health server on :8080")
		if err := server.StartServer(":8080"); err != nil {
			fmt.Printf("[ArgusGo] ❌ Health server error: %v\n", err)
		}
	}()

	for {
		runScanCycle(esClient)
		time.Sleep(5 * time.Second)
	}
}

// runScanCycle loads rules, executes them, and saves alerts.
func runScanCycle(esClient *es.Client) {
	now := time.Now().Format("15:04:05")
	fmt.Printf("\n[ArgusGo] 👁️  Scan Cycle Started at %s\n", now)

	rules, err := alert.FetchESQueryAlertRules(esClient)
	if err != nil || len(rules) == 0 {
		fmt.Println("[ArgusGo] ℹ️  Waiting for rules in 'esquery_alert' index...")
		time.Sleep(30 * time.Second)
		return
	}

	for _, rule := range rules {
		executeAndSaveAlerts(esClient, rule)
	}
}

// executeAndSaveAlerts runs a rule and saves any generated alerts.
func executeAndSaveAlerts(esClient *es.Client, rule schema.ESQueryAlertRule) {
	fmt.Printf("[ArgusGo] 🚨 Executing Rule: %s\n", rule.Name)
	alerts, err := alert.ExecuteESQueryAlertRule(esClient, rule)
	if err != nil {
		fmt.Printf("[ArgusGo] ❌ Error executing rule %s: %v\n", rule.Name, err)
		return
	}
	for _, a := range alerts {
		if err := alert.SaveAlert(esClient, a); err != nil {
			fmt.Printf("[ArgusGo] ❌ Error saving alert: %v\n", err)
		}
	}
}
