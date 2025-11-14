# IngestKit

**High-Performance, Schema-First Data Ingestion Platform**

Self-hosted data ingestion platform for collecting high-volume events with type safety, reliability guarantees, and auto-generated APIs.

## Features

- **Schema-First:** Define events in YAML, get type-safe APIs and models
- **High Performance:** 18,897 events/sec with PostgreSQL COPY protocol
- **Zero Data Loss:** At-least-once delivery, 100% reliability validated
- **Production-Ready:** Smart batching, retry logic, dead letter queue
- **Developer-Friendly:** Auto-generated Go models with pgx optimization
- **Self-Hosted:** Full control over your data and infrastructure
- **Load Tested:** Validated at 7,224 RPS with 216K events (100% success)
- **Modern Stack:** PostgreSQL (COPY), Go (Fiber), Redpanda (Kafka)

## Performance Characteristics

**Measured Performance (production-ready with COPY protocol):**

| Throughput | Batch Latency | Success Rate | Events Tested | DB Write Method |
|------------|--------------|--------------|---------------|-----------------|
| 7,224 RPS  | 15ms avg     | 100%         | 216,818 events | COPY protocol   |
| 18,897 events/sec | ~15ms | 100%    | 30 sec test    | Bulk insert     |
| p95: 92ms  | p99: 168ms   | 0 errors     | 0 failures    | Zero data loss  |

**Key Optimizations:**

- **PostgreSQL COPY protocol**: 3-4x faster than multi-row INSERT
- **pgx connection pooling**: 50 max connections, 10 idle, 1-hour lifecycle
- **Smart batching**: 100 events OR 20ms timeout (whichever first)
- **Zero data loss**: At-least-once delivery with AutoCommitMarks pattern
- **100% reliability**: All load tests passed with zero failures

**Batch Processing Performance:**
- Average: 15ms per batch (100 events)
- P95: <92ms (API latency)
- Throughput: 18,897 events/second per consumer
- Latency: Sub-100ms end-to-end (API to DB)

### Latest Test Results

**10,000 RPS Load Test (30 seconds):**
```
Total events sent:    216,818
Actual RPS achieved:  7,224 (72% of 10K target)
Success rate:         100% (zero failures)
API latency p95:      91.99ms
API latency p99:      168.16ms
Consumer throughput:  18,897 events/sec
DB write errors:      0
Data loss:            0
```

**5,000 RPS Load Test (30 seconds):**
```
Total events sent:    126,381
Actual RPS achieved:  4,210
Success rate:         100%
API latency p95:      24.96ms
Consumer throughput:  12,409 events/sec
```

**Performance Scaling:**

| RPS Target | Events Sent | Success Rate | p95 Latency | Consumer Peak |
|------------|-------------|--------------|-------------|---------------|
| 100 RPS    | 1,001       | 100%         | 11.37ms     | 6,947/s       |
| 5,000 RPS  | 126,381     | 100%         | 24.96ms     | 12,409/s      |
| 10,000 RPS | 216,818     | 100%         | 91.99ms     | 18,897/s      |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Make
- Go 1.25+ (required for latest dependencies)
- k6 (optional, for load testing: `brew install k6`)

### 1. Clone and Setup

```bash
# Quick start (creates .env, starts services)
make quickstart
```

This will:
- Copy `.env.example` to `.env`
- Start PostgreSQL and Redpanda
- Create the `ingestkit.events` topic
- Start Redpanda Console UI

### 2. Initialize and Build

```bash
# Initialize Go project
make init-go

# Generate code from schema
make generate

# Build applications
make build

# Create database schema
make db-create
```

### 3. Run Services

```bash
# Terminal 1: Start API server
make run-api

# Terminal 2: Start consumer worker
make run-consumer
```

### 4. Verify Everything Works

```bash
# Check health
curl http://localhost:8080/health

# Send test event
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user_1",
    "email": "test@example.com",
    "signup_source": "web",
    "metadata": {}
  }'

# Check consumer metrics
make metrics

# Check database
make db-event-counts
```

### 5. Run Load Tests (Optional)

```bash
# Quick validation (100 RPS for 10s)
make loadtest-quick

# Baseline test (500 RPS for 30s)
make loadtest-baseline

# Production test (1000 RPS for 1 min)
make loadtest-production

# High throughput test (10000 RPS for 30s)
make loadtest-high

# Monitor metrics during tests
make metrics-watch
```

See [LOADTEST.md](LOADTEST.md) for comprehensive load testing guide.

## Access Services

