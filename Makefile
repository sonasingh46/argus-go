.PHONY: setup-index clean-index seed-rules alert-rules ingest-metrics

setup-index:
	go run scripts/setup_indices.go

clean-index:
	go run scripts/cleanup_indices.go

seed-rules:
	go run scripts/example_esquery_alert_rules.go

alert-rules:
	go run scripts/example_esquery_alert_rules.go

ingest-metrics:
	go run scripts/ingest_metrics.go

grouping-rules:
	go run scripts/create_grouping_rules.go

it:
	go test -v -count=1 ./integration/...
