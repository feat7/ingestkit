# IngestKit Examples

Complete example applications demonstrating IngestKit integration in real-world scenarios.

## Overview

This directory contains:

1. **Quick Start Examples** - Simple scripts to test IngestKit
2. **Complete Applications** - Full-featured example apps with Prisma-style workflow

## Complete Example Applications

### E-Commerce Express (TypeScript)

**Location**: `ecommerce-express/`

A complete e-commerce API built with Express.js showing event tracking for:
- Product views with source attribution
- Add to cart actions
- Checkout flow
- Order completion with discounts

**Tech Stack**: Express.js, TypeScript, IngestKit auto-generated client

**Features**:
- 📦 Realistic product catalog
- 🛒 Shopping cart management
- 💰 Discount code support
- 📊 Complete purchase funnel tracking
- ✅ Type-safe event tracking

[**View Example →**](./ecommerce-express/)

### Blog Analytics (Python)

**Location**: `blog-flask/`

A blog platform built with Flask demonstrating content analytics:
- Article views and reading analytics
- Social sharing tracking
- Comment engagement (including replies)
- Newsletter subscriptions
- Search query analysis

**Tech Stack**: Flask, Python, Pydantic models, IngestKit auto-generated client

**Features**:
- 📰 Multi-category blog articles
- 🔍 Full-text search
- 💬 Nested comment system
- 📧 Newsletter subscriptions
- ✅ Pydantic validation

[**View Example →**](./blog-flask/)

### Why These Examples Are Special

Both complete examples demonstrate the **Prisma-style workflow** that makes IngestKit easy to integrate:

1. **Initialize** - One command creates project structure
   ```bash
   npx ingestkit init --typescript  # or --python
   ```

2. **Define Events** - Edit `ingestkit/schema.yaml` with your events
   ```yaml
   events:
     product_viewed:
       fields:
         user_id: { type: string, required: true }
         product_id: { type: string, required: true }
   ```

3. **Generate Client** - One command creates type-safe client
   ```bash
   npx ingestkit generate
   ```

4. **Use in Code** - Import and use with full type safety
   ```typescript
   import { Client } from './ingestkit'
   const analytics = new Client()
   await analytics.productViewed.send({ userId, productId })
   ```

**No manual SDK copying, no configuration hassle** - just like Prisma, but for event tracking!

## Quick Start Examples

## Prerequisites

1. **Start IngestKit**:
   ```bash
   cd ..
   make quickstart
   ```

2. **Generate SDKs**:
   ```bash
   # Python SDK
   ./bin/ingestkit sdk generate --lang python

   # TypeScript SDK
   ./bin/ingestkit sdk generate --lang typescript
   ```

## Run Examples

### Python Example

```bash
# Install dependencies
pip install requests pydantic

# Run example
python examples/python-quickstart.py
```

**Expected Output**:
```
🚀 IngestKit Python SDK - Quick Start Example

✓ Client initialized

📝 Example 1: Send user signup event
   ✓ User signup tracked: {'success': True, 'event_id': 12345}

💳 Example 2: Send purchase event
   ✓ Purchase tracked: {'success': True, 'event_id': 12346}

👁️  Example 3: Send page view event
   ✓ Page view tracked: {'success': True, 'event_id': 12347}

📦 Example 4: Send batch of signups
   ✓ Batch tracked: {'success': True, 'events_received': 10}

==================================================
✅ All events sent successfully!
==================================================
```

### TypeScript Example

```bash
# Install ts-node if needed
npm install -g ts-node typescript

# Run example
npx ts-node examples/typescript-quickstart.ts
```

## Verify Events Were Saved

Connect to database and query:

```bash
make db-connect
```

Then run:

```sql
-- View all events for this demo
SELECT * FROM events_user_signup
WHERE tenant_id = 'quickstart-demo'
ORDER BY timestamp DESC
LIMIT 10;

-- Count events by type
SELECT 'signups' as event_type, COUNT(*) as count
FROM events_user_signup
WHERE tenant_id = 'quickstart-demo'
UNION ALL
SELECT 'purchases', COUNT(*)
FROM events_purchase
WHERE tenant_id = 'quickstart-demo'
UNION ALL
SELECT 'page_views', COUNT(*)
FROM events_page_view
WHERE tenant_id = 'quickstart-demo';
```

## What Each Example Shows

### Python Example (`python-quickstart.py`)
- ✅ Pydantic models with validation
- ✅ Type-safe client methods
- ✅ Single event tracking
- ✅ Batch event tracking
- ✅ Error handling

### TypeScript Example (`typescript-quickstart.ts`)
- ✅ TypeScript interfaces
- ✅ Async/await patterns
- ✅ Fetch API usage
- ✅ Type checking at compile time
- ✅ Runtime error handling

## Running Complete Examples

### E-Commerce Express

**Quick Start (Recommended):**
```bash
cd ecommerce-express
./run.sh  # Automatic setup and start

# In another terminal, run the test flow
bash test-flow.sh
```

**Manual Setup:**
```bash
cd ecommerce-express
../../bin/ingestkit generate  # Generate TypeScript client
npm install
npm start

# In another terminal, run the test flow
bash test-flow.sh
```

### Blog Flask

**Quick Start (Recommended):**
```bash
cd blog-flask
./run.sh  # Automatic setup and start

# In another terminal, run the test
./test-flow-simple.sh
```

**Manual Setup:**
```bash
cd blog-flask
../../bin/ingestkit generate  # Generate Python client
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python app.py

# In another terminal, run the test
./test-flow-simple.sh
```

## Comparison: Quick Start vs Complete Examples

| Feature | Quick Start | Complete Examples |
|---------|-------------|-------------------|
| **Purpose** | Test IngestKit basics | Real-world integration patterns |
| **Complexity** | Single file script | Full application |
| **Events** | Generic (signup, purchase) | Domain-specific (product_viewed, article_shared) |
| **Schema** | Uses default schema | Custom schema per use case |
| **Workflow** | Old SDK generation | Prisma-style (`init` → `generate` → use) |
| **Use Case** | Quick testing | Learning integration patterns |
| **Time to Run** | 30 seconds | 2-3 minutes |

## Next Steps

1. **Run complete examples** - See realistic integration patterns
2. **Modify the examples** - Change tenant IDs, add more fields
3. **Create your own events** - Edit `ingestkit/schema.yaml` in examples
4. **Integrate into your app** - Follow the Prisma-style workflow
5. **Build analytics** - Write SQL queries on your events

## Troubleshooting

### "Connection refused"
- Make sure IngestKit is running: `make run-api` and `make run-consumer`
- Check API is accessible: `curl http://localhost:8080/health`

### "Authentication failed"
- Default API key is `dev_key_1234567890` (from `.env` file)
- Make sure you're using the correct key in examples

### "Validation error"
- Check your event matches the schema in `schema/events.yaml`
- Ensure required fields are provided
- Check enum values match schema

### Events not in database
- Check consumer is running: `curl http://localhost:8081/health`
- Check consumer metrics: `make metrics`
- Check dead letter queue: `make db-dlq-check`