- **API Server:** http://localhost:8080
  - Health: http://localhost:8080/health
- **Consumer Metrics:** http://localhost:8081/metrics
  - Health: http://localhost:8081/health
- **Redpanda Console:** http://localhost:8090
- **PostgreSQL:** `localhost:5433` (use `make db-connect`)
  - Note: Uses port 5433 by default to avoid conflicts with existing PostgreSQL installations
  - Can be changed via `POSTGRES_PORT` environment variable

## Schema Definition

Define your events in `schema/events.yaml`:

```yaml
version: "1.0"
metadata:
  name: ingestkit-events
  description: Event tracking schema

events:
  user_signup:
    description: Fired when a new user signs up
    fields:
      user_id:
        type: string
        required: true
        indexed: true
        description: Unique user identifier
      email:
        type: string
        required: true
        description: User email address
      signup_source:
        type: string
        values: [web, mobile, api]
        description: Where the signup originated
      utm_campaign:
        type: string
        description: Marketing campaign identifier
      metadata:
        type: jsonb
        description: Additional flexible metadata

  purchase:
    description: Fired when a user makes a purchase
    fields:
      user_id:
        type: string
        required: true
        indexed: true
      order_id:
        type: string
        required: true
      amount:
        type: decimal
        required: true
      currency:
        type: string
        values: [USD, EUR, GBP, INR]
      payment_method:
        type: string
        values: [card, paypal, stripe, razorpay]
      items:
        type: jsonb
        description: Array of purchased items

  page_view:
    description: Fired when a user views a page
    fields:
      user_id:
        type: string
        indexed: true
      session_id:
        type: string
        required: true
      page_url:
        type: string
        required: true
      page_title:
        type: string
      referrer:
        type: string
      duration_ms:
        type: integer
        description: Time spent on page (milliseconds)
      metadata:
        type: jsonb
```

After modifying the schema:

```bash
# Regenerate code
make generate

# Rebuild applications
make build

# Apply database changes (for new events)
make db-create
```

## Makefile Commands

### Infrastructure

```bash
make up              # Start PostgreSQL and Redpanda
make down            # Stop all services
make restart         # Restart all services
make logs            # View all logs
make status          # Show service status
make health          # Check service health
make clean           # Clean up everything
```

### Development

```bash
make init-go         # Initialize Go project
make generate        # Generate code from schema
make build           # Build API and consumer
make run-api         # Run API server
make run-consumer    # Run consumer worker
make test            # Run tests (when implemented)
```

### Database

```bash
make db-connect      # Connect to PostgreSQL with psql
make db-create       # Create database schema
make db-drop         # Drop all tables (DANGEROUS)
make db-reset        # Drop and recreate database
make db-stats        # Show database statistics
make db-event-counts # Show event counts by type
make db-dlq-check    # Check dead letter queue
```

### Load Testing

```bash
make loadtest-quick      # Quick test - 100 RPS for 10s
make loadtest-baseline   # Baseline - 500 RPS for 30s
make loadtest-production # Production - 1000 RPS for 1 min
make loadtest-batch      # Batch test - 100 RPS with batches
make loadtest-high       # High throughput - 10000 RPS (requires RATE_LIMIT_RPS=20000)
```

### Monitoring

```bash
make metrics         # Check consumer metrics
make metrics-health  # Check consumer health
make metrics-watch   # Watch metrics in real-time
```

## Architecture

```
Client Apps
    ↓ (HTTP POST)
Go API Server (Fiber)
    ↓ (validate & publish)
Redpanda (Kafka-compatible)
    ↓ (consume batches: 100 events OR 20ms)
Consumer Worker (4 parallel workers)
    ↓ (COPY protocol batch write + retry)
PostgreSQL (partitioned tables + pgxpool)
    ↓ (query)
Query API (future)
```

### Performance Optimizations

**Database Layer:**
- **PostgreSQL COPY protocol**: Binary bulk loading (3-4x faster than INSERT)
- **pgxpool**: Application-level connection pooling (50 max, 10 idle, 1h lifecycle)
- **pgx/v5**: High-performance PostgreSQL driver (native COPY support)
- Eliminates SQL parsing overhead for batch operations

**Consumer Layer:**
- **Smart batching**: 100 events OR 20ms timeout (whichever first)
- **AutoCommitMarks pattern**: Only commit offsets after successful DB write
- **At-least-once delivery**: Guaranteed data integrity with retry logic
- **Parallel workers**: 4 concurrent consumers for high throughput

