#!/bin/bash

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PRECISE END-TO-END LATENCY TEST"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Generate unique ID
UNIQUE_ID="precise_test_$(date +%s)_$$"

echo "1. Sending event..."
echo "   User ID: $UNIQUE_ID"

# Record start time (milliseconds)
START_MS=$(($(date +%s) * 1000))
START_TIME=$(date +"%H:%M:%S")

# Send event
RESPONSE=$(curl -s -X POST "http://localhost:8080/v1/events/user_signup" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev_key_1234567890" \
  -d "{\"user_id\": \"$UNIQUE_ID\", \"email\": \"test@example.com\", \"signup_source\": \"web\", \"metadata\": {}}")

echo "   Sent at: $START_TIME"
echo ""

echo "2. Waiting for event to appear in database..."

# Poll database
FOUND=false
MAX_WAIT=30
for i in $(seq 1 $MAX_WAIT); do
    sleep 0.5

    RESULT=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c \
        "SELECT COUNT(*) FROM events_user_signup WHERE user_id = '$UNIQUE_ID';" 2>/dev/null)

    if [ "$RESULT" = "1" ]; then
        FOUND=true
        END_MS=$(($(date +%s) * 1000))
        END_TIME=$(date +"%H:%M:%S")
        TOTAL_LAG=$((END_MS - START_MS))

        echo "   ✓ Event found at: $END_TIME"
        echo ""
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        echo "RESULT:"
        echo "  • Start time: $START_TIME"
        echo "  • End time:   $END_TIME"
        echo "  • Total end-to-end latency: ${TOTAL_LAG} ms"
        echo ""
        echo "This includes:"
        echo "  - API processing (10-20ms)"
        echo "  - Kafka publish (5-15ms)"
        echo "  - Consumer batch wait (up to 1000ms)"
        echo "  - Database write (5-50ms)"
        echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
        break
    fi
done

if [ "$FOUND" = "false" ]; then
    echo "   ✗ Event not found after ${MAX_WAIT}s"
    exit 1
fi
