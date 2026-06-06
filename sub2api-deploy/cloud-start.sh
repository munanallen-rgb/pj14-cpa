#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

SUB2API_PORT="${SUB2API_PORT:-18080}"
SUB2API_URL="${SUB2API_URL:-http://127.0.0.1:${SUB2API_PORT}}"
ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.cloud.yml}"
SKIP_BOOTSTRAP="${SKIP_BOOTSTRAP:-0}"
REQUIRE_ALL="${REQUIRE_ALL:-0}"
START_DASHBOARD="${START_DASHBOARD:-1}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Generating $ENV_FILE"
  python3 sub2api-deploy/cloud_tools.py generate-env --output "$ENV_FILE" --force
fi

echo "Starting Docker Compose stack"
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d

echo "Waiting for Sub2API health at $SUB2API_URL/health"
for attempt in $(seq 1 60); do
  if curl -fsS "$SUB2API_URL/health" >/dev/null 2>&1; then
    echo "Sub2API is healthy"
    break
  fi
  if [[ "$attempt" == "60" ]]; then
    echo "Timed out waiting for Sub2API health" >&2
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps
    exit 1
  fi
  sleep 2
done

if [[ "$SKIP_BOOTSTRAP" != "1" ]]; then
  echo "Bootstrapping Sub2API CPA pool"
  python3 sub2api-deploy/cloud_tools.py bootstrap \
    --base-url "$SUB2API_URL" \
    --env-file "$ENV_FILE" \
    --upstream-mode cloud
fi

if [[ "$START_DASHBOARD" == "1" ]]; then
  echo "Preparing CPA dashboard read-only database role"
  ENV_FILE="$ENV_FILE" COMPOSE_FILE="$COMPOSE_FILE" bash sub2api-deploy/create-dashboard-db-role.sh

  echo "Starting CPA dashboard"
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d cpa-dashboard
fi

echo "Stack status"
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps

echo "Basic verification"
VERIFY_ARGS=(
  sub2api-deploy/cloud_tools.py
  verify
  --sub2api-url "$SUB2API_URL"
)

if [[ "$REQUIRE_ALL" == "1" ]]; then
  VERIFY_ARGS+=(--require-all-cpa-models)
else
  VERIFY_ARGS+=(--skip-direct-cpa-checks)
fi

python3 "${VERIFY_ARGS[@]}"