**How COPY Protocol Works:**
```go
// Traditional multi-row INSERT (slow)
INSERT INTO events (tenant_id, user_id, ...)
VALUES ($1, $2, ...), ($3, $4, ...), ... // 100 rows

// PostgreSQL COPY (3-4x faster)
pool.CopyFrom(ctx, pgx.Identifier{"events"}, columns,
    pgx.CopyFromSlice(events, func(i int) ([]interface{}, error) {
        // Binary protocol - no SQL parsing
        return []interface{}{event.TenantID, event.UserId, ...}, nil
    }))
```

**Why It's Fast:**
1. Binary protocol (no SQL parsing)
2. Single round-trip for multiple events
3. Optimized for bulk data loading
4. Connection pooling reduces overhead

### Components

**Implemented:**

- [x] **Schema Compiler** (`cmd/schema-compiler/`)
  - Parses YAML schema definitions
  - Generates SQL DDL (tables, indexes, partitions)
  - Generates Go models with validation tags
  - Generates storage layer (batch + single writes)

- [x] **Ingestion API** (`cmd/api/`)
  - HTTP API with Fiber framework
  - Request validation with go-playground/validator
  - Bearer token authentication
  - Rate limiting (configurable per tenant)
  - Single event and batch endpoints
  - Publishes to Redpanda

- [x] **Consumer Worker** (`cmd/consumer/`)
  - Consumes from Redpanda with franz-go
  - Smart batching (100 events OR 20ms timeout - whichever first)
  - At-least-once delivery with AutoCommitMarks pattern
  - 4 parallel workers for high throughput
  - Automatic retry logic (3 attempts with exponential backoff)
  - Dead letter queue for failed events
  - Prometheus metrics exposition
  - 100% data integrity validated

- [x] **Storage Layer** (`generated/storage/`)
  - Auto-generated from schema (pgx + COPY protocol)
  - PostgreSQL COPY protocol for batch writes (3-4x faster)
  - pgxpool connection pooling (50 max, 10 idle)
  - Single and batch insert methods
  - Optimized JSONB handling
  - 18,897 events/sec throughput per consumer

- [x] **Load Testing Suite**
  - k6 test scripts (5 scenarios)
  - Real-time metrics monitoring
  - Comprehensive performance validation
  - See LOADTEST.md

**Planned:**
- [ ] Query API
- [ ] SDK generators (Go, Python, etc.)
- [ ] Advanced analytics

## API Usage

### Single Event

```bash
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "usr_123",
    "email": "user@example.com",
    "signup_source": "web",
    "metadata": {"referrer": "google"}
  }'

# Response: {"event_id": 12345}
```

### Batch Events

```bash
curl -X POST http://localhost:8080/v1/events/user_signup/batch \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {
        "user_id": "usr_123",
        "email": "user1@example.com",
        "signup_source": "web",
        "metadata": {}
      },
      {
        "user_id": "usr_124",
        "email": "user2@example.com",
        "signup_source": "mobile",
        "metadata": {}
      }
    ]
  }'

# Response: {"event_ids": [12345, 12346]}
```

### Using Go SDK (Generated Models)

```go
package main

import (
    "github.com/feat7/ingestkit/generated/models"
    "github.com/feat7/ingestkit/pkg/client"
)

func main() {
    // Create client
    c := client.New("your-api-key", "http://localhost:8080")

    // Track event (type-safe!)
    event := &models.UserSignup{
        UserId:       "usr_123",
        Email:        "user@example.com",
        SignupSource: "web",
        Metadata:     json.RawMessage(`{"referrer": "google"}`),
    }

    err := c.TrackUserSignup(event)
    if err != nil {
        panic(err)
    }
}
```

## Configuration

### Environment Variables Reference

IngestKit uses environment variables for all configuration. Copy `.env.example` to `.env` and customize as needed.

#### PostgreSQL Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `POSTGRES_DB` | `ingestkit` | PostgreSQL database name |
| `POSTGRES_USER` | `ingestkit` | PostgreSQL username |
| `POSTGRES_PASSWORD` | `ingestkit_dev` | PostgreSQL password |
| `POSTGRES_PORT` | `5433` | PostgreSQL port (5433 avoids conflicts with existing installs) |
| `DB_HOST` | `localhost` | PostgreSQL host for API/Consumer |
| `DB_PORT` | `5433` | PostgreSQL port for API/Consumer |
| `DB_NAME` | `ingestkit` | Database name for API/Consumer |
| `DB_USER` | `ingestkit` | Database user for API/Consumer |
| `DB_PASSWORD` | `ingestkit_dev` | Database password for API/Consumer |

