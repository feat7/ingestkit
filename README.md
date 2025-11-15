# IngestKit

**High-Performance, Schema-First Event Ingestion Platform**

A self-hosted event tracking system with type-safe SDKs, automated migrations, and zero data loss guarantees.

---

## What is IngestKit?

IngestKit is an event ingestion platform built for developers who need:
- **Type Safety**: Define events in YAML, get type-safe SDKs automatically
- **Performance**: 15,600 events/sec with PostgreSQL COPY protocol
- **Reliability**: At-least-once delivery, dead letter queue, automatic retries
- **Developer Experience**: Prisma-style migrations, auto-generated code, Docker deployments

**Use Cases**: Product analytics, audit logging, user behavior tracking, event streaming

---

## Features

### Core Capabilities

- **Schema-First Design**: Single YAML file generates SQL DDL, Go models, and client SDKs
- **Auto-Generated SDKs**: Type-safe Python and TypeScript clients with retry logic
- **Automated Migrations**: Prisma-style migration generation using Atlas (auto-generates SQL)
- **Multi-Tenant**: Automatic table partitioning per tenant
- **Zero Data Loss**: Kafka buffering with at-least-once delivery guarantees
- **High Performance**: PostgreSQL COPY protocol (3-4x faster than INSERT)

### Production Features

- Smart batching (500 events OR 20ms timeout)
- Automatic retry with exponential backoff
- Dead letter queue for failed events
- Prometheus metrics endpoint
- Zero-downtime Docker deployments
- Health check endpoints

---

## Quick Start (2 Minutes)

### Prerequisites

- **Docker & Docker Compose**
- **Make**

### One Command to Start

```bash
make start
```

That's it! This will:
- Build Docker images
- Start PostgreSQL, Redpanda, API, and Consumer
- Apply database migrations automatically
- Show you what to do next

### Verify It's Working

```bash
# Send a test event
make test-event

# View consumer metrics
make metrics

# Check events in database
make db-event-counts
```

### Local Development (Without Docker)

If you prefer running services locally:

```bash
# One-time setup
make start-local

# Then in separate terminals:
make run-api        # Terminal 1
make run-consumer   # Terminal 2
```

---

## Architecture

```
Client Apps (Python/TypeScript SDK)
    ↓ HTTP POST /v1/events/:type
API Server (Fiber :8080)
    ↓ Validate schema + Publish
Redpanda (:19092)
    ↓ Batch consume (500 events OR 20ms)
Consumer Workers (4 parallel)
    ↓ Auto-partition + COPY protocol
PostgreSQL (:5433, partitioned by tenant_id)
```

**Key Design Decisions:**
- **API Server validates before Kafka**: Prevents invalid data from entering the queue
- **Multi-tenancy via API keys**: Each key maps to a tenant, enforced server-side
- **Auto-partitioning**: Tables like `events_user_signup_tenant_alpha` created automatically
- **COPY protocol**: Bulk inserts 3-4x faster than individual INSERTs

---

## Using IngestKit

### Python Example

```python
from ingestkit import Client

# Initialize client
analytics = Client(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890"
)

# Send events (type-safe!)
analytics.send_user_signup({
    "user_id": "usr_123",
    "email": "user@example.com",
    "signup_source": "web"
})

# Batch sending
analytics.send_user_signup_batch([event1, event2, event3])

# Guaranteed delivery (waits for Kafka ACK)
response = analytics.send_user_signup(event, sync=True)
```

### TypeScript Example

```typescript
import { IngestKitClient } from './ingestkit';

const client = new IngestKitClient({
  apiUrl: 'http://localhost:8080',
  apiKey: 'dev_key_1234567890'
});

// Send event
await client.sendUserSignup({
  userId: 'usr_123',
  email: 'user@example.com',
  signupSource: 'web'
});

// Guaranteed delivery
await client.sendUserSignup(event, { sync: true });
```

---

## Schema Management

### Define Events

Edit `schema/events.yaml`:

```yaml
version: "1.0"

events:
  user_signup:
    description: "Fired when a new user signs up"
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

### Making Schema Changes

**With Docker (simplest)**:

```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Apply changes (auto-generates migrations, rebuilds, restarts)
make docker-reload
```

**Manual workflow** (for more control):

```bash
# 1. Edit schema
vim schema/events.yaml

# 2. Auto-generate migration SQL with Atlas
make migrate-auto NAME=add_user_country

