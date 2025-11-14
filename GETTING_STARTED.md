# Getting Started with IngestKit

A complete guide for new developers to start using IngestKit in their applications.

---

## Quick Overview

**IngestKit** is a high-performance event ingestion platform. You define events in YAML, IngestKit generates:
- Database schema (PostgreSQL)
- Type-safe models
- Client SDKs (Python, TypeScript)
- Storage layer with automatic batching

---

## 5-Minute Quick Start

### Step 1: Start IngestKit

```bash
# Clone and start services
git clone <repo-url>
cd ingestkit
make quickstart
```

This starts:
- API server on `:8080`
- Consumer on `:8081`
- PostgreSQL on `:5433`
- Redpanda (Kafka) on `:19092`

### Step 2: Generate SDK for Your Language

```bash
# Build the CLI
go build -o bin/ingestkit ./cmd/cli

# Generate Python SDK
./bin/ingestkit sdk generate --lang python

# OR Generate TypeScript SDK
./bin/ingestkit sdk generate --lang typescript
```

### Step 3: Send Your First Event

Choose your language below:

---

## Python SDK Usage

### Installation

```bash
# Install dependencies
pip install requests pydantic

# Copy generated SDK to your project
cp -r generated/sdk/python/ myproject/ingestkit/
```

### Complete Example

```python
# example.py
from ingestkit import Client, UserSignup, Purchase
# Or use: from ingestkit import IngestKitClient  (both work, Client is an alias)

# Initialize client
client = Client(
    api_url="http://localhost:8080",
    api_key="dev_key_1234567890"  # From .env file
)

# Example 1: Send a user signup event
signup_event = UserSignup(
    tenant_id="acme-corp",
    user_id="user_123",
    email="alice@example.com",
    signup_source="web",
    utm_campaign="spring_2024"
)

response = client.send_user_signup(signup_event)
print(f"✓ Event sent: {response}")

# Example 2: Send a purchase event
purchase_event = Purchase(
    tenant_id="acme-corp",
    user_id="user_123",
    order_id="order_456",
    amount=99.99,
    currency="USD",
    payment_method="stripe",
    items={"sku": "PROD-001", "quantity": 2}
)

response = client.send_purchase(purchase_event)
print(f"✓ Purchase tracked: {response}")

# Example 3: Send batch of events
batch = [signup_event, purchase_event]
# Note: Both events must be same type for batch
signups = [UserSignup(tenant_id="acme-corp", user_id=f"user_{i}", email=f"user{i}@example.com", signup_source="web") for i in range(10)]
response = client.send_user_signup_batch(signups)
print(f"✓ Batch sent: {response}")
```

### Run the Example

```bash
python example.py
```

### Output

```
✓ Event sent: {'success': True, 'event_id': 12345}
✓ Purchase tracked: {'success': True, 'event_id': 12346}
✓ Batch sent: {'success': True, 'events_received': 10}
```

---

## TypeScript SDK Usage

### Installation

```bash
# Copy generated SDK to your project
cp -r generated/sdk/typescript/* src/ingestkit/
```

### Complete Example

```typescript
// example.ts
import { IngestKitClient, Models } from './ingestkit';

// Initialize client
const client = new IngestKitClient({
    apiUrl: "http://localhost:8080",
    apiKey: "dev_key_1234567890"
});

// Example 1: Send a user signup event
const signupEvent: Models.UserSignup = {
    tenant_id: "acme-corp",
    user_id: "user_123",
    email: "alice@example.com",
    signup_source: "web",
    utm_campaign: "spring_2024"
};

const response = await client.sendUserSignup(signupEvent);
console.log(`✓ Event sent:`, response);

// Example 2: Send a purchase event
const purchaseEvent: Models.Purchase = {
    tenant_id: "acme-corp",
    user_id: "user_123",
    order_id: "order_456",
    amount: 99.99,
    currency: "USD",
    payment_method: "stripe",
    items: { sku: "PROD-001", quantity: 2 }
};

const purchaseResponse = await client.sendPurchase(purchaseEvent);
console.log(`✓ Purchase tracked:`, purchaseResponse);

// Example 3: Send batch of events
const signups: Models.UserSignup[] = Array.from({ length: 10 }, (_, i) => ({
    tenant_id: "acme-corp",
    user_id: `user_${i}`,
    email: `user${i}@example.com`,
    signup_source: "web"
}));

const batchResponse = await client.sendUserSignupBatch(signups);
console.log(`✓ Batch sent:`, batchResponse);
```

### Run the Example

```bash
# With Node.js
npx ts-node example.ts

# OR compile and run
tsc example.ts && node example.js
```

---

## Go Usage (Manual Client)

Go models are auto-generated, but you need to send HTTP requests manually (Go client SDK coming soon).

### Complete Example

