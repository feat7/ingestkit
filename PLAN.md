# IngestKit Development Plan

**Last Updated:** 2025-11-12
**Status:** Planning Phase
**Target:** MVP in 4 weeks

---

## Project Overview

**IngestKit** is a self-hosted, schema-first data ingestion platform for high-volume event collection with reliability guarantees.

### Key Value Propositions
- **Schema-First:** Type-safe APIs with auto-generated SDKs
- **High Performance:** 10k-30k events/second, sub-100ms latency
- **Zero Data Loss:** Message broker buffering with durability
- **Self-Hosted:** Full control, no vendor lock-in
- **Developer-Friendly:** Auto-generated clients, great DX

### Target Users
- SaaS companies wanting Segment alternative (on-prem, cost-effective)
- Teams needing audit logging with compliance requirements
- Companies with high-volume event collection needs
- Organizations requiring schema validation and type safety

---

## Current State

### Phase: POC Complete - Ready for MVP ✅
- [x] Initial research and architecture design
- [x] Schema approach decision: **Schema-First (Option 2)**
- [x] Target use case identified: **SaaS Product Analytics**
- [x] Tech stack finalized: **Go + Fiber + Redpanda + PostgreSQL 18**
- [x] Development infrastructure ready (Docker Compose + Makefile)
- [x] **POC 0.1: Schema Tooling - COMPLETE ✅**
- [x] **POC 0.2: Performance Benchmark - COMPLETE ✅** (1.4x-4.3x faster queries)
- [x] **POC 0.3: End-to-End Spike - COMPLETE ✅** (Full pipeline validated)
- [ ] MVP development started

### Repository Status
- **Branch:** munich (development branch)
- **Infrastructure:**
  - Docker Compose with PostgreSQL 18, Redpanda, Console
  - Makefile with 30+ development commands
  - Database initialization with metadata tables
- **Schema Compiler:**
  - CLI tool: `./bin/ingestkit` or `make generate`
  - YAML schema parser with validation
  - SQL DDL generator (partitioned tables + indexes)
  - Go struct generator (type-safe with validation tags)
  - **Storage writer generator** (type-safe database writes) ⭐
  - Example schema: 3 event types (user_signup, purchase, page_view)
- **Generated Code (all from schema):**
  - `generated/sql/schema.sql` - PostgreSQL DDL
  - `generated/models/events.go` - Type-safe Go structs
  - `generated/storage/writer.go` - Type-safe storage writer ⭐
- **Working Pipeline:**
  - API server (cmd/api) - accepts events via HTTP
  - Consumer worker (cmd/consumer) - uses generated storage writer
  - End-to-end tested with all 3 event types
- **Next:** MVP Phase 1 - Production-ready features

### Quick Start Commands
```bash
make quickstart    # Setup and start all services
make health        # Check service health
make db-connect    # Connect to PostgreSQL
make logs          # View all logs
make help          # Show all available commands
```

---

## Key Decisions Made

### 1. Schema Approach: Schema-First ✅
**Decision:** Use developer-defined YAML schemas with auto-generated code and normalized database tables.

**Why:**
- 5-10x faster queries than JSONB
- Type safety catches bugs at compile time
- Auto-generated SDKs provide excellent DX
- Schema serves as living documentation
- Still flexible via metadata JSONB fields

**Example:**
```yaml
# ingestkit.schema.yaml
events:
  user_signup:
    fields:
      user_id: { type: string, required: true, indexed: true }
      email: { type: string, required: true }
      signup_source: { type: string, values: [web, mobile, api] }
      metadata: { type: jsonb }
```

### 2. Tech Stack
**Selected:**
- **API:** Go with Fiber framework (see rationale below)
- **Message Broker:** Redpanda (Kafka-compatible, simpler ops)
- **Storage:** PostgreSQL 18 with partitioning
  - Released Sept 2025 (latest stable)
  - Up to **3x I/O performance** improvements with async I/O
  - Better index optimization for complex queries
  - **UUID v7 support** (timestamp-ordered, perfect for event IDs)
  - Parallel GIN index builds (faster JSONB indexing)
