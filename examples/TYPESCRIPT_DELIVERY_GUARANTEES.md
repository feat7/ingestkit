# IngestKit TypeScript SDK - Delivery Guarantees

The IngestKit TypeScript SDK provides Promise-based delivery guarantees with callback support, similar to Kafka's producer API.

## Features

✅ **Promise-based confirmation** - Know when events are delivered
✅ **Success/failure callbacks** - Async notifications (like Sentry)
✅ **Native async/await support** - Clean TypeScript syntax
✅ **Browser and Node.js compatible** - Works everywhere
✅ **Type-safe** - Full TypeScript type definitions

---

## Pattern 1: Async/Await (Recommended)

**Best for:** TypeScript/JavaScript applications with async/await support
**Behavior:** Returns Promise that resolves when delivered

```typescript
import { IngestKitClient } from './ingestkit';

const client = new IngestKitClient({
  apiUrl: 'http://localhost:8080',
  apiKey: process.env.INGESTKIT_API_KEY,
  debug: true
});

// Await delivery confirmation
async function trackUserSignup(userId: string, email: string) {
  try {
    const result = await client.sendUserSignup({
      user_id: userId,
      email,
      signup_source: 'web'
    });

    console.log('✅ Event delivered:', result);
    // If you reach here, event is GUARANTEED delivered
  } catch (error) {
    console.error('❌ Failed to deliver event:', error);
    // Handle failure - maybe queue locally
  }
}
```

---

## Pattern 2: Fire-and-Forget with Callbacks

**Best for:** High-volume events where you want async notifications
**Behavior:** Returns instantly, callbacks notify you later

```typescript
import { IngestKitClient } from './ingestkit';

const client = new IngestKitClient({
  apiUrl: 'http://localhost:8080',
  apiKey: process.env.INGESTKIT_API_KEY,

  // Success callback
  onSuccess: (eventData, result) => {
    console.log('✅ Event delivered:', result.request_id);
    // Update metrics, log to Datadog, etc.
  },

  // Failure callback
  onFailure: (eventData, error) => {
    console.error('❌ Event failed:', error.message);
    // Write to local file, alert PagerDuty, etc.
    saveToLocalQueue(eventData);
  }
});

// Send events - callbacks handle the rest
function trackPageView(page: string) {
  // Returns immediately - callbacks called async
  client.sendPageView({ page, user_id: '123' });
  // Continue processing without waiting
}
```

---

## Pattern 3: Promise.race for Timeouts

**Best for:** Critical events with timeout requirements
**Behavior:** Fails fast if delivery takes too long

```typescript
async function trackPayment(payment: PaymentEvent) {
  try {
    const result = await Promise.race([
      client.sendPaymentCompleted(payment),
      new Promise((_, reject) =>
        setTimeout(() => reject(new Error('Timeout')), 5000)
      )
    ]);

    console.log('✅ Payment event delivered:', result);
    return { success: true };

  } catch (error) {
    if (error.message === 'Timeout') {
      console.warn('⚠️ Delivery timeout - event may still succeed');
      // Don't fail payment - event might still be delivered
    } else {
      console.error('❌ Delivery failed:', error);
    }

    // Queue for retry
    await queueForRetry(payment);
    return { success: false, queued: true };
  }
}
```

---

## Pattern 4: Promise.all for Batch Guarantees

**Best for:** Ensuring multiple related events all succeed
**Behavior:** All-or-nothing batch processing

```typescript
async function onboardUser(user: User) {
  try {
    // Send multiple related events
    await Promise.all([
      client.sendUserSignup({
        user_id: user.id,
        email: user.email,
        signup_source: 'web'
      }),
      client.sendEmailVerificationSent({
        user_id: user.id,
        email: user.email
      }),
      client.sendWelcomeEmailSent({
        user_id: user.id
      })
    ]);

    console.log('✅ All onboarding events delivered');
    // Safe to mark user as fully onboarded

  } catch (error) {
    console.error('❌ Some events failed:', error);
    // Rollback or retry
  }
}
```

---

## Pattern 5: Fire-and-Forget (No Callbacks)

**Best for:** Non-critical events where you don't care about delivery
**Behavior:** Fire and forget - Promise ignored