```go
// example.go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"

    "github.com/feat7/ingestkit/generated/models"
)

type IngestKitClient struct {
    APIURL string
    APIKey string
    Client *http.Client
}

func NewClient(apiURL, apiKey string) *IngestKitClient {
    return &IngestKitClient{
        APIURL: apiURL,
        APIKey: apiKey,
        Client: &http.Client{},
    }
}

func (c *IngestKitClient) SendEvent(eventType string, event interface{}) error {
    url := fmt.Sprintf("%s/v1/events/%s", c.APIURL, eventType)

    jsonData, err := json.Marshal(event)
    if err != nil {
        return fmt.Errorf("failed to marshal event: %w", err)
    }

    req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        return fmt.Errorf("failed to create request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))

    resp, err := c.Client.Do(req)
    if err != nil {
        return fmt.Errorf("request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API error: status %d", resp.StatusCode)
    }

    return nil
}

func main() {
    // Initialize client
    client := NewClient("http://localhost:8080", "dev_key_1234567890")

    // Example 1: Send a user signup event
    signup := &models.UserSignup{
        TenantID:     "acme-corp",
        UserId:       "user_123",
        Email:        "alice@example.com",
        SignupSource: "web",
        UtmCampaign:  "spring_2024",
    }

    if err := client.SendEvent("user_signup", signup); err != nil {
        panic(err)
    }
    fmt.Println("✓ User signup event sent")

    // Example 2: Send a purchase event
    purchase := &models.Purchase{
        TenantID:      "acme-corp",
        UserId:        "user_123",
        OrderId:       "order_456",
        Amount:        99.99,
        Currency:      "USD",
        PaymentMethod: "stripe",
    }

    if err := client.SendEvent("purchase", purchase); err != nil {
        panic(err)
    }
    fmt.Println("✓ Purchase event sent")
}
```

### Run the Example

```bash
go run example.go
```

---

## Real-World Integration Examples

### Example 1: Express.js App (TypeScript)

```typescript
// server.ts
import express from 'express';
import { IngestKitClient, Models } from './ingestkit';

const app = express();
const ingestkit = new IngestKitClient({
    apiUrl: process.env.INGESTKIT_URL || "http://localhost:8080",
    apiKey: process.env.INGESTKIT_API_KEY || "dev_key_1234567890"
});

app.use(express.json());

// Track user signups
app.post('/api/signup', async (req, res) => {
    const { email, userId } = req.body;

    // Send event to IngestKit
    const event: Models.UserSignup = {
        tenant_id: "my-app",
        user_id: userId,
        email: email,
        signup_source: "api"
    };

    try {
        await ingestkit.sendUserSignup(event);
        console.log(`✓ Tracked signup for ${email}`);
    } catch (error) {
        console.error('Failed to track event:', error);
        // Don't fail the request if tracking fails
    }

    res.json({ success: true, userId });
});

app.listen(3000, () => {
    console.log('Server running on :3000');
});
```

### Example 2: Django App (Python)

```python
# views.py
from django.http import JsonResponse
from django.views.decorators.http import require_POST
from ingestkit import IngestKitClient, Purchase
import os

# Initialize client once
ingestkit = IngestKitClient(
    api_url=os.getenv('INGESTKIT_URL', 'http://localhost:8080'),
    api_key=os.getenv('INGESTKIT_API_KEY', 'dev_key_1234567890')
)

@require_POST
def checkout(request):
    # Process payment...
    order = process_order(request)

    # Track purchase event
    event = Purchase(
        tenant_id="my-shop",
        user_id=request.user.id,
        order_id=order.id,
        amount=order.total,
        currency="USD",
        payment_method=request.POST.get('payment_method'),
        items=order.items_json()
    )

    try:
        ingestkit.send_purchase(event)
        print(f"✓ Tracked purchase {order.id}")
    except Exception as e:
        print(f"Warning: Failed to track event: {e}")
        # Don't fail checkout if tracking fails

    return JsonResponse({'order_id': order.id})
```

### Example 3: Go Web Service

```go
// handlers.go
package main

import (
    "encoding/json"
    "net/http"

    "github.com/feat7/ingestkit/generated/models"
)

var ingestkitClient *IngestKitClient

func init() {
    ingestkitClient = NewClient(
        "http://localhost:8080",
        "dev_key_1234567890",
    )
}

func pageViewHandler(w http.ResponseWriter, r *http.Request) {
    // Track page view
    event := &models.PageView{
        TenantID:  "my-site",
        SessionId: getSessionID(r),
        PageUrl:   r.URL.String(),
        PageTitle: r.URL.Query().Get("title"),
        Referrer:  r.Referer(),
    }

    // Send event (async, don't block request)
    go func() {
        if err := ingestkitClient.SendEvent("page_view", event); err != nil {
            log.Printf("Failed to track page view: %v", err)
        }
    }()

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "tracked",
    })
}
```

