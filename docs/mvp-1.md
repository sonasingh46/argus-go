# MVP-1: Alert Ingestion and Grouping Service

## Overview

Transform ArgusGo into a high-throughput alert ingestion service that receives millions of events, applies grouping logic, and sends notifications. This is the foundation for an alert management platform.

## Functional Requirements

### 1. Event Ingestion

Expose an HTTP endpoint to ingest events at high volume.

**Event Schema (Request Payload):**
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

| Field | Description |
|-------|-------------|
| `event_manager_id` | ID of the event manager (namespace/tenant) this event belongs to |
| `summary` | Human-readable description of the alert |
| `severity` | Alert severity level |
| `action` | Client intent: `trigger` = create/activate, `resolve` = request to close |
| `class` | Classification/category of the alert |
| `dedupKey` | Unique identifier for deduplication |

**System-Managed Fields (not in request, added by system):**

| Field | Description |
|-------|-------------|
| `type` | `parent` or `child` - determined by grouping logic |
| `status` | `active` or `resolved` - current state managed by system |

### 2. Event Manager

A logical namespace/tenant abstraction for teams.

- Each team creates their own Event Manager
- Events are routed based on `event_manager_id` in payload
- Event Manager links to exactly one Grouping Rule (1:1 for MVP)
- CRUD operations via REST API

**Event Manager Schema:**
```json
{
    "id": "string",
    "name": "string",
    "description": "string",
    "grouping_rule_id": "string",
    "notification_config": {
        "webhook_url": "string"
    },
    "created_at": "timestamp",
    "updated_at": "timestamp"
}
```

### 3. Grouping Rules

Rules that determine how incoming events are grouped.

- Single field grouping key (MVP)
- Fixed time window (MVP)
- CRUD operations via REST API

**Grouping Rule Schema:**
```json
{
    "id": "string",
    "name": "string",
    "grouping_key": "string",
    "time_window_minutes": "integer",
    "created_at": "timestamp",
    "updated_at": "timestamp"
}
```

**Example:** If `grouping_key = "class"` and `time_window_minutes = 5`:
- First event with `class: "database"` becomes a **parent** alert
- Subsequent events with `class: "database"` within 5 minutes become **child** alerts linked to that parent

### 4. Alert Lifecycle

**Parent/Child Determination:**
- System determines `type` based on grouping rules
- If no matching parent exists in the time window, event becomes a new parent
- If a matching parent exists, event becomes a child linked to that parent

**Resolution Logic:**
- `action: trigger` - Creates or updates an alert as active
- `action: resolve` - Requests resolution of an alert
- A **child** alert can be resolved independently
- A **parent** alert only transitions to `status: resolved` when ALL its children are also resolved
- System tracks resolve requests for parent alerts until all children are resolved

### 5. Notifications

- Notifications are sent only for **parent** alerts
- Triggered on: new parent created, parent resolved
- Delivery: Best effort (fire and forget for MVP)
- Future: Add async retry queue for failed deliveries

**Notification Payload:**
```json
{
    "alert_id": "string",
    "dedupKey": "string",
    "event_manager_id": "string",
    "summary": "string",
    "severity": "string",
    "status": "string",
    "type": "parent",
    "child_count": "integer",
    "timestamp": "timestamp"
}
```

## Non-Functional Requirements

### Scale

| Metric | Target |
|--------|--------|
| Throughput | 1 million events per minute (~16,667 events/sec) |
| Latency | < 100ms from ingestion to notification |

### Performance Implications

- In-memory state for grouping lookups (no synchronous DB calls on hot path)
- Horizontal scaling with partitioning
- Async persistence (write-behind)
- Stateless ingestion layer

### Database Requirements

- Open source with permissive license (Apache 2.0 preferred)
- High write throughput
- Low-latency reads for state lookups

### Code Quality

- Integration tests for every feature
- Unit tests are mandatory
- Modular, idiomatic Go code
- Small, composable units

## API Endpoints (MVP)

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

### Alerts (Read-only for MVP)
```
GET    /v1/alerts
GET    /v1/alerts/{dedupKey}
GET    /v1/alerts/{dedupKey}/children
```

## Out of Scope for MVP-1

- Composite grouping keys (multiple fields)
- Rolling/sliding time windows
- At-least-once notification delivery
- Event routing to multiple event managers
- Alert suppression/maintenance windows
- Alert escalation policies
- User authentication/authorization
- Multi-region deployment

## Open Questions

1. Should we support batch event ingestion (`POST /v1/events/batch`)?
2. Alert retention policy - how long to keep resolved alerts?
3. Should parent alerts auto-close after time window expires with no activity?