```typescript
function trackAnalytics(event: string) {
  // Don't await - just fire it off
  client.sendAnalyticsEvent({ event });

  // Continue immediately
  console.log('Event sent (fire-and-forget)');
}
```

---

## Express.js Integration

```typescript
import express from 'express';
import { IngestKitClient } from './ingestkit';

const app = express();
const ingestkit = new IngestKitClient({
  onSuccess: (data, result) => console.log('Event delivered:', result.request_id),
  onFailure: (data, error) => console.error('Event failed:', error)
});

// High-volume endpoint - fire and forget
app.get('/page/:page', async (req, res) => {
  // Don't await - return fast
  ingestkit.sendPageView({
    page: req.params.page,
    user_id: req.user?.id
  });

  res.send('OK');
});

// Critical endpoint - wait for delivery
app.post('/checkout', async (req, res) => {
  const order = await createOrder(req.body);

  try {
    // WAIT for event delivery before responding
    await ingestkit.sendOrderPlaced({
      order_id: order.id,
      amount: order.total,
      user_id: req.user.id
    });

    res.json({ order_id: order.id, status: 'confirmed' });
  } catch (error) {
    // Event delivery failed
    res.status(500).json({ error: 'Failed to track order' });
  }
});
```

---

## Next.js App Router Integration

```typescript
// app/actions/track-event.ts
'use server';

import { IngestKitClient } from '@/lib/ingestkit';

const client = new IngestKitClient({
  apiKey: process.env.INGESTKIT_API_KEY!,
  onFailure: (data, error) => {
    // Log to your error tracking service
    console.error('IngestKit event failed:', error);
  }
});

export async function trackUserAction(action: string, userId: string) {
  // Server action - can be async
  await client.sendUserAction({
    action,
    user_id: userId,
    timestamp: new Date().toISOString()
  });
}

// app/components/TrackButton.tsx
'use client';

import { trackUserAction } from '@/app/actions/track-event';

export function TrackButton() {
  return (
    <button onClick={() => {
      // Fire and forget - doesn't block UI
      trackUserAction('button_clicked', 'user123');
    }}>
      Click Me
    </button>
  );
}
```

---

## React Component Integration

```tsx
import { useEffect } from 'react';
import { IngestKitClient } from './ingestkit';

// Initialize once
const ingestkit = new IngestKitClient({
  apiKey: process.env.REACT_APP_INGESTKIT_API_KEY,
  onFailure: (data, error) => {
    console.error('Event tracking failed:', error);
  }
});

function ProductPage({ productId }: { productId: string }) {
  useEffect(() => {
    // Track page view on mount
    ingestkit.sendProductViewed({
      product_id: productId,
      timestamp: Date.now()
    });
  }, [productId]);

  const handleAddToCart = async () => {
    // Critical event - wait for confirmation
    try {
      await ingestkit.sendAddToCart({
        product_id: productId,
        user_id: getCurrentUser().id
      });

      showNotification('Added to cart');
    } catch (error) {
      showError('Failed to add to cart');
    }
  };

  return <button onClick={handleAddToCart}>Add to Cart</button>;
}
```

---

## Error Handling

```typescript
import { ClientError, ServerError, NetworkError } from './ingestkit';

async function trackEventWithRetry(event: any) {
  try {
    await client.sendEvent(event);
    console.log('✅ Event delivered');

  } catch (error) {
    if (error instanceof ClientError) {
      // 4xx - client error, don't retry
      console.error('Client error (won\'t retry):', error.statusCode);
      // Fix data and try again later

    } else if (error instanceof ServerError) {
      // 5xx - server error after retries exhausted
      console.error('Server error (retries exhausted):', error.statusCode);
      // Queue for manual review

    } else if (error instanceof NetworkError) {
      // Network error after retries exhausted
      console.error('Network error:', error.message);
      // Queue for retry when connection restored

    } else {
      console.error('Unknown error:', error);
    }
  }
}
```

---

## TypeScript Types

