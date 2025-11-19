# Development Guide

Complete guide for developing and extending IngestKit.

Last updated: 2025-11-15

---

## Table of Contents

- [Development Environment Setup](#development-environment-setup)
- [Project Structure](#project-structure)
- [Development Workflow](#development-workflow)
- [Adding New Event Types](#adding-new-event-types)
- [Schema Management](#schema-management)
- [Testing](#testing)
- [Building and Deployment](#building-and-deployment)

---

## Development Environment Setup

### Prerequisites

- **Go 1.24+** (tested with 1.25) - Backend services
- **Docker & Docker Compose** - Infrastructure (PostgreSQL, Redpanda)
- **Make** - Build automation
- **Optional**: Python 3.9+ and Node.js 18+ for SDK development

### Initial Setup

```bash
# Clone the repository
git clone <repo-url>
cd ingestkit

# Start infrastructure
make up

# Build IngestKit CLI and services
make build

# Generate code from schema
make generate

# Create database tables
make db-create

# Run the API server
make run-api

# In another terminal, run the consumer
make run-consumer
```

### Environment Configuration

Create `.env` file in project root:

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
POSTGRES_HOST=localhost
POSTGRES_PORT=5433
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev
POSTGRES_DB=ingestkit

# Kafka
REDPANDA_ADDR=localhost:19092
REDPANDA_TOPIC=ingestkit.events
```

---

## Project Structure

```
ingestkit/
├── cmd/                      # Main applications
│   ├── api/                  # HTTP API server
│   ├── consumer/             # Kafka consumer
│   └── cli/                  # IngestKit CLI tool
├── internal/                 # Private application code
│   ├── api/                  # API handlers and middleware
│   │   └── middleware/       # Auth, CORS, rate limiting
│   ├── messaging/            # Kafka producer/consumer
│   ├── schema/               # Schema parsing and generators
│   │   └── templates/        # Code generation templates
│   ├── storage/              # PostgreSQL operations
│   │   ├── partitions.go     # Auto-partition management
│   │   └── dlq.go            # Dead letter queue
│   └── validation/           # Event validation
├── generated/                # Auto-generated code (gitignored)
│   ├── sql/                  # SQL DDL
│   ├── models/               # Go structs
│   ├── storage/              # COPY protocol writers
│   ├── consumer/             # Event handlers
│   └── sdk/                  # Client SDKs
│       ├── python/           # Python SDK
│       └── typescript/       # TypeScript SDK
├── schema/                   # Schema definitions
│   └── events.yaml           # Single source of truth
├── init-db/                  # Database initialization
│   └── 01-init.sql           # Metadata tables
├── examples/                 # Example applications
│   ├── blog-flask/           # Python/Flask blog analytics
│   └── ecommerce-express/    # TypeScript/Express e-commerce
└── docs/                     # Documentation
```

---

## Development Workflow

### Typical Development Cycle

1. **Edit Schema**: Modify `schema/events.yaml`
2. **Generate Code**: Run `make generate`
3. **Update Database**: Run `make db-create` (creates new columns/tables)
4. **Build Services**: Run `make build`
5. **Restart Services**: Restart API and consumer
6. **Test Changes**: Use examples or curl

### Hot Reload During Development

```bash
# Terminal 1: Watch schema and regenerate
while true; do
  inotifywait -e modify schema/events.yaml
  make generate && make build
done

# Terminal 2: API server (restart manually after rebuild)
make run-api

# Terminal 3: Consumer (restart manually after rebuild)
make run-consumer
```

### Debugging Tips

**Check API Server Logs:**
```bash
# API logs show validation errors, authentication issues
make run-api

# Look for:
# ✓ Event types, API key count, rate limit settings
# ✗ Schema validation failures, auth errors
```

**Check Consumer Logs:**
```bash
# Consumer logs show batch processing, partition creation
make run-consumer

# Look for:
# 📦 Batch: N events
# ✅ Wrote N event_type events
# ⚠️ Errors in processing or DLQ writes
```

**Check Database State:**
```bash
# Connect to database
make db-connect

# Check event counts
SELECT
  tablename,
  (SELECT COUNT(*) FROM events_user_signup) as count
FROM pg_tables
WHERE tablename LIKE 'events_%';

# Check DLQ
SELECT * FROM ingestkit_meta.dead_letter_queue;
```

**Check Kafka Messages:**
```bash
# List topics
docker exec -it redpanda rpk topic list

# Consume from topic
docker exec -it redpanda rpk topic consume ingestkit.events
```

---

## Adding New Event Types

### Step 1: Update Schema

Edit `schema/events.yaml`:

```yaml
events:
  # ... existing events ...

  new_event_name:
    description: "Description of your event"
    fields:
      field_name:
        type: string
        required: true
        description: "Field description"

      optional_field:
        type: integer
        required: false
        description: "Optional field"
```

### Step 2: Generate Code

```bash
make generate
```

This generates:
- SQL DDL in `generated/sql/schema.sql`
- Go models in `generated/models/events.go`
- Storage writers in `generated/storage/writer.go`
- Consumer handlers in `generated/consumer/handler.go`

### Step 3: Update Database

```bash
make db-create
```

This executes the generated SQL to create tables.

### Step 4: Regenerate SDKs (Optional)

```bash
# Python SDK
./bin/ingestkit sdk generate --lang python --api-url http://localhost:8080

# TypeScript SDK
./bin/ingestkit sdk generate --lang typescript --api-url http://localhost:8080
```

### Step 5: Test New Event

```bash
curl -X POST http://localhost:8080/v1/events/new_event_name \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "field_name": "value",
    "optional_field": 123
  }'
```

---

## Schema Management

### Schema Version Control

The schema version is defined in `schema/events.yaml`:

```yaml
version: "1.0"
```

### Schema Evolution Best Practices

1. **Additive Changes Only** (for backwards compatibility):
   - Add new optional fields
   - Add new event types
   - Don't remove or rename fields

2. **Breaking Changes** (requires version bump):
   - Change field types
   - Make optional fields required
   - Remove fields
   - Rename fields

3. **Version Bump Process:**
   ```yaml
   version: "2.0"  # Increment version
   ```

   Then regenerate everything:
   ```bash
   make generate
   make db-create
   make build
   ```

### Fetching Schema from Server

Clients can fetch the current schema:

```bash
# Fetch schema with ETag support
curl -i http://localhost:8080/schema

# With caching
curl -H "If-None-Match: <previous-etag>" http://localhost:8080/schema
```

### Pushing Schema Updates (Supabase-style)

**⚠️  Important: Schema push requires admin privileges and manual reload**

Schema push is protected by a dedicated `ADMIN_SCHEMA_KEY` environment variable. This endpoint is NOT available to regular API keys - only to platform administrators.

**Setup Admin Key (Server)**

Add to your server `.env`:
```bash
ADMIN_SCHEMA_KEY=admin_secret_key_here
```

**Method 1: Using IngestKit CLI (Recommended)**

```bash
# Push schema to local server (requires admin key)
ingestkit schema push --api-key admin_secret_key_here

# Push schema to remote server
ingestkit schema push \
  --api-url https://api.ingestkit.com \
  --api-key admin_secret_key_here

# Or use environment variables
export INGESTKIT_API_URL=https://api.ingestkit.com
export INGESTKIT_API_KEY=admin_secret_key_here
ingestkit schema push
```

**Method 2: Using curl**

```bash
# Push updated schema to server (requires admin key)
curl -X POST http://localhost:8080/v1/schema/push \
  -H "Authorization: Bearer admin_secret_key_here" \
  -H "Content-Type: application/x-yaml" \
  --data-binary @schema/events.yaml
```

**What Happens After Push**

Server will:
- ✅ Validate the schema before accepting
- ✅ Create automatic backup (e.g., `schema/events.yaml.backup.1731672000`)
- ✅ Update schema file
- ✅ Return validation results with event count

**⚠️  IMPORTANT: Manual Steps Required**

The schema push **does NOT automatically reload** the server or regenerate code. You must:

**Method 1: Using Makefile (Recommended)**

```bash
# Apply all schema changes (generate + build + db-create)
make schema-apply

# Then restart services:
make run-api      # In one terminal
make run-consumer # In another terminal
```

**Method 2: Manual Steps**

```bash
# 1. Regenerate code from new schema
make generate

# 2. Rebuild binaries
make build

# 3. Apply database migrations
make db-create

# 4. Restart API server
# Stop current API and run:
make run-api

# 5. Restart Consumer
# Stop current consumer and run:
make run-consumer
```

This manual workflow is intentional for safety - hot-reloading schemas in production could cause data inconsistencies or downtime.

---

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific package
go test ./internal/validation/...
```

### Integration Tests

```bash
# Start infrastructure
make up

# Run integration tests
go test -tags=integration ./tests/integration/...
```

### Load Testing

```bash
# Quick load test (100 RPS for 10s)
make loadtest-quick

# Full load test (1000 RPS for 60s)
make loadtest
```

### Manual Testing with Examples

```bash
# Blog example (Python)
cd examples/blog-flask
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python main.py

# E-commerce example (TypeScript)
cd examples/ecommerce-express
npm install
npm start
```

---

## Building and Deployment

### Building Binaries

```bash
# Build all binaries (api, consumer, ingestkit CLI)
make build

# Build individual components
go build -o bin/api ./cmd/api
go build -o bin/consumer ./cmd/consumer
go build -o bin/ingestkit ./cmd/cli
```

### Docker Deployment

```bash
# Build Docker images
docker build -t ingestkit-api -f Dockerfile.api .
docker build -t ingestkit-consumer -f Dockerfile.consumer .

# Run with Docker Compose
docker-compose up -d
```

### Production Checklist

- [ ] Set production API keys (not dev_key_*)
- [ ] Configure appropriate rate limits
- [ ] Set CONSUMER_WORKERS based on load
- [ ] Enable connection pooling
- [ ] Configure log levels (not DEBUG)
- [ ] Set up monitoring (Prometheus /metrics endpoints)
- [ ] Configure TLS for API
- [ ] Set up Redpanda cluster (not single node)
- [ ] Configure PostgreSQL replication
- [ ] Set up automated backups
- [ ] Configure DLQ alerting

### Environment Variables (Production)

```bash
# API Server
API_PORT=8080
LOG_LEVEL=info
RATE_LIMIT_RPS=1000

# Consumer
CONSUMER_WORKERS=8  # Scale based on load
CONSUMER_BATCH_SIZE=500
CONSUMER_BATCH_TIMEOUT_MS=20

# Database
DATABASE_URL=postgres://user:pass@postgres:5432/ingestkit?sslmode=require

# Kafka
KAFKA_BROKERS=kafka1:9092,kafka2:9092,kafka3:9092
KAFKA_TOPIC=ingestkit.events
```

---

## Common Development Tasks

### Resetting the Database

```bash
# ⚠️  Warning: Deletes all data
make db-reset
```

### Checking Database Statistics

```bash
make db-stats
```

### Viewing Prometheus Metrics

```bash
# API metrics
curl http://localhost:8080/metrics

# Consumer metrics
curl http://localhost:8081/metrics
```

### Adding a New Middleware

1. Create middleware in `internal/api/middleware/`
2. Add to middleware chain in `cmd/api/main.go`
3. Test with integration tests

Example:
```go
// internal/api/middleware/custom.go
func CustomMiddleware() fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Middleware logic
        return c.Next()
    }
}

// cmd/api/main.go
app.Use(middleware.CustomMiddleware())
```

---

## Troubleshooting

### Code Generation Issues

**Problem**: Generated code doesn't match schema

**Solution**:
```bash
# Clean and regenerate
rm -rf generated/
make generate
make build
```

### Database Migration Issues

**Problem**: Schema changed but tables not updated

**Solution**:
```bash
# For development (⚠️  loses data)
make db-reset
make db-create

# For production: Write manual migration
psql $DATABASE_URL -f migrations/001_add_column.sql
```

### Import Path Issues

**Problem**: Import errors after renaming

**Solution**:
```bash
# Update go.mod module path
go mod edit -module github.com/yourorg/ingestkit
find . -name "*.go" -exec sed -i 's|old/import/path|new/import/path|g' {} +
go mod tidy
```

---

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes
4. Add tests
5. Run `make test`
6. Submit PR

---

## Additional Resources

- [Docker Deployment](docker-deployment.md)
- [Load Testing](load-testing.md)
- [Automated Migrations](automated-migrations.md)
