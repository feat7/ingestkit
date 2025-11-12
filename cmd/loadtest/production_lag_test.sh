#!/bin/bash

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "PRODUCTION TEST: 1000 RPS with Real-Time Lag Tracking"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Record initial state
echo "📊 Recording initial state..."
INITIAL_DB=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
    SELECT (SELECT COUNT(*) FROM events_user_signup) +
           (SELECT COUNT(*) FROM events_purchase) +
           (SELECT COUNT(*) FROM events_page_view);
" | tr -d ' ')

INITIAL_CONSUMER=$(curl -s http://localhost:8081/metrics | grep "^ingestkit_events_processed_total" | awk '{print $2}')

echo "  • Database events: $INITIAL_DB"
echo "  • Consumer processed: $INITIAL_CONSUMER"
echo ""

# Start k6 test in background
echo "🚀 Starting production test (1000 RPS for 60 seconds)..."
k6 run loadtest/production.js > /tmp/k6_production.log 2>&1 &
K6_PID=$!

# Wait for test to ramp up
sleep 5

echo ""
echo "📈 Measuring lag in real-time (sampling every 3 seconds)..."
echo ""
printf "%-12s %-20s %-20s %-15s\n" "Time" "Avg Lag (ms)" "DB Total Events" "Events/sec"
printf "%-12s %-20s %-20s %-15s\n" "────────────" "────────────────────" "────────────────────" "───────────────"

LAST_DB=$INITIAL_DB
LAST_TIME=$(date +%s)

for i in {1..15}; do
    # Measure lag of latest 100 events
    LAG=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
        SELECT ROUND(AVG(EXTRACT(EPOCH FROM (NOW() - timestamp)) * 1000))
        FROM (
            SELECT timestamp FROM events_user_signup ORDER BY event_id DESC LIMIT 50
        ) recent;
    " 2>/dev/null | tr -d ' ')

    # Get current DB count
    CURRENT_DB=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
        SELECT (SELECT COUNT(*) FROM events_user_signup) +
               (SELECT COUNT(*) FROM events_purchase) +
               (SELECT COUNT(*) FROM events_page_view);
    " | tr -d ' ')

    # Calculate events per second
    CURRENT_TIME=$(date +%s)
    TIME_DIFF=$((CURRENT_TIME - LAST_TIME))
    EVENT_DIFF=$((CURRENT_DB - LAST_DB))
    if [ $TIME_DIFF -gt 0 ]; then
        EPS=$((EVENT_DIFF / TIME_DIFF))
    else
        EPS=0
    fi

    TIMESTAMP=$(date +"%H:%M:%S")
    printf "%-12s %-20s %-20s %-15s\n" "$TIMESTAMP" "${LAG}ms" "$CURRENT_DB" "$EPS"

    LAST_DB=$CURRENT_DB
    LAST_TIME=$CURRENT_TIME

    sleep 3
done

echo ""
echo "⏳ Waiting for k6 to complete..."
wait $K6_PID

# Final measurements
sleep 5

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "FINAL VERIFICATION"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

FINAL_DB=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
    SELECT (SELECT COUNT(*) FROM events_user_signup) +
           (SELECT COUNT(*) FROM events_purchase) +
           (SELECT COUNT(*) FROM events_page_view);
" | tr -d ' ')

FINAL_CONSUMER=$(curl -s http://localhost:8081/metrics | grep "^ingestkit_events_processed_total" | awk '{print $2}')

EVENTS_SENT=$(grep "http_reqs" /tmp/k6_production.log | tail -1 | awk '{print $2}')
DB_DELTA=$((FINAL_DB - INITIAL_DB))
CONSUMER_DELTA=$((FINAL_CONSUMER - INITIAL_CONSUMER))

echo "K6 Test Results:"
echo "  • HTTP requests sent: $EVENTS_SENT"
echo ""
echo "Consumer:"
echo "  • Initial: $INITIAL_CONSUMER"
echo "  • Final: $FINAL_CONSUMER"
echo "  • Delta: $CONSUMER_DELTA events processed"
echo ""
echo "Database:"
echo "  • Initial: $INITIAL_DB"
echo "  • Final: $FINAL_DB"
echo "  • Delta: $DB_DELTA events written"
echo ""

# Check final lag
FINAL_LAG=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
    SELECT ROUND(AVG(EXTRACT(EPOCH FROM (NOW() - timestamp)) * 1000))
    FROM (
        SELECT timestamp FROM events_user_signup ORDER BY event_id DESC LIMIT 100
    ) recent;
" 2>/dev/null | tr -d ' ')

echo "Latest Events Lag:"
echo "  • Average lag of last 100 events: ${FINAL_LAG}ms"
echo ""

# Verification
if [ "$DB_DELTA" -ge "$((EVENTS_SENT * 95 / 100))" ] 2>/dev/null; then
    echo "✅ VERIFIED: Database wrote $DB_DELTA events (expected ~$EVENTS_SENT)"
else
    echo "⚠️  WARNING: Database delta ($DB_DELTA) doesn't match sent ($EVENTS_SENT)"
    echo "   Consumer may still be processing. Wait a few seconds and check again."
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