- **Deployment:** Docker Compose (MVP), Kubernetes (later)

**Deferred for post-MVP:**
- Redis (use in-memory rate limiting for MVP)
- Redpanda Connect (add if transformation logic gets complex)
- Elasticsearch (optional for search use cases)

### 3. Primary Use Case: SaaS Product Analytics ✅
Target companies that need:
- Segment-like functionality but self-hosted
- Schema control and validation
- Better query performance for real-time dashboards
- Cost savings ($50k/year Segment → $0 self-hosted)

### 4. Multi-Tenancy Strategy: Row-Level Isolation ✅
Use PostgreSQL partitioning with tenant_id for isolation.
Can migrate large tenants to dedicated databases later.

### 5. Schema Evolution Strategy: Versioning ✅
Support multiple schema versions simultaneously for backward compatibility.

---

## Architecture

### High-Level Flow
```
Client Apps
    ↓ (HTTP POST)
Go API (Fiber)
    ↓ (validate & publish)
Redpanda (buffer)
    ↓ (consume)
Consumer Worker
    ↓ (write)
PostgreSQL (storage)
    ↓ (query)
Query API
    ↓ (results)
Client Apps
```

### Components

#### 1. Schema Compiler
**Purpose:** Parse YAML schemas and generate artifacts
**Input:** `ingestkit.schema.yaml`
**Output:**
- SQL DDL files (CREATE TABLE statements)
- Go structs with validation tags
- SDK code (Go, Python, TypeScript)

#### 2. Ingestion API
**Purpose:** Accept events via HTTP, validate, publish to Redpanda
**Endpoints:**
- `POST /v1/events/:type` - Single event ingestion
- `POST /v1/events/:type/batch` - Bulk ingestion (up to 1000 events)
- `GET /health` - Health check
- `GET /metrics` - Prometheus metrics

**Features:**
- Schema validation
- API key authentication
- Rate limiting (in-memory for MVP)
- Request ID tracking

#### 3. Consumer Workers
**Purpose:** Consume from Redpanda, write to PostgreSQL
**Features:**
- Batch inserts for performance
- Retry logic with exponential backoff
- Dead letter queue for failed events
- Metrics emission

#### 4. Query API
**Purpose:** Query stored events
**Endpoints:**
- `GET /v1/events/:type` - Query events with filters
- `GET /v1/events/:type/:id` - Get single event
- `POST /v1/events/:type/query` - Advanced queries (JSON body)

**Features:**
- Pagination
- Filtering (by timestamp, indexed fields)
- Aggregations (count, sum, avg)

---

## Development Phases

### Phase 0: Proof of Concept (Week 1) 🔄

#### POC 0.1: Schema Tooling (Days 1-2) ✅
**Goal:** Validate schema-first developer experience

- [x] Create `schema/` directory with example YAML
- [x] Write YAML parser in Go
- [x] Generate SQL DDL from schema
- [x] Generate Go structs from schema
- [x] Manual test: Apply DDL, compile generated code
- **Success Criteria:** ✅ Generated code feels natural to use

**Results:**
- CLI tool: `./bin/ingestkit schema compile`
- Makefile integration: `make generate`
- Generated clean SQL DDL with partitioning and indexes
- Generated type-safe Go structs with validation tags
- Successfully applied schema to PostgreSQL 18
- Generated code compiles without errors

#### POC 0.2: Performance Benchmark (Days 3-4) ✅
**Goal:** Prove normalized schema is faster than JSONB

- [x] Setup PostgreSQL with Docker
- [x] Create two schemas: normalized vs JSONB
- [x] Write benchmark script (insert 100k events)
- [x] Run query benchmarks (filtering, aggregations)
- [x] Document results
- **Success Criteria:** ✅ Normalized is faster for queries (1.4x to 4.3x improvement)

