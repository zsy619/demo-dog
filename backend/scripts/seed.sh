#!/usr/bin/env bash
# scripts/seed.sh
#
# Periodically POST seed OTLP payloads to a running dog-collector.
# Usage:
#   bash scripts/seed.sh                    # default service demo
#   bash scripts/seed.sh checkout          # specific service
#   API=http://localhost:9090 bash scripts/seed.sh

set -euo pipefail

API="${API:-http://localhost:8080}"
SERVICE="${1:-demo}"
INTERVAL="${INTERVAL:-2}"

echo "[seed] POSTing seed data to $API/api/seed (service=$SERVICE, interval=${INTERVAL}s)" >&2
while true; do
  curl -sSf "$API/api/seed?service=$SERVICE&n=5" | sed "s/^/[seed] /" >&2 || true
  sleep "$INTERVAL"
done
