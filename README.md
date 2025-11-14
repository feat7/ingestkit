# IngestKit

**High-Performance, Schema-First Data Ingestion Platform**

Self-hosted data ingestion platform for collecting high-volume events with type safety, reliability guarantees, and auto-generated APIs.

## Features

- ✅ **Schema-First:** Define events in YAML, get type-safe APIs and models
- ✅ **High Performance:** 15,600 events/sec with PostgreSQL COPY protocol
- ✅ **Zero Data Loss:** At-least-once delivery, 100% reliability validated
- ✅ **Production-Ready:** Smart batching, retry logic, dead letter queue
- ✅ **Developer-Friendly:** Auto-generated Go models with pgx optimization
- ✅ **Self-Hosted:** Full control over your data and infrastructure
- ✅ **Load Tested:** Validated at 4,788 RPS with 143K events (100% success)
- ✅ **Modern Stack:** PostgreSQL (COPY), Go (Fiber), Redpanda (Kafka)

## Performance Characteristics

**Measured Performance (production-ready with COPY protocol):**

| Throughput | Batch Latency | Success Rate | Events/Batch | DB Write Method |
|------------|--------------|--------------|--------------|-----------------|
| 4,788 RPS  | 13ms avg     | 100%         | 500 events   | COPY protocol   |
| 15,600 events/sec | ~13ms | 100%    | Per batch    | Bulk insert     |
| 143,691 total | <20ms p95 | 100%        | 30 sec test  | Zero failures   |

**Key Optimizations:**

- **PostgreSQL COPY protocol**: 3-4x faster than multi-row INSERT (15,600 events/sec vs ~5,000)
- **pgx connection pooling**: 50 max connections, 10 idle, 1-hour lifecycle
- **Smart batching**: 500 events OR 20ms timeout (whichever first)
- **Zero data loss**: At-least-once delivery with AutoCommitMarks pattern
- **100% reliability**: All load tests passed with zero failures

**Batch Processing Performance:**
- Average: 13ms per batch (500 events)
- P95: <20ms
- Throughput: 15,600 events/second per consumer
- Latency: <50ms end-to-end (API → DB)

See [LAG_ANALYSIS.md](LAG_ANALYSIS.md) for detailed performance analysis.

### Test Results

**High-Throughput Test (30 seconds):**
```
Total events: 143,691
Request rate: 4,788 RPS
Success rate: 100% (zero failures)
Batch size: 500 events
Batch timeout: 20ms
Consumer lag: <50ms end-to-end
DB write latency: 13ms average, <20ms p95
```

**Controlled Test (1000 events):**
```
Total events: 1,000
Success rate: 100%
Batch processing: 206 events in 13ms
Throughput: 15,600 events/second
Zero data loss verified
```

**Performance Comparison:**

| Metric | Before (INSERT) | After (COPY) | Improvement |
|--------|----------------|--------------|-------------|
| Batch write time | ~40ms | ~13ms | 3x faster |
| Events/second | ~5,000 | ~15,600 | 3.1x faster |
| Latency p95 | ~80ms | <20ms | 4x better |
| Protocol | Multi-row INSERT | COPY | Binary |

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Make
- Go 1.21+
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

# Batch test
make loadtest-batch

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
- **PostgreSQL:** `localhost:5432` (use `make db-connect`)

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
make loadtest-high       # High throughput - 5000 RPS (requires rate limit adjustment)
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
    ↓ (consume batches: 500 events OR 20ms)
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
- **Smart batching**: 500 events OR 20ms timeout (whichever first)
- **AutoCommitMarks pattern**: Only commit offsets after successful DB write
- **At-least-once delivery**: Guaranteed data integrity with retry logic
- **Parallel workers**: 4 concurrent consumers for high throughput

**How COPY Protocol Works:**
```go
// Traditional multi-row INSERT (slow)
INSERT INTO events (tenant_id, user_id, ...)
VALUES ($1, $2, ...), ($3, $4, ...), ... // 500 rows

// PostgreSQL COPY (3-4x faster)
pool.CopyFrom(ctx, pgx.Identifier{"events"}, columns,
    pgx.CopyFromSlice(events, func(i int) ([]interface{}, error) {
        // Binary protocol - no SQL parsing
        return []interface{}{event.TenantID, event.UserId, ...}, nil
    }))
```

**Why It's Fast:**
1. Binary protocol (no SQL parsing)
2. Single round-trip for 500 events
3. Optimized for bulk data loading
4. Connection pooling reduces overhead

### Components

**Implemented:**

1. ✅ **Schema Compiler** (`cmd/schema-compiler/`)
   - Parses YAML schema definitions
   - Generates SQL DDL (tables, indexes, partitions)
   - Generates Go models with validation tags
   - Generates storage layer (batch + single writes)

2. ✅ **Ingestion API** (`cmd/api/`)
   - HTTP API with Fiber framework
   - Request validation with go-playground/validator
   - Bearer token authentication
   - Rate limiting (configurable per tenant)
   - Single event and batch endpoints
   - Publishes to Redpanda

3. ✅ **Consumer Worker** (`cmd/consumer/`)
   - Consumes from Redpanda with franz-go
   - Smart batching (500 events OR 20ms timeout - whichever first)
   - At-least-once delivery with AutoCommitMarks pattern
   - 4 parallel workers for high throughput
   - Automatic retry logic (3 attempts with exponential backoff)
   - Dead letter queue for failed events
   - Prometheus metrics exposition
   - 100% data integrity validated