**Results (100k events tested):**

**Insert Performance:**
- Normalized: 3m25s (486 events/sec)
- JSONB: 3m23s (492 events/sec)
- **Conclusion:** Nearly identical insert performance (~1% difference)

**Query Performance:**
1. **Filter by indexed field (user_id):**
   - Normalized: 1.65ms
   - JSONB: 7.10ms
   - **Speedup: 4.32x faster** ⭐

2. **Time range query:**
   - Normalized: 4.51ms
   - JSONB: 6.40ms
   - **Speedup: 1.42x faster**

3. **Aggregation (SUM of amounts):**
   - Normalized: 4.79ms
   - JSONB: 13.96ms
   - **Speedup: 2.92x faster** ⭐

**Key Insights:**
- ✅ Normalized schema provides **1.4x to 4.3x faster queries** depending on query type
- ✅ Insert performance is identical (no write penalty for normalization)
- ✅ Indexed field queries show the biggest improvement (4.3x)
- ✅ Aggregations are nearly 3x faster (crucial for analytics use case)
- ⚠️ Didn't reach 5-10x target, but 1.4-4.3x is still significant
- 💡 PostgreSQL 18's improved GIN indexing helps JSONB more than expected

**Validation:** Schema-first approach is validated for query-heavy analytics workloads!

#### POC 0.3: End-to-End Spike (Days 5-7) ✅
**Goal:** Validate full architecture works

- [x] Minimal Go API (single endpoint)
- [x] Redpanda setup with Docker
- [x] Producer: API → Redpanda
- [x] Consumer: Redpanda → PostgreSQL
- [x] Manual test: POST event, verify in DB
- **Success Criteria:** ✅ Event flows through all components

**Results:**
- **API Server:** Fiber-based HTTP API with health check and ingestion endpoint
  - `GET /health` - Service health check
  - `POST /v1/events/:type` - Event ingestion
  - Connected to Redpanda on localhost:19092
- **Producer:** franz-go based Redpanda producer
  - Synchronous publishing for POC
  - Event envelope with schema version, tenant_id, event_id
  - Partitioning by tenant_id
- **Consumer Worker:** franz-go based consumer with **generated** PostgreSQL writer
  - Consumer group: ingestkit-consumer
  - Uses generated storage writer (type-safe, zero runtime overhead)
  - Automatic event type routing to generated Write methods
  - Per-event-type table writes
- **Storage Code Generation:** ⭐ **Truly Schema-Driven**
  - `make generate` now produces `generated/storage/writer.go`
  - Type-safe Write methods for each event type (WriteUserSignup, WritePurchase, etc.)
  - Zero runtime overhead - pure compiled Go code
  - No hardcoded event handlers - fully schema-driven!
- **End-to-End Test Results (with generated code):**
  - ✅ user_signup event: API → Redpanda → Consumer (generated writer) → PostgreSQL
  - ✅ purchase event: API → Redpanda → Consumer (generated writer) → PostgreSQL
  - ✅ page_view event: API → Redpanda → Consumer (generated writer) → PostgreSQL
  - ✅ All event data correctly persisted in normalized tables
  - ✅ Health endpoint responsive
  - ✅ **Add new events in YAML, regenerate, it just works!**

**Validation:** Complete event pipeline works with **generated code**! True schema-driven architecture achieved.

**Checkpoint:** ✅ All POCs Complete - Architecture validated, proceed to MVP!

---

### Phase 1: MVP Core (Weeks 2-3)

#### Milestone 1.1: Schema Compiler (Week 2, Days 1-3)
**Goal:** Production-ready schema tooling

- [ ] CLI tool: `ingestkit schema compile`
- [ ] YAML schema parser with validation
- [ ] SQL DDL generator
  - [ ] Table creation with proper types
  - [ ] Indexes for marked fields
  - [ ] Partitioning by tenant_id
