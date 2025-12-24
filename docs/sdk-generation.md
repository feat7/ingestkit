# SDK Generation Guide

Complete guide for generating and using IngestKit SDKs.

Last updated: 2025-11-15

---

## Table of Contents

- [Overview](#overview)
- [Generating SDKs](#generating-sdks)
- [Python SDK](#python-sdk)
- [TypeScript SDK](#typescript-sdk)
- [SDK Features](#sdk-features)
- [Advanced Usage](#advanced-usage)
- [Troubleshooting](#troubleshooting)

---

## Overview

IngestKit automatically generates type-safe client SDKs from your schema definition. SDKs provide:

- **Type Safety**: Compile-time validation of event structures
- **Delivery Guarantees**: Fire-and-forget or synchronous delivery modes
- **Automatic Retry**: Exponential backoff for transient failures
- **Request Tracking**: Unique request IDs for debugging
- **Connection Pooling**: Efficient HTTP connection management

### Supported Languages

- ✅ **Python 3.9+** - Full support with Pydantic models
- ✅ **TypeScript/JavaScript** - Full support with TypeScript types
- ⏳ **Go** - Planned (server-side models already generated)
- ⏳ **Java** - Planned

---

## Installation

### For SDK Users (Recommended)

Install the IngestKit CLI to generate type-safe clients for your application:

**Python:**
```bash
pip install ingestkit
```

**Node.js:**
```bash
npm install ingestkit
```

### For Server Operators

If you're self-hosting IngestKit, the CLI is built with the project:

```bash
make build
# Binary available at ./bin/ingestkit
```

---

## Generating SDKs

### Method 1: From Remote Server (SDK Users)

If you're connecting to an existing IngestKit server:

```bash
# Initialize project structure
ingestkit init --python   # or --typescript

# Generate client from server schema
ingestkit generate --schema-url https://your-server.com/schema
```

This fetches the schema from the server and generates a type-safe client in `./ingestkit/`.

### Method 2: From Local Schema (Server Operators)

If you're running IngestKit locally:

```bash
# Using the local binary
./bin/ingestkit init --python
./bin/ingestkit generate --api-url http://localhost:8080
```

### Method 3: Automated in Makefile

Add to your project's Makefile:

```makefile
.PHONY: generate-sdk
generate-sdk:
	@ingestkit generate --schema-url $(INGESTKIT_URL)/schema
	@echo "SDK generated in ingestkit/"
```

---

## Python SDK

### Setup

```bash
# Install CLI (downloads Go binary)
pip install ingestkit

# Initialize in your project
cd your-project
ingestkit init --python
ingestkit generate --schema-url https://your-server.com/schema
```

### Basic Usage

```python
from ingestkit import IngestKitClient
from ingestkit.models import UserSignup

# Initialize client
client = IngestKitClient(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890"
)

# Send event (fire-and-forget)
event = UserSignup(
    user_id="user_123",
    email="user@example.com",
    signup_source="web"
)
future = client.send_user_signup(event)

# Optional: Wait for confirmation
result = future.result(timeout=5)
print(f"Event delivered: {result['request_id']}")
```

### Delivery Modes

#### 1. Fire-and-Forget (Default)

```python
# Returns Future immediately, non-blocking
future = client.send_user_signup(event)

# Do other work...

# Check result later if needed
result = future.result()  # Blocks until delivered
```

#### 2. Synchronous Delivery (Guaranteed)

```python
# Blocks until event is confirmed by Kafka
result = client.send_user_signup(event, wait=True)

# If this completes, event is GUARANTEED delivered
print(f"Delivered: {result['request_id']}")
```

#### 3. Callback Pattern

```python
def on_success(event_data, result):
    print(f"✅ Event delivered: {result['request_id']}")

def on_failure(event_data, error):
    print(f"❌ Event failed: {error}")
    # Save to disk for retry later
    save_to_disk(event_data)

client = IngestKitClient(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890",
    on_success=on_success,
    on_failure=on_failure
)

# Send events - callbacks handle results
client.send_user_signup(event1)
client.send_user_signup(event2)
```

#### 4. Context Manager (Auto-Flush)

```python
# All events guaranteed delivered before context exits
with IngestKitClient(api_url=..., api_key=...) as client:
    client.send_user_signup(event1)
    client.send_purchase(event2)
    client.send_page_view(event3)
# <-- Blocks here until all events are delivered
```

### Batch Operations

```python
from ingestkit.models import Purchase

# Prepare batch
purchases = [
    Purchase(user_id="user1", order_id="order1", amount=99.99, currency="USD"),
    Purchase(user_id="user2", order_id="order2", amount=149.99, currency="USD"),
    # ... up to 1000 events
]

# Send batch (fire-and-forget)
future = client.send_purchase_batch(purchases)

# Or send batch synchronously
result = client.send_purchase_batch(purchases, wait=True)
```

### Configuration Options

```python
client = IngestKitClient(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890",
    tenant_id="my_tenant",           # Override tenant
    timeout=30,                       # Request timeout (seconds)
    max_retries=3,                    # Retry attempts for 5xx errors
    retry_backoff_base=1.0,           # Exponential backoff base (seconds)
    debug=True,                       # Enable debug logging
    background=True,                  # Enable background worker thread
    queue_size=10000,                 # Max events in queue
    on_success=on_success_callback,   # Success callback
    on_failure=on_failure_callback    # Failure callback
)
```

### Environment Variables

```bash
# .env file
INGESTKIT_API_URL=http://localhost:8080
INGESTKIT_API_KEY=dev_key_1234567890
INGESTKIT_TENANT_ID=default
```

```python
# Automatically loads from environment
client = IngestKitClient()  # No args needed
```

### Error Handling

```python
from ingestkit import ClientError, ServerError, NetworkError, QueueFullError

try:
    result = client.send_user_signup(event, wait=True)
except ClientError as e:
    # 4xx errors (validation, auth, etc.)
    print(f"Client error: {e.status_code} - {e}")
    # Don't retry, fix the request
except ServerError as e:
    # 5xx errors (after retries exhausted)
    print(f"Server error: {e.status_code} - {e}")
    # Retry later or alert ops team
except NetworkError as e:
    # Network/timeout errors
    print(f"Network error: {e}")
    # Retry later
except QueueFullError as e:
    # Event queue is full (background mode only)
    print(f"Queue full: {e}")
    # Slow down or increase queue_size
```

---

## TypeScript SDK

### Setup

```bash
# Install CLI (downloads Go binary)
npm install ingestkit

# Initialize in your project
cd your-project
npx ingestkit init --typescript
npx ingestkit generate --schema-url https://your-server.com/schema
```

### Basic Usage

```typescript
import { IngestKitClient } from './ingestkit/client';
import type { UserSignup } from './ingestkit/models';

// Initialize client
const client = new IngestKitClient({
  apiUrl: 'http://localhost:8080',
  apiKey: 'dev_key_1234567890'
});

// Send event (async by default)
const event: UserSignup = {
  user_id: 'user_123',
  email: 'user@example.com',
  signup_source: 'web'
};

const result = await client.sendUserSignup(event);
console.log(`Event delivered: ${result.request_id}`);
```

### Delivery Modes

#### 1. Asynchronous (Default)

```typescript
// Fire-and-forget (API returns 202 Accepted)
const result = await client.sendUserSignup(event);
// Returns immediately after API accepts the event
```

#### 2. Synchronous Delivery

```typescript
// Wait for Kafka confirmation (API returns 200 OK)
const result = await client.sendUserSignup(event, { sync: true });
// Returns only after event is confirmed by Kafka
```

### Batch Operations

```typescript
import type { Purchase } from './ingestkit/models';

const purchases: Purchase[] = [
  { user_id: 'user1', order_id: 'order1', amount: 99.99, currency: 'USD' },
  { user_id: 'user2', order_id: 'order2', amount: 149.99, currency: 'USD' },
  // ... up to 1000 events
];

// Send batch
const result = await client.sendPurchaseBatch(purchases);

// Or synchronous
const syncResult = await client.sendPurchaseBatch(purchases, { sync: true });
```

### Configuration Options

```typescript
const client = new IngestKitClient({
  apiUrl: 'http://localhost:8080',
  apiKey: 'dev_key_1234567890',
  tenantId: 'my_tenant',      // Override tenant
  timeout: 30000,              // Request timeout (ms)
  maxRetries: 3,               // Retry attempts for 5xx errors
  debug: true                  // Enable debug logging
});
```

### Environment Variables

```bash
# .env file
INGESTKIT_API_URL=http://localhost:8080
INGESTKIT_API_KEY=dev_key_1234567890
INGESTKIT_TENANT_ID=default
```

```typescript
// Automatically loads from environment
const client = new IngestKitClient();
```

### Error Handling

```typescript
try {
  await client.sendUserSignup(event, { sync: true });
} catch (error: any) {
  if (error.response) {
    // HTTP error response
    if (error.response.status >= 400 && error.response.status < 500) {
      console.error('Client error:', error.response.data);
      // Fix the request, don't retry
    } else if (error.response.status >= 500) {
      console.error('Server error:', error.response.data);
      // Retry later
    }
  } else if (error.request) {
    console.error('Network error:', error.message);
    // Retry later
  } else {
    console.error('Error:', error.message);
  }
}
```

---

## SDK Features

### 1. Type Safety

All event fields are validated at compile time:

```python
# Python - Pydantic validation
event = UserSignup(
    user_id="123",
    email="invalid-email",  # ✗ Pydantic will validate format
    signup_source="invalid"  # ✗ Must be one of defined values
)
```

```typescript
// TypeScript - Type checking
const event: UserSignup = {
  user_id: "123",
  email: "user@example.com",
  signup_source: "web",
  extra_field: "value"  // ✗ TypeScript error: unknown property
};
```

### 2. Automatic Retry Logic

Both SDKs implement exponential backoff:

```
Attempt 1: Immediate
Attempt 2: Wait 1s
Attempt 3: Wait 2s
Attempt 4: Wait 4s (max_retries=3, gives up)
```

Only retries on:
- 5xx server errors
- Network timeouts
- Connection errors

Does NOT retry on:
- 4xx client errors (bad request, auth, etc.)

### 3. Request ID Tracking

Every request gets a unique ID for debugging:

```python
result = client.send_user_signup(event, wait=True)
print(f"Request ID: {result['request_id']}")
```

Use request ID to:
- Search API logs
- Track event through the system
- Debug delivery issues

### 4. Connection Pooling

Both SDKs use connection pooling for efficiency:

- **Python**: `requests.Session` with HTTPAdapter
- **TypeScript**: `axios` with keepalive

This significantly reduces latency for high-throughput scenarios.

### 5. Schema Versioning

SDKs are versioned based on schema version:

```python
# Python
from ingestkit import __version__
print(f"SDK version: {__version__}")  # Matches schema version
```

```typescript
// TypeScript
import { CLIENT_VERSION } from './ingestkit/client';
console.log(`SDK version: ${CLIENT_VERSION}`);
```

---

## Advanced Usage

### Custom Loggers

#### Python

```python
import logging

logger = logging.getLogger('my_app.ingestkit')
logger.setLevel(logging.DEBUG)

client = IngestKitClient(
    api_url=...,
    api_key=...,
    logger=logger,
    debug=True
)
```

#### TypeScript

```typescript
const client = new IngestKitClient({
  apiUrl: ...,
  apiKey: ...,
  debug: true  // Logs to console
});
```

### Graceful Shutdown

#### Python

```python
import atexit

client = IngestKitClient(...)

# Automatic flush on exit
atexit.register(client.close)

# Or manual flush
client.flush(timeout=5.0)
client.close()
```

#### TypeScript

```typescript
// Flush pending requests before shutdown
process.on('SIGTERM', async () => {
  // Client automatically waits for pending requests
  process.exit(0);
});
```

### Health Checks

```python
# Python
if client.health_check():
    print("API is healthy")
else:
    print("API is down")
```

```typescript
// TypeScript
const isHealthy = await client.healthCheck();
console.log(`API health: ${isHealthy}`);
```

---

## Troubleshooting

### SDK Import Errors

**Problem**: `ModuleNotFoundError: No module named 'ingestkit'`

**Solution**:
```bash
# Ensure SDK is in Python path
export PYTHONPATH="${PYTHONPATH}:/path/to/generated/sdk/python"

# Or install as editable package
cd generated/sdk/python
pip install -e .
```

### Type Validation Errors

**Problem**: Pydantic validation errors

**Solution**:
```python
# Check schema for field requirements
# Ensure all required fields are provided
# Check field types match schema definition

# Debug validation
from pydantic import ValidationError
try:
    event = UserSignup(...)
except ValidationError as e:
    print(e.json())  # Shows exactly which fields failed
```

### Authentication Errors

**Problem**: `401 Unauthorized`

**Solution**:
```bash
# Verify API key is correct
# Check API key format: should be "key:tenant_id"
# Ensure API key is loaded in server .env

# Test with curl
curl -H "Authorization: Bearer dev_key_1234567890" \
     http://localhost:8080/health
```

### Delivery Timeout

**Problem**: Events timing out with `wait=True`

**Solution**:
```python
# Increase timeout
result = client.send_user_signup(event, wait=True, timeout=30)

# Or check consumer is running
# Check Redpanda is healthy
# Check database connectivity
```

### Queue Full Errors

**Problem**: `QueueFullError: Event queue full`

**Solution**:
```python
# Option 1: Increase queue size
client = IngestKitClient(queue_size=50000)

# Option 2: Slow down event generation
time.sleep(0.001)  # Small delay between events

# Option 3: Use blocking mode (no queue)
client = IngestKitClient(background=False)
```

---

## Best Practices

### 1. Reuse Client Instances

```python
# ❌ Bad: Creates new connection pool for each request
def send_event(data):
    client = IngestKitClient(...)
    client.send_user_signup(data)

# ✅ Good: Reuse client with connection pooling
client = IngestKitClient(...)

def send_event(data):
    client.send_user_signup(data)
```

### 2. Use Batch Operations for High Volume

```python
# ❌ Bad: Individual requests for 1000 events
for event in events:
    client.send_purchase(event)

# ✅ Good: Batch request (much faster)
client.send_purchase_batch(events)
```

### 3. Handle Failures Gracefully

```python
# ✅ Good: Callback pattern for async handling
def on_failure(event_data, error):
    # Log error
    logger.error(f"Event failed: {error}")

    # Save to disk for later retry
    with open('failed_events.jsonl', 'a') as f:
        json.dump(event_data, f)
        f.write('\n')

client = IngestKitClient(on_failure=on_failure)
```

### 4. Use Synchronous Mode for Critical Events

```python
# Critical event - wait for confirmation
client.send_payment_completed(payment_event, wait=True)

# Non-critical events - fire and forget
client.send_page_view(page_view_event)  # wait=False (default)
```

### 5. Monitor Queue Size

```python
# Check queue size periodically
if client.event_queue.qsize() > 5000:
    logger.warning("Event queue is getting large")
    # Slow down or scale up
```

---

## Schema Updates

When the schema changes:

1. **Regenerate SDK**:
   ```bash
   ./bin/ingestkit sdk generate --lang python
   ```

2. **Update Application**:
   - Update imports if event names changed
   - Add fields for new event types
   - Update tests

3. **Deploy**:
   - Deploy new SDK with application
   - Ensure backwards compatibility if possible

---

## Additional Resources

- [Development Guide](development.md)
- [API Reference](../README.md#api-reference)
- [Example Applications](../examples/)
