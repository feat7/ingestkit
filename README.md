# IngestKit

**High-Performance, Schema-First Data Ingestion Platform**

Self-hosted event tracking with type safety, auto-generated SDKs, and zero data loss guarantees.

## Features

- **Schema-First**: YAML → SQL DDL, Go models, Python/TypeScript SDKs
- **High Performance**: 15,600 events/sec with PostgreSQL COPY protocol
- **Zero Data Loss**: At-least-once delivery, 100% reliability validated
- **Auto-Partitions**: Tables auto-create per tenant on first event
- **Production-Ready**: Smart batching, retry logic, dead letter queue
- **Developer-Friendly**: Auto-generated SDKs with type safety

**Performance**: 15,600 events/sec | P95 latency: <20ms | 100% success rate

## Quick Start

### Prerequisites
- Docker & Docker Compose
- Make
- Go 1.22+

### Setup (5 minutes)

```bash
# 1. Start infrastructure
make quickstart

# 2. Initialize and build
make init-go
make generate
make build
make db-create

# 3. Run services (separate terminals)
make run-api        # Terminal 1: API on :8080
make run-consumer   # Terminal 2: Consumer on :8081

# 4. Test it
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_user",
    "email": "test@example.com",
    "signup_source": "web",
    "metadata": {}
  }'

# Check it worked
make metrics
make db-event-counts
```

## Using IngestKit

### Working Examples

**Blog Analytics** (Python/Flask):
```bash
cd examples/blog-flask
./run.sh
./test-flow-simple.sh  # In another terminal
```

**E-commerce** (TypeScript/Express):
```bash
cd examples/ecommerce-express
npm install
npm start
./test-flow.sh  # In another terminal
```

### Integration

**Python:**
```python
from ingestkit import Client

analytics = Client(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890"
)

# Track events (type-safe!)
analytics.send_user_signup({
    "user_id": "usr_123",
    "email": "user@example.com",
    "signup_source": "web"
})
```

**TypeScript:**
```typescript
import { Client } from './ingestkit';

const analytics = new Client({
  apiUrl: 'http://localhost:8080',
  apiKey: 'dev_key_1234567890'
});

await analytics.sendUserSignup({
  userId: 'usr_123',
  email: 'user@example.com',
  signupSource: 'web'
});
```

## Common Commands

```bash
# Infrastructure
make up              # Start PostgreSQL + Redpanda
make down            # Stop services
make restart         # Restart everything

# Development
make generate        # Generate code from schema
make build           # Build binaries
make run-api         # Run API (:8080)
make run-consumer    # Run consumer (:8081)

# Database
make db-connect      # Connect with psql
make db-create       # Create schema
make db-stats        # Show statistics
make db-event-counts # Count events by type

# Testing
make loadtest-quick  # 100 RPS smoke test
make metrics         # View consumer metrics
```

## Architecture

```
Client Apps (Python/TypeScript SDK)
    ↓ HTTP POST
API Server (Fiber :8080)
    ↓ Validate & Publish
Redpanda (:19092)
    ↓ Batch Consume (500 events OR 20ms)
Consumer Workers (4 parallel)
    ↓ Auto-create partitions + COPY protocol
PostgreSQL (:5433, partitioned by tenant)
```

**Key Features:**
- Auto-partition creation per tenant
- PostgreSQL COPY protocol (3-4x faster than INSERT)
- Smart batching with configurable timeout
- At-least-once delivery with AutoCommitMarks
- Prometheus metrics at :8081/metrics

## Schema Definition

Define events in `schema/events.yaml`:

```yaml
version: "1.0"

events:
  user_signup:
    description: Fired when a new user signs up
    fields:
      user_id:
        type: string
        required: true
        indexed: true
      email:
        type: string
        required: true
      signup_source:
        type: string
        values: [web, mobile, api]
      metadata:
        type: jsonb
```

After modifying schema:
```bash
make generate && make build && make db-create
```

## Services

| Service | Port | URL |
|---------|------|-----|
| API Server | 8080 | http://localhost:8080/health |
| Consumer Metrics | 8081 | http://localhost:8081/metrics |
| Redpanda Console | 8090 | http://localhost:8090 |
| PostgreSQL | 5433 | `make db-connect` |

**Note:** Port 5433 avoids conflicts with existing PostgreSQL installations.

## Documentation

- **[CLAUDE.md](CLAUDE.md)** - Quick reference for developers
- **[docs/](docs/)** - Detailed guides (development, SDKs, deployment)
- **[LOADTEST.md](LOADTEST.md)** - Performance testing guide
- **[ISSUES.md](ISSUES.md)** - Known issues and improvements

## Key Concepts

**Auto-Partitions**: Tables automatically created per tenant on first event. No manual partition management needed.

**Schema Distribution**: Clients fetch schema from API (`/schema` endpoint). No local schema files needed in production.

**Delivery Guarantees**:
- Default: 202 Accepted (async, high throughput)
- With `?sync=true`: 200 OK (guaranteed Kafka ACK)

**Multi-Tenancy**: Each API key maps to a tenant. Events automatically partitioned by `tenant_id`.

## Status

**Production-Ready** ✅

- [x] Schema-driven code generation (SQL, Go, Python, TypeScript)
- [x] High-performance API (15,600 events/sec validated)
- [x] Auto-partition creation per tenant
- [x] Zero data loss guarantees
- [x] Working examples (blog, e-commerce)
- [x] Comprehensive testing

**Latest Performance:**
- Throughput: 15,600 events/sec
- Latency: 13ms avg, <20ms p95
- Success Rate: 100% (zero failures)

## Contributing

See [CLAUDE.md](CLAUDE.md) for architecture and development guide.

## License

TBD

---

**Need Help?**
- Quick Reference: [CLAUDE.md](CLAUDE.md)
- Detailed Docs: [docs/](docs/)
- Load Testing: [LOADTEST.md](LOADTEST.md)
- Issues: [ISSUES.md](ISSUES.md)