- [ ] Go struct generator
  - [ ] Proper struct tags (json, validate)
  - [ ] Validation rules (required, enum, etc.)
- [ ] Tests for generator logic
- [ ] Example schemas (user_signup, purchase, page_view)

**Deliverable:** `ingestkit schema compile` generates SQL + Go code

#### Milestone 1.2: Ingestion API (Week 2, Days 4-7)
**Goal:** Accept and validate events

- [ ] Project structure setup
  - [ ] `cmd/api/` - API server
  - [ ] `cmd/consumer/` - Worker
  - [ ] `internal/schema/` - Schema logic
  - [ ] `internal/validation/` - Validators
  - [ ] `pkg/client/` - Go SDK
- [ ] Fiber API setup
  - [ ] Middleware: logging, recovery, CORS
  - [ ] Middleware: API key auth
  - [ ] Middleware: rate limiting (in-memory)
- [ ] Event ingestion endpoints
  - [ ] `POST /v1/events/:type`
  - [ ] `POST /v1/events/:type/batch`
  - [ ] Request validation against schema
- [ ] Redpanda producer
  - [ ] Connection pooling
  - [ ] Error handling
  - [ ] Metrics (events published, errors)
- [ ] Health check endpoint
- [ ] Unit tests

**Deliverable:** API accepts events, validates, publishes to Redpanda

#### Milestone 1.3: Consumer & Storage (Week 3, Days 1-3)
**Goal:** Persist events to PostgreSQL

- [ ] Consumer worker setup
  - [ ] Redpanda consumer group
  - [ ] Configurable concurrency
  - [ ] Graceful shutdown
- [ ] PostgreSQL writer
  - [ ] Batch inserts (configurable size)
  - [ ] Connection pooling
  - [ ] Prepared statements for performance
- [ ] Error handling
  - [ ] Retry logic with backoff
  - [ ] Dead letter queue (failed events table)
- [ ] Metrics emission
  - [ ] Events consumed/second
  - [ ] Write latency
  - [ ] Error rates
- [ ] Tests

**Deliverable:** Events flow from Redpanda to PostgreSQL

#### Milestone 1.4: Query API (Week 3, Days 4-5)
**Goal:** Query stored events

- [ ] Query endpoints
  - [ ] `GET /v1/events/:type` with filters
  - [ ] `GET /v1/events/:type/:id`
  - [ ] Pagination (limit, offset)
  - [ ] Filtering (timestamp range, indexed fields)
- [ ] SQL query builder
  - [ ] Safe parameterization (prevent injection)
  - [ ] Support for common operators (eq, gt, lt, in)
- [ ] Tests

**Deliverable:** Can query events via REST API

---

### Phase 2: Developer Experience (Week 3-4)

#### Milestone 2.1: SDK Generation (Week 3, Days 6-7)
**Goal:** Auto-generate client libraries

- [ ] Go SDK generator
  - [ ] Typed functions (e.g., `TrackUserSignup()`)
  - [ ] Batch support
  - [ ] Error handling
- [ ] Python SDK generator (basic)
  - [ ] Dataclasses for events
  - [ ] HTTP client
- [ ] README for generated SDKs

**Deliverable:** `ingestkit schema compile` generates Go + Python SDKs

#### Milestone 2.2: Deployment & Docs (Week 4)
**Goal:** Easy to run and understand

- [ ] Docker Compose setup
  - [ ] API service
  - [ ] Consumer service
  - [ ] Redpanda
  - [ ] PostgreSQL
  - [ ] Volume mounts for persistence
- [ ] Configuration
  - [ ] Environment variables
  - [ ] Config file support (YAML)
  - [ ] Sensible defaults
- [ ] Documentation
  - [ ] README.md with quickstart
  - [ ] Architecture diagram
  - [ ] Schema definition guide
  - [ ] API reference
  - [ ] Deployment guide
