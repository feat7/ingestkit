# IngestKit Delivery Guarantees - Usage Examples

The IngestKit Python SDK now provides Kafka-style delivery guarantees with multiple usage patterns.

## Features

✅ **Future-based confirmation** - Know when events are delivered (like Kafka producer)
✅ **Success/Failure callbacks** - Async notifications (like Sentry)
✅ **Wait parameter** - Synchronous guarantees when needed (like Redis)
✅ **Context manager** - Auto-flush on exit
✅ **Non-blocking by default** - Fast performance

---

## Pattern 1: Fire-and-Forget with Callbacks (Recommended for Most Use Cases)

**Best for:** General event tracking, logging, analytics
**Behavior:** Returns instantly, callbacks notify you later

```python
from ingestkit import Client
import logging

logger = logging.getLogger(__name__)

def handle_success(event_data, result):
    """Called when event is delivered to Kafka"""
    logger.info(f"✅ Event delivered: {result['request_id']}")
    logger.debug(f"Event data: {event_data}")

def handle_failure(event_data, error):
    """Called when event fails after all retries"""
    logger.error(f"❌ Event failed: {error}")
    # Write to local file for manual retry
    with open('/var/log/ingestkit/failed_events.jsonl', 'a') as f:
        f.write(json.dumps(event_data) + '\n')

# Initialize client with callbacks
client = Client(
    on_success=handle_success,
    on_failure=handle_failure
)

# Send events - returns INSTANTLY
# Callbacks are called asynchronously in background thread
client.send_user_signup({
    "user_id": "123",
    "email": "user@example.com",
    "signup_source": "web"
})

client.send_article_viewed({
    "article_id": "post-456",
    "read_time": 120
})

# Continue processing immediately - no blocking!
```

---

## Pattern 2: Guaranteed Delivery with `wait=True` (Critical Events)

**Best for:** Payments, orders, critical state changes
**Behavior:** Blocks until server confirms delivery

```python
from ingestkit import Client

client = Client()

# Regular events - fire and forget
client.send_page_view({"page": "/home"})  # Returns instantly

# Critical event - wait for confirmation
try:
    result = client.send_payment_completed({
        "payment_id": "pay_123",
        "amount": 99.99,
        "user_id": "user_456"
    }, wait=True, timeout=5)  # Blocks up to 5 seconds

    print(f"✅ Payment event GUARANTEED delivered: {result['request_id']}")
    # If you reach here, event is in Kafka - guaranteed!

except TimeoutError:
    print("⚠️ Delivery timeout - event may still succeed later")

except ClientError as e:
    print(f"❌ 4xx error - event rejected: {e}")
    # Fix data and retry

except ServerError as e:
    print(f"❌ 5xx error after retries: {e}")
    # Server issue - maybe queue locally
```

---

## Pattern 3: Context Manager (Batch Processing)

**Best for:** Batch jobs, scripts, migrations
**Behavior:** Ensures ALL events delivered before continuing

```python
from ingestkit import Client

def import_users_from_csv(csv_file):
    """Import users and GUARANTEE all events delivered before returning"""

    with Client() as client:
        for row in csv.DictReader(csv_file):
            # Send events - non-blocking
            client.send_user_signup({
                "user_id": row['id'],
                "email": row['email'],
                "signup_source": "import"
            })

        # Context manager automatically waits for ALL events
        # Raises exception if any failed

    print("✅ All user events guaranteed delivered!")
    # Safe to delete CSV or mark import complete
```

---

## Pattern 4: Manual Future Handling (Advanced)

**Best for:** Advanced async workflows, custom error handling
**Behavior:** Full control over Future lifecycle

```python
from ingestkit import Client
from concurrent.futures import Future, as_completed

client = Client()

# Send multiple events and collect Futures
futures = []

for user_id in user_ids:
    future = client.send_user_notification({
        "user_id": user_id,
        "message": "Welcome!"
    })
    futures.append((user_id, future))

# Do other work while events are being sent...
do_other_work()

# Wait for all events with custom timeout handling
for user_id, future in futures:
    try:
        result = future.result(timeout=10)
        print(f"✅ Notification sent to {user_id}")
    except TimeoutError:
        print(f"⚠️ Timeout for {user_id}")
    except Exception as e:
        print(f"❌ Failed for {user_id}: {e}")
```

---

## Pattern 5: Hybrid - Callbacks + Selective Wait

**Best for:** Applications with both high-volume and critical events
**Behavior:** Fast by default, guaranteed when needed

```python
from ingestkit import Client
import logging

logger = logging.getLogger(__name__)

# Setup global handlers for all events
def log_delivery(event_data, result):
    logger.info(f"Event delivered: {result['request_id']}")

def alert_failure(event_data, error):
    logger.error(f"Event failed: {error}")
    send_alert_to_pagerduty(event_data, error)

client = Client(
    on_success=log_delivery,
    on_failure=alert_failure
)

# Django view example
def process_order(request):
    # High-volume events - fire and forget
    client.send_page_view({
        "user_id": request.user.id,
        "page": request.path
    })

    # Create order
    order = Order.objects.create(...)

    # Critical event - wait for guarantee
    client.send_order_placed({
        "order_id": order.id,
        "amount": order.total,
        "user_id": request.user.id
    }, wait=True)  # Blocks until confirmed

    # Only mark order as "confirmed" if event delivery succeeded
    order.status = 'confirmed'
    order.save()

    return JsonResponse({"order_id": order.id})
```

