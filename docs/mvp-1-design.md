# MVP-1: System Design

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                     Clients                                         │
│                          (Teams sending alert events)                               │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                                 Load Balancer                                       │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                        INGEST SERVICE (Stateless, N replicas)                       │
│                                                                                     │
│  Responsibilities:                                                                  │
│  • Receive HTTP requests (POST /v1/events)                                          │
│  • Validate event payload                                                           │
│  • Compute partition key: hash(event_manager_id + grouping_key_value)               │
│  • Publish event to message queue with partition key                                │
│  • Return 202 Accepted immediately                                                  │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                          MESSAGE QUEUE (Kafka or NATS)                              │
│                                                                                     │
│  • Partitioned by routing key                                                       │
│  • Guarantees ordering within partition                                             │
│  • Decouples ingestion from processing                                              │
│  • Handles backpressure                                                             │
└─────────────────────────────────────────────────────────────────────────────────────┘
                                         │
                                         ▼
┌─────────────────────────────────────────────────────────────────────────────────────┐
│                      PROCESSOR SERVICE (N consumers, 1 per partition)               │
│                                                                                     │
│  Responsibilities:                                                                  │
│  • Consume events from assigned partition                                           │
│  • Lookup event manager config + grouping rule (from cache)                         │
│  • Check state store for existing parent in time window                             │
│  • Determine: create parent / link as child / resolve                               │
│  • Update state store (Redis)                                                       │
│  • Async: persist alert to database                                                 │
│  • Async: trigger notification if parent alert                                      │
└─────────────────────────────────────────────────────────────────────────────────────┘
                    │                         │                         │
                    ▼                         ▼                         ▼
     ┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
     │    STATE STORE       │  │    PERSISTENCE       │  │   NOTIFICATION       │
     │      (Redis)         │  │       (DB)           │  │     SERVICE          │
     │                      │  │                      │  │                      │
     │ • Active alerts      │  │ • Historical alerts  │  │ • Webhook delivery   │
     │ • Parent-child map   │  │ • Event managers     │  │ • Best effort        │
     │ • Grouping windows   │  │ • Grouping rules     │  │ • Fire and forget    │
     │ • < 1ms lookups      │  │ • Audit trail        │  │                      │
     └──────────────────────┘  └──────────────────────┘  └──────────────────────┘
```

## Components

### 1. Ingest Service

**Purpose:** High-throughput HTTP endpoint for receiving events.

**Characteristics:**
- Stateless - can scale horizontally without coordination
- Fast response - validates and queues, returns 202 immediately
- No business logic - just validation and routing

**Key Operations:**
1. Parse and validate JSON payload
2. Lookup event manager to get grouping rule (cached)
3. Extract grouping key value from event
4. Compute partition key: `hash(event_manager_id + grouping_key_value)`
5. Publish to message queue
6. Return 202 Accepted

**Why separate from Processor?**
- Ingestion must never block on processing
- Different scaling characteristics
- Can absorb traffic spikes via queue

### 2. Message Queue

**Purpose:** Decouple ingestion from processing, ensure ordering.

**Requirements:**
- Partitioned - events with same key go to same partition
- Ordered - within a partition, events processed in order
- Durable - survives restarts (optional for MVP)
- High throughput - 16K+ messages/sec

**Options:**

| Option | License | Pros | Cons |
|--------|---------|------|------|
| **Kafka** | Apache 2.0 | Battle-tested, exactly-once, huge ecosystem | Heavier, needs Zookeeper/KRaft |
| **NATS JetStream** | Apache 2.0 | Simpler, lower latency, lightweight | Less mature persistence |
| **Redpanda** | BSL | Kafka-compatible, no JVM | License concerns |

**Recommendation:** Start with **Kafka** for reliability, or **NATS** for simplicity in MVP.

### 3. Processor Service

**Purpose:** Core business logic - grouping, state management, alert lifecycle.

**Characteristics:**
- One consumer per partition (no coordination needed)
- Maintains local cache of configs
- All state operations go through Redis

**Key Operations:**
1. Consume event from queue
2. Fetch event manager config (from cache, fallback to DB)
3. Fetch grouping rule (from cache)
4. Query state store for existing parent:
   - Key pattern: `parent:{event_manager_id}:{grouping_key}:{grouping_value}`
   - Check if exists and within time window
5. If parent exists: create child, link to parent
6. If no parent: create as new parent
7. If action=resolve: handle resolution logic
8. Update state store
9. Async: persist to database
10. Async: send notification (if applicable)

### 4. State Store (Redis)

**Purpose:** Fast in-memory state for grouping decisions.

**Why Redis?**
- Sub-millisecond latency
- Rich data structures (hashes, sorted sets, lists)
- Built-in TTL for automatic cleanup
- Can be clustered for scale

**Data Structures:**

```
# Active parent lookup (with TTL = time_window)
parent:{event_manager_id}:{grouping_key}:{grouping_value} -> {
    "dedupKey": "...",
    "created_at": "...",
    "child_count": N
}

# Alert state
alert:{dedupKey} -> {
    "event_manager_id": "...",
    "type": "parent|child",
    "status": "active|resolved",
    "parent_dedupKey": "..." (if child),
    "resolve_requested": true|false (for parents)
}

# Parent-child relationship
children:{parent_dedupKey} -> SET of child dedupKeys