- [ ] Example application
  - [ ] Sample schema
  - [ ] Sample app using generated SDK
  - [ ] Dashboard queries (SQL examples)

**Deliverable:** Someone can run IngestKit in 30 minutes

---

### Phase 3: Production Readiness (Week 4+)

#### Milestone 3.1: Testing & Validation
- [ ] Load testing
  - [ ] Target: 30k events/second
  - [ ] Measure: latency, throughput, resource usage
  - [ ] Tool: k6 or vegeta
- [ ] Failure testing
  - [ ] Redpanda down (verify buffering in API)
  - [ ] PostgreSQL down (verify DLQ)
  - [ ] Network partitions
- [ ] Integration tests
  - [ ] End-to-end event flow
  - [ ] Schema validation edge cases
  - [ ] Query API correctness

#### Milestone 3.2: Observability
- [ ] Prometheus metrics
  - [ ] API: request rate, latency, errors
  - [ ] Consumer: lag, throughput, DLQ size
- [ ] Structured logging
  - [ ] JSON format
  - [ ] Request ID propagation
  - [ ] Log levels
- [ ] Grafana dashboards
  - [ ] System health
  - [ ] Event throughput
  - [ ] Error rates

#### Milestone 3.3: Operational Tools
- [ ] CLI tools
  - [ ] `ingestkit schema migrate` - Handle schema changes
  - [ ] `ingestkit schema validate` - Validate YAML
  - [ ] `ingestkit replay` - Replay events from DLQ
- [ ] Database migrations
  - [ ] Schema versioning
  - [ ] Migration runner
- [ ] Runbooks
  - [ ] Common failure scenarios
  - [ ] Recovery procedures

---

## Technical Specifications

### Go Framework: Fiber (Recommended) ✅

**Why Fiber over Gin:**

| Feature | Fiber | Gin |
|---------|-------|-----|
| **Performance** | ~30% faster | Fast |
| **Memory** | Lower allocation | Good |
| **API Style** | Express.js-like | More Go-idiomatic |
| **Bulk Operations** | Better support | Standard |
| **WebSockets** | Built-in | Via gorilla |
| **Community** | Growing (14k★) | Mature (75k★) |

**Recommendation:** Use **Fiber** for IngestKit because:
1. **Performance matters:** You're targeting 30k events/second
2. **Bulk ingestion:** Fiber handles large payloads well
3. **Modern API:** Cleaner middleware, better ergonomics
4. **Built-in features:** Compression, CORS, rate limiting middleware

**Alternative:** If team prefers Go-idiomatic style, Gin is solid. Performance difference won't matter until very high scale.

### Database Schema Pattern

**Per-event-type tables:**
```sql
-- Generated from schema
CREATE TABLE events_user_signup (
    tenant_id VARCHAR(255) NOT NULL,
    event_id BIGSERIAL,
    timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_id VARCHAR(255) NOT NULL,
    email VARCHAR(255) NOT NULL,
    signup_source VARCHAR(50),
    metadata JSONB,
    PRIMARY KEY (tenant_id, event_id)
) PARTITION BY LIST (tenant_id);

CREATE INDEX idx_user_signup_timestamp ON events_user_signup(timestamp);
CREATE INDEX idx_user_signup_user_id ON events_user_signup(user_id);
```

**Benefits:**
- Optimal indexes per event type
- Query performance (no type filtering needed)
- Schema evolution per type
- Clear separation of concerns

### Message Format (Redpanda)

**Envelope:**
```json
{
  "schema_version": "v1",
  "event_type": "user_signup",
  "tenant_id": "acme_corp",
  "event_id": "01HGQK9T8XYZ...",
  "timestamp": "2025-11-12T10:30:00Z",
  "payload": {
    "user_id": "usr_123",
    "email": "user@example.com",
    "signup_source": "web"
  }
}
```

