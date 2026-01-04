# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ArgusGo is a Proof of Concept (PoC) Go application for **alert deduplication and grouping**. It continuously scans ingested metrics against defined alert rules, generates alerts, and applies deduplication/grouping logic to reduce alert noise. This is NOT production-grade.

## Build & Run Commands

```bash
# Development environment (Docker: Elasticsearch + Kibana)
make setup-devenv      # Start containers
make destroy-devenv    # Stop containers

# Elasticsearch indices
make setup-index       # Create indices with mappings
make clean-index       # Delete all indices

# Seed data
make alert-rules       # Seed alert rules
make grouping-rules    # Seed grouping rules
make ingest-metrics    # Ingest test metrics

# Application
make run               # Start alert processing loop

# Testing (requires running Elasticsearch)
make it                # Run integration tests
```

**Typical development workflow:**
```bash
make setup-devenv && make setup-index && make alert-rules && make grouping-rules && make ingest-metrics
make run  # In one terminal
make it   # In another terminal (integration tests)
```

## Architecture

```
cmd/main.go                         # Entry point - infinite loop scanning alert rules
internal/
  alert/
    esquery_alert.go                # Core execution logic, deduplication, grouping
    fetch_alert_rules.go            # Fetch rules from ES
    save_alerts.go                  # Persist alerts to ES
  es/client.go                      # Elasticsearch client wrapper
  server/health.go                  # Health endpoint (/healthz on :8080)
schema/alert.go                     # Data models (ESQueryAlertRule, Alert, GroupingRule)
integration/alert_lifecycle_test.go # Ginkgo integration tests
scripts/                            # Index setup, cleanup, seeding utilities
```

## Elasticsearch Indices

| Index | Purpose |
|-------|---------|
| `metrics` | Input data (CPU, memory, etc) |
| `esquery_alert` | Alert rule definitions |
| `argusgo-alerts` | Generated alerts |
| `grouping_rules` | Rules for grouping alerts |

## Key Concepts

**Dedup Key:** Format is `{ruleID}_{dedupRulesKey}_{field1}_{field2}...` - derived from fields in alert rule's `dedup_rules`. One active alert per dedup_key.

**Alert States:** ACTIVE (condition met) or RESOLVED (condition no longer met).

**Alert Grouping:** New alerts within a time window can be linked to a parent alert. Parent maintains `grouped_alerts` list of child dedup_keys. `alert_type` is "parent" or "grouped".

**Processing Loop:** Every 5 seconds, fetches rules from ES, executes each rule's query, groups hits by dedup_key, creates/updates/resolves alerts based on threshold matches.

## Testing

Uses Ginkgo v2 + Gomega. Integration tests require running Elasticsearch.

```bash
# Run single test file
go test -v ./integration/...

# Run specific test
go test -v ./integration/... -ginkgo.focus="test description"
```

## Debugging

```bash
curl -X GET "localhost:9200/argusgo-alerts/_search?pretty"  # View alerts
curl -X GET "localhost:9200/esquery_alert/_search?pretty"   # View rules
curl http://localhost:8080/healthz                           # Health check
```
