# IngestKit

> **High-Performance Event Ingestion System**

[![Release](https://img.shields.io/github/v/release/feat7/ingestkit)](https://github.com/feat7/ingestkit/releases)
[![CI](https://github.com/feat7/ingestkit/actions/workflows/release.yml/badge.svg)](https://github.com/feat7/ingestkit/actions)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![PyPI](https://img.shields.io/pypi/v/ingestkit)](https://pypi.org/project/ingestkit/)
[![npm](https://img.shields.io/npm/v/ingestkit)](https://www.npmjs.com/package/ingestkit)

Handle **30k+ events/sec** on a $14 server with zero data loss. Schema-first design with auto-generated type-safe SDKs.

---

## Quick Start

### Install

```bash
pip install ingestkit
```
Or: `npm install -g ingestkit`

### Run Server

```bash
ingestkit init --server
ingestkit server start
```

That's it! Server running at `http://localhost:8080`

### Send Events

```bash
curl -X POST http://localhost:8080/v1/events/user_signup \
  -H "Authorization: Bearer dev_key_1234567890" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "123", "email": "user@example.com"}'
```

---

## Features

- **30,000+ RPS** on a single $14/month server
- **Zero Data Loss** with Kafka-compatible buffering (Redpanda)
- **Schema-First** - Define events in YAML, get type-safe SDKs
- **Multi-Tenant** - Built-in API key management and data isolation
- **Observability** - Prometheus metrics and structured logging

---

## Define Your Events

Edit `schema/events.yaml`:

```yaml
events:
  user_signup:
    fields:
      user_id:
        type: string
        required: true
      email:
        type: string
        required: true
      plan:
        type: string
        values: [free, pro, enterprise]
```

Apply changes:

```bash
ingestkit schema apply
```

---

## Type-Safe SDKs

Generate clients for your app:

```bash
# Python
ingestkit init --python
ingestkit generate --schema-url http://localhost:8080/schema
```

Use the generated client:

```python
from ingestkit import Client

client = Client()
client.user_signup.send(user_id="123", email="user@example.com")
```

```typescript
// TypeScript
import { Client } from './ingestkit';

const client = new Client();
await client.userSignup.send({ userId: "123", email: "user@example.com" });
```

---

## Server Commands

| Command | Description |
|---------|-------------|
| `ingestkit server start` | Start server (Docker) |
| `ingestkit server stop` | Stop server |
| `ingestkit server logs` | View logs |
| `ingestkit server status` | Check status |
| `ingestkit schema apply` | Apply schema changes |

---

## Architecture

```
Client App → IngestKit API → Redpanda (Kafka) → Consumer → PostgreSQL
```

- **API**: Validates events, publishes to Kafka
- **Consumer**: Batches events, writes to PostgreSQL with COPY protocol
- **PostgreSQL**: Partitioned tables per tenant

---

## Configuration

Environment variables (set in `.env`):

| Variable | Default | Description |
|----------|---------|-------------|
| `API_PORT` | `8080` | API server port |
| `API_KEY_1` | `dev_key:default` | API key (format: `key:tenant`) |
| `CONSUMER_WORKERS` | `4` | Parallel workers |
| `CONSUMER_BATCH_SIZE` | `500` | Events per batch |

---

## Benchmarks

Tested on Hetzner CAX31 ($14/mo: 8 vCPU, 16GB RAM):

| Metric | Result |
|--------|--------|
| Throughput | 30,000 RPS |
| Latency (p95) | < 15ms |
| Data Loss | 0% |

---

## Documentation

- [Project Overview](PROJECT_OVERVIEW.md)
- [SDK Generation](docs/sdk-generation.md)
- [Docker Deployment](docs/docker-deployment.md)
- [Development Guide](docs/development.md)

---

## Development

For contributors building from source:

```bash
git clone https://github.com/feat7/ingestkit.git
cd ingestkit
make setup && make start
```

See [Development Guide](docs/development.md) for details.

---

## License

Apache 2.0 - See [LICENSE](LICENSE)
