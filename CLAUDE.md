# CLAUDE.md - IngestKit Quick Reference

**For AI Assistants & Developers**

This file provides essential context for working with IngestKit. Detailed documentation is in the `/docs` folder.

Last updated: 2025-11-15

---

## Project Overview

**IngestKit** is a high-performance, schema-first data ingestion platform for collecting high-volume events with type safety and reliability guarantees.

### Core Features
- **Schema-First**: YAML → SQL DDL, Go models, SDKs (Python, TypeScript)
- **High Throughput**: 15,600 events/sec using PostgreSQL COPY protocol
- **Zero Data Loss**: At-least-once delivery with auto-commit on DB write
- **Auto-Partitions**: Tables auto-create per tenant on first event
- **Production-Ready**: Batching, retry, dead letter queue, metrics

### Tech Stack
- Go 1.22 + Fiber + pgx/v5 (COPY protocol)
- PostgreSQL (partitioned by tenant)
- Redpanda (Kafka-compatible)
- Schema compiler generates: SQL DDL, Go models, Python/TypeScript SDKs

### Performance
- **Throughput**: 15,600 events/sec per consumer
- **Latency**: 13ms average batch, <20ms p95
- **Success Rate**: 100% (zero data loss)

---

## Architecture

### Data Flow
```
Client Apps (Python/TypeScript SDK)
    ↓ HTTP POST /v1/events/:type?sync=true
API Server (Fiber :8080)
    ↓ Validate & Publish to Kafka
Redpanda (:19092)
    ↓ Batch Consume (500 events OR 20ms)
Consumer Workers (4 parallel :8081/metrics)
    ↓ Auto-create partitions + COPY protocol
PostgreSQL (:5433, partitioned by tenant_id)
```

### Components

| Component | Path | Purpose |
|-----------|------|---------|
| Schema Compiler | `cmd/schema-compiler/` | YAML → SQL, Go, SDKs |
| API Server | `cmd/api/` | HTTP ingestion + validation |
| Consumer | `cmd/consumer/` | Kafka → PostgreSQL |
| Storage Layer | `generated/storage/` | COPY protocol writers |
| Partition Manager | `internal/storage/partitions.go` | Auto-create tenant partitions |

---

## Key Patterns

### 1. Auto-Partition Creation
**New tenants automatically get dedicated partitions:**
```go
// internal/storage/partitions.go
pm.EnsurePartition(ctx, "events_product_viewed", "ecommerce-demo")
// Creates: events_product_viewed_ecommerce_demo
```

### 2. Schema Distribution
**Clients fetch schema from API:**
```bash
# Production: Client fetches from server
curl http://api.ingestkit.com/schema

# No local schema files needed on client side
```

### 3. Delivery Guarantees
**SDKs support sync delivery:**
```python
# Python
client.send_user_signup(event, wait=True)  # Waits for Kafka ACK

# TypeScript
await client.sendUserSignup(event, { sync: true });  # Waits for Kafka ACK
```

### 4. Configuration
**Makefile loads `.env` automatically:**
```bash
make run-api       # Sources .env before running
make run-consumer  # Sources .env before running
```

---

## Quick Reference

### Essential Commands

```bash
# Setup
make up              # Start PostgreSQL + Redpanda
make generate        # Generate code from schema/events.yaml
make build           # Build API + consumer binaries
make db-create       # Create database tables

# Development
make run-api         # Start API (:8080)
make run-consumer    # Start consumer (:8081)

# Database
make db-connect      # psql connection
make db-stats        # Statistics
make db-reset        # ⚠️  Drop and recreate

# Testing
make loadtest-quick  # 100 RPS, 10s
```

### Ports

| Service | Port | Purpose |
|---------|------|---------|
| API | 8080 | HTTP events endpoint |
| Consumer Metrics | 8081 | Prometheus `/metrics` |
| PostgreSQL | 5433 | Database (non-standard to avoid conflicts) |
| Redpanda | 19092 | Kafka protocol |

### Environment Variables (Key)

```bash
# Server .env
API_KEY_1=dev_key_1234567890:default
API_KEY_2=sk_test_tenant_alpha:tenant_alpha
API_KEY_3=dev_key_ecommerce:ecommerce-demo

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500

# Database
POSTGRES_PORT=5433
```

---

## Production Workflow

### Server Side (IngestKit Platform)
```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Generate code
make generate

# 3. Apply to database
make db-create

# 4. Build and deploy
make build
make run-api &
make run-consumer &
```

### Client Side (Apps)
```bash
# No local schema needed - fetch from server
cd my-app
ingestkit generate --schema-url https://api.ingestkit.com/schema

# Use generated SDK
npm install
npm start
```

**Partitions auto-create on first event for each tenant.**

---

## Important Notes

### Auto-Partitions
✅ **Partitions are created automatically** when consumer processes first event for a tenant.
❌ **No manual** `CREATE TABLE` commands needed.

### Schema Distribution
✅ **Server exposes** `/schema` endpoint
✅ **Clients fetch** schema from API
❌ **No symlinks** or local schema files in production

### Delivery Guarantees
- **Default** (`sync=false`): 202 Accepted (async, high throughput)
- **With** `?sync=true`: 200 OK (guaranteed Kafka ACK, lower throughput)

### Port 5433
**PostgreSQL runs on 5433** (not 5432) to avoid conflicts with existing installations.

---

## Detailed Documentation

See `/docs` folder for detailed guides:

- **[Development](docs/development.md)** - Workflows, adding events, working with examples
- **[SDK Generation](docs/sdk-generation.md)** - Python & TypeScript SDK details
- **[Common Tasks](docs/common-tasks.md)** - Step-by-step guides
- **[Debugging](docs/debugging.md)** - Troubleshooting common issues
- **[Testing](docs/testing.md)** - Test strategy and patterns
- **[Performance](docs/performance.md)** - Tuning and optimization
- **[Deployment](docs/deployment.md)** - Production checklist
- **[Improvements](docs/improvements.md)** - Known issues and future work

---

## Quick Troubleshooting

**Consumer not processing events?**
```bash
curl http://localhost:8081/health
make metrics
make db-dlq-check
```

**401 Unauthorized?**
- Check API key is in server `.env`
- Restart API: `make run-api`
- Verify: API logs should show "count=3" (or your number of keys)

**Validation errors?**
- Field names must be `snake_case` (not camelCase)
- Check schema: `schema/events.yaml`
- Regenerate: `make generate && make build`

---

**Version**: 1.0
**Status**: Production-ready POC with COPY protocol + auto-partitions