# 3. Review generated SQL
cat migrations/*add_user_country.up.sql

# 4. Apply and restart
make db-migrate-up
make docker-reload  # Docker
# OR
make generate && make build && restart services  # Local
```

**See**: [docs/automated-migrations.md](docs/automated-migrations.md)

---

## Common Commands

### Quick Start

```bash
make start           # ⚡ Start everything with Docker (one command!)
make start-local     # ⚡ Setup for local development
make test-event      # Send a test event
make down            # Stop everything
```

### Development

```bash
make up              # Start PostgreSQL + Redpanda
make generate        # Generate code from schema
make build           # Build API + consumer binaries
make run-api         # Run API server (:8080)
make run-consumer    # Run consumer (:8081)
```

### Database

```bash
make db-migrate-up        # Apply migrations
make db-migrate-down      # Rollback migration
make db-migrate-version   # Show current version
make db-connect           # Connect with psql
make db-stats             # Show event counts
make db-event-counts      # Count by event type
```

### Docker Deployment

```bash
make docker-build    # Build Docker images
make docker-up       # Start full stack (includes migrations)
make docker-reload   # Zero-downtime rolling update
```

### Testing

```bash
make test            # Run unit tests
make loadtest-quick  # 100 RPS smoke test
make metrics         # View consumer metrics
```

---

## Services & Ports

| Service | Port | Purpose |
|---------|------|---------|
| API Server | 8080 | Event ingestion endpoint |
| Consumer Metrics | 8081 | Prometheus `/metrics` |
| PostgreSQL | 5433 | Database (non-standard port to avoid conflicts) |
| Redpanda | 19092 | Kafka protocol |
| Redpanda Console | 8090 | Web UI for Kafka |

---

## Working Examples

**Blog Analytics** (Python + Flask):
```bash
cd examples/blog-flask
./run.sh
./test-flow-simple.sh  # In another terminal
```

**E-commerce** (TypeScript + Express):
```bash
cd examples/ecommerce-express
npm install
npm start
./test-flow.sh  # In another terminal
```

---

## Performance

**Validated Performance** (measured with loadtest):
- **Throughput**: 15,600 events/sec per consumer worker
- **Latency**: 13ms average, <20ms p95
- **Success Rate**: 100% (zero data loss in testing)

**Why so fast?**
- PostgreSQL COPY protocol (bulk inserts)
- Batch processing (500 events OR 20ms timeout)
- Async Kafka publishing (10ms response time)
- Connection pooling (50 max connections)

See: [LOADTEST.md](LOADTEST.md)

---

## Documentation

- **[CLAUDE.md](CLAUDE.md)** - Quick reference for developers and AI assistants
- **[docs/development.md](docs/development.md)** - Development workflow
- **[docs/automated-migrations.md](docs/automated-migrations.md)** - Prisma-style migrations with Atlas
- **[docs/migrations.md](docs/migrations.md)** - Manual migration workflow
- **[docs/docker-deployment.md](docs/docker-deployment.md)** - Zero-downtime deployments
- **[docs/sdk-generation.md](docs/sdk-generation.md)** - SDK generation guide
- **[LOADTEST.md](LOADTEST.md)** - Performance testing guide

---

## What's Working

✅ **Schema-driven code generation** (SQL, Go models, SDKs)
✅ **Automated migrations** (Atlas + golang-migrate)
✅ **High-performance API** (15,600 events/sec tested)
✅ **Multi-tenant partitioning** (auto-creates tables per tenant)
✅ **Zero data loss** (at-least-once delivery)
✅ **Type-safe SDKs** (Python, TypeScript)
✅ **Dead letter queue** (failed event recovery)
✅ **Docker deployments** (zero-downtime rolling updates)
✅ **Working examples** (blog, e-commerce)

---

## What's Missing

⚠️ **Query API**: You can write events but not read them (no GET endpoints yet)
⚠️ **Aggregations**: No built-in analytics (use external tools like Metabase)
⚠️ **Distributed rate limiting**: Current rate limiter is single-server only
⚠️ **Data retention policies**: Events stored forever (no auto-cleanup)
⚠️ **Secrets management**: API keys in environment variables (no Vault integration)
⚠️ **Audit logging**: No tracking of who accessed what

**For production use**: Pair IngestKit with Metabase, Superset, or Redash for querying and visualization.

---

## Configuration

### Environment Variables

Create `.env` in project root:

```bash
# API Server
API_PORT=8080
API_KEY_1=dev_key_1234567890:default
API_KEY_2=sk_test_tenant_alpha:tenant_alpha
API_KEY_3=dev_key_ecommerce:ecommerce-demo

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500
CONSUMER_BATCH_TIMEOUT_MS=20

# Database
POSTGRES_PORT=5433
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev
POSTGRES_DB=ingestkit

# Kafka
REDPANDA_ADDR=localhost:19092
REDPANDA_TOPIC=ingestkit.events
```

---

## Troubleshooting

**Consumer not processing?**
```bash
curl http://localhost:8081/health
make metrics
make db-dlq-check  # Check dead letter queue
```

**401 Unauthorized?**
- Check API key is in `.env`
- Restart API: `make run-api`
- Verify: API logs should show "count=3" (or your key count)

**Validation errors?**
- Field names must be `snake_case` (not camelCase)
- Regenerate: `make generate && make build`

**Database connection failed?**
- PostgreSQL is on port **5433** (not 5432)
- Check: `make db-connect`

---

## Contributing

See [docs/development.md](docs/development.md) for development workflow.

---

## License

TBD

---

**Built with:**
- Go 1.24+ (Fiber web framework)
- PostgreSQL (with COPY protocol)
- Redpanda (Kafka-compatible)
- Atlas (migration generation)
- Docker (deployment)
