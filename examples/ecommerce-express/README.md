# E-Commerce Express Example

A complete example showing how to integrate IngestKit into an Express.js e-commerce application for event tracking and analytics.

## What This Example Shows

This example demonstrates a realistic e-commerce application that tracks:

- **Product Views** - When users view product pages
- **Add to Cart** - When users add items to their cart
- **Checkout Started** - When users begin the checkout process
- **Order Completed** - When orders are successfully placed

## Architecture

```
Express.js API
    ↓
IngestKit TypeScript Client (auto-generated)
    ↓
IngestKit API Server
    ↓
PostgreSQL (for analytics)
```

## Setup Instructions

### 1. Prerequisites

Make sure IngestKit server is running:

```bash
# From the IngestKit root directory
cd ../..
make up              # Start PostgreSQL and Redpanda
make run-api         # Start API server (port 8080)
make run-consumer    # Start consumer (port 8081)
```

### 2. Initialize IngestKit (Already Done)

This example already has IngestKit initialized with:
- Custom schema for e-commerce events (`ingestkit/schema.yaml`)
- Configuration file (`ingestkit.config.json`)

To see how this was created:
```bash
# This is what was run (you don't need to run this)
# npx ingestkit init --typescript
# Then schema.yaml was customized for e-commerce events
```

### 3. Generate IngestKit Client

Generate the type-safe TypeScript client from the schema:

```bash
# Make sure you're in the example directory
cd examples/ecommerce-express

# Generate the client
../../bin/ingestkit generate
```

This creates:
- `ingestkit/client.ts` - Type-safe client with methods for each event
- `ingestkit/models.ts` - TypeScript interfaces
- `ingestkit/index.ts` - Barrel exports

### 4. Install Dependencies

```bash
npm install
```

### 5. Configure Environment

```bash
cp .env.example .env
```

The default API key (`dev_key_1234567890`) should work with the default IngestKit setup.

### 6. Run the Server

**Option A: Quick Start (Recommended)**

```bash
./run.sh
```

This script will:
- Check if IngestKit server is running
- Generate the client if needed
- Install dependencies if needed
- Start the Express app

**Option B: Manual Start**

```bash
npm start
```

The server will start on `http://localhost:3000`.

## Usage Examples

### 1. List Products

```bash
curl http://localhost:3000/products
```

### 2. View a Product (Tracks `product_viewed` event)

```bash
curl http://localhost:3000/products/prod_001 \
  -H "X-User-Id: user_123" \
  -H "X-Session-Id: session_456"
```

### 3. Add to Cart (Tracks `added_to_cart` event)

```bash
curl -X POST http://localhost:3000/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: user_123" \
  -H "X-Session-Id: session_456" \
  -d '{
    "productId": "prod_001",
    "quantity": 2
  }'
```

### 4. View Cart

```bash
curl http://localhost:3000/cart \
  -H "X-User-Id: user_123"
```

### 5. Start Checkout (Tracks `checkout_started` event)

```bash
curl -X POST http://localhost:3000/checkout/start \
  -H "Content-Type: application/json" \
  -H "X-User-Id: user_123" \
  -H "X-Session-Id: session_456"
```

### 6. Complete Order (Tracks `order_completed` event)

```bash
curl -X POST http://localhost:3000/checkout/complete \
  -H "Content-Type: application/json" \
  -H "X-User-Id: user_123" \
  -H "X-Session-Id: session_456" \
  -d '{
    "paymentMethod": "credit_card",
    "discountCode": "SAVE10",
    "shippingAddress": {
      "street": "123 Main St",
      "city": "San Francisco",
      "state": "CA",
      "zip": "94102",
      "country": "US"
    }
  }'
```

## Complete Purchase Flow

Here's a complete flow showing a user browsing and purchasing:

```bash
# Set user and session IDs
USER_ID="user_$(date +%s)"
SESSION_ID="session_$(date +%s)"

echo "User: $USER_ID"
echo "Session: $SESSION_ID"
echo

# 1. Browse products
echo "1. Viewing products..."
curl -s http://localhost:3000/products | jq '.products[] | "\(.id): \(.name) - $\(.price)"'
echo

# 2. View specific product
echo "2. Viewing Wireless Headphones..."
curl -s http://localhost:3000/products/prod_001 \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" | jq .
echo

# 3. Add to cart
echo "3. Adding to cart..."
curl -s -X POST http://localhost:3000/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" \
  -d '{"productId":"prod_001","quantity":2}' | jq .
echo

# 4. Add another item
echo "4. Adding another item..."
curl -s -X POST http://localhost:3000/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" \
  -d '{"productId":"prod_004","quantity":1}' | jq .
echo

# 5. View cart
echo "5. Viewing cart..."
curl -s http://localhost:3000/cart \
  -H "X-User-Id: $USER_ID" | jq .
echo

# 6. Start checkout
echo "6. Starting checkout..."
CART_ID=$(curl -s -X POST http://localhost:3000/checkout/start \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" | jq -r .cartId)
echo "Cart ID: $CART_ID"
echo

# 7. Complete order
echo "7. Completing order..."
curl -s -X POST http://localhost:3000/checkout/complete \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" \
  -d "{
    \"cartId\": \"$CART_ID\",
    \"paymentMethod\": \"credit_card\",
    \"discountCode\": \"SAVE10\",
    \"shippingAddress\": {
      \"street\": \"123 Main St\",
      \"city\": \"San Francisco\",
      \"state\": \"CA\",
      \"zip\": \"94102\",
      \"country\": \"US\"
    }
  }" | jq .
echo

echo "Complete purchase flow finished!"
```

