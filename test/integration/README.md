# Integration Tests

End-to-end integration tests for IngestKit that verify the full pipeline: API → Redpanda → Consumer → Database.

## Prerequisites

- Docker and Docker Compose
- Go 1.24+
- IngestKit services running (API, Consumer, PostgreSQL, Redpanda)

## Running Integration Tests

### 1. Start IngestKit Services

```bash
# From project root
make up        # Start PostgreSQL and Redpanda
make db-create # Create database schema
make generate  # Generate code from schema
make build     # Build API and consumer binaries
```

### 2. Start API and Consumer

In separate terminals:

```bash
# Terminal 1: API Server
make run-api

# Terminal 2: Consumer Worker
make run-consumer
```

### 3. Run Integration Tests

```bash
# Run integration tests
INTEGRATION_TEST=true go test -v -tags=integration ./test/integration/...
```

## What's Tested

The integration test suite covers:

1. **Single Event Ingestion**
   - Send single event via API
   - Verify event reaches database
   - Validate data integrity

2. **Batch Event Ingestion**
   - Send batch of 10 events via API
   - Verify all events reach database
   - Validate batch processing

3. **Validation Error Handling**
   - Missing required fields
   - Invalid enum values
   - Unknown event types

## Test Flow

```
┌─────────────────────────────────────────────────────────┐
│                    Integration Test                     │
└─────────────────────────────────────────────────────────┘
              │
              ▼
    ┌─────────────────┐
    │  Wait for API   │  (health check polling)
    └─────────────────┘
              │
              ▼
    ┌─────────────────┐
    │  Send Event(s)  │  (HTTP POST with Bearer auth)
    └─────────────────┘
              │
              ▼
    ┌─────────────────┐
    │  Wait for       │  (sleep for batch timeout + DB write)
    │  Processing     │
    └─────────────────┘
              │
              ▼
    ┌─────────────────┐
    │  Query Database │  (verify event data)
    └─────────────────┘
              │
              ▼
    ┌─────────────────┐
    │  Assert Results │
    └─────────────────┘
```

## Configuration

Tests use these defaults (matching `.env.example`):

- **API URL**: `http://localhost:8080`
- **API Key**: `dev_key_1234567890`
- **Database**: `postgres://ingestkit:ingestkit_dev@localhost:5433/ingestkit`

To customize, modify constants in `e2e_test.go`:

```go
const (
    apiURL = "http://localhost:8080"
    apiKey = "dev_key_1234567890"
    dbConnStr = "postgres://..."
)
```

## Troubleshooting

### Tests Skip Automatically

If you see "Skipping integration test":
- Ensure `INTEGRATION_TEST=true` is set
- Run: `INTEGRATION_TEST=true go test -v -tags=integration ./test/integration/...`

### API Not Ready

If test fails with "API did not become ready in time":
- Verify API is running: `curl http://localhost:8080/health`
- Check API logs for errors
- Ensure port 8080 is not blocked

### Database Connection Errors

If test fails with database errors:
- Verify PostgreSQL is running: `docker-compose ps postgres`
- Check database exists: `make db-connect`
- Verify connection string matches your `.env` file

### Events Not Reaching Database

If events don't appear in database:
- Check consumer is running and processing events
- View consumer metrics: `make metrics`
- Check consumer logs for errors
- Verify Redpanda is running: `docker-compose ps redpanda`

## Adding More Tests

To add new integration tests:

1. Create a new test function in `e2e_test.go`:
```go
func TestMyNewFeature(t *testing.T) {
    if os.Getenv("INTEGRATION_TEST") != "true" {
        t.Skip("Skipping integration test")
    }

    // Your test logic here
}
```

2. Or add a new sub-test to `TestEndToEnd`:
```go
t.Run("MyNewFeature", func(t *testing.T) {
    testMyNewFeature(t, db)
})
```

## CI/CD Integration

For CI/CD pipelines, use a script like:

```bash
#!/bin/bash
set -e

# Start services
docker-compose up -d
make db-create
make generate
make build

# Start API and consumer in background
./bin/api &
API_PID=$!
./bin/consumer &
CONSUMER_PID=$!

# Run integration tests
INTEGRATION_TEST=true go test -v -tags=integration ./test/integration/...

# Cleanup
kill $API_PID $CONSUMER_PID
docker-compose down
```
