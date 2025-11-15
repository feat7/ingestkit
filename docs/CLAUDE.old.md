# CLAUDE.md - IngestKit Developer Guide

**For AI Assistants & Human Developers**

This document provides comprehensive guidance for working with the IngestKit codebase. Last updated: 2025-11-14

---

## Table of Contents

1. [Project Overview](#project-overview)
2. [Architecture](#architecture)
3. [Codebase Structure](#codebase-structure)
4. [Key Patterns & Conventions](#key-patterns--conventions)
5. [Development Workflows](#development-workflows)
6. [Testing Strategy](#testing-strategy)
7. [Common Tasks](#common-tasks)
8. [Debugging Guide](#debugging-guide)
9. [Performance Considerations](#performance-considerations)
10. [Areas for Improvement](#areas-for-improvement)

---

## Project Overview

**IngestKit** is a high-performance, schema-first data ingestion platform for collecting high-volume events with type safety, reliability guarantees, and auto-generated APIs.

### Core Features

- **Schema-First Design**: YAML schema definitions generate SQL DDL, Go models, and storage layers
- **High Throughput**: 15,600 events/second using PostgreSQL COPY protocol
- **Zero Data Loss**: At-least-once delivery with AutoCommitMarks pattern
- **Production-Ready**: Smart batching, retry logic, dead letter queue
- **Type Safety**: Auto-generated Go models with validation

### Tech Stack

- **Language**: Go 1.22
- **Web Framework**: Fiber (v2)
- **Database**: PostgreSQL with pgx/v5 (COPY protocol)
- **Message Broker**: Redpanda (Kafka-compatible) via franz-go
- **Validation**: go-playground/validator
- **Schema**: YAML-based custom schema compiler

### Performance Characteristics

- **Throughput**: 15,600 events/sec per consumer
- **Batch Latency**: 13ms average, <20ms p95
- **Success Rate**: 100% (zero data loss validated)
- **Protocol**: PostgreSQL COPY (3-4x faster than multi-row INSERT)

---

## Architecture

### Data Flow

```
Client Apps
    ↓ HTTP POST
API Server (Fiber)
    ↓ Validate & Publish
Redpanda (Kafka)
    ↓ Batch Consume (500 events OR 20ms)
Consumer Workers (4 parallel)
    ↓ COPY Protocol + Retry
PostgreSQL (pgxpool)
```

### Component Architecture

#### 1. Schema Compiler (`cmd/schema-compiler/`)

**Purpose**: Transforms YAML event schemas into usable code and database structures.

**Generates**:
- SQL DDL (tables, indexes, partitions) → `generated/sql/`
- Go event models with validation tags → `generated/models/`
- Storage layer (batch + single writers) → `generated/storage/`

**Key Files**:
- `internal/schema/parser.go` - YAML parsing
- `internal/schema/sql_generator.go` - SQL DDL generation
- `internal/schema/go_generator.go` - Go model generation
- `internal/schema/storage_generator.go` - Storage layer generation

**Patterns**:
- Code generation uses Go templates
- Schema version tracking
- Field type mapping (YAML → SQL → Go)

#### 2. API Server (`cmd/api/`)

**Purpose**: HTTP API for event ingestion with validation and rate limiting.

**Endpoints**:
- `POST /v1/events/:type` - Single event ingestion
- `POST /v1/events/:type/batch` - Batch event ingestion
- `GET /health` - Health check

**Middleware Stack** (execution order):
1. Recovery (panic handling)
2. Logger (request logging)
3. Request ID (generates unique IDs)
4. CORS (cross-origin resource sharing)
5. Auth (Bearer token validation)
6. Rate Limit (token bucket algorithm)

**Key Files**:
- `cmd/api/main.go` - Main server setup
- `internal/api/middleware/` - Middleware implementations
- `internal/validation/validator.go` - Schema validation

**Patterns**:
- Event IDs: UUID v7 for time-ordered, sortable uniqueness
- Tenant isolation via `tenant_id` in context
- Async publishing with error channels
- Type-safe context locals with safety checks

#### 3. Consumer Worker (`cmd/consumer/`)

**Purpose**: Consumes events from Redpanda and writes to PostgreSQL with reliability guarantees.

**Features**:
- **Smart Batching**: 500 events OR 20ms timeout (whichever first)
- **Parallel Workers**: 4 concurrent consumers
- **Retry Logic**: 3 attempts with exponential backoff (1s → 2s → 4s)
- **Dead Letter Queue**: Failed events after max retries
- **Metrics Exposition**: Prometheus-compatible `/metrics` endpoint

**Key Files**:
- `cmd/consumer/main.go` - Worker orchestration
- `internal/messaging/consumer.go` - Core consumption logic
- `internal/messaging/producer.go` - Redpanda publishing
- `internal/storage/dlq.go` - Dead letter queue

**Patterns**:
- **AutoCommitMarks**: Only commit offsets after successful DB write
- **At-least-once delivery**: Events may be redelivered on failure
- **Error Classification**: Retriable vs non-retriable errors
- **Thread Safety**: RWMutex for metrics access

#### 4. Storage Layer (`generated/storage/`)

**Purpose**: High-performance batch writes to PostgreSQL using COPY protocol.

**Key Features**:
- **COPY Protocol**: Binary bulk loading (3-4x faster than INSERT)
- **Connection Pooling**: pgxpool with 50 max connections, 10 idle
- **Type-Specific Writers**: Generated per event type
- **JSONB Handling**: Efficient marshaling for flexible fields

**Key Files**:
- `generated/storage/writer.go` - Generated storage implementation
- Uses `github.com/jackc/pgx/v5/pgxpool`

**Patterns**:
```go
// COPY is much faster than INSERT for bulk data
pool.CopyFrom(ctx, pgx.Identifier{"events"}, columns,
    pgx.CopyFromSlice(events, func(i int) ([]interface{}, error) {
        return []interface{}{event.TenantID, event.UserID, ...}, nil
    }))
```

---

## Codebase Structure

```
ingestkit/
├── cmd/
│   ├── api/                    # HTTP API server
│   ├── consumer/               # Event consumer worker
│   ├── schema-compiler/        # Schema → Code generator
│   └── loadtest/              # Load testing utility
├── internal/
│   ├── api/middleware/        # API middleware (auth, rate limit, etc.)
│   ├── messaging/             # Kafka producer/consumer
│   ├── ratelimit/             # Token bucket rate limiter
│   ├── schema/                # Schema parsing & code generation
│   ├── storage/               # DLQ and storage utilities
│   └── validation/            # Event validation
├── generated/                 # Auto-generated code (gitignored)
│   ├── sql/                  # Database DDL
│   ├── models/               # Go event models
│   └── storage/              # Storage layer
├── schema/
│   └── events.yaml           # Event schema definitions
├── docker-compose.yml        # Local development environment
├── Makefile                  # Development commands
├── ISSUES.md                 # Tracked bugs and improvements
├── PLAN.md                   # Development roadmap
└── CLAUDE.md                 # This file
```

### Important Files

| File | Purpose | Auto-Generated? |
|------|---------|----------------|
| `schema/events.yaml` | Event definitions | No |
| `go.mod` | Go dependencies | No |
| `docker-compose.yml` | Local services | No |
| `Makefile` | Dev commands | No |
| `generated/**` | All generated code | Yes |
| `.env` | Local config (gitignored) | No |
| `.env.example` | Config template | No |

---

## Key Patterns & Conventions

### 1. Naming Conventions

- **Go**: PascalCase for exports, camelCase for internal
- **Database**: snake_case for tables and columns
- **YAML**: snake_case for field names
- **Constants**: UPPER_SNAKE_CASE or PascalCase depending on scope

### 2. Error Handling

```go
// Pattern: Always wrap errors with context
if err := doSomething(); err != nil {
    return fmt.Errorf("failed to do something: %w", err)
}

// Pattern: Type assertion safety checks
tenantID, ok := c.Locals("tenant_id").(string)
if !ok {
    return SendError(c, fiber.StatusInternalServerError,
        ErrCodeInternal, "Missing tenant context")
}

// Pattern: Error classification (retriable vs non-retriable)
if isRetriableError(err) {
    // Retry with backoff
} else {
    // Send to DLQ immediately
}
```

### 3. Context Usage

```go
// Pattern: Always pass context as first parameter
func ProcessBatch(ctx context.Context, batch *Batch) error {
    // Check for cancellation
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
    }

    // Use context for database operations
    return db.Write(ctx, batch)
}
```

### 4. Concurrency Patterns

```go
// Pattern: Protect shared state with RWMutex
type Consumer struct {
    metrics   *ConsumerMetrics
    metricsMu sync.RWMutex
}

func (c *Consumer) GetMetrics() ConsumerMetrics {
    c.metricsMu.RLock()
    defer c.metricsMu.RUnlock()
    return *c.metrics  // Return copy, not pointer
}

func (c *Consumer) updateMetrics() {
    c.metricsMu.Lock()
    c.metrics.EventsProcessed++
    c.metricsMu.Unlock()
}
```

### 5. Configuration Pattern

```go
// Pattern: Environment variables with defaults
func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

// Pattern: Validate configuration at startup
func validateConfig(config *Config) error {
    if config.RateLimit < 0 {
        return errors.New("rate limit must be positive")
    }
    return nil
}
```

### 6. Database Ports

**Default**: Port 5433 (not 5432)

**Reason**: Avoids conflicts with existing PostgreSQL installations that use the standard 5432 port.

**Configuration**:
- `POSTGRES_PORT` env var in `.env`
- `docker-compose.yml` maps `${POSTGRES_PORT:-5433}:5432`
- Consumer defaults to 5433 in `cmd/consumer/main.go`

### 7. Logging Patterns

```go
// Pattern: Structured log messages with emojis for visibility
log.Printf("✅ Batch processed successfully: %d events in %v",
    len(batch), duration)
log.Printf("❌ DB write error: %v", err)
log.Printf("⚠️  Warning: %s", message)
log.Printf("🚀 Starting consumer...")

// Pattern: Periodic logging to avoid log flooding
if pollCounter%100 == 0 || hasError {
    log.Printf("📥 Poll #%d statistics...", pollCounter)
}
```

---

## Development Workflows

### Initial Setup

```bash
# 1. Copy environment template
cp .env.example .env

# 2. Start infrastructure
make up  # Starts PostgreSQL (5433) and Redpanda

# 3. Initialize Go modules
make init-go

# 4. Generate code from schema
make generate

# 5. Build binaries
make build

# 6. Create database schema
make db-create
```

### Daily Development

```bash
# Start services in separate terminals
make run-api        # Terminal 1: API server on :8080
make run-consumer   # Terminal 2: Consumer with metrics on :8081

# Check health
curl http://localhost:8080/health
curl http://localhost:8081/health

# View metrics
make metrics

# Send test event
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "test", "email": "test@example.com", "signup_source": "web", "metadata": {}}'
```

### Schema Modification Workflow

1. **Edit schema**: `schema/events.yaml`
2. **Regenerate code**: `make generate`
3. **Rebuild**: `make build`
4. **Update database**: `make db-create` (for new events)
5. **Restart services**

### Load Testing

```bash
# Quick smoke test
make loadtest-quick      # 100 RPS for 10s

# Baseline performance
make loadtest-baseline   # 500 RPS for 30s

# Production simulation
make loadtest-production # 1000 RPS for 1 min

# Monitor during test
make metrics-watch
```

### Database Operations

```bash
# Connect to database
make db-connect

# View statistics
make db-stats
make db-event-counts

# Check dead letter queue
make db-dlq-check

# Reset database (⚠️ DESTRUCTIVE)
make db-reset
```

**IMPORTANT: Partition Creation**

IngestKit uses PostgreSQL partitioned tables by tenant. When you add new event types, you must create partitions for each tenant:

```bash
# For the "default" tenant (used in development and examples)
docker exec ingestkit-postgres psql -U ingestkit ingestkit -c \
  "CREATE TABLE IF NOT EXISTS events_article_viewed_default PARTITION OF events_article_viewed FOR VALUES IN ('default');"

# For production tenants
docker exec ingestkit-postgres psql -U ingestkit ingestkit -c \
  "CREATE TABLE IF NOT EXISTS events_article_viewed_tenant1 PARTITION OF events_article_viewed FOR VALUES IN ('tenant1');"
```

**Batch creation for all new events:**
```bash
for table in article_viewed article_shared comment_posted newsletter_subscribed search_performed; do
  docker exec ingestkit-postgres psql -U ingestkit ingestkit -c \
    "CREATE TABLE IF NOT EXISTS events_${table}_default PARTITION OF events_${table} FOR VALUES IN ('default');"
done
```

Without partitions, you'll see errors like:
```
ERROR: no partition of relation "events_article_viewed" found for row (SQLSTATE 23514)
```

---

## Testing Strategy

### Current Test Coverage

✅ **Unit Tests**:
- `internal/validation/validator_test.go` - Event validation
- `internal/api/middleware/auth_test.go` - Authentication

⚠️ **Missing Tests** (see ISSUES.md):
- Consumer batch processing
- Middleware (rate limit, CORS, request ID, errors)
- Schema generators (SQL, Go, storage)
- Integration tests (end-to-end)
- Edge cases (unicode, empty strings, long values)

### Testing Patterns

```go
// Pattern: Table-driven tests
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        event   interface{}
        wantErr bool
    }{
        {"valid event", validEvent, false},
        {"missing field", invalidEvent, true},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := validator.ValidateEvent("user_signup", tt.event)
            if (err != nil) != tt.wantErr {
                t.Errorf("wanted error: %v, got: %v", tt.wantErr, err)
            }
        })
    }
}
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package
go test ./internal/validation/...

# Verbose output
go test -v ./...
```

---

## Common Tasks

### Adding a New Event Type

1. **Define in schema** (`schema/events.yaml`):
```yaml
events:
  order_placed:
    description: Fired when an order is placed
    fields:
      order_id:
        type: string
        required: true
        indexed: true
      user_id:
        type: string
        required: true
        indexed: true
      total_amount:
        type: decimal
        required: true
      items:
        type: jsonb
```

2. **Regenerate code**:
```bash
make generate
```

3. **Rebuild**:
```bash
make build
```

4. **Create table**:
```bash
make db-create
```

5. **Create partition for default tenant**:
```bash
# IMPORTANT: Must create partition for each tenant
docker exec ingestkit-postgres psql -U ingestkit ingestkit -c \
  "CREATE TABLE IF NOT EXISTS events_order_placed_default PARTITION OF events_order_placed FOR VALUES IN ('default');"
```

6. **Regenerate consumer handler**:
```bash
# The schema compiler auto-generates the consumer handler
make generate
make build
```

The generated handler (`generated/consumer/handler.go`) will automatically include your new event type:
```go
// Auto-generated code handles unmarshaling
case "order_placed":
    var event models.OrderPlaced
    if err := unmarshalEvent(envelope, &event); err != nil {
        return fmt.Errorf("failed to unmarshal order_placed: %w", err)
    }
    orderPlaceds = append(orderPlaceds, &event)
```

**Note:** The consumer handler is now fully code-generated. No manual updates needed!

7. **Restart services**

### Working Examples

IngestKit includes complete working examples in the `examples/` directory:

**Blog Analytics (Python/Flask)** - `examples/blog-flask/`

A fully functional blog application demonstrating:
- Article view tracking with read time and referrer
- Search analytics with query and results tracking
- Social sharing events (Twitter, LinkedIn)
- Comment posting with threading support
- Newsletter subscription tracking

Run the example:
```bash
cd examples/blog-flask
./run.sh  # Starts Flask app on :5002

# In another terminal, test the user journey
./test-flow-simple.sh
```

Features demonstrated:
- Auto-configured client reading from `ingestkit.config.json`
- Environment variable substitution (`${INGESTKIT_API_KEY}`)
- Accepts both dictionaries and Pydantic models
- Clean imports: `from ingestkit import Client`

Query the tracked data:
```sql
-- Article views by category
SELECT category, COUNT(*) as views
FROM events_article_viewed
GROUP BY category
ORDER BY views DESC;

-- Search analytics
SELECT query, AVG(results_count) as avg_results, COUNT(*) as searches
FROM events_search_performed
GROUP BY query
ORDER BY searches DESC;

-- Social sharing by platform
SELECT platform, COUNT(*) as shares
FROM events_article_shared
GROUP BY platform;
```

**E-commerce (TypeScript/Express)** - `examples/ecommerce-express/`

Coming soon: Product views, cart operations, checkout funnel tracking.

### Adding a New Middleware

1. **Create file**: `internal/api/middleware/yourmiddleware.go`

```go
package middleware

import "github.com/gofiber/fiber/v2"

func YourMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Pre-processing

        // Call next handler
        err := c.Next()

        // Post-processing
        return err
    }
}
```

2. **Register in API** (`cmd/api/main.go`):
```go
app.Use(middleware.YourMiddleware())
```

3. **Add tests**: `internal/api/middleware/yourmiddleware_test.go`

### Troubleshooting Port Conflicts

```bash
# Check what's using port 5433
lsof -i :5433

# Change port in .env
echo "POSTGRES_PORT=5434" >> .env
echo "DB_PORT=5434" >> .env

# Restart
make restart
```

### Viewing Logs

```bash
# API logs
# (stdout when running with make run-api)

# Consumer logs
# (stdout when running with make run-consumer)

# Docker logs
docker-compose logs -f postgres
docker-compose logs -f redpanda
```

---

## Debugging Guide

### Common Issues & Solutions

#### 1. Consumer Not Processing Events

**Symptoms**: Events published but not appearing in database

**Debug Steps**:
```bash
# 1. Check consumer is running
curl http://localhost:8081/health

# 2. Check metrics
make metrics

# 3. Check consumer logs for errors
# Look for: unmarshal failures, DB write errors

# 4. Check DLQ
make db-dlq-check

# 5. Check Redpanda
# View Redpanda Console: http://localhost:8090
```

**Common Causes**:
- Consumer not running
- Database connection issues
- Schema mismatch (regenerate with `make generate`)
- Validation failures

#### 2. Database Connection Errors

**Symptoms**: `connection refused` or `authentication failed`

**Debug Steps**:
```bash
# 1. Check PostgreSQL is running
docker-compose ps postgres

# 2. Verify port
echo $DB_PORT  # Should be 5433

# 3. Test connection
make db-connect

# 4. Check environment variables
cat .env | grep DB_
```

**Solutions**:
- Ensure `POSTGRES_PORT=5433` in `.env`
- Restart: `make restart`
- Check port conflicts: `lsof -i :5433`

#### 3. API Rate Limiting

**Symptoms**: 429 Too Many Requests

**Debug Steps**:
```bash
# Check rate limit config
cat .env | grep RATE_LIMIT

# Increase limit temporarily
export RATE_LIMIT_RPS=10000
make run-api
```

#### 4. Validation Errors

**Symptoms**: 400 Bad Request with validation error

**Debug**:
- Check field names match schema (snake_case)
- Check required fields are present
- Check enum values match schema
- Check data types (string vs int vs decimal)

**Example**:
```json
{
  "user_id": "123",           // ✅ string
  "email": "test@example.com", // ✅ string
  "signup_source": "web",      // ✅ enum: web|mobile|api
  "metadata": {}               // ✅ object (will become JSONB)
}
```

#### 5. Generated Code Out of Sync

**Symptoms**: Type errors, undefined fields

**Solution**:
```bash
# Regenerate all code
make generate

# Rebuild
make build

# Restart services
```

---

## Performance Considerations

### Bottlenecks & Optimizations

#### 1. Database Writes

**Current**: PostgreSQL COPY protocol
- **Performance**: 15,600 events/sec
- **Latency**: 13ms average per batch

**Optimization Applied**:
- ✅ Use COPY instead of INSERT (3-4x faster)
- ✅ Connection pooling (50 max, 10 idle)
- ✅ Batch size: 500 events
- ✅ Batch timeout: 20ms

**Future Optimizations**:
- [ ] Partitioning by time (monthly/weekly)
- [ ] Parallel batch writers
- [ ] Write-ahead log tuning

#### 2. Consumer Batching

**Current**: 500 events OR 20ms timeout

**Tuning**:
```bash
# .env
CONSUMER_BATCH_SIZE=500      # Larger = better throughput, higher latency
CONSUMER_BATCH_TIMEOUT=20ms  # Smaller = lower latency, less throughput
```

**Trade-offs**:
- Large batch + long timeout = high throughput, high latency
- Small batch + short timeout = low latency, lower throughput

#### 3. Rate Limiting

**Current**: Token bucket algorithm (in-memory)

**Settings**:
```bash
RATE_LIMIT_RPS=1000  # Requests per second
```

**Limitations**:
- Not distributed (per-process limit)
- No burst control beyond RPS

**Future**:
- [ ] Redis-based rate limiting (distributed)
- [ ] Per-tenant rate limits
- [ ] Burst allowances

#### 4. Logging Overhead

**Optimizations Applied**:
- ✅ Periodic logging (every 100 polls instead of every poll)
- ✅ Log only on errors or milestones

**Settings** (in code):
- Consumer: Logs every 100 batches or on error
- API: Logs every request (via Fiber logger middleware)

---

## Areas for Improvement

### High Priority (see ISSUES.md for details)

1. **Enum Validation Tags** - Fix space-separated oneof values
2. **Magic Numbers** - Define named constants
3. **SQL Injection Risk** - Validate/escape default values
4. **Error Classification** - Use error types instead of string matching
5. **Event Marshaling** - Optimize with `json.RawMessage`

### Medium Priority

6. **Duplicate Event Routing** - Generate switch statement
7. **Structured Logging** - Use zerolog/zap
8. **Configuration Validation** - Validate env vars at startup
9. **Schema Field Descriptions** - Complete all descriptions
10. **PLAN.md Status** - Update milestone completion

### Testing Gaps

11. **Consumer Tests** - Batch processing, retry logic, error classification
12. **Middleware Tests** - Rate limit, CORS, request ID, errors
13. **Schema Generator Tests** - Golden file tests
14. **Integration Tests** - End-to-end API → DB flow
15. **Edge Case Tests** - Unicode, empty strings, long values

### Documentation

16. **Package Godoc** - Add package-level documentation
17. **Environment Variables** - Complete README documentation
18. **API Documentation** - OpenAPI/Swagger spec

### Architecture

19. **Schema Versioning** - Migration strategy
20. **Multi-tenancy** - Enhanced tenant isolation
21. **Query API** - Event retrieval and filtering
22. **SDK Generation** - Python, Node.js, etc.

---

## Deployment Considerations

### Production Checklist

#### Environment

- [ ] Set realistic `RATE_LIMIT_RPS` based on capacity
- [ ] Configure `CONSUMER_WORKERS` based on CPU cores
- [ ] Set appropriate `CONSUMER_BATCH_SIZE` (start with 500)
- [ ] Use production API keys (not `dev_key_*`)
- [ ] Set `ENVIRONMENT=production`
- [ ] Configure `LOG_LEVEL=info` or `warn`

#### Database

- [ ] PostgreSQL 14+ (tested with 18)
- [ ] Connection pooling configured (pgxpool settings)
- [ ] Regular backups configured
- [ ] Monitoring queries and slow logs
- [ ] Consider partitioning for high volume

#### Redpanda/Kafka

- [ ] Topic replication factor ≥ 2
- [ ] Retention policy configured
- [ ] Monitoring lag and throughput
- [ ] Consumer group isolation

#### Monitoring

- [ ] Prometheus scraping `/metrics` endpoint
- [ ] Alerts on high error rates
- [ ] Alerts on consumer lag
- [ ] Dashboard for throughput, latency, errors
- [ ] DLQ monitoring

#### Security

- [ ] TLS for database connections (`sslmode=require`)
- [ ] HTTPS for API (reverse proxy)
- [ ] Rotate API keys regularly
- [ ] Network segmentation (VPC)
- [ ] Secrets management (not .env files)

### Scaling Guidelines

#### Horizontal Scaling

**API Servers**:
- Stateless, scale freely behind load balancer
- Each instance handles RPS independently

**Consumers**:
- Scale by increasing `CONSUMER_WORKERS`
- Each worker in consumer group gets partition assignment
- Add more consumer instances for more parallelism

**Database**:
- Read replicas for query load
- Partitioning for write scaling
- Consider timescale for time-series workloads

#### Vertical Scaling

**API**: CPU-bound (validation, serialization)
- More CPU cores = higher throughput

**Consumer**: I/O-bound (database writes)
- More CPU for parallel workers
- Faster disk for database

**Database**: Memory + Disk I/O
- More RAM for caching
- SSD for write performance

---

## Quick Reference

### Makefile Commands

```bash
# Infrastructure
make up              # Start services
make down            # Stop services
make restart         # Restart all
make clean           # Clean everything

# Development
make generate        # Generate code from schema
make build           # Build API and consumer
make run-api         # Run API (:8080)
make run-consumer    # Run consumer (:8081)

# Database
make db-connect      # psql connection
make db-create       # Create schema
make db-reset        # ⚠️  Drop and recreate
make db-stats        # Statistics
make db-event-counts # Count by type
make db-dlq-check    # Check DLQ

# Load Testing
make loadtest-quick      # 100 RPS, 10s
make loadtest-baseline   # 500 RPS, 30s
make loadtest-production # 1000 RPS, 1min

# Monitoring
make metrics         # View consumer metrics
make metrics-watch   # Watch metrics (updates)
```

### Ports

| Service | Port | Purpose |
|---------|------|---------|
| API | 8080 | HTTP API |
| Consumer Metrics | 8081 | Prometheus metrics |
| PostgreSQL | 5433 | Database (non-standard to avoid conflicts) |
| Redpanda Kafka | 19092 | Message broker |
| Redpanda Console | 8090 | Web UI |
| Redpanda Schema Registry | 18081 | Schema management |

### Environment Variables

See `.env.example` for complete list. Key variables:

```bash
# Database
POSTGRES_PORT=5433
DB_HOST=localhost
DB_PORT=5433

# API
API_PORT=8080
RATE_LIMIT_RPS=1000
API_KEY_1=dev_key_1234567890:default

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500
CONSUMER_BATCH_TIMEOUT=20ms

# Redpanda
REDPANDA_ADDR=localhost:19092
REDPANDA_TOPIC=ingestkit.events
```

---

## Getting Help

### Resources

- **README.md** - Getting started, features, quick start
- **PLAN.md** - Development roadmap and architecture decisions
- **ISSUES.md** - Tracked bugs and improvements (48 issues)
- **LOADTEST.md** - Load testing guide
- **LAG_ANALYSIS.md** - Performance analysis

### Common Questions

**Q: Why port 5433 instead of 5432?**
A: Avoids conflicts with existing PostgreSQL installations. Configurable via `POSTGRES_PORT`.

**Q: Can I change the batch size?**
A: Yes, set `CONSUMER_BATCH_SIZE` in `.env`. Larger = higher throughput but higher latency.

**Q: How do I add a new event type?**
A: Edit `schema/events.yaml`, run `make generate`, `make build`, `make db-create`.

**Q: Why are events not appearing in the database?**
A: Check consumer is running, check logs, check DLQ, verify schema matches.

**Q: How do I reset everything?**
A: `make clean && make up && make db-create`

---

## Contributing

When making changes:

1. **Update schema** → Regenerate code → Rebuild
2. **Add tests** for new features
3. **Update documentation** (README, this file, ISSUES.md)
4. **Run load tests** before claiming performance improvements
5. **Check ISSUES.md** for known issues to avoid duplicating fixes

---

**Last Updated**: 2025-11-14
**Version**: 1.0 (Milestone 1.3.1 complete - Production-ready POC with COPY protocol)
**Status**: ✅ All critical issues fixed (14/48 total issues resolved)

