# Docker Deployment Guide

Complete guide for deploying IngestKit with Docker and achieving zero-downtime schema updates.

Last updated: 2025-11-15

---

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Zero-Downtime Schema Updates](#zero-downtime-schema-updates)
- [Architecture](#architecture)
- [Configuration](#configuration)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

---

## Overview

IngestKit supports Docker-based deployment with:

- **Zero-downtime rolling updates** for schema changes
- **Health checks** for graceful container lifecycle management
- **Automatic dependency management** with Docker Compose
- **Multi-stage builds** for optimized image sizes
- **Non-root containers** for security

### Services

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| API | ingestkit-api | 8080 | HTTP event ingestion |
| Consumer | ingestkit-consumer | 8081 | Kafka → PostgreSQL |
| PostgreSQL | postgres:18-alpine | 5433 | Event storage |
| Redpanda | redpanda:v23.3.5 | 19092 | Message broker |

---

## Quick Start

### 1. Start Full Stack

```bash
# Build images and start all services
make docker-up
```

Output:
```
✓ Full stack started!
  API:              http://localhost:8080
  Consumer Metrics: http://localhost:8081/metrics
  PostgreSQL:       localhost:5433
  Redpanda:         localhost:19092
  Redpanda Console: http://localhost:8090
```

### 2. Verify Services

```bash
# Check health
curl http://localhost:8080/health

# Check consumer metrics
curl http://localhost:8081/health

# View logs
make docker-logs
```

### 3. Send Test Event

```bash
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "test_001",
    "email": "test@example.com",
    "signup_source": "web"
  }'
```

---

## Zero-Downtime Schema Updates

### The Problem

Traditional deployment:
1. Change schema → regenerate code → rebuild binaries
2. **Stop services** (downtime begins)
3. Deploy new binaries
4. **Start services** (downtime ends)

**Result**: 2-5 seconds of downtime per deployment

### The Solution

Docker rolling updates:
1. Change schema → regenerate code → rebuild binaries → build Docker images
2. Start **new container** with health check
3. Wait for new container to be **healthy**
4. Stop **old container**

**Result**: Zero downtime - service always available

### How It Works

```bash
# Single command for complete zero-downtime update
make docker-reload
```

**What happens:**

```
Step 1/5: Generating code from schema...
  ✓ Generated models, storage, consumer handlers

Step 2/5: Building binaries...
  ✓ Built API and consumer binaries

Step 3/5: Applying database migrations...
  ✓ Schema applied to PostgreSQL

Step 4/5: Building Docker images...
  ✓ Built ingestkit-api:latest
  ✓ Built ingestkit-consumer:latest

Step 5/5: Rolling update...
  ✓ API server reloaded (old: 1/1, new: 1/1)
  ✓ Consumer reloaded (old: 1/1, new: 1/1)

✓ Zero-downtime reload complete!
  Services are running with new schema
```

### Behind the Scenes

Docker Compose `up -d --no-deps --build`:
1. Builds new image with updated code
2. Starts new container (e.g., `api_2`)
3. Waits for health check to pass
4. Stops old container (e.g., `api_1`)
5. Removes old container

**Health Check Configuration:**
```yaml
healthcheck:
  test: ["CMD", "wget", "--spider", "http://localhost:8080/health"]
  interval: 10s
  timeout: 3s
  retries: 3
  start_period: 5s
```

New container must respond to `/health` within 5 seconds startup + 3 retries = ~20 seconds max before considered unhealthy.

---

## Architecture

### Multi-Stage Builds

Both API and consumer use multi-stage builds:

**Dockerfile.api:**
```dockerfile
# Stage 1: Build
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o api ./cmd/api

# Stage 2: Runtime
FROM alpine:latest
RUN apk add ca-certificates tzdata
COPY --from=builder /build/api .
COPY schema/events.yaml schema/events.yaml
EXPOSE 8080
CMD ["./api"]
```

**Benefits:**
- Build tools (~500MB) not included in runtime image
- Final image: ~20MB vs ~500MB
- Faster deployments
- Smaller attack surface

### Health Checks

Health checks enable:
- **Graceful startup**: Container marked "starting" until healthy
- **Graceful shutdown**: Old container receives SIGTERM, drains connections
- **Rolling updates**: New container healthy before old stops
- **Load balancer integration**: Unhealthy containers removed from pool

### Networking

All services on `ingestkit` bridge network:
- **Internal hostnames**: `postgres`, `redpanda`, `api`, `consumer`
- **External ports**: 8080 (API), 8081 (metrics), 5433 (postgres), 19092 (redpanda)

---

## Configuration

### Environment Variables

Set in `.env` file:

```bash
# API Configuration
API_PORT=8080
RATE_LIMIT_RPS=20000
API_KEY_1=dev_key_1234567890:default
API_KEY_2=sk_test_tenant_alpha:tenant_alpha
ADMIN_SCHEMA_KEY=admin_secret_key_here

# Consumer Configuration
CONSUMER_WORKERS=4
CONSUMER_BATCH_SIZE=500
CONSUMER_BATCH_TIMEOUT_MS=20

# Database
POSTGRES_USER=ingestkit
POSTGRES_PASSWORD=ingestkit_dev
POSTGRES_DB=ingestkit
POSTGRES_PORT=5433

# Redpanda
REDPANDA_TOPIC=ingestkit.events
CONSUMER_GROUP_ID=ingestkit-consumer

# Scaling (optional)
API_REPLICAS=2
CONSUMER_REPLICAS=2
```

### Scaling

Scale services horizontally:

```bash
# Scale API to 3 instances
docker-compose up -d --scale api=3

# Scale consumer to 2 instances
docker-compose up -d --scale consumer=2
```

**Note**: Requires load balancer for API (not included in docker-compose.yml)

---

## Production Deployment

### Checklist

- [ ] Set production API keys (not `dev_key_*`)
- [ ] Set `ADMIN_SCHEMA_KEY` for schema push protection
- [ ] Configure appropriate `RATE_LIMIT_RPS`
- [ ] Set `CONSUMER_WORKERS` based on load
- [ ] Enable TLS termination (use reverse proxy like Nginx/Traefik)
- [ ] Set up monitoring (Prometheus scrapes `/metrics`)
- [ ] Configure log aggregation (stdout/stderr → logging service)
- [ ] Set up automated backups for PostgreSQL
- [ ] Configure DLQ alerting
- [ ] Test rolling updates in staging first

### Using with Docker Swarm

```bash
# Deploy stack
docker stack deploy -c docker-compose.yml ingestkit

# Update with zero downtime
docker service update --image ingestkit-api:v2 ingestkit_api
```

### Using with Kubernetes

Convert docker-compose.yml to Kubernetes manifests:

```bash
# Using kompose
kompose convert -f docker-compose.yml

# Deploy
kubectl apply -f .
```

See [Kubernetes deployment guide](kubernetes-deployment.md) for details.

---

## Troubleshooting

### Container Won't Start

**Check logs:**
```bash
make docker-logs-api
make docker-logs-consumer
```

**Common issues:**
- Missing `.env` file → Run `make setup`
- Database not ready → Wait for `postgres` health check
- Port conflict → Change `API_PORT` or `POSTGRES_PORT` in `.env`

### Health Check Failing

**Symptoms:** Container repeatedly restarting

**Debug:**
```bash
# Exec into container
docker exec -it ingestkit-api sh

# Test health endpoint manually
wget -O- http://localhost:8080/health
```

**Common causes:**
- API not binding to `0.0.0.0` (check `API_PORT` env)
- Schema file missing in container
- Database connection failing

### Rolling Update Stuck

**Symptoms:** New container starts but old container doesn't stop

**Check:**
```bash
# View container status
docker-compose ps

# Check health status
docker inspect ingestkit-api | grep Health -A 10
```

**Solution:**
```bash
# Force recreation
docker-compose up -d --force-recreate api
```

### Schema Changes Not Applied

**Symptoms:** Events rejected after `docker-reload`

**Verify schema version:**
```bash
# Check running version
curl http://localhost:8080/schema | head -1
# Should show: version: "1.0" (or your version)

# Check consumer logs for schema version
make docker-logs-consumer | grep "schema versions"
# Should show: map[1.0:N] where N is event count
```

**Solution:**
```bash
# Ensure schema-apply ran before docker-reload
make schema-apply
make docker-build
make docker-reload
```

---

## Performance Tuning

### Build Cache

Speed up builds with BuildKit:

```bash
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1
make docker-build
```

### Multi-Platform Builds

Build for different architectures:

```bash
docker buildx build --platform linux/amd64,linux/arm64 \
  -t ingestkit-api:latest -f Dockerfile.api .
```

### Resource Limits

Set memory/CPU limits in docker-compose.yml:

```yaml
api:
  deploy:
    resources:
      limits:
        cpus: '2'
        memory: 2G
      reservations:
        cpus: '1'
        memory: 1G
```

---

## Additional Commands

```bash
# Build without cache
docker-compose build --no-cache

# View resource usage
docker stats

# Clean up unused images
docker system prune -a

# Export logs
docker-compose logs api > api.log

# Restart single service
docker-compose restart api

# Stop and remove volumes (⚠️  deletes data)
docker-compose down -v
```

---

## Next Steps

- Set up CI/CD for automated deployments
- Configure monitoring and alerting
- Implement blue-green deployment
- Set up database replication
- Configure TLS certificates

---

## Related Documentation

- [Development Guide](development.md)
- [Schema Management](development.md#schema-management)
- [Production Checklist](development.md#production-checklist)