```typescript
import { IngestKitClient, IngestKitClientConfig, APIResponse } from './ingestkit';
import * as Models from './ingestkit/models';

// Type-safe event
const event: Models.UserSignup = {
  user_id: '123',
  email: 'user@example.com',
  signup_source: 'web', // Type error if invalid value!
  metadata: { source: 'campaign1' }
};

// Type-safe config
const config: IngestKitClientConfig = {
  apiUrl: 'http://localhost:8080',
  apiKey: 'key_123',
  debug: true,
  onSuccess: (eventData, result: APIResponse) => {
    console.log('Delivered:', result);
  },
  onFailure: (eventData, error: Error) => {
    console.error('Failed:', error);
  }
};

const client = new IngestKitClient(config);
```

---

## Advanced: Custom Retry Logic

```typescript
async function sendEventWithCustomRetry(
  event: any,
  maxRetries: number = 5,
  baseDelay: number = 1000
) {
  for (let attempt = 0; attempt < maxRetries; attempt++) {
    try {
      // Try with timeout
      const result = await Promise.race([
        client.sendEvent(event),
        new Promise((_, reject) =>
          setTimeout(() => reject(new Error('Custom timeout')), 10000)
        )
      ]);

      console.log(`✅ Delivered on attempt ${attempt + 1}`);
      return result;

    } catch (error) {
      if (attempt === maxRetries - 1) {
        console.error('❌ Failed after all retries');
        throw error;
      }

      const delay = baseDelay * Math.pow(2, attempt);
      console.warn(`⚠️ Attempt ${attempt + 1} failed, retrying in ${delay}ms`);
      await new Promise(resolve => setTimeout(resolve, delay));
    }
  }
}
```

---

## Comparison with Python SDK

| Feature | Python SDK | TypeScript SDK |
|---------|------------|----------------|
| Async handling | `Future` objects | `Promise` objects |
| Callbacks | `on_success`, `on_failure` | `onSuccess`, `onFailure` |
| Wait for delivery | `wait=True` parameter | `await` Promise |
| Timeout | `timeout` parameter | `Promise.race()` |
| Background worker | Threading | Native Promises |
| Context manager | `with Client():` | N/A (use try/finally) |

---

## Best Practices

### 1. Initialize Once, Use Everywhere

```typescript
// lib/ingestkit.ts
export const ingestkit = new IngestKitClient({
  apiKey: process.env.INGESTKIT_API_KEY!,
  onFailure: logFailureToDatadog
});

// pages/api/track.ts
import { ingestkit } from '@/lib/ingestkit';
await ingestkit.sendEvent(event);
```

### 2. Use Callbacks for Monitoring

```typescript
// Good ✅ - Global monitoring
const client = new IngestKitClient({
  onSuccess: (data, result) => {
    datadog.increment('ingestkit.events.success');
  },
  onFailure: (data, error) => {
    datadog.increment('ingestkit.events.failure');
    sentry.captureException(error);
  }
});
```

### 3. Await Only Critical Events

```typescript
// Good ✅ - Fast for high-volume, guaranteed for critical
client.sendPageView(event);  // Don't await
await client.sendPayment(payment);  // Await critical events

// Bad ❌ - Blocks on every event
await client.sendPageView(event);  // Slow!
```

### 4. Handle Errors Gracefully

```typescript
// Good ✅ - Don't fail request on tracking failure
try {
  await client.sendEvent(event);
} catch (error) {
  console.error('Tracking failed:', error);
  // Continue processing - don't fail the request
}

// Bad ❌ - Fails request if tracking fails
await client.sendEvent(event);  // Throws, breaks app
```

---

## Performance Characteristics

| Pattern | Latency | Use Case |
|---------|---------|----------|
| Fire-and-forget (no await) | <1ms | 99% of events |
| Fire-and-forget + callbacks | <1ms | Monitoring needed |
| Await Promise | 1-10ms | Critical events |
| Promise.race with timeout | 1-10ms or timeout | Time-sensitive |

---

## Summary

**TypeScript SDK provides three ways to handle events:**

1. **Fire-and-forget** (no await) - Fastest, use for 90% of events
2. **Await Promise** - Guaranteed delivery, use for critical events
3. **Callbacks** - Async notifications, use for monitoring

**Choose based on your needs:**
- High-volume events → Fire-and-forget
- Critical events → Await
- Want notifications → Add callbacks
- Need timeout → Use Promise.race

The TypeScript SDK is **fast by default** (non-blocking Promises) but **guaranteed when you need it** (await) - just like the Python SDK!
