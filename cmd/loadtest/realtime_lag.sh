#!/bin/bash

# Real-time lag measurement
# Sends a single event and measures how long it takes to appear in the database

API_URL="${API_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-dev_key_1234567890}"

# Generate unique user_id
UNIQUE_ID="lag_test_$$_$(date +%s)"

echo "Sending test event with user_id: $UNIQUE_ID"
START_TIME=$(date +%s%3N)  # milliseconds

# Send event
curl -s -X POST "$API_URL/v1/events/user_signup" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d "{
    \"user_id\": \"$UNIQUE_ID\",
    \"email\": \"test@example.com\",
    \"signup_source\": \"web\",
    \"metadata\": {}
  }" > /dev/null

echo "Event sent at: $START_TIME ms"
echo "Waiting for event to appear in database..."

# Poll database until event appears
FOUND=false
MAX_WAIT=30  # 30 seconds max wait
ELAPSED=0

while [ $FOUND = false ] && [ $ELAPSED -lt $MAX_WAIT ]; do
  RESULT=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
    SELECT
      COUNT(*),
      EXTRACT(EPOCH FROM timestamp) * 1000 as api_timestamp_ms
    FROM events_user_signup
    WHERE user_id = '$UNIQUE_ID'
  " 2>/dev/null)

  if [ -n "$RESULT" ]; then
    COUNT=$(echo "$RESULT" | cut -d'|' -f1)
    if [ "$COUNT" = "1" ]; then
      FOUND=true
      DB_TIME=$(date +%s%3N)
      API_TIMESTAMP=$(echo "$RESULT" | cut -d'|' -f2)

      # Calculate lags
      TOTAL_LAG=$((DB_TIME - START_TIME))

      echo ""
      echo "✓ Event found in database!"
      echo ""
      echo "Results:"
      echo "  • Total end-to-end latency: $TOTAL_LAG ms"
      echo "    (from API request to database write)"
      echo ""
      echo "Components:"
      echo "  • API processing + Kafka publish: ~10-20 ms"
      echo "  • Kafka → Consumer: ~instant"
      echo "  • Consumer batch wait: up to 1000 ms"
      echo "  • Database write: ~10-50 ms"
      break
    fi
  fi

  sleep 0.1  # Check every 100ms
  ELAPSED=$((ELAPSED + 1))
done

if [ $FOUND = false ]; then
  echo "✗ Event not found after ${MAX_WAIT}s"
  exit 1
fi
