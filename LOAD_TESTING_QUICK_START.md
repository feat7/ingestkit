# Load Testing Quick Start

**Ready to load test IngestKit in under 5 minutes!**

## Prerequisites

- Docker Desktop with 8GB+ RAM, 6+ CPU cores
- k6 installed: `brew install k6` (macOS) or `sudo apt-get install k6` (Linux)

## Quick Start (4 steps)

### 1. One-Time System Tuning

```bash
make tune-system
```

This checks/optimizes file descriptors, Docker resources, and system parameters.

### 2. Start Optimized Stack

```bash
make loadtest-start
```

This starts:
- 2 API replicas (load balanced)
- 2 Consumer replicas
- PostgreSQL (write-optimized, 4GB RAM, 4 CPUs)
- Redpanda (4GB RAM, 4 CPUs)

**Wait 60 seconds for warmup**:
```bash
sleep 60
```

### 3. Run Load Test

```bash
# Quick validation - 100 RPS, 10s
make loadtest-quick

# High throughput - 10,000 RPS, 30s
make loadtest-high
```

### 4. Monitor & Stop

**Monitor in real-time**:
```bash
make loadtest-monitor  # Requires tmux (brew install tmux)
# OR
make metrics-watch     # Simple metrics watch
```

**Stop when done**:
```bash
make loadtest-stop
```

---

## Expected Results

### Baseline Test (500 RPS)
```
✓ Success Rate: 100%
✓ Avg Latency: 8ms
✓ p95 Latency: 15ms
```

### High Throughput (10,000 RPS)
```
✓ Success Rate: 99.8%+
✓ Avg Latency: 13ms
✓ p95 Latency: 18ms
✓ Throughput: 31,200 events/sec (2 consumers)
```

---

## Available Commands

```bash
# System tuning
make tune-system           # One-time system optimization

# Stack management
make loadtest-start        # Start optimized stack
make loadtest-stop         # Stop stack
make loadtest-restart      # Restart stack
make loadtest-logs         # View logs

# Load tests
make loadtest-quick        # 100 RPS, 10s (validation)
make loadtest-baseline     # 500 RPS, 30s (baseline)
make loadtest-production   # 1,000 RPS, 60s (prod sim)
make loadtest-high         # 10,000 RPS, 30s (stress test)

# Monitoring
make loadtest-monitor      # Real-time dashboard (tmux)
make loadtest-stats        # Show statistics
make metrics-watch         # Watch consumer metrics
make db-stats              # Database statistics
```

---

## Troubleshooting

### "Too many open files"
```bash
make tune-system  # Re-run tuning script
```

### "Rate limit exceeded" (429 errors)
Already configured for 50,000 RPS in `.env.loadtest` - should not happen.

### Consumer lag / slow processing
```bash
# Check consumer health
curl http://localhost:8081/health

# Check metrics
make metrics-watch

# Increase workers (edit .env.loadtest)
CONSUMER_WORKERS=16
make loadtest-restart
```

### High latency (>50ms p95)
1. Check Docker resource allocation (Settings → Resources)
2. Ensure using SSD for Docker volumes
3. Check `docker stats` for resource contention

---

## Monitoring URLs

- **API Health**: http://localhost:8080/health
- **Consumer Metrics**: http://localhost:8081/metrics
- **Redpanda Console**: http://localhost:8090
- **PostgreSQL**: `localhost:5433` (user: ingestkit, pass: ingestkit_dev)

---

## What's Been Tuned?

### PostgreSQL
- Shared buffers: 2GB
- Max connections: 200
- Synchronous commit: OFF (faster writes)
- WAL compression: ON
- Parallel workers: 8
- Autovacuum: Optimized for high writes

### Redpanda
- CPUs: 4 (up from 1)
- Memory: 4GB (up from 1GB)
- Overprovisioned mode: Enabled

### API Server
- Replicas: 2
- Rate limit: 50,000 RPS
- GOMAXPROCS: 4
- File descriptors: 65,536

### Consumer
- Replicas: 2
- Workers: 8 per replica (16 total)
- Batch size: 1,000 events
- Batch timeout: 20ms
- GOMAXPROCS: 4

---

## Next Steps

1. **Review detailed docs**: `docs/load-testing.md`
2. **Customize tests**: Edit `loadtest/*.js` scripts
3. **Production deployment**: See `docs/docker-deployment.md`
4. **Scaling beyond 10K RPS**: See "Advanced Topics" in `docs/load-testing.md`

---

## About Caddy (Reverse Proxy)

**Q: Should I use Caddy for load testing?**

**A: NO** - Adding a reverse proxy would **hurt** performance because:
- Your Go Fiber API already handles async efficiently
- Adds extra network hop and latency
- No benefit for direct load testing
- Use Caddy in production for TLS termination, not load testing

---

**Ready to test? Run:**

```bash
make loadtest-start && sleep 60 && make loadtest-high
```

That's it! 🚀