---

## Comparison with Other Systems

### vs Redis SET

```python
# Redis - Synchronous guarantee
redis.set('key', 'value')  # Blocks until Redis confirms
# If this completes, data is guaranteed stored

# IngestKit - Same behavior with wait=True
client.send_event(event, wait=True)  # Blocks until Kafka confirms
# If this completes, event is guaranteed in Kafka
```

### vs Kafka Producer

```python
# Kafka producer
from kafka import KafkaProducer

producer = KafkaProducer(acks='all')
future = producer.send('topic', event)
metadata = future.get(timeout=10)  # Block for confirmation

# IngestKit - Identical API
client = Client()
future = client.send_event(event)
result = future.result(timeout=10)  # Block for confirmation
```

### vs Sentry Client

```python
# Sentry - Fire and forget with callbacks
import sentry_sdk

sentry_sdk.capture_exception(error)  # Non-blocking, sends async

# IngestKit - Same pattern
client = Client(on_failure=handle_failure)
client.send_event(event)  # Non-blocking, sends async
```

---

## Performance Characteristics

| Pattern | Returns In | Network Calls | Use Case |
|---------|-----------|---------------|----------|
| Fire-and-forget | Microseconds | Async | 99% of events |
| Fire-and-forget + callbacks | Microseconds | Async | Monitoring needed |
| wait=True | 1-10ms | Sync | Critical events |
| Context manager | Batch end | Sync | Batch processing |
| Manual Future | Microseconds | Async | Advanced control |

---

## Best Practices

### 1. Use Callbacks for Monitoring

```python
# ✅ Good - Global monitoring
client = Client(
    on_success=log_to_datadog,
    on_failure=alert_to_pagerduty
)
```

### 2. Use wait=True Sparingly

```python
# ✅ Good - Only for critical events
client.send_page_view(event)  # Fast
client.send_payment(payment, wait=True)  # Guaranteed

# ❌ Bad - Blocks on every event
for event in thousands_of_events:
    client.send_event(event, wait=True)  # Slow!
```

### 3. Use Context Manager for Batch Jobs

```python
# ✅ Good - Guaranteed completion
with Client() as client:
    for item in batch:
        client.send_event(item)
# All events delivered

# ❌ Bad - May lose events on crash
client = Client()
for item in batch:
    client.send_event(item)
# Script could exit before events sent
```

### 4. Handle Timeouts Gracefully

```python
# ✅ Good - Handle timeout without failing request
try:
    client.send_event(event, wait=True, timeout=2)
except TimeoutError:
    logger.warning("Delivery timeout - queued for retry")
    queue_for_retry(event)
    # Don't fail the request - event may still succeed

# ❌ Bad - Fail request on timeout
client.send_event(event, wait=True, timeout=2)  # Raises, breaks app
```

---

## Django Integration Example

```python
# settings.py
from ingestkit import Client

# Initialize once at startup
INGESTKIT_CLIENT = Client(
    on_success=lambda data, result: logger.info(f"Event sent: {result['request_id']}"),
    on_failure=lambda data, error: logger.error(f"Event failed: {error}")
)

# views.py
from django.conf import settings

def create_order(request):
    # Fast - no blocking
    settings.INGESTKIT_CLIENT.send_page_view({
        "user_id": request.user.id,
        "page": "/checkout"
    })

    order = Order.objects.create(...)

    # Guaranteed - blocks until confirmed
    settings.INGESTKIT_CLIENT.send_order_placed({
        "order_id": order.id
    }, wait=True)

    return JsonResponse({"order_id": order.id})
```

---

## Testing

```python
# In tests, use background=False for synchronous behavior
def test_event_sending():
    client = Client(background=False)  # Blocking mode

    response = client.send_user_signup({
        "user_id": "test_user"
    })

    # In blocking mode, response is immediate
    assert response['status'] == 'accepted'
```

---

## Migration Guide

### From Old SDK (No Guarantees)

```python
# Old code
client = Client()
client.send_event(event)  # No way to know if it worked

# New code - Add callbacks for visibility
client = Client(
    on_success=lambda data, result: print("✅ Delivered"),
    on_failure=lambda data, error: print(f"❌ Failed: {error}")
)
client.send_event(event)  # Same behavior, but with notifications
```

### Adding Guarantees to Critical Events

```python
# Old code - No guarantee
client.send_payment(payment)
mark_payment_as_sent()  # Might be premature!

# New code - Guaranteed
client.send_payment(payment, wait=True)  # Blocks until confirmed
mark_payment_as_sent()  # Safe - we know it's delivered
```

---

## Summary

Choose the pattern that fits your use case:

- **90% of events:** Fire-and-forget with callbacks
- **Critical events:** `wait=True` for guarantees
- **Batch jobs:** Context manager for auto-flush
- **Advanced:** Manual Future handling

The SDK is **fast by default** (non-blocking) but **guaranteed when you need it** - just like Kafka's producer API!
