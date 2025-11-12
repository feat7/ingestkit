# IngestKit Load Testing Guide

This guide covers how to perform load testing on IngestKit to measure performance, identify bottlenecks, and validate system capacity.

## Table of Contents

- [Quick Start](#quick-start)
- [Load Test Scenarios](#load-test-scenarios)
- [Adjusting Rate Limits](#adjusting-rate-limits)
- [Monitoring During Tests](#monitoring-during-tests)
- [Understanding Results](#understanding-results)
- [Advanced Configuration](#advanced-configuration)
- [Troubleshooting](#troubleshooting)

## Quick Start

### 1. Prerequisites

Ensure all services are running:

```bash
make up        # Start PostgreSQL and Redpanda
make build     # Build all applications including loadtest
make run-api   # Start API server (in separate terminal)
make run-consumer  # Start consumer worker (in separate terminal)
```

### 2. Run a Quick Test

```bash
make loadtest-quick
```

This runs a 10-second validation test at 100 RPS.

### 3. Run Baseline Test

```bash
make loadtest-baseline
```

This establishes a performance baseline at 500 RPS for 30 seconds.

## Load Test Scenarios

The following pre-configured scenarios are available via Makefile targets:

### Quick Validation
```bash
make loadtest-quick
```
- **Target:** 100 RPS
- **Duration:** 10 seconds
- **Workers:** 5
- **Use Case:** Quick validation that system is working

### Baseline
```bash
make loadtest-baseline
```
- **Target:** 500 RPS
- **Duration:** 30 seconds
- **Workers:** 10
- **Use Case:** Establish performance baseline

### Production Simulation
```bash
make loadtest-production
```
- **Target:** 1,000 RPS
- **Duration:** 1 minute
- **Workers:** 20
- **Use Case:** Simulate realistic production load
- **Note:** Matches default rate limit (1000 RPS)

### Batch Testing
```bash
make loadtest-batch
```
- **Target:** 100 RPS with 10 events/batch = 1,000 EPS
- **Duration:** 30 seconds
- **Workers:** 10
- **Use Case:** Test batch endpoint performance

### High Throughput
```bash
make loadtest-high
```
- **Target:** 5,000 RPS
- **Duration:** 30 seconds
- **Workers:** 50
- **Use Case:** Push system to higher loads
- **Requirements:** Set `RATE_LIMIT_RPS=10000` or higher

### Extreme Load
```bash
make loadtest-extreme
```
- **Target:** 10,000 RPS
- **Duration:** 1 minute
- **Workers:** 100
- **Use Case:** Maximum stress testing
- **Requirements:** Set `RATE_LIMIT_RPS=20000` or higher

### Sustained Load
```bash
make loadtest-sustained
```
- **Target:** 2,000 RPS
- **Duration:** 5 minutes
- **Workers:** 30
- **Use Case:** Test system stability over time
- **Requirements:** Set `RATE_LIMIT_RPS=5000` or higher

### Batch Heavy
```bash
make loadtest-batch-heavy
```
- **Target:** 50 RPS with 100 events/batch = 5,000 EPS
- **Duration:** 30 seconds
- **Workers:** 10
- **Use Case:** Test extreme batching scenarios

### Custom
```bash
RPS=2000 DURATION=60s WORKERS=30 BATCH=5 make loadtest-custom
```
- **Configuration:** Fully customizable via environment variables

## Adjusting Rate Limits

The default API rate limit is **1000 RPS per tenant**. For load tests exceeding this, you'll need to adjust the limit.

### Method 1: Environment Variable (Temporary)

Stop the API and restart with a higher limit:

```bash
# Stop current API
pkill -f "./bin/api"

# Start with higher rate limit
RATE_LIMIT_RPS=10000 ./bin/api
```

### Method 2: Update .env File (Persistent)

Edit `.env` file:

```bash
# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_RPS=10000  # <-- Change this value
```

Then restart the API:

```bash
make run-api
```

### Method 3: Disable Rate Limiting (Testing Only)

**WARNING: Only use this for load testing, never in production!**

Edit `.env`:

```bash
RATE_LIMIT_ENABLED=false
```

### Recommended Limits by Scenario

| Scenario | Recommended RATE_LIMIT_RPS |
|----------|---------------------------|
| Quick/Baseline | 1000 (default) |
| Production | 1000 (default) |
| Batch | 1000 (default) |
| High Throughput | 10000 |
| Extreme | 20000 |
| Sustained | 5000 |
| Batch Heavy | 10000 |

## Monitoring During Tests

### Real-time Metrics

**Consumer Metrics (Prometheus format):**
```bash
make metrics
```

**Consumer Health:**
```bash
make metrics-health
```

**Watch Metrics (auto-refresh every 2 seconds):**
```bash
make metrics-watch
```

### Database Statistics

**Table sizes and row counts:**
```bash
make db-stats
```

**Event counts by type:**
```bash
make db-event-counts
```

**Check dead letter queue:**
```bash
make db-dlq-check
```

### Multiple Terminal Setup

For comprehensive monitoring during load tests:

**Terminal 1:** Run load test
```bash
make loadtest-production
```

**Terminal 2:** Watch consumer metrics
```bash
make metrics-watch
```

**Terminal 3:** Watch database stats
```bash
watch -n 5 "make db-event-counts"
```

**Terminal 4:** Monitor consumer logs
```bash
tail -f /tmp/consumer-test.log
```

## Understanding Results

### Load Test Output

The load test tool provides detailed statistics:

```
📊 FINAL RESULTS
═══════════════════════════════════════════════════════════════
Duration:          30.5s
Total Requests:    15234
Total Events:      15234
Successful:        15150 (99.45%)
Failed:            84 (0.55%)

Requests/sec:      499.48
Events/sec:        499.48

Latency Distribution:
  min:    12ms
  p50:    45ms
  p75:    67ms
  p90:    89ms
  p95:    102ms
  p99:    145ms
  max:    234ms
  avg:    52ms

Error Breakdown:
  HTTP 429: 84  (rate limit exceeded)
═══════════════════════════════════════════════════════════════
```

### Key Metrics to Monitor

**1. Success Rate**
- **Target:** >99%
- **Issues if lower:** Rate limiting, service errors, database issues

**2. Latency Percentiles**
- **p50 (median):** Expected: 20-50ms
- **p95:** Expected: <100ms
- **p99:** Expected: <200ms
- **Issues if higher:** Database bottleneck, CPU saturation, network issues

**3. Throughput**
- **RPS:** Requests per second
- **EPS:** Events per second (RPS × batch size)
- **Compare to target:** Should match or be slightly below target RPS

**4. Consumer Metrics** (from `make metrics`)
- **ingestkit_events_processed_total:** Total events successfully written to database
- **ingestkit_events_failed_total:** Failed events (should be near zero)
- **ingestkit_events_dlq_total:** Events sent to dead letter queue
- **ingestkit_batch_latency_ms_avg:** Average time to process a batch
- **ingestkit_events_per_second:** Current processing rate

### Expected Performance

Based on Milestone 1.3 results:

- **Batch Processing Latency:** 15ms average
- **Connection Pool:** 50 max connections
- **Throughput:** 1000+ RPS sustainable
- **Success Rate:** >99% within rate limits

### Red Flags

🚨 **High failure rate (>1%):**
- Check if RATE_LIMIT_RPS is high enough
- Check database connection pool exhaustion
- Check for consumer errors in logs

🚨 **High p95/p99 latency (>200ms):**
- Database performance issues
- Connection pool saturation
- Consumer falling behind

🚨 **Events in DLQ:**
- Check DLQ with `make db-dlq-check`
- Review error messages
- Check consumer logs

🚨 **Consumer falling behind:**
- Check `ingestkit_events_per_second` vs target RPS
- Increase consumer workers (edit CONSUMER_WORKERS in .env)
- Increase database connection pool

## Advanced Configuration

### Custom Load Test Parameters

Run the load test binary directly for full control:

```bash
./bin/loadtest \
  -url http://localhost:8080 \
  -key dev_key_1234567890 \
  -rps 2000 \
  -duration 60s \
  -workers 30 \
  -batch 10 \
  -warmup 10s \
  -report 5s
```

**Parameters:**
- `-url`: API endpoint (default: http://localhost:8080)
- `-key`: API key for authentication
- `-rps`: Target requests per second
- `-duration`: Test duration (e.g., 30s, 1m, 2h)
- `-workers`: Number of concurrent workers
- `-batch`: Events per batch (1 = single events, >1 = batch endpoint)
- `-warmup`: Warmup time before measurements start
- `-report`: Progress report interval

### Scenario Configuration File

Scenarios are defined in `loadtest-scenarios.yaml` for reference. You can create custom configurations:

```yaml
scenarios:
  my-custom-test:
    description: "My custom load test"
    target_rps: 3000
    duration: 120s
    workers: 40
    batch_size: 5
    warmup: 15s
```

### Event Type Distribution

The load test generates events with equal distribution across all three types:
- `user_signup` (33%)
- `purchase` (33%)
- `page_view` (33%)

To test specific event types, you would need to modify `cmd/loadtest/main.go`.

## Troubleshooting

### Problem: High error rate (HTTP 429)

**Cause:** Rate limiting
**Solution:**
```bash
# Increase rate limit
RATE_LIMIT_RPS=10000 ./bin/api
```

### Problem: "Connection refused" errors

**Cause:** API server not running
**Solution:**
```bash
make run-api
```

### Problem: Events not appearing in database

**Cause:** Consumer not running or Redpanda not available
**Solution:**
```bash
# Check consumer is running
make run-consumer

# Check Redpanda
make redpanda-info

# Check consumer metrics
make metrics-health
```

### Problem: High latency (p99 > 500ms)

**Cause:** Database performance or connection pool exhaustion
**Solution:**
```bash
# Check database stats
make db-stats

# Check active connections
docker-compose exec postgres psql -U ingestkit -d ingestkit -c \
  "SELECT count(*) FROM pg_stat_activity WHERE datname='ingestkit';"

# Consider increasing connection pool in generated/storage/writer.go
# MaxOpenConns: 50 -> 100
```

### Problem: Consumer can't keep up

**Cause:** Processing slower than ingestion rate
**Solution:**
1. Check batch processing latency: `make metrics`
2. Review consumer configuration in `internal/messaging/consumer.go`:
   - Increase `BatchSize` (default: 100)
   - Increase `Workers` (default: 4)
3. Verify database write performance with `make db-stats`

### Problem: Out of memory

**Cause:** Too many concurrent workers or large batches
**Solution:**
```bash
# Reduce workers or batch size
./bin/loadtest -rps 5000 -workers 25 -batch 50
```

### Problem: Database "too many connections"

**Cause:** Connection pool exhausted
**Solution:**
Edit `generated/storage/writer.go`:
```go
func DefaultConfig() *Config {
    return &Config{
        MaxOpenConns:    100,  // Increase from 50
        MaxIdleConns:    20,   // Increase from 10
        ConnMaxLifetime: time.Hour,
    }
}
```

Then rebuild:
```bash
make build
```

## Performance Tuning Tips

### 1. Database Optimization

**PostgreSQL configuration** (edit `docker-compose.yml`):
```yaml
postgres:
  command:
    - "postgres"
    - "-c"
    - "max_connections=200"
    - "-c"
    - "shared_buffers=256MB"
    - "-c"
    - "effective_cache_size=1GB"
```

### 2. Consumer Tuning

**Batch size** (`internal/messaging/consumer.go`):
```go
BatchSize: 200,  // Increase from 100
```

**Worker count**:
```go
Workers: 8,  // Increase from 4
```

### 3. API Tuning

**Fiber configuration** (`cmd/api/main.go`):
```go
app := fiber.New(fiber.Config{
    Prefork:       true,  // Enable multi-process mode
    CaseSensitive: true,
    StrictRouting: false,
    ServerHeader:  "IngestKit",
    AppName:       "IngestKit API",
})
```

### 4. Redpanda Tuning

Increase partitions for better parallelism:
```bash
docker-compose exec redpanda rpk topic create ingestkit.events -p 10 -r 1
```

## Next Steps

After load testing:

1. **Document results** - Record baseline performance metrics
2. **Set monitoring alerts** - Based on p95/p99 latencies
3. **Plan capacity** - Use results to estimate production requirements
4. **Optimize bottlenecks** - Address any identified performance issues
5. **Repeat testing** - After any configuration changes

## Related Documentation

- [TEST_RESULTS.md](TEST_RESULTS.md) - End-to-end test results
- [PLAN.md](PLAN.md) - Project roadmap and milestones
- [README.md](README.md) - General project overview
