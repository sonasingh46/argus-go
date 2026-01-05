# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

ArgusGo is a high-throughput alert ingestion and grouping service. It receives millions of events, applies grouping logic based on configurable rules, and manages alert lifecycles. This is MVP-1 implementation with in-memory storage (designed to be easily pluggable with Kafka, Redis, and PostgreSQL).

## Architecture

The application uses a clean architecture with the following key components:

```
cmd/argus/main.go              # Application entry point
internal/
  api/                         # HTTP handlers and routing (Fiber)
    server.go                  # Server setup and middleware
    event_manager_handler.go   # Event Manager CRUD
    grouping_rule_handler.go   # Grouping Rule CRUD
    alert_handler.go           # Alerts (read-only)
    ingest_handler.go          # Event ingestion endpoint
  config/                      # YAML configuration loading
  domain/                      # Core business entities
    event.go                   # Event model and validation
    alert.go                   # Alert model (parent/child, status)
    event_manager.go           # Event Manager model
    grouping_rule.go           # Grouping Rule model
  ingest/                      # Event ingestion service
    service.go                 # Validates, enriches, publishes to queue
  processor/                   # Alert processing service
    service.go                 # Grouping logic, state management
  queue/                       # Message queue abstraction
    queue.go                   # Producer/Consumer interfaces
    memory/                    # In-memory queue implementation
  store/                       # Storage abstractions
    state_store.go             # Redis-like state store interface
    repository.go              # DB repository interfaces
    memory/                    # In-memory implementations
  notification/                # Notification service (stubbed)
config/config.yaml             # Configuration file
integration/                   # Ginkgo integration tests
```

## Build & Run Commands

```bash
# Build binary
make build

# Run application
make run

# Run all tests
make test

# Run unit tests only
make test-unit

# Run integration tests only
make it

# Format code
make fmt

# Clean build artifacts
make clean
```

## Key Concepts

### Event Manager
A namespace/tenant abstraction. Each team creates an Event Manager that links to a Grouping Rule.

### Grouping Rule
Defines how alerts are grouped:
- `grouping_key`: Field to group by (e.g., "class")
- `time_window_minutes`: How long a parent alert accepts children

### Alert Lifecycle
1. First event with a unique grouping value → **Parent Alert**
2. Subsequent events with same grouping value (within time window) → **Child Alert**
3. Child alerts can be resolved independently
4. Parent alerts resolve only when ALL children are resolved

### Alert States
- `active`: Alert condition is present
- `resolved`: Alert has been resolved

### Alert Types
- `parent`: Root alert that groups children
- `child`: Alert grouped under a parent

## API Endpoints

### Event Ingestion
```
POST /v1/events
```

### Event Manager CRUD
```
POST   /v1/event-managers
GET    /v1/event-managers
GET    /v1/event-managers/{id}
PUT    /v1/event-managers/{id}
DELETE /v1/event-managers/{id}
```

### Grouping Rules CRUD
```
POST   /v1/grouping-rules
GET    /v1/grouping-rules
GET    /v1/grouping-rules/{id}
PUT    /v1/grouping-rules/{id}
DELETE /v1/grouping-rules/{id}
```

### Alerts (Read-only)
```
GET    /v1/alerts
GET    /v1/alerts/{dedupKey}
GET    /v1/alerts/{dedupKey}/children
```

### Health Check
```
GET    /healthz
```

## Event Payload

```json
{
    "event_manager_id": "string",
    "summary": "string",
    "severity": "high | medium | low",
    "action": "trigger | resolve",
    "class": "string",
    "dedupKey": "string"
}
```

## Testing

Uses standard Go testing for unit tests and Ginkgo v2 + Gomega for integration tests.

```bash
# Run single test
go test -v ./internal/domain/... -run TestEvent_Validate

# Run specific Ginkgo test
go test -v ./integration/... -ginkgo.focus="should create a parent alert"
```

## Design Principles

- **Interface-based design**: All storage and queue components use interfaces for easy pluggability
- **Small, focused functions**: Each function does one thing well
- **SOLID principles**: Single responsibility, dependency injection
- **Idiomatic Go**: Error handling, naming conventions, package organization
- **Test coverage**: Unit tests for logic, integration tests for flows

## Plugging in Real Implementations

The in-memory implementations can be replaced with real ones:

1. **Queue**: Implement `queue.Producer` and `queue.Consumer` interfaces for Kafka
2. **State Store**: Implement `store.StateStore` interface for Redis
3. **Repositories**: Implement `store.*Repository` interfaces for PostgreSQL

## Configuration

Edit `config/config.yaml`:

```yaml
server:
  host: "0.0.0.0"
  port: 8080

kafka:
  brokers: ["localhost:9092"]
  topic: "argus-events"

redis:
  host: "localhost"
  port: 6379

postgres:
  host: "localhost"
  database: "argus"
```
