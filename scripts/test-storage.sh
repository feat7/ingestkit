#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
project="ingestkit-storage-$(date +%s)-$$"
compose=(docker compose -p "$project" -f tests/compose.yml)
trap '"${compose[@]}" down -v --remove-orphans >/dev/null' EXIT
"${compose[@]}" up -d --wait postgres
"${compose[@]}" exec -T postgres psql -U ingestkit -d ingestkit_test -v ON_ERROR_STOP=1 -c "CREATE TABLE legacy_sentinel(id int); INSERT INTO legacy_sentinel VALUES(7); CREATE TABLE schema_migrations(version bigint,dirty boolean); INSERT INTO schema_migrations VALUES(20251115110430,false);"
"${compose[@]}" run --rm migrate
port=$("${compose[@]}" port postgres 5432)
export INGESTKIT_TEST_DATABASE_URL="postgres://ingestkit:local-test-only@${port}/ingestkit_test?sslmode=disable"
go test -tags=integration ./internal/persistence -count=1 "$@"
