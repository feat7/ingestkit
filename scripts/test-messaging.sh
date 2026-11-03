#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
project="ingestkit-broker-$(date +%s)-$$"
export INGESTKIT_BROKER_PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1",0)); print(s.getsockname()[1]); s.close()')
compose=(docker compose -p "$project" -f tests/compose.yml)
trap '"${compose[@]}" down -v --remove-orphans >/dev/null' EXIT
"${compose[@]}" up -d --wait postgres redpanda
"${compose[@]}" run --rm migrate
"${compose[@]}" run --rm topics
port=$("${compose[@]}" port postgres 5432)
export INGESTKIT_TEST_DATABASE_URL="postgres://ingestkit:local-test-only@${port}/ingestkit_test?sslmode=disable"
export INGESTKIT_TEST_BROKER="127.0.0.1:${INGESTKIT_BROKER_PORT}"
go test -race -tags=integration ./internal/messaging -count=1 "$@"