---

## Complete Working Examples

IngestKit includes production-ready example applications you can run immediately:

### Blog Analytics (Python/Flask)

Location: `examples/blog-flask/`

A complete blog application tracking:
- Article views with metadata (author, category, read time)
- Search queries and results
- Social sharing across platforms
- Comment posting
- Newsletter subscriptions

**Run it:**
```bash
cd examples/blog-flask
./run.sh  # Starts Flask app on :5002

# Test the complete user journey
./test-flow-simple.sh
```

**Features:**
- Auto-configured client (`Client()` reads from `ingestkit.config.json`)
- Environment variable substitution
- Accepts both dictionaries and Pydantic models
- Production-ready error handling

**View tracked data:**
```sql
-- Top articles by views
SELECT article_title, author, category, COUNT(*) as views
FROM events_article_viewed
GROUP BY article_title, author, category
ORDER BY views DESC
LIMIT 10;

-- Search analytics
SELECT query, AVG(results_count)::int as avg_results, COUNT(*) as searches
FROM events_search_performed
GROUP BY query
ORDER BY searches DESC;
```

### E-commerce (TypeScript/Express)

Location: `examples/ecommerce-express/`

Coming soon: Product catalog, shopping cart, checkout funnel.

---

## Querying Your Data

After events are ingested, query them directly in PostgreSQL:

```sql
-- Connect to database
psql -h localhost -p 5433 -U ingestkit -d ingestkit

-- View recent signups
SELECT
    user_id,
    email,
    signup_source,
    timestamp
FROM events_user_signup
WHERE tenant_id = 'acme-corp'
ORDER BY timestamp DESC
LIMIT 10;

-- Purchase analytics
SELECT
    DATE(timestamp) as date,
    COUNT(*) as orders,
    SUM(amount) as revenue,
    AVG(amount) as avg_order_value
FROM events_purchase
WHERE tenant_id = 'acme-corp'
  AND timestamp > NOW() - INTERVAL '30 days'
GROUP BY DATE(timestamp)
ORDER BY date DESC;

-- Funnel analysis
SELECT
    COUNT(DISTINCT s.user_id) as signups,
    COUNT(DISTINCT p.user_id) as purchasers,
    ROUND(COUNT(DISTINCT p.user_id)::decimal / COUNT(DISTINCT s.user_id) * 100, 2) as conversion_rate
FROM events_user_signup s
LEFT JOIN events_purchase p ON s.user_id = p.user_id
WHERE s.tenant_id = 'acme-corp'
  AND s.timestamp > NOW() - INTERVAL '30 days';
```

---

## Adding New Event Types

### 1. Define Event in Schema

Edit `schema/events.yaml`:

```yaml
events:
  video_watched:
    description: Fired when a user watches a video
    fields:
      user_id:
        type: string
        required: true
        indexed: true
        description: User who watched the video
      video_id:
        type: string
        required: true
        indexed: true
        description: Video identifier
      duration_seconds:
        type: integer
        required: true
        description: Video duration in seconds
      watch_percentage:
        type: integer
        required: true
        description: Percentage of video watched (0-100)
      metadata:
        type: jsonb
        description: Additional video metadata
```

### 2. Regenerate Code

```bash
# Regenerate SQL, Go models, storage, and consumer
./bin/ingestkit schema compile

# Regenerate Python SDK
./bin/ingestkit sdk generate --lang python

# Regenerate TypeScript SDK
./bin/ingestkit sdk generate --lang typescript

# Apply database changes
make db-create
```

### 3. Use New Event

**Python:**
```python
from ingestkit import VideoWatched

event = VideoWatched(
    tenant_id="my-app",
    user_id="user_123",
    video_id="vid_abc",
    duration_seconds=120,
    watch_percentage=75
)

client.send_video_watched(event)
```

**TypeScript:**
```typescript
const event: Models.VideoWatched = {
    tenant_id: "my-app",
    user_id: "user_123",
    video_id: "vid_abc",
    duration_seconds: 120,
    watch_percentage: 75
};

await client.sendVideoWatched(event);
```

---

## Production Deployment

### Environment Variables

```bash
# .env.production
API_PORT=8080
DB_HOST=postgres.production.com
DB_PORT=5432
DB_NAME=ingestkit_prod
DB_USER=ingestkit
DB_PASSWORD=<secure-password>

REDPANDA_ADDR=redpanda.production.com:9092
REDPANDA_TOPIC=ingestkit.events.prod

# Change this!
API_KEY_1=prod_<random-secure-key>:acme-corp
API_KEY_2=prod_<random-secure-key>:another-tenant

RATE_LIMIT_RPS=5000
CONSUMER_WORKERS=8
CONSUMER_BATCH_SIZE=1000
```