**Redpanda Topic Strategy:**
- Option A: Single topic `ingestkit.events` (simpler)
- Option B: Per-type topics `ingestkit.events.user_signup` (better scaling)
- **Recommendation:** Start with Option A, migrate to B if needed

### Configuration Structure

**config.yaml:**
```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

redpanda:
  brokers:
    - localhost:9092
  topic: ingestkit.events

database:
  host: localhost
  port: 5432
  database: ingestkit
  user: ingestkit
  password: ${DB_PASSWORD}
  max_connections: 50

consumer:
  group_id: ingestkit-consumer
  workers: 4
  batch_size: 100

auth:
  api_keys:
    - key: ${API_KEY_1}
      tenant_id: acme_corp

rate_limiting:
  enabled: true
  requests_per_second: 1000
```

---

## Project Structure

```
ingestkit/
├── cmd/
│   ├── api/              # API server
│   │   └── main.go
│   ├── consumer/         # Consumer worker
│   │   └── main.go
│   └── cli/              # CLI tools (schema compile, etc.)
│       └── main.go
├── internal/
│   ├── api/              # API handlers
│   │   ├── middleware/
│   │   └── routes/
│   ├── consumer/         # Consumer logic
│   ├── schema/           # Schema compiler
│   │   ├── parser.go     # YAML parser
│   │   ├── generator.go  # Code generators
│   │   └── templates/    # Go templates for codegen
│   ├── storage/          # Database operations
│   ├── messaging/        # Redpanda producer/consumer
│   └── validation/       # Validation logic
├── pkg/
│   └── client/           # Go SDK (also generated)
├── schema/
│   └── events.yaml       # Schema definitions
├── generated/            # Auto-generated code
│   ├── sql/
│   ├── models/
│   └── sdks/
├── deployments/
│   ├── docker-compose.yml
│   └── kubernetes/
├── examples/
│   └── quickstart/       # Example app
├── docs/
│   ├── architecture.md
│   ├── schema-guide.md
│   └── api-reference.md
├── tests/
│   ├── integration/
│   └── load/
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

---

## Success Metrics

### MVP Success Criteria
- [ ] 30k events/second sustained throughput
- [ ] Sub-100ms p95 ingestion latency
- [ ] Zero data loss under normal failures (Redpanda buffering works)
- [ ] Schema compilation generates working code
- [ ] Can run locally with `docker-compose up`
- [ ] Documentation enables 30-min quickstart

### Post-MVP Success (6 months)
- [ ] 3+ internal teams using IngestKit in production
- [ ] 99.9% uptime over 30 days
- [ ] Cost savings vs Segment alternative demonstrated
- [ ] Positive developer feedback on DX
- [ ] Operational burden acceptable (< 2 hours/week maintenance)

---

## Risk Register

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Schema-first too rigid | High | Medium | Include metadata JSONB, support schema versioning |
| Performance targets not met | High | Low | POC benchmarking validates early |
| Operational complexity too high | Medium | Medium | Excellent docs, monitoring, managed offering |
| Developers prefer JSONB flexibility | Medium | Medium | Market research, validate with 3+ teams |
| Redpanda learning curve | Low | Medium | Use Kafka compatibility, good docs exist |

---

## Open Questions

### Technical
- [ ] **Partitioning strategy:** Time-based (monthly) or tenant-based or both?
- [ ] **SDK languages:** Start with Go + Python, or add TypeScript/Java?
- [ ] **Authentication:** API keys only, or support OAuth/JWT?
- [ ] **Encryption:** At-rest? In-transit? Both?

### Product
- [ ] **Open source from day 1** or internal first?
- [ ] **License:** MIT, Apache 2.0, or AGPL?
- [ ] **Managed offering:** Plan from start or later?
- [ ] **Community:** Discord, GitHub Discussions, or Slack?

### Business
- [ ] **Primary user:** Developers or DevOps or both?
- [ ] **Go-to-market:** Developer-led growth or enterprise sales?
- [ ] **Support model:** Community or paid tiers?

---

## Next Steps (Immediate)

### This Week
1. [x] Finalize tech stack decisions
2. [ ] Initialize Go project structure
3. [ ] Setup development environment (Docker Compose with Redpanda + PostgreSQL)
4. [ ] POC 0.1: Schema tooling prototype
5. [ ] POC 0.2: Performance benchmark

### This Month
1. [ ] Complete all POCs
2. [ ] Build MVP core (schema compiler, API, consumer)
3. [ ] Load testing and validation
4. [ ] Internal dogfooding with 1 team

### Communication
- **Weekly updates:** Progress, blockers, decisions needed
- **Demo:** End of each phase (POC, MVP, Production)
- **Feedback:** Continuous from potential users

---

## Resources

### Required Infrastructure
- **Development:**
  - Docker Desktop
  - Go 1.21+
  - PostgreSQL 15+
  - Redpanda (via Docker)

- **Production (estimated for 30k events/sec):**
  - API: 2x 2GB RAM instances (horizontal scale)
  - Consumer: 2x 2GB RAM instances
  - Redpanda: 3-node cluster (2GB RAM each)
  - PostgreSQL: 8GB RAM + SSD storage

### Team Roles Needed
- [ ] Backend engineer (Go) - Lead
- [ ] DevOps engineer - Deployment/monitoring
- [ ] Technical writer - Documentation
- [ ] QA engineer - Testing/validation

---

## Changelog

### 2025-11-12 - Initial Plan
- Created comprehensive development plan
- Decided on schema-first approach
- Selected Fiber as Go framework
- Defined MVP scope (4 weeks)
- Outlined project structure
- Set success criteria

### 2025-11-12 - Infrastructure Setup
- Created Docker Compose configuration
  - PostgreSQL 18 (latest, released Sept 2025) with auto-init script
  - Redpanda (Kafka-compatible message broker)
  - Redpanda Console (web UI)
  - Redis and pgAdmin (optional, via `--profile full`)
- Created comprehensive Makefile with 30+ commands
  - Infrastructure management (up, down, restart, logs)
  - Database operations (connect, create, drop, reset)
  - Redpanda operations (topics, consume, info)
  - Development workflow (build, run, test, lint)
- Created `.env.example` with all configuration options
- Created `.gitignore` for Go project
- Created `README.md` with quick start guide
- Created database initialization script (`init-db/01-init.sql`)
  - Metadata schema for tracking
  - Dead letter queue table
  - API keys management
  - Event types registry
  - Ingestion stats tracking

### 2025-11-12 - POC 0.1: Schema Tooling Complete ✅
- Initialized Go project with dependencies (Fiber, franz-go, lib/pq, yaml.v3)
- Created complete project structure (cmd/, internal/, pkg/, schema/, generated/)
- Created example schema with 3 event types (user_signup, purchase, page_view)
- Built schema compiler components:
  - `internal/schema/parser.go` - YAML parser with validation
  - `internal/schema/sql_generator.go` - PostgreSQL DDL generator
  - `internal/schema/go_generator.go` - Go struct generator with validation tags
  - `cmd/cli/main.go` - CLI tool with `schema compile` and `schema validate` commands
- Generated artifacts successfully:
  - SQL DDL with partitioned tables, indexes, and comments
  - Type-safe Go structs with JSON and validation tags
  - All generated code compiles without errors
- Integrated with Makefile: `make generate` command
- Tested end-to-end: Schema → SQL → Database (applied successfully)
- **Key Learning:** Schema-first approach produces clean, maintainable code

### 2025-11-12 - POC 0.2: Performance Benchmark Complete ✅
- Created JSONB-only schema for fair comparison
- Built comprehensive benchmark program (`tests/benchmarks/`)
  - Automated setup and data cleanup
  - 100k event insertion test (33k per event type)
  - Three query benchmark scenarios
  - Color-coded performance reports
- **Insert Performance Results:**
  - Normalized: 486 events/sec
  - JSONB: 492 events/sec
  - Conclusion: Identical performance (~1% difference)
- **Query Performance Results:**
  - Indexed field filter: **4.32x faster** (1.65ms vs 7.10ms)
  - Time range query: **1.42x faster** (4.51ms vs 6.40ms)
  - Aggregation (SUM): **2.92x faster** (4.79ms vs 13.96ms)
- **Key Findings:**
  - No write penalty for normalized schema
  - Consistent query performance improvement across all scenarios
  - Biggest wins on indexed filters and aggregations
  - PostgreSQL 18's GIN improvements help JSONB more than expected
- **Validation:** Schema-first approach **confirmed faster for analytics queries**
- **Key Learning:** 1.4x-4.3x speedup validates the normalized approach, especially for analytics use cases

### 2025-11-12 - POC 0.3: End-to-End Spike Complete ✅
- Built complete event pipeline from API to PostgreSQL
- **API Server** (`cmd/api/main.go`):
  - Fiber-based HTTP server with logging and recovery middleware
  - Health endpoint: `GET /health`
  - Ingestion endpoint: `POST /v1/events/:type`
  - Graceful shutdown handling
- **Messaging Layer** (`internal/messaging/`):
  - `producer.go` - franz-go based Redpanda producer with event envelopes
  - `consumer.go` - franz-go based consumer with handler pattern
  - Event envelope structure with schema version and tenant partitioning
- **Storage Layer** (`internal/storage/writer.go`):
  - PostgreSQL writer with per-event-type table routing
  - Handles user_signup, purchase, and page_view events
  - JSONB marshaling for flexible fields
- **Consumer Worker** (`cmd/consumer/main.go`):
  - Standalone consumer process with PostgreSQL writer
  - Consumer group: ingestkit-consumer
  - Automatic commit on successful write
- **End-to-End Testing:**
  - Sent 3 test events (1 of each type)
  - All events successfully flowed: API → Redpanda → PostgreSQL
  - Verified data in normalized tables
  - Health endpoint confirmed service status
- **Validation:** Complete pipeline works! Event flow validated end-to-end.
- **Key Learning:** franz-go provides excellent Kafka/Redpanda integration for Go

### 2025-11-12 - Storage Writer Code Generation ⭐
**Making IngestKit Truly Schema-Driven**

- **Problem Identified:** Consumer had hardcoded event handlers that wouldn't work with user-defined events
- **Solution:** Added storage writer code generation to the schema compiler
- **Implementation:**
  - Created `internal/schema/storage_generator.go` - generates type-safe Write methods
  - Updated CLI to generate `generated/storage/writer.go` alongside SQL and models
  - Each event type gets a dedicated WriteEventName() method
  - Automatic JSONB marshaling for flexible fields
  - Zero runtime overhead - pure compiled Go code
- **Consumer Updates:**
  - Removed hardcoded `internal/storage/writer.go`
  - Updated to use `generated/storage` package
  - Type-safe event routing with generated models
  - unmarshalEvent() helper for envelope → typed struct conversion
- **End-to-End Validation:**
  - Regenerated all code with `make generate`
  - Rebuilt and tested complete pipeline
  - Verified events flow through generated code successfully
- **Key Achievement:** IngestKit is now **truly schema-driven**
  - Users define events in YAML
  - Run `make generate` to produce SQL, models, AND storage writer
  - Zero hardcoded event handling - everything is generated
  - Add new events → regenerate → it just works!
- **Performance:** No runtime overhead (vs hardcoded version)
- **Key Learning:** Code generation > runtime reflection for type safety and performance

---

**Status:** All POCs Complete ✅ - Full architecture validated with true schema-driven design!
**Next:** Start MVP development (Production API + Error Handling + SDKs)
**Next Review:** Weekly MVP progress check
