#!/bin/bash

# Complete e-commerce purchase flow demonstration

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Generate unique user and session IDs
USER_ID="user_$(date +%s)"
SESSION_ID="session_$(date +%s)"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}IngestKit E-Commerce Example${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "User: $USER_ID"
echo "Session: $SESSION_ID"
echo

# 1. Browse products
echo -e "${GREEN}1. Browsing products...${NC}"
curl -s http://localhost:3000/products | jq '.products[] | "\(.id): \(.name) - $\(.price)"'
echo

# 2. View specific product
echo -e "${GREEN}2. Viewing Wireless Headphones...${NC}"
curl -s http://localhost:3000/products/prod_001 \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" | jq .product
echo

# 3. Add to cart
echo -e "${GREEN}3. Adding 2x Wireless Headphones to cart...${NC}"
curl -s -X POST http://localhost:3000/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" \
  -d '{"productId":"prod_001","quantity":2}' | jq '{success, cartTotal}'
echo

# 4. Add another item
echo -e "${GREEN}4. Adding 1x Laptop Stand to cart...${NC}"
curl -s -X POST http://localhost:3000/cart/add \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" \
  -d '{"productId":"prod_004","quantity":1}' | jq '{success, cartTotal}'
echo

# 5. View cart
echo -e "${GREEN}5. Viewing cart...${NC}"
curl -s http://localhost:3000/cart \
  -H "X-User-Id: $USER_ID" | jq '{items: .items | length, total}'
echo

# 6. Start checkout
echo -e "${GREEN}6. Starting checkout...${NC}"
CART_ID=$(curl -s -X POST http://localhost:3000/checkout/start \
  -H "Content-Type: application/json" \
  -H "X-User-Id: $USER_ID" \
  -H "X-Session-Id: $SESSION_ID" | jq -r .cartId)
echo "Cart ID: $CART_ID"
echo

# 7. Complete order
echo -e "${GREEN}7. Completing order with 10% discount...${NC}"
ORDER_RESPONSE=$(curl -s -X POST http://localhost:3000/checkout/complete \
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
  }")

ORDER_ID=$(echo $ORDER_RESPONSE | jq -r .orderId)
TOTAL=$(echo $ORDER_RESPONSE | jq -r .totalAmount)

echo "Order ID: $ORDER_ID"
echo "Total: \$$TOTAL (after 10% discount)"
echo

echo -e "${BLUE}========================================${NC}"
echo -e "${GREEN}✅ Complete purchase flow finished!${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "Events tracked:"
echo "  - 1x product_viewed"
echo "  - 2x added_to_cart"
echo "  - 1x checkout_started"
echo "  - 1x order_completed"
echo
echo "Query events in database with:"
echo "  make db-connect"
echo "  SELECT * FROM events_order_completed WHERE tenant_id = 'ecommerce-demo' ORDER BY timestamp DESC LIMIT 1;"
