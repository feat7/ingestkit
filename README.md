# IngestKit

**High-Performance, Schema-First Data Ingestion Platform**

Self-hosted data ingestion platform for collecting high-volume events with type safety, reliability guarantees, and auto-generated SDKs.

## Features

- **Schema-First:** Define your events in YAML, get type-safe APIs and SDKs
- **High Performance:** 10k-30k events/second on modest hardware
- **Zero Data Loss:** Durable message broker buffering with Redpanda
- **Developer-Friendly:** Auto-generated clients for Go, Python, and more
- **Self-Hosted:** Full control over your data and infrastructure
- **Query API:** Fast queries on normalized PostgreSQL schema
- **Modern Stack:** PostgreSQL 18 with 3x I/O performance, Go with Fiber, Redpanda

## Tech Stack

- **Database:** PostgreSQL 18 (latest, released Sept 2025)
  - Up to 3x faster I/O with async operations
  - Built-in UUID v7 for timestamp-ordered event IDs
  - Parallel GIN index builds for JSONB
  - Better query optimization
- **API Framework:** Go with Fiber (high-performance, Express.js-like)
- **Message Broker:** Redpanda (Kafka-compatible, simpler operations)
- **Deployment:** Docker Compose (development), Kubernetes-ready

## Quick Start

### Prerequisites

- Docker & Docker Compose
- Make (optional, but recommended)
- Go 1.21+ (for development)

### 1. Clone and Setup

```bash
# Navigate to project directory
cd ingestkit

# Quick start (creates .env, starts services, creates topic)
make quickstart
```

This will:
- Copy `.env.example` to `.env`
- Start PostgreSQL and Redpanda
- Create the `ingestkit.events` topic
- Start Redpanda Console UI

### 2. Access Services

Once running, you can access:

- **Redpanda Console:** http://localhost:8090 (view topics, messages)
- **PostgreSQL:** `localhost:5432` (use `make db-connect` to connect)
- **Redpanda (Kafka API):** `localhost:19092`

### 3. Initialize Go Project (For Development)

```bash
# Initialize Go modules and install dependencies
make init-go

# Create project structure
mkdir -p cmd/api cmd/consumer cmd/cli internal pkg schema
```

### 4. Define Your Schema

Create `schema/events.yaml`:

```yaml
events:
  user_signup:
    fields:
      user_id: { type: string, required: true, indexed: true }
      email: { type: string, required: true }
      signup_source: { type: string, values: [web, mobile, api] }
      metadata: { type: jsonb }

  purchase:
    fields:
      user_id: { type: string, required: true, indexed: true }
      order_id: { type: string, required: true }
      amount: { type: decimal, required: true }
      currency: { type: string, default: "USD" }
      items: { type: jsonb }
```

### 5. Generate Code (Coming Soon)

```bash
# Generate SQL, Go structs, and SDKs from schema
make generate

# Apply database schema
make db-create
```

### 6. Build and Run

```bash
# Build applications
make build

# Run API server (in one terminal)
make run-api

# Run consumer worker (in another terminal)
make run-consumer
```

## Makefile Commands

### Infrastructure

```bash
make up              # Start PostgreSQL and Redpanda
make up-full         # Start all services (includes Redis, pgAdmin)
make down            # Stop all services
make restart         # Restart all services
make logs            # View all logs
make status          # Show service status
make health          # Check service health
```

### Database

```bash
make db-connect      # Connect to PostgreSQL with psql
make db-create       # Create database schema
make db-drop         # Drop all tables (DANGEROUS)
make db-reset        # Drop and recreate database
```

### Redpanda

```bash
make redpanda-topics        # List all topics
make redpanda-create-topic  # Create ingestkit.events topic
make redpanda-consume       # Consume messages from topic
make redpanda-info          # Show cluster info
```

### Development

```bash
make build           # Build all Go applications
make run-api         # Run API server
make run-consumer    # Run consumer worker
make test            # Run unit tests
make fmt             # Format Go code
make lint            # Lint Go code
```

### Utilities

```bash
make clean           # Clean up containers, volumes, and build artifacts
make setup           # Initial setup (create .env)
make quickstart      # Complete setup and start everything
make help            # Show all available commands
```

## Architecture

