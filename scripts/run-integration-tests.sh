#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE_FILE=docker-compose.test.yml

echo "Starting test services with $COMPOSE_FILE"
docker compose -f "$COMPOSE_FILE" up -d --build

function wait_for() {
  local url=$1
  local timeout=${2:-60}
  echo "Waiting for $url (timeout ${timeout}s)"
  local start=$(date +%s)
  while true; do
    if curl -sSf "$url" >/dev/null 2>&1; then
      echo "$url is up"
      return 0
    fi
    now=$(date +%s)
    if (( now - start >= timeout )); then
      echo "Timed out waiting for $url" >&2
      return 1
    fi
    sleep 1
  done
}

# Wait for server and ml-service
wait_for "http://localhost:8080/api/v1/servicemap" 60

# ML service only accepts POST; wait with a small POST health check
function wait_for_post() {
  local url=$1
  local payload=$2
  local timeout=${3:-60}
  echo "Waiting for POST $url (timeout ${timeout}s)"
  local start=$(date +%s)
  while true; do
    if curl -sSf -X POST -H "Content-Type: application/json" -d "$payload" "$url" >/dev/null 2>&1; then
      echo "POST $url succeeded"
      return 0
    fi
    now=$(date +%s)
    if (( now - start >= timeout )); then
      echo "Timed out waiting for POST $url" >&2
      return 1
    fi
    sleep 1
  done
}

wait_for_post "http://localhost:5000/analyze" '{"application_id":"_health"}' 60 || true

echo "Running functional checks"
# Check service map
if ! curl -sSf "http://localhost:8080/api/v1/servicemap" | grep -q "services"; then
  echo "Service map check failed" >&2
  docker compose -f "$COMPOSE_FILE" logs --no-color
  docker compose -f "$COMPOSE_FILE" down
  exit 2
fi

# Check ML analyze endpoint with sample payload
RESP=$(curl -sSf -X POST http://localhost:5000/analyze -H "Content-Type: application/json" \
  -d '{"application_id":"integration-test","metrics":{"error_rate":0.01}}') || true
if [ -z "$RESP" ]; then
  echo "ML analyze endpoint failed or returned empty response" >&2
  docker compose -f "$COMPOSE_FILE" logs --no-color
  docker compose -f "$COMPOSE_FILE" down
  exit 3
fi

echo "Integration checks passed"

# Teardown
docker compose -f "$COMPOSE_FILE" down