# Pending parent resolution
pending_resolve:{parent_dedupKey} -> {
    "requested_at": "...",
    "remaining_children": N
}
```

### 5. Persistence (Database)

**Purpose:** Durable storage for alerts, configs, and audit trail.

**Requirements:**
- High write throughput (async writes from processor)
- Reasonable read latency for API queries
- Schema flexibility for evolving alert structure

**Options:**

| Option | License | Pros | Cons |
|--------|---------|------|------|
| **PostgreSQL** | PostgreSQL (permissive) | Familiar, ACID, rich queries | Write scaling limits |
| **Cassandra** | Apache 2.0 | Excellent write throughput, linear scale | Complex operations, eventual consistency |
| **ScyllaDB** | AGPL | Cassandra-compatible, faster | License concerns |
| **ClickHouse** | Apache 2.0 | Excellent for analytics, fast writes | Column-oriented, not ideal for CRUD |

**Recommendation:**
- **PostgreSQL** for MVP (simpler, familiar, good enough for initial scale)
- Migrate to **Cassandra** if write throughput becomes bottleneck

### 6. Notification Service

**Purpose:** Deliver webhook notifications for parent alerts.

**MVP Approach:**
- Embedded in Processor Service (not separate)
- Fire and forget - make HTTP POST, don't retry on failure
- Log failures for debugging

**Future Enhancement:**
- Separate service with retry queue
- Dead letter queue for persistent failures
- Exponential backoff

## Data Flow

### Event Ingestion Flow

```
1. Client sends POST /v1/events
         │
         ▼
2. Ingest Service validates payload
         │
         ▼
3. Lookup event manager (cached) to get grouping_key field
         │
         ▼
4. Extract grouping_key_value from event (e.g., event.class)
         │
         ▼
5. Compute partition_key = hash(event_manager_id + grouping_key_value)
         │
         ▼
6. Publish to Kafka partition
         │
         ▼
7. Return 202 Accepted to client
```

### Event Processing Flow

```
1. Processor consumes event from partition
         │
         ▼
2. Fetch event manager config + grouping rule
         │
         ▼
3. Query Redis: GET parent:{em_id}:{key}:{value}
         │
         ├─── Parent exists and in window ───┐
         │                                   │
         ▼                                   ▼
4a. No parent found                    4b. Parent found
    Create as parent                       Create as child
    SET parent:... with TTL                Link to parent
    SET alert:{dedupKey}                   SADD children:{parent}
                                           SET alert:{dedupKey}
         │                                   │
         └───────────────┬───────────────────┘
                         ▼
5. Async: Persist alert to PostgreSQL
                         │
                         ▼
6. If parent alert: Send notification webhook
```

### Resolution Flow

```
1. Event arrives with action=resolve
         │
         ▼
2. Lookup alert:{dedupKey} in Redis
         │
         ├─── Alert is CHILD ────────────────┐
         │                                   │
         ▼                                   ▼
3a. Alert is PARENT                    3b. Mark child resolved
    Check: any active children?            Decrement parent.remaining_children
         │                                   │
         ├─ Yes: Mark resolve_requested      ├─ If remaining == 0 and
         │       Wait for children           │    parent.resolve_requested:
         │                                   │    Resolve parent too
         ├─ No: Resolve immediately          │
         │                                   │
         └───────────────┬───────────────────┘
                         ▼
4. Update Redis + Async persist
                         │
                         ▼
5. If parent resolved: Send notification
```

## Partitioning Strategy

**Goal:** Events that could potentially be grouped must go to the same partition.

**Partition Key:** `hash(event_manager_id + grouping_key_value)`

**Example:**
- Event Manager "team-a" has grouping rule with `grouping_key = "class"`
- Event: `{event_manager_id: "team-a", class: "database", ...}`
- Partition key: `hash("team-a" + "database")`
- All events from team-a with class=database go to same partition

**Why this works:**
- Same partition = same consumer = no distributed coordination
- Grouping decision is local to one processor
- No locking or distributed transactions needed

**Scaling:**
- More partitions = more parallelism
- Rebalancing when adding consumers
- Hot partitions possible if one grouping_key_value is very frequent

## Technology Stack (Proposed)

| Component | Technology | License | Rationale |
|-----------|------------|---------|-----------|
| Language | Go | BSD | Already using, excellent for this workload |
| HTTP Server | net/http or Fiber | MIT | High performance |
| Message Queue | Kafka | Apache 2.0 | Reliable, battle-tested |
| State Store | Redis | BSD | Fast, rich data structures |
| Database | PostgreSQL | PostgreSQL | Simple MVP, migrate later if needed |
| Containerization | Docker | Apache 2.0 | Already using |

## Latency Budget

Target: < 100ms end-to-end

| Step | Budget | Notes |
|------|--------|-------|
| HTTP receive + validate | 5ms | |
| Kafka publish | 10ms | Async, but wait for ack |
| Kafka consume | 5ms | Near real-time |
| Redis lookups | 2ms | 1-2 operations |
| Redis writes | 2ms | |
| Async DB write | 0ms | Non-blocking |
| Webhook call | 50ms | Fire and forget |
| **Total** | ~74ms | Buffer for variance |

## Open Design Questions

1. **Kafka vs NATS:** Which to use? Kafka is more robust, NATS is simpler.

2. **Notification embedding:** Keep notifications in Processor, or separate service from day 1?

3. **Config caching:** How long to cache event manager/grouping rule configs? TTL? Invalidation?

4. **Partition count:** How many Kafka partitions to start with? (Suggest: 32, can increase)

5. **Redis clustering:** Single Redis instance for MVP, or clustered from start?

6. **API service:** Separate service for CRUD APIs (event managers, grouping rules, alerts query), or part of Ingest Service?

## Future Considerations (Post-MVP)

- **Composite grouping keys:** Multiple fields in grouping rule
- **Rolling windows:** Sliding time windows instead of fixed
- **At-least-once notifications:** Retry queue with exponential backoff
- **Metrics & observability:** Prometheus metrics, distributed tracing
- **Multi-tenancy:** Rate limiting, quotas per event manager
- **High availability:** Redis Sentinel/Cluster, Kafka replication
