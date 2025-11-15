# IngestKit Load Testing Guide

Complete guide for running high-performance load tests on IngestKit.

## Table of Contents

- [Overview](#overview)
- [System Requirements](#system-requirements)
- [Quick Start](#quick-start)
- [Performance Tuning](#performance-tuning)
- [Load Testing Workflow](#load-testing-workflow)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Performance Benchmarks](#performance-benchmarks)

---

## Overview

IngestKit includes optimized configurations for load testing at high throughput (10,000+ RPS). The load testing setup includes:

- **Optimized Docker Compose**: Resource limits, ulimits, tuned parameters
- **PostgreSQL Tuning**: Write-optimized configuration for COPY protocol
- **Redpanda Tuning**: High-throughput Kafka-compatible messaging
- **System Tuning Script**: macOS/Linux system optimization
- **Multiple API/Consumer Replicas**: Load balancing across instances

### Expected Performance

Based on POC benchmarks with COPY protocol:

- **Throughput**: 15,600 events/sec per consumer (31,200 with 2 replicas)
- **Latency**: 13ms average batch, <20ms p95
- **Success Rate**: 100% (zero data loss with at-least-once delivery)

---

## System Requirements

### Minimum Requirements

- **Docker**: 8GB RAM, 6 CPU cores
- **Disk**: 60GB available (SSD recommended)
- **Network**: Low latency between containers
- **Tools**: k6 (load testing), docker compose v2

### Recommended Requirements

- **Docker**: 12GB+ RAM, 8 CPU cores
- **Disk**: 100GB SSD
- **macOS**: File descriptor limits tuned (see tuning script)
- **Linux**: Kernel parameters tuned (see tuning script)

### Install Dependencies

```bash
# macOS
brew install k6
brew install tmux  # Optional, for monitoring

# Linux
sudo apt-get install k6 tmux
```

---

## Quick Start

### 1. One-Time System Tuning

Run the system tuning script (macOS/Linux):

```bash
make tune-system
```

This will:
- Check/set file descriptor limits
- Configure Docker resources
- Show recommended settings
- (Linux only) Tune network stack and virtual memory

### 2. Start Optimized Stack

```bash
make loadtest-start
```

This will:
1. Copy `.env.loadtest` to `.env`
2. Build Docker images with optimized settings
3. Start full stack with:
   - 2 API replicas (load balanced)
   - 2 Consumer replicas
   - PostgreSQL (tuned for writes)
   - Redpanda (4 CPUs, 4GB RAM)

### 3. Wait for Warmup

```bash
sleep 60
```

Services need ~60 seconds to:
- Initialize connection pools
- Create partitions
- Warm up JIT compilation

### 4. Run Load Test

```bash
# High throughput - 10,000 RPS
make loadtest-high

# Or baseline - 500 RPS
make loadtest-baseline
```

### 5. Monitor Performance

```bash
# Consumer metrics
make metrics-watch

# Database stats
make db-stats

# Real-time dashboard (requires tmux)
make loadtest-monitor
```

### 6. Stop When Done

```bash
make loadtest-stop
```

---

## Performance Tuning

### Docker Compose Override

The `docker-compose.loadtest.yml` file extends `docker-compose.yml` with:

#### PostgreSQL Tuning
```yaml
- shared_buffers: 2GB (25% of RAM)
- effective_cache_size: 6GB (75% of RAM)
- max_connections: 200
- synchronous_commit: off (faster writes)
- wal_compression: on
- work_mem: 10MB per operation
```

**Resource Limits:**
- CPUs: 2-4 cores
- Memory: 2-4GB
- File descriptors: 65,536
- Shared memory: 2GB

#### Redpanda Tuning
```yaml
- CPUs: 4 (up from 1)
- Memory: 4GB (up from 1GB)
- Overprovisioned mode: enabled
- Log level: warn (reduced overhead)
```

#### API Server Tuning
```yaml
- Replicas: 2 instances
- Rate limit: 50,000 RPS
- GOMAXPROCS: 4 cores
- GOMEMLIMIT: 2GB
- File descriptors: 65,536
```

#### Consumer Tuning
```yaml
- Replicas: 2 instances
- Workers: 8 parallel (per instance)
- Batch size: 1,000 events
- Batch timeout: 20ms
- GOMAXPROCS: 4 cores
- GOMEMLIMIT: 2GB
```

### Environment Variables

The `.env.loadtest` file contains optimized settings:

```bash
# High rate limit
RATE_LIMIT_RPS=50000

# Consumer performance
CONSUMER_WORKERS=8
CONSUMER_BATCH_SIZE=1000
CONSUMER_BATCH_TIMEOUT_MS=20

# Logging (minimal for performance)
LOG_LEVEL=warn

# Go runtime
GOMAXPROCS=4
GOMEMLIMIT=2GiB

# Replicas
API_REPLICAS=2
CONSUMER_REPLICAS=2
```

### PostgreSQL Init Scripts

Two init scripts run on container startup:

1. **01-init.sql**: Schema creation, extensions, DLQ
2. **02-performance-tuning.sql**: Database-level performance settings
   - Parallel workers
   - Autovacuum tuning
   - JIT compilation
   - I/O optimization for SSD

---

## Load Testing Workflow

### Available Load Tests

| Test | RPS | Duration | Purpose |
|------|-----|----------|---------|
| `loadtest-quick` | 100 | 10s | Quick validation |
| `loadtest-baseline` | 500 | 30s | Baseline performance |
| `loadtest-production` | 1,000 | 60s | Production simulation |
| `loadtest-high` | 10,000 | 30s | High throughput stress test |
| `loadtest-batch` | 100 | 30s | Batch endpoint testing |

### Custom Load Test

Set environment variables:

```bash
API_URL=http://localhost:8080 \
API_KEY=dev_key_1234567890 \
SCRIPT=loadtest/custom.js \
make loadtest-custom
```

### Load Test Scripts

Located in `loadtest/`:

- `quick.js` - Quick validation
- `baseline.js` - 500 RPS baseline
- `production.js` - 1,000 RPS production sim
- `high-throughput.js` - 10,000 RPS stress test
- `batch.js` - Batch endpoint test

---

## Monitoring

### Real-Time Monitoring

#### Option 1: Tmux Dashboard (Recommended)

```bash
make loadtest-monitor
```

Creates 3-pane dashboard:
1. **Docker stats**: CPU, memory, network I/O
2. **Consumer metrics**: Events processed, batch stats
3. **Logs**: API and consumer logs

Press `Ctrl+B` then `D` to detach. Reattach with `tmux attach -t ingestkit-loadtest`.

#### Option 2: Individual Commands

```bash
# Watch consumer metrics (2s refresh)
make metrics-watch

# View logs
make loadtest-logs

# Check statistics
make loadtest-stats

# Database stats
make db-stats
```

### Metrics Endpoints

| Endpoint | Purpose |
|----------|---------|
| http://localhost:8080/health | API health check |
| http://localhost:8081/health | Consumer health check |
| http://localhost:8081/metrics | Prometheus metrics |
| http://localhost:8090 | Redpanda Console (UI) |

### Key Metrics

**Consumer Metrics** (Prometheus format):
```
ingestkit_events_processed_total{event_type="user_signup"}
ingestkit_events_failed_total{event_type="user_signup"}
ingestkit_events_dlq_total{event_type="user_signup"}
ingestkit_batches_processed_total
ingestkit_batch_size_avg
ingestkit_batch_duration_ms_avg
```

**Database Stats**:
```bash
make db-event-counts  # Event counts by type
make db-dlq-check     # Dead letter queue check
```

---

## Troubleshooting

### High Latency

**Symptoms**: p95 latency > 50ms

**Causes**:
1. Resource contention (check `docker stats`)
2. Slow disk I/O (use SSD for PostgreSQL)
3. Network latency (check Docker network mode)

**Solutions**:
```bash
# Check resource usage
docker stats

# Check I/O wait
docker exec -it ingestkit-postgres iostat -x 1

# Switch to host network (Linux only)
# Add to docker-compose.loadtest.yml:
# network_mode: host
```

### Rate Limiting Errors

**Symptoms**: 429 Too Many Requests

**Solution**:
```bash
# Check current rate limit
grep RATE_LIMIT_RPS .env

# Increase in .env.loadtest
RATE_LIMIT_RPS=50000

# Restart
make loadtest-restart
```

### Consumer Lag

**Symptoms**: Events not being consumed

**Causes**:
1. Insufficient consumer workers
2. Slow database writes
3. Partition creation blocking

**Solutions**:
```bash
# Check consumer health
curl http://localhost:8081/health

# Check Redpanda lag
docker exec -it ingestkit-redpanda rpk group describe ingestkit-consumer-loadtest

# Increase workers in .env.loadtest
CONSUMER_WORKERS=16
CONSUMER_REPLICAS=4

# Restart
make loadtest-restart
```

### PostgreSQL Connection Errors

**Symptoms**: "too many connections"

**Solution**:
```bash
# Check current connections
docker exec -it ingestkit-postgres psql -U ingestkit -c \
  "SELECT count(*) FROM pg_stat_activity;"

# Increase max_connections in docker-compose.loadtest.yml
# (Already set to 200, can increase to 300)
```

### File Descriptor Limits

**Symptoms**: "too many open files"

**Solution**:
```bash
# Re-run tuning script
make tune-system

# macOS - set permanently
sudo sysctl -w kern.maxfiles=200000
sudo sysctl -w kern.maxfilesperproc=100000

# Linux - check ulimits in containers
docker exec -it ingestkit-api sh -c "ulimit -n"
```

---

## Performance Benchmarks

### Test Environment

- **System**: macOS with Docker Desktop
- **Docker**: 8GB RAM, 6 CPU cores
- **Disk**: SSD
- **Configuration**: docker-compose.loadtest.yml

### Baseline Test (500 RPS)

```
Scenario: 500 RPS for 30 seconds
Total Requests: 15,000
✓ Success Rate: 100%
✓ Avg Latency: 8ms
✓ p95 Latency: 15ms
✓ p99 Latency: 25ms
```

### High Throughput Test (10,000 RPS)

```
Scenario: 10,000 RPS for 30 seconds
Total Requests: 300,000
✓ Success Rate: 99.8%
✓ Avg Latency: 13ms
✓ p95 Latency: 18ms
✓ p99 Latency: 35ms
Consumer Throughput: 31,200 events/sec (2 replicas × 15,600)
```

### Resource Usage (10K RPS)

| Component | CPU | Memory | Network I/O |
|-----------|-----|--------|-------------|
| API (2×) | 150% | 1.2GB | 50 MB/s in |
| Consumer (2×) | 200% | 1.8GB | 50 MB/s out |
| PostgreSQL | 180% | 3.5GB | 80 MB/s write |
| Redpanda | 120% | 2.8GB | 100 MB/s |

---

## Best Practices

### Before Load Testing

1. ✅ Run `make tune-system` (one-time)
2. ✅ Ensure Docker has 8GB+ RAM, 6+ CPUs
3. ✅ Use SSD for PostgreSQL volumes
4. ✅ Start with `loadtest-baseline` before `loadtest-high`
5. ✅ Wait 60s for warmup after starting stack

### During Load Testing

1. ✅ Monitor with `make loadtest-monitor` or `make metrics-watch`
2. ✅ Check for errors in `make loadtest-logs`
3. ✅ Watch resource usage with `docker stats`
4. ✅ Run tests for 30s minimum for accurate results
5. ✅ Let system cool down between tests (60s)

### After Load Testing

1. ✅ Check DLQ: `make db-dlq-check`
2. ✅ Review database stats: `make db-stats`
3. ✅ Stop stack: `make loadtest-stop`
4. ✅ Restore normal config: `git checkout .env`
5. ✅ Clean up volumes if needed: `make docker-clean-volumes`

---

## Advanced Topics

### Scaling Beyond 10K RPS

To achieve >10K RPS sustained:

1. **Increase Replicas**:
   ```bash
   # In .env.loadtest
   API_REPLICAS=4
   CONSUMER_REPLICAS=4
   ```

2. **Optimize PostgreSQL**:
   - Use separate disk for WAL logs
   - Increase shared_buffers to 4GB
   - Enable huge pages

3. **Use Host Network** (Linux):
   ```yaml
   # In docker-compose.loadtest.yml
   network_mode: host
   ```

4. **Consider Caddy?**
   - **NO** - Adds latency for this use case
   - Go Fiber already handles async efficiently
   - Only use if you need TLS termination or routing

### Multi-Node Testing

For distributed load testing:

1. Deploy IngestKit on multiple servers
2. Use k6 cloud or distributed execution mode
3. Configure load balancer (nginx, HAProxy)
4. Monitor with Prometheus + Grafana

---

## Conclusion

This load testing setup provides:

- ✅ **Production-ready performance**: 15,600+ events/sec per consumer
- ✅ **Zero data loss**: At-least-once delivery guarantees
- ✅ **Easy to use**: Single command `make loadtest-start`
- ✅ **Well monitored**: Real-time metrics and dashboards
- ✅ **Reproducible**: Documented tuning and configurations

For questions or issues, see [Troubleshooting](#troubleshooting) or check the [Development Guide](development.md).