### Docker Deployment

```bash
# Build
docker build -t ingestkit-api -f cmd/api/Dockerfile .
docker build -t ingestkit-consumer -f cmd/consumer/Dockerfile .

# Run API (3 replicas)
docker run -d --name ingestkit-api-1 \
  --env-file .env.production \
  -p 8080:8080 \
  ingestkit-api

# Run Consumer (2 replicas)
docker run -d --name ingestkit-consumer-1 \
  --env-file .env.production \
  ingestkit-consumer
```

### Monitoring

```bash
# Check API health
curl http://localhost:8080/health

# Check consumer metrics
curl http://localhost:8081/metrics

# View consumer stats
make metrics
```

---

## Troubleshooting

### Event Not Appearing in Database

```bash
# 1. Check API received the event
curl http://localhost:8080/health

# 2. Check consumer is running
curl http://localhost:8081/health

# 3. Check consumer metrics
make metrics

# 4. Check dead letter queue
make db-dlq-check

# 5. Check consumer logs
# Look for unmarshaling errors or DB write failures
```

### Validation Errors

```python
# Common validation errors:

# 1. Missing required field
event = UserSignup(
    tenant_id="test",
    # Missing user_id - will fail validation!
    email="test@example.com"
)

# 2. Invalid enum value
event = Purchase(
    tenant_id="test",
    user_id="123",
    order_id="ord_1",
    amount=99.99,
    currency="INVALID",  # Should be USD, EUR, GBP, or INR
    payment_method="card"
)

# 3. Type mismatch
event = Purchase(
    tenant_id="test",
    user_id="123",
    order_id="ord_1",
    amount="99.99",  # Should be float, not string!
    currency="USD"
)
```

### Rate Limiting

```bash
# If you see 429 Too Many Requests:

# Increase rate limit in .env
RATE_LIMIT_RPS=10000

# Or batch your events
events = [event1, event2, event3]
client.send_user_signup_batch(events)  # 1 request instead of 3
```

---

## SDK Reference

### Python SDK

**Client Methods:**
- `IngestKitClient(api_url: str, api_key: str, timeout: int = 30)`
- `send_{event_type}(event: EventModel) -> Dict`
- `send_{event_type}_batch(events: List[EventModel]) -> Dict`

**Event Models:**
- All models are Pydantic `BaseModel` subclasses
- Automatic validation on instantiation
- Type hints for IDE autocomplete

### TypeScript SDK

**Client Methods:**
- `new IngestKitClient({ apiUrl, apiKey, timeout? })`
- `send{EventType}(event: Models.EventType): Promise<APIResponse>`
- `send{EventType}Batch(events: Models.EventType[]): Promise<APIResponse>`

**Event Interfaces:**
- All events are TypeScript interfaces
- Compile-time type checking
- IntelliSense support

---

## Next Steps

1. **Try the Examples**: Copy-paste the examples above and send your first events
2. **Define Your Events**: Edit `schema/events.yaml` with your event types
3. **Integrate**: Add IngestKit SDK to your application
4. **Query**: Write SQL queries to analyze your data
5. **Deploy**: Use Docker to deploy to production

---

## Support

- **Issues**: Check `ISSUES.md` for known issues
- **Architecture**: Read `CLAUDE.md` for system architecture
- **Performance**: See `LOADTEST.md` for load testing guide

---

## Comparison with Alternatives

| Feature | IngestKit | Segment | Mixpanel | Custom Solution |
|---------|-----------|---------|----------|-----------------|
| **Hosting** | Self-hosted | Cloud | Cloud | Self-hosted |
| **Cost** | Free (infra only) | $120+/mo | $89+/mo | Dev time |
| **Data Ownership** | Full | Limited | Limited | Full |
| **Throughput** | 15K+ events/sec | Limited by plan | Limited by plan | Varies |
| **Schema Validation** | ✅ Required | ❌ Optional | ❌ Optional | Manual |
| **Type Safety** | ✅ Auto-generated SDKs | ❌ Generic | ❌ Generic | Manual |
| **SQL Queries** | ✅ Direct PostgreSQL | ❌ API only | ❌ UI only | ✅ Varies |
| **Setup Time** | 5 minutes | 10 minutes | 10 minutes | Days/weeks |

**When to use IngestKit:**
- Need full data ownership and control
- High event volume (millions/day)
- Want to run SQL queries on raw data
- Need compliance (GDPR, HIPAA) with on-prem deployment
- Budget-conscious (vs $1000+/mo for Segment/Mixpanel)

**When NOT to use IngestKit:**
- Want managed service with zero ops
- Need pre-built dashboards and analytics UI
- Small volume (<100K events/day) where SaaS makes sense
