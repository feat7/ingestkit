# IngestKit

**High-Performance, Schema-First Event Ingestion Platform**

Self-hosted event tracking with type-safe SDKs, automated migrations, and zero data loss guarantees.

## What is IngestKit?

IngestKit is an event ingestion platform for developers who need:
- **Type Safety**: Define events in YAML, get type-safe SDKs automatically
- **Performance**: 15,600 events/sec with PostgreSQL COPY protocol
- **Reliability**: At-least-once delivery, dead letter queue, automatic retries
- **Developer Experience**: Prisma-style migrations, auto-generated code

**Use Cases**: Product analytics, audit logging, user behavior tracking, event streaming

---

## Getting Started

Choose your path:

### Option A: Self-Host IngestKit (Server Operators)

Run the complete IngestKit platform on your infrastructure.

```bash
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
make start
```

This builds and starts everything: PostgreSQL, Redpanda, API server, and consumer.

[Continue to Self-Hosting Guide →](#self-hosting-ingestkit)

### Option B: Use IngestKit SDKs (Application Developers)

Send events to an existing IngestKit server.

```bash
pip install ingestkit   # or: npm install -g ingestkit

cd my-app
ingestkit init --python
ingestkit generate --schema-url https://your-ingestkit-server.com/schema
```

[Continue to SDK Guide →](#using-ingestkit-sdks)

---

## Self-Hosting IngestKit

### Prerequisites

- Docker & Docker Compose
- Make
- Go 1.24+ (for local development)

### Quick Start

```bash
# Start everything with Docker
make start

# Verify it's working
make test-event
make metrics
make db-event-counts
```

### Local Development

```bash
# Start infrastructure only
make up

# Build and run locally
make build
make run-api        # Terminal 1
make run-consumer   # Terminal 2
```

### Architecture

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

### Schema Management

Define events in `schema/events.yaml`:

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
```

Apply changes:

```bash
# With Docker
make docker-reload

# Manual workflow
make migrate-auto NAME=add_user_country
make db-migrate-up
make generate && make build
```

### Common Commands

```bash
# Quick Start
make start           # Start everything with Docker
make start-local     # Setup for local development
make test-event      # Send a test event
make down            # Stop everything

# Development
make generate        # Generate code from schema
make build           # Build binaries

# Database
make db-migrate-up   # Apply migrations
make db-connect      # Connect with psql
make db-stats        # Show statistics

# Docker
make docker-reload   # Zero-downtime update
```

### Services & Ports

| Service | Port | Purpose |
|---------|------|---------|
| API Server | 8080 | Event ingestion endpoint |
| Consumer Metrics | 8081 | Prometheus `/metrics` |
| PostgreSQL | 5433 | Database |
| Redpanda | 19092 | Kafka protocol |
| Redpanda Console | 8090 | Web UI |

---

## Using IngestKit SDKs

### Installation

```bash
# Python
pip install ingestkit

# Node.js
npm install -g ingestkit
```

### Initialize Project

```bash
cd my-app
ingestkit init --python    # or --typescript
```

This creates:
```
my-app/
├── ingestkit/
│   ├── schema.yaml    # Define your events
│   └── client.py      # Generated (after ingestkit generate)
└── ingestkit.config.json
```

### Generate Client

```bash
# From local schema
ingestkit generate

# From server schema
ingestkit generate --schema-url https://your-server.com/schema
```

### Use in Code

**Python:**
```python
from ingestkit import Client

client = Client()
client.user_signup.send(
    user_id="123",
    email="user@example.com",
    signup_source="web"
)
```

**TypeScript:**
```typescript
import { Client } from './ingestkit'

const client = new Client()
await client.userSignup.send({
  userId: "123",
  email: "user@example.com",
  signupSource: "web"
})
```

---

## Performance

Validated benchmarks:
- **Throughput**: 15,600 events/sec per consumer
- **Latency**: 13ms average, <20ms p95
- **Success Rate**: 100% (zero data loss)

Why it's fast:
- PostgreSQL COPY protocol (bulk inserts)
- Batch processing (500 events OR 20ms timeout)
- Async Kafka publishing

See: [docs/load-testing.md](docs/load-testing.md)

---

## Examples

**Blog Analytics** (Python + Flask):
```bash
cd examples/blog-flask
./run.sh
```

**E-commerce** (TypeScript + Express):
```bash
cd examples/ecommerce-express
./run.sh
```

---

## Documentation

- [docs/development.md](docs/development.md) - Development workflow
- [docs/automated-migrations.md](docs/automated-migrations.md) - Prisma-style migrations
- [docs/migrations.md](docs/migrations.md) - Manual migration workflow
- [docs/docker-deployment.md](docs/docker-deployment.md) - Zero-downtime deployments
- [docs/sdk-generation.md](docs/sdk-generation.md) - SDK generation guide
- [docs/load-testing.md](docs/load-testing.md) - Performance testing

---

## Current Status

**Working:**
- Schema-driven code generation (SQL, Go models, SDKs)
- Automated migrations with Atlas
- High-performance API (15,600 events/sec tested)
- Multi-tenant partitioning
- Zero data loss (at-least-once delivery)
- Type-safe SDKs (Python, TypeScript)
- Dead letter queue
- Docker deployments with zero-downtime updates

**Not Yet Implemented:**
- Query API (write-only currently)
- Built-in analytics (use Metabase/Superset)
- Distributed rate limiting
- Data retention policies
- Secrets management (Vault integration)

---

## Configuration

Create `.env` in project root:

```bash
# API Server
API_PORT=8080
API_KEY_1=dev_key_1234567890:default
API_KEY_2=sk_test_tenant_alpha:tenant_alpha

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500

# Database
POSTGRES_PORT=5433
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev
POSTGRES_DB=ingestkit

# Kafka
REDPANDA_ADDR=localhost:19092
```

---

## Troubleshooting

**Consumer not processing?**
```bash
curl http://localhost:8081/health
make metrics
make db-dlq-check
```

**401 Unauthorized?**
- Check API key in `.env`
- Restart API: `make run-api`

**Validation errors?**
- Field names must be `snake_case`
- Regenerate: `make generate && make build`

**Database connection failed?**
- PostgreSQL uses port **5433** (not 5432)

---

## Contributing

See [docs/development.md](docs/development.md) for development workflow.

---

## License

MIT

---

**Built with:** Go, PostgreSQL, Redpanda, Atlas, Docker