```
Client Apps
    ↓ (HTTP POST)
Go API (Fiber)
    ↓ (validate & publish)
Redpanda (buffer)
    ↓ (consume)
Consumer Worker
    ↓ (write)
PostgreSQL 18 (storage)
    ↓ (query)
Query API
```

### Components

1. **Schema Compiler:** Generates SQL DDL, Go structs, and SDKs from YAML
2. **Ingestion API:** HTTP API for event ingestion with validation
3. **Consumer Workers:** Process events from Redpanda and write to PostgreSQL
4. **Query API:** Query stored events with filtering and aggregation

## Development

### Project Structure

```
ingestkit/
├── cmd/
│   ├── api/              # API server
│   ├── consumer/         # Consumer worker
│   └── cli/              # CLI tools
├── internal/             # Internal packages
│   ├── api/
│   ├── consumer/
│   ├── schema/
│   └── storage/
├── pkg/                  # Public packages
│   └── client/           # Go SDK
├── schema/               # Schema definitions
│   └── events.yaml
├── generated/            # Auto-generated code
│   ├── sql/
│   ├── models/
│   └── sdks/
├── docker-compose.yml
├── Makefile
└── README.md
```

### Testing

```bash
# Unit tests
make test

# Integration tests (requires services running)
make test-integration

# Load tests
make test-load
```

## Configuration

Environment variables (`.env` file):

```bash
# PostgreSQL
POSTGRES_DB=ingestkit
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev

# API
API_PORT=8080
API_KEY_1=your_api_key_here

# Consumer
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=100

# See .env.example for all options
```

## Usage Example

### API Ingestion

```bash
# Single event
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "usr_123",
    "email": "user@example.com",
    "signup_source": "web"
  }'

# Batch ingestion
curl -X POST http://localhost:8080/v1/events/user_signup/batch \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "events": [
      {"user_id": "usr_123", "email": "user1@example.com", "signup_source": "web"},
      {"user_id": "usr_124", "email": "user2@example.com", "signup_source": "mobile"}
    ]
  }'
```

### Using Go SDK (Auto-Generated)

```go
package main

import (
    "github.com/yourusername/ingestkit/pkg/client"
    "github.com/yourusername/ingestkit/generated/models"
)

func main() {
    // Create client
    c := client.New("your-api-key", "http://localhost:8080")

    // Track event (type-safe!)
    err := c.TrackUserSignup(models.UserSignupEvent{
        UserID:       "usr_123",
        Email:        "user@example.com",
        SignupSource: "web",
    })

    if err != nil {
        panic(err)
    }
}
```

### Querying Events

```bash
# Query events
curl http://localhost:8080/v1/events/user_signup?limit=100

# Filter by timestamp
curl http://localhost:8080/v1/events/user_signup?start=2025-01-01&end=2025-01-31

# Filter by indexed field
curl http://localhost:8080/v1/events/user_signup?user_id=usr_123
```

## Roadmap

### Phase 0: POC (Week 1)
- [x] Docker Compose setup
- [x] Makefile commands
- [ ] Schema tooling prototype
- [ ] Performance benchmarks
- [ ] End-to-end spike

### Phase 1: MVP Core (Weeks 2-3)
- [ ] Schema compiler (YAML → SQL + Go)
- [ ] Ingestion API with validation
- [ ] Consumer worker
- [ ] Query API
- [ ] Go SDK generator

### Phase 2: Developer Experience (Week 3-4)
- [ ] Python SDK generator
- [ ] Documentation
- [ ] Example application
- [ ] Load testing

### Phase 3: Production Readiness
- [ ] Observability (metrics, logging)
- [ ] Schema migrations
- [ ] Advanced querying
- [ ] Multi-tenancy
- [ ] S3 archival

## Performance Targets

- **Throughput:** 10k-30k events/second (boosted by PostgreSQL 18's 3x I/O improvements)
- **Latency:** Sub-100ms p95 ingestion
- **Reliability:** Zero data loss under normal failures (Redpanda buffering)
- **Resource Usage:** 2-4GB RAM for API/Consumer
- **Query Performance:** 5-10x faster than JSONB-only solutions (normalized schema + PG18 optimizations)

## Contributing

See `PLAN.md` for detailed development plan and architecture decisions.

## License

TBD

## Support

For issues, questions, or contributions, please open an issue on GitHub.

---

**Status:** Early development (POC phase)

See `PLAN.md` for detailed roadmap and current progress.