#### Redpanda (Kafka) Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `REDPANDA_ADDR` | `localhost:19092` | Redpanda broker address |
| `REDPANDA_TOPIC` | `ingestkit.events` | Kafka topic for events |
| `CONSUMER_GROUP_ID` | `ingestkit-consumer` | Kafka consumer group ID |

#### API Server Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8080` | HTTP port for API server |
| `SCHEMA_PATH` | `schema/events.yaml` | Path to event schema file |
| `RATE_LIMIT_RPS` | `10000` | Rate limit requests per second (configurable up to 20000+) |

#### API Keys & Authentication

| Variable | Format | Description |
|----------|--------|-------------|
| `API_KEY_1` to `API_KEY_10` | `key:tenant_id` | API keys (e.g., `dev_key_123:default`) |

**Example**:
```bash
API_KEY_1=dev_key_1234567890:default
API_KEY_2=sk_prod_abc123:acme_corp
API_KEY_3=sk_test_xyz789:tenant_alpha
```

#### Consumer Worker Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CONSUMER_WORKERS` | `4` | Number of parallel consumer workers |
| `CONSUMER_BATCH_SIZE` | `100` | Max events per batch |
| `CONSUMER_BATCH_TIMEOUT` | `20ms` | Max time to wait for batch |
| `METRICS_PORT` | `8081` | Prometheus metrics HTTP port |

#### Optional Services (--profile full)

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_PORT` | `6379` | Redis port for caching |
| `PGADMIN_EMAIL` | `admin@ingestkit.local` | pgAdmin login email |
| `PGADMIN_PASSWORD` | `admin` | pgAdmin login password |

### Configuration Validation

Both API and Consumer validate configuration at startup. Invalid values will cause the service to fail fast with a clear error message.

**Example validation errors**:
- `RATE_LIMIT_RPS` must be between 1 and 1,000,000
- `API_PORT` must be between 1 and 65535
- `REDPANDA_ADDR` must include port (e.g., `localhost:19092`)
- `SCHEMA_PATH` must point to an existing file

## Project Structure

```
ingestkit/
├── cmd/
│   ├── api/                 # API server (implemented)
│   ├── consumer/            # Consumer worker (implemented)
│   ├── schema-compiler/     # Schema compiler (implemented)
│   └── loadtest/           # Load test utilities
├── internal/
│   ├── messaging/          # Kafka producer/consumer
│   ├── ratelimit/          # Token bucket rate limiter
│   └── schema/             # Schema parsing and generation
├── generated/              # Auto-generated code
│   ├── sql/               # Database schema DDL
│   ├── models/            # Go event models
│   └── storage/           # Storage layer (writers)
├── schema/
│   └── events.yaml        # Event definitions
├── loadtest/              # k6 load test scripts
│   ├── quick.js
│   ├── baseline.js
│   ├── production.js
│   ├── batch.js
│   └── high-throughput.js
├── docker-compose.yml
├── Makefile
├── LOADTEST.md            # Load testing guide
├── CLAUDE.md              # Developer guide
└── README.md
```

## Development Progress

### Milestone Status

- [x] **Milestone 1.1: POC Infrastructure**
  - PostgreSQL and Redpanda setup
  - Makefile commands
  - Docker Compose configuration
  - Schema definition format

- [x] **Milestone 1.2: API Server**
  - Go API with Fiber framework
  - Request validation
  - Authentication (Bearer tokens)
  - Rate limiting (token bucket)
  - Single and batch endpoints
  - Redpanda publishing
  - Full middleware stack

- [x] **Milestone 1.3: Consumer & Storage**
  - Schema compiler (YAML to SQL + Go models + Storage)
  - Consumer with franz-go client
  - Batch processing (hybrid time/size)
  - Retry logic with exponential backoff
  - Dead letter queue integration
  - Prometheus metrics
  - Parallel workers

- [x] **Milestone 1.3.1: Performance Optimization**
  - PostgreSQL COPY protocol implementation (3-4x improvement)
  - pgx/v5 migration from database/sql
  - pgxpool connection pooling (50 max, 10 idle)
  - Smart batching (100 events OR 20ms timeout)
  - AutoCommitMarks pattern for at-least-once delivery
  - Schema generator updates for COPY support
  - Validated: 18,897 events/sec, 100% reliability

- [x] **Milestone 1.4: Load Testing & Validation**
  - k6 load test suite (5 scenarios)
  - Real-time metrics monitoring
  - Performance analysis and documentation
  - Bug fixes (API key parsing, JSONB handling)
  - Comprehensive validation
  - Tested up to 10,000 RPS target (achieved 7,224 RPS sustained)
  - Zero data loss across 216,818+ events

### Future Milestones

- [ ] **Milestone 2.0: Enhanced Developer Experience**
  - pgAdmin UI for database inspection
  - Redis caching layer
  - Query API for event retrieval
  - Advanced filtering

- [ ] **Milestone 3.0: SDK & Tooling**
  - SDK generators (Python, Node.js, etc.)
  - Schema versioning and migrations
  - Multi-tenancy improvements
  - S3 archival for cold storage
  - Advanced analytics and aggregations

## Load Testing

IngestKit includes a comprehensive load testing suite with validated performance:

```bash
# Quick smoke test
make loadtest-quick      # 100 RPS for 10s

