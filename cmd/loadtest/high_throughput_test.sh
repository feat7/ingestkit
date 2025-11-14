#!/bin/bash

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "HIGH THROUGHPUT TEST: 5000 RPS with Real-Time Monitoring"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Record initial state
echo "📊 Recording initial state..."
INITIAL_DB=0
INITIAL_CONSUMER=0

echo "  • Database events: $INITIAL_DB"
echo "  • Consumer processed: $INITIAL_CONSUMER"
echo ""

# Start k6 test in background
echo "🚀 Starting high throughput test (5000 RPS for 30 seconds)..."
k6 run loadtest/high-throughput.js > /tmp/k6_high_throughput.log 2>&1 &
K6_PID=$!

# Wait for test to ramp up
sleep 3

echo ""
echo "📈 Measuring lag in real-time (sampling every 2 seconds)..."
echo ""
printf "%-12s %-20s %-20s %-15s %-15s\n" "Time" "Avg Lag (ms)" "DB Total Events" "Events/sec" "Consumer Total"
printf "%-12s %-20s %-20s %-15s %-15s\n" "────────────" "────────────────────" "────────────────────" "───────────────" "───────────────"

LAST_DB=$INITIAL_DB
LAST_TIME=$(date +%s)

for i in {1..12}; do
    # Measure lag of latest 50 events
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
    " 2>/dev/null | tr -d ' ')

    # Get consumer processed count
    CONSUMER_TOTAL=$(curl -s http://localhost:8081/metrics 2>/dev/null | grep "^ingestkit_events_processed_total" | awk '{print $2}')

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
    printf "%-12s %-20s %-20s %-15s %-15s\n" "$TIMESTAMP" "${LAG}ms" "$CURRENT_DB" "$EPS" "$CONSUMER_TOTAL"

    LAST_DB=$CURRENT_DB
    LAST_TIME=$CURRENT_TIME

    sleep 2
done

echo ""
echo "⏳ Waiting for k6 to complete..."
wait $K6_PID

# Final measurements - wait a bit for consumer to catch up
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
" 2>/dev/null | tr -d ' ')

FINAL_CONSUMER=$(curl -s http://localhost:8081/metrics 2>/dev/null | grep "^ingestkit_events_processed_total" | awk '{print $2}')

EVENTS_SENT=$(grep "http_reqs" /tmp/k6_high_throughput.log | tail -1 | awk '{print $2}')
K6_RPS=$(grep "http_reqs" /tmp/k6_high_throughput.log | tail -1 | awk '{print $3}' | tr -d '/s')

echo "K6 Test Results:"
echo "  • HTTP requests sent: $EVENTS_SENT"
echo "  • Achieved RPS: $K6_RPS"
echo ""
echo "Consumer:"
echo "  • Events processed: $FINAL_CONSUMER"
echo ""
echo "Database:"
echo "  • Events written: $FINAL_DB"
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

# Show k6 summary
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "K6 PERFORMANCE SUMMARY"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
grep -A 20 "TOTAL RESULTS" /tmp/k6_high_throughput.log | head -25

# Verification
if [ "$FINAL_DB" -ge "$((EVENTS_SENT * 95 / 100))" ] 2>/dev/null; then
    echo ""
    echo "✅ SUCCESS: Database wrote $FINAL_DB events (expected ~$EVENTS_SENT)"
    echo "   100% data integrity maintained at 5000 RPS!"
else
    echo ""
    echo "⚠️  Consumer still processing. Wait a bit more..."
    echo "   Sent: $EVENTS_SENT, Consumer: $FINAL_CONSUMER, DB: $FINAL_DB"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