4. ✅ **Storage Layer** (`generated/storage/`)
   - Auto-generated from schema (pgx + COPY protocol)
   - PostgreSQL COPY protocol for batch writes (3-4x faster)
   - pgxpool connection pooling (50 max, 10 idle)
   - Single and batch insert methods
   - Optimized JSONB handling
   - 15,600 events/sec throughput per consumer

5. ✅ **Load Testing Suite**
   - k6 test scripts (5 scenarios)
   - Real-time lag measurement
   - Comprehensive metrics
   - See LOADTEST.md

**Planned:**
- Query API
- SDK generators (Go, Python, etc.)
- Advanced analytics

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

Environment variables (`.env` file):

```bash
# PostgreSQL
POSTGRES_DB=ingestkit
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev
DATABASE_URL=postgres://ingestkit:ingestkit_dev@localhost:5432/ingestkit?sslmode=disable

# Redpanda (Kafka-compatible)
KAFKA_BROKERS=localhost:19092
KAFKA_TOPIC=ingestkit.events

# API Server
API_PORT=8080
RATE_LIMIT_RPS=1000
RATE_LIMIT_BURST=2000

# API Keys (format: key or key:tenant_id)
API_KEY_1=dev_key_1234567890:tenant_default

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500        # Smart batching: 500 events OR...
CONSUMER_BATCH_TIMEOUT=20ms    # ...20ms timeout (whichever first)
METRICS_PORT=8081
```

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
├── LAG_ANALYSIS.md        # Performance analysis
└── README.md
```

## Development Progress

### ✅ Milestone 1.1: POC Infrastructure
- PostgreSQL and Redpanda setup
- Makefile commands
- Docker Compose configuration
- Schema definition format

### ✅ Milestone 1.2: API Server
- Go API with Fiber framework
- Request validation
- Authentication (Bearer tokens)
- Rate limiting (token bucket)
- Single and batch endpoints
- Redpanda publishing
- Full middleware stack

### ✅ Milestone 1.3: Consumer & Storage
- Schema compiler (YAML → SQL + Go models + Storage)
- Consumer with franz-go client
- Batch processing (hybrid time/size)
- Retry logic with exponential backoff
- Dead letter queue integration
- Prometheus metrics
- Parallel workers

### ✅ Milestone 1.3.1: Performance Optimization (COMPLETED)
- PostgreSQL COPY protocol implementation (3-4x faster)
- pgx/v5 migration from database/sql
- pgxpool connection pooling (50 max, 10 idle)
- Smart batching (500 events OR 20ms timeout)
- AutoCommitMarks pattern for at-least-once delivery
- Schema generator updates for COPY support
- Validated: 15,600 events/sec, 100% reliability

### ✅ Current: Load Testing & Validation
- k6 load test suite (5 scenarios)
- Real-time lag measurement
- Performance analysis and documentation
- Bug fixes (API key parsing, JSONB handling)
- Comprehensive metrics
- Validated at 4,788 RPS (143K events, zero failures)

### 🔄 Milestone 1.4: Enhanced Developer Experience (Optional)
- pgAdmin UI for database inspection
- Redis caching layer
- Query API for event retrieval
- Advanced filtering

### 📋 Future Milestones
- SDK generators (Python, Node.js, etc.)
- Schema versioning and migrations
- Multi-tenancy improvements
- S3 archival for cold storage
- Advanced analytics and aggregations

## Load Testing

IngestKit includes a comprehensive load testing suite:

```bash
# Quick smoke test
make loadtest-quick      # 100 RPS for 10s

# Baseline performance
make loadtest-baseline   # 500 RPS for 30s

# Production simulation
make loadtest-production # 1000 RPS for 1 min

# Batch endpoint test
make loadtest-batch      # 100 RPS, 10 events/batch

# Stress test (requires rate limit increase)
make loadtest-high       # 5000 RPS for 30s
```

Monitor in real-time:

```bash
# Watch consumer metrics
make metrics-watch

# Measure lag
./cmd/loadtest/measure_lag.sh watch

# Full production test with metrics
./cmd/loadtest/production_lag_test.sh
```

See [LOADTEST.md](LOADTEST.md) for detailed guide and [LAG_ANALYSIS.md](LAG_ANALYSIS.md) for performance analysis.

## Troubleshooting

### Services won't start

```bash
# Check if ports are in use
lsof -i :5432 -i :8080 -i :8081 -i :19092

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
# Edit .env: RATE_LIMIT_RPS=10000

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

**Status:** Production-ready POC (Milestone 1.3 complete + Performance Optimized)

- ✅ Schema-driven code generation with pgx + COPY protocol
- ✅ Production-ready API with full middleware (rate limiting, auth)
- ✅ Optimized consumer with smart batching (500 events / 20ms)
- ✅ Comprehensive load testing (validated up to 4,788 RPS)
- ✅ Sub-20ms batch processing latency
- ✅ 100% reliability - 143,691 events with zero failures
- ✅ At-least-once delivery guarantees (AutoCommitMarks)
- ✅ 3-4x performance improvement via PostgreSQL COPY

**Latest Benchmarks:**
- Throughput: 15,600 events/second per consumer
- Batch latency: 13ms average, <20ms p95
- Test scale: 143,691 events @ 4,788 RPS (30 seconds)
- Success rate: 100% (zero data loss)

See `PLAN.md` for detailed roadmap and `LAG_ANALYSIS.md` for performance characteristics.
