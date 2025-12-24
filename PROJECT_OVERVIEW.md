# Project Overview: IngestKit

IngestKit is a high-performance, scalable event ingestion system designed to handle thousands of events per second with zero data loss. It provides a robust foundation for building real-time analytics, audit logging, and data pipeline applications.

## Installation

```bash
pip install ingestkit    # Python
npm install -g ingestkit  # Node.js
```

Or download binaries from [GitHub Releases](https://github.com/feat7/ingestkit/releases).

## Architecture

IngestKit follows a decoupled, event-driven architecture using the **Producer-Consumer** pattern.

```mermaid
graph LR
    Client[Client App] -->|POST /events| API[IngestKit API]
    API -->|Validate| Schema[Schema Validator]
    API -->|Produce| Redpanda[Redpanda (Kafka)]
    Redpanda -->|Consume| Consumer[IngestKit Consumer]
    Consumer -->|Batch Insert| DB[(PostgreSQL)]
    
    subgraph "Control Plane"
        Admin[Admin] -->|Push Schema| API
        API -->|Update| Schema
    end
```

### Core Components

1.  **IngestKit API (`cmd/api`)**:
    *   **Role**: The entry point for all events.
    *   **Tech**: Go (Fiber framework).
    *   **Responsibilities**:
        *   **Authentication**: Validates API keys and identifies tenants.
        *   **Validation**: Validates incoming JSON payloads against the defined schema (`schema/events.yaml`).
        *   **Ingestion**: Publishes valid events to Redpanda (Kafka). Supports both Sync (wait for ACK) and Async (fire and forget) modes.
        *   **Schema Management**: Exposes endpoints to fetch and update the event schema dynamically.

2.  **Redpanda (Message Broker)**:
    *   **Role**: The durable buffer between ingestion and storage.
    *   **Tech**: Redpanda (Kafka-compatible, C++).
    *   **Responsibilities**:
        *   **Durability**: Persists events to disk immediately.
        *   **Decoupling**: Allows the API to accept events faster than the database can write them.
        *   **Ordering**: Guarantees order within partitions.

3.  **IngestKit Consumer (`cmd/consumer`)**:
    *   **Role**: The worker that processes events and writes them to storage.
    *   **Tech**: Go.
    *   **Responsibilities**:
        *   **Consumption**: Reads messages from Redpanda topics.
        *   **Batching**: Aggregates multiple events into a single database transaction for high throughput.
        *   **Storage**: Inserts events into PostgreSQL partitioned tables.
        *   **Error Handling**: Retries failed batches and moves permanently failed messages to a Dead Letter Queue (DLQ).

4.  **PostgreSQL (Storage)**:
    *   **Role**: The primary system of record.
    *   **Tech**: PostgreSQL 16+.
    *   **Responsibilities**:
        *   **Data Storage**: Stores events in structured tables (`events_user_signup`, `events_purchase`, etc.).
        *   **Partitioning**: Uses declarative partitioning (e.g., by time or type) for efficient management of massive datasets.

## 📂 Project Structure

The codebase follows the Standard Go Project Layout.

```
.
├── bin/                 # Compiled binaries
├── cmd/                 # Main applications
│   ├── api/             # API server entrypoint
│   ├── cli/             # CLI tool for schema management
│   └── consumer/        # Consumer worker entrypoint
├── docs/                # Documentation
├── generated/           # Code generated from schema
│   ├── models/          # Go structs for events
│   └── sql/             # SQL schema definitions
├── internal/            # Private application code
│   ├── api/             # API handlers and middleware
│   ├── config/          # Configuration loading
│   ├── consumer/        # Consumer logic
│   ├── logger/          # Structured logging setup
│   ├── messaging/       # Kafka/Redpanda wrapper (Producer/Consumer)
│   ├── schema/          # Schema parsing and validation logic
│   └── validation/      # Event validation logic
├── migrations/          # Database migrations (golang-migrate)
├── schema/              # Event schema definitions (events.yaml)
├── scripts/             # Utility scripts
├── docker-compose.yml   # Docker infrastructure
└── Makefile             # Build and automation commands
```

## 🚀 Key Features

### 1. Schema-Driven Development
Everything starts with `schema/events.yaml`. This single source of truth defines your event types and their structure.
*   **Code Generation**: `make generate` creates Go structs, SQL tables, and validation logic automatically.
*   **Runtime Validation**: The API rejects any event that doesn't match the schema.

### 2. High Performance
*   **Batching**: The consumer groups events (e.g., 500 at a time) to minimize database round-trips.
*   **Async Ingestion**: The API can accept events immediately (HTTP 202) while processing happens in the background.
*   **Concurrency**: Tunable worker pools in the consumer.

### 3. Reliability
*   **Dead Letter Queue (DLQ)**: Events that fail processing (e.g., DB errors) are saved to a `dead_letter_queue` table, ensuring no data is lost.
*   **Graceful Shutdown**: Services finish processing in-flight requests before stopping.
*   **Health Checks**: `/health` endpoints for Kubernetes/Docker probes.

### 4. Multi-Tenancy
*   **API Keys**: Built-in support for multiple API keys, each mapped to a `tenant_id`.
*   **Data Isolation**: Every event is tagged with `tenant_id` for downstream filtering.

## 🛠️ Development Workflow

1.  **Define Schema**: Edit `schema/events.yaml` to add a new event type.
2.  **Generate Code**: Run `make generate` to update Go models and SQL.
3.  **Run Infrastructure**: `make up` starts Postgres and Redpanda.
4.  **Run App**: `make dev` runs the API and Consumer.
5.  **Test**: `make test-event` sends a sample payload.

## 📦 Deployment

*   **Docker**: Full stack defined in `docker-compose.yml`.
*   **Configuration**: All configuration via environment variables (see `.env.example`).
*   **Production Ready**: Includes resource limits, health checks, and structured logging.

## 📚 Documentation

*   **[Development Guide](docs/development.md)**: Detailed setup and workflow.
*   **[Docker Deployment](docs/docker-deployment.md)**: How to run in production.
*   **[Load Testing](docs/load-testing.md)**: Performance benchmarks and tuning.
*   **[SDK Generation](docs/sdk-generation.md)**: How to build client libraries.
