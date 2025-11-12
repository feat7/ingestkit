#!/bin/bash

# This script runs a load test and measures lag concurrently

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "CONCURRENT LAG MEASUREMENT TEST"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Start k6 test in background
echo "Starting load test (500 RPS)..."
k6 run loadtest/baseline.js > /tmp/k6_output.log 2>&1 &
K6_PID=$!

# Wait for test to start
sleep 3

echo "Load test running, measuring lag every 2 seconds..."
echo ""

# Measure lag during the test
for i in {1..10}; do
    LAG=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
        SELECT ROUND(AVG(EXTRACT(EPOCH FROM (NOW() - timestamp)) * 1000))
        FROM (
            SELECT timestamp FROM events_user_signup ORDER BY event_id DESC LIMIT 100
        ) recent;
    " 2>/dev/null)

    TIMESTAMP=$(date +"%H:%M:%S")
    echo "[$TIMESTAMP] Latest 100 events average lag: ${LAG} ms"

    sleep 2
done

# Wait for k6 to finish
wait $K6_PID

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Test complete. See /tmp/k6_output.log for full results."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
