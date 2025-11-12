#!/bin/bash

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to measure lag for a specific event type
measure_lag() {
    local event_type=$1
    local table_name="events_${event_type}"

    # Query to get latest events and calculate lag
    result=$(docker-compose exec -T postgres psql -U ingestkit -d ingestkit -t -A -c "
        WITH latest_events AS (
            SELECT
                event_id,
                timestamp,
                NOW() - timestamp AS lag
            FROM ${table_name}
            ORDER BY event_id DESC
            LIMIT 10
        )
        SELECT
            COUNT(*) as count,
            ROUND(AVG(EXTRACT(EPOCH FROM lag) * 1000)) as avg_lag_ms,
            ROUND(MIN(EXTRACT(EPOCH FROM lag) * 1000)) as min_lag_ms,
            ROUND(MAX(EXTRACT(EPOCH FROM lag) * 1000)) as max_lag_ms
        FROM latest_events;
    ")

    echo "$result"
}

# Function to display lag stats
display_lag() {
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}End-to-End Latency Measurement${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    for event_type in "user_signup" "purchase" "page_view"; do
        result=$(measure_lag "$event_type")

        if [ -n "$result" ] && [ "$result" != "|" ]; then
            IFS='|' read -r count avg min max <<< "$result"

            if [ -n "$count" ] && [ "$count" != "0" ]; then
                echo -e "${YELLOW}Event Type: ${event_type}${NC}"
                echo "  • Events sampled: $count"
                echo "  • Avg lag: ${avg} ms"
                echo "  • Min lag: ${min} ms"
                echo "  • Max lag: ${max} ms"
                echo ""
            fi
        fi
    done
}

# Main execution
case "$1" in
    "watch")
        echo -e "${BLUE}Watching latency in real-time (Ctrl+C to stop)...${NC}"
        echo ""
        while true; do
            clear
            display_lag
            sleep 2
        done
        ;;
    "once")
        display_lag
        ;;
    *)
        echo "Usage: $0 {watch|once}"
        echo ""
        echo "  watch - Continuously monitor latency every 2 seconds"
        echo "  once  - Display latency once and exit"
        exit 1
        ;;
esac
