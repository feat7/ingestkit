#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
INGESTKIT_INTEGRATION=1 go test -race -tags=integration ./tests/integration -count=1 "$@"