Save this as `test-flow.sh` and run with `bash test-flow.sh`.

## Verify Events in Database

Connect to the database and query your events:

```bash
# From IngestKit root
make db-connect
```

Then run these queries:

```sql
-- View all product views for this demo
SELECT
  user_id,
  product_name,
  product_category,
  price,
  source,
  timestamp
FROM events_product_viewed
WHERE tenant_id = 'ecommerce-demo'
ORDER BY timestamp DESC
LIMIT 10;

-- View cart additions
SELECT
  user_id,
  product_name,
  quantity,
  price,
  cart_total,
  timestamp
FROM events_added_to_cart
WHERE tenant_id = 'ecommerce-demo'
ORDER BY timestamp DESC
LIMIT 10;

-- View completed orders
SELECT
  order_id,
  user_id,
  total_amount,
  payment_method,
  num_items,
  discount_code,
  discount_amount,
  timestamp
FROM events_order_completed
WHERE tenant_id = 'ecommerce-demo'
ORDER BY timestamp DESC
LIMIT 10;

-- Conversion funnel analysis
SELECT
  'Product Views' as stage,
  COUNT(*) as count,
  COUNT(DISTINCT user_id) as unique_users
FROM events_product_viewed
WHERE tenant_id = 'ecommerce-demo'
UNION ALL
SELECT
  'Added to Cart',
  COUNT(*),
  COUNT(DISTINCT user_id)
FROM events_added_to_cart
WHERE tenant_id = 'ecommerce-demo'
UNION ALL
SELECT
  'Checkout Started',
  COUNT(*),
  COUNT(DISTINCT user_id)
FROM events_checkout_started
WHERE tenant_id = 'ecommerce-demo'
UNION ALL
SELECT
  'Orders Completed',
  COUNT(*),
  COUNT(DISTINCT user_id)
FROM events_order_completed
WHERE tenant_id = 'ecommerce-demo';

-- Revenue by product
SELECT
  product_name,
  COUNT(*) as times_purchased,
  SUM(quantity) as total_quantity,
  SUM(price * quantity) as total_revenue
FROM events_added_to_cart
WHERE tenant_id = 'ecommerce-demo'
GROUP BY product_name
ORDER BY total_revenue DESC;
```

## Code Highlights

### How Events Are Tracked

In `server.js`, events are tracked using the auto-generated client:

```javascript
const { Client } = require('./ingestkit');
const analytics = new Client();

// Track product view
await analytics.productViewed.send({
  userId,
  sessionId,
  productId: product.id,
  productName: product.name,
  productCategory: product.category,
  price: product.price.toString(),
  currency: 'USD',
  source: req.query.source || 'direct',
});

// Track add to cart
await analytics.addedToCart.send({
  userId,
  sessionId,
  productId,
  productName,
  quantity,
  price: price.toString(),
  currency: 'USD',
  cartTotal: cartTotal.toString(),
});
```

### Custom Event Schema

The schema in `ingestkit/schema.yaml` defines e-commerce-specific events:

- **product_viewed** - Tracks product page views with source attribution
- **added_to_cart** - Tracks items added to cart with quantity and pricing
- **checkout_started** - Tracks checkout initiation with full cart details
- **order_completed** - Tracks successful orders with payment and shipping info

Each event includes:
- User and session identifiers
- Product/cart details
- Monetary amounts (as decimal for precision)
- JSONB metadata fields for flexible additional data

## Prisma-Style Workflow

This example demonstrates the Prisma-style workflow:

1. **Initialize** - `npx ingestkit init --typescript` creates the project structure
2. **Define Schema** - Edit `ingestkit/schema.yaml` with your events
3. **Generate Client** - `npx ingestkit generate` creates type-safe client
4. **Use in Code** - Import and use with full TypeScript support

Changes to the schema are reflected instantly after running `generate` again.

## What You Can Build

With this pattern, you can track and analyze:

- **Product Performance** - Which products get viewed most? Which convert best?
- **Conversion Funnels** - Where do users drop off in the purchase flow?
- **Cart Abandonment** - Who starts checkout but doesn't complete?
- **Revenue Analytics** - Revenue by product, category, time period
- **User Behavior** - Session analysis, repeat purchases, product affinities
- **Marketing Attribution** - Which sources drive the most valuable customers?

## Next Steps

1. **Customize Events** - Add more fields or events relevant to your business
2. **Add More Endpoints** - Track search, wishlists, reviews, returns
3. **Build Dashboards** - Create SQL queries or use BI tools
4. **A/B Testing** - Add experiment tracking to metadata
5. **Personalization** - Use event data to personalize recommendations

## Troubleshooting

### "Client not generated yet" error

Run `../../bin/ingestkit generate` from this directory.

### Events not appearing in database

1. Check consumer is running: `curl http://localhost:8081/health`
2. Check metrics: `make metrics` (from root)
3. Check DLQ: `make db-dlq-check`

### TypeScript errors

Make sure you've generated the client after any schema changes.

## Learn More

- [IngestKit README](../../README.md) - Full documentation
- [SDK Generation Guide](../../docs/sdk-generation.md) - CLI workflow guide
- [Schema Reference](../../schema/events.yaml) - Schema syntax examples