# Baseline performance
make loadtest-baseline   # 500 RPS for 30s

# Production simulation
make loadtest-production # 1000 RPS for 1 min

# Batch endpoint test
make loadtest-batch      # 100 RPS, 10 events/batch

# High throughput test
make loadtest-high       # 10000 RPS for 30s
```

Monitor in real-time:

```bash
# Watch consumer metrics
make metrics-watch

# Check event counts
make db-event-counts
```

See [LOADTEST.md](LOADTEST.md) for detailed guide and configuration options.

## Troubleshooting

### Port Conflicts

**PostgreSQL Port (Default: 5433)**

IngestKit uses port 5433 by default to avoid conflicts with existing PostgreSQL installations that typically use 5432. If you need to change the port:

```bash
# Option 1: Set in .env file
echo "POSTGRES_PORT=5434" >> .env

# Option 2: Export environment variable
export POSTGRES_PORT=5434

# Then restart services
make restart
```

**If port 5433 is also in use:**

```bash
# Check what's using the port
lsof -i :5433

# Kill the process or choose a different port
export POSTGRES_PORT=5434
make restart
```

**Note:** If you change the PostgreSQL port, you must also update:
- `DATABASE_URL` in your `.env` file
- `DB_PORT` environment variable for the consumer (if set)

Example with custom port:
```bash
# .env
POSTGRES_PORT=5434
DATABASE_URL=postgres://ingestkit:ingestkit_dev@localhost:5434/ingestkit?sslmode=disable
DB_PORT=5434
```

### Services won't start

```bash
# Check if ports are in use
lsof -i :5433 -i :8080 -i :8081 -i :19092

# Clean restart
make clean
make up
```

### Consumer not processing events

```bash
# Check consumer logs
docker-compose logs consumer

# Check consumer health
curl http://localhost:8081/health

# Check metrics
make metrics

# Check DLQ for failed events
make db-dlq-check
```

### Database connection issues

```bash
# Verify PostgreSQL is running
docker-compose ps postgres

# Test connection
make db-connect

# Check database stats
make db-stats
```

### Load test failures

```bash
# Check API is running
curl http://localhost:8080/health

# Check consumer is running
curl http://localhost:8081/health

# Increase rate limit if needed
# Edit .env: RATE_LIMIT_RPS=20000

# Restart API
make run-api
```

## Contributing

See `PLAN.md` for detailed development plan and architecture decisions.

Contributions are welcome! Please:
1. Check PLAN.md for roadmap and current focus
2. Follow existing code structure
3. Add tests for new features
4. Update documentation

## License

TBD

## Support

For issues, questions, or contributions, please open an issue on GitHub.

---

**Status:** Production-ready (Milestone 1.4 complete - Load Tested & Validated)

- [x] Schema-driven code generation with pgx + COPY protocol
- [x] Production-ready API with full middleware (rate limiting, auth)
- [x] Optimized consumer with smart batching (100 events / 20ms)
- [x] Comprehensive load testing (validated up to 10,000 RPS target)
- [x] Sub-100ms API latency (p95: 92ms at 7,224 RPS)
- [x] 100% reliability - 216,818 events with zero failures
- [x] At-least-once delivery guarantees (AutoCommitMarks)
- [x] Consumer throughput: 18,897 events/second

**Latest Benchmarks:**
- Throughput: 18,897 events/second per consumer
- API latency: p95 92ms, p99 168ms (at 7,224 RPS)
- Test scale: 216,818 events @ 7,224 RPS (30 seconds)
- Success rate: 100% (zero data loss)
- Consumer batch latency: 15ms average

See `PLAN.md` for detailed roadmap and `LOADTEST.md` for performance testing guide.
