#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.cloud.yml}"

if [[ -f "$ENV_FILE" ]]; then
  set -a
  # shellcheck disable=SC1090
  . "$ENV_FILE"
  set +a
fi

DASHBOARD_DB_USER="${CPA_DASHBOARD_DATABASE_USER:-sub2api_dashboard}"
DASHBOARD_DB_PASSWORD="${CPA_DASHBOARD_DATABASE_PASSWORD:-}"

if [[ -z "$DASHBOARD_DB_PASSWORD" ]]; then
  echo "CPA_DASHBOARD_DATABASE_PASSWORD is required in $ENV_FILE" >&2
  exit 1
fi

echo "Starting Postgres if needed"
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d sub2api-postgres

echo "Waiting for Postgres"
for attempt in $(seq 1 60); do
  if docker exec sub2api-postgres pg_isready -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" >/dev/null 2>&1; then
    break
  fi
  if [[ "$attempt" == "60" ]]; then
    echo "Timed out waiting for Postgres" >&2
    exit 1
  fi
  sleep 2
done

echo "Ensuring read-only role for CPA dashboard"
docker exec \
  -e DASHBOARD_DB_USER="$DASHBOARD_DB_USER" \
  -e DASHBOARD_DB_PASSWORD="$DASHBOARD_DB_PASSWORD" \
  sub2api-postgres sh -c '
PGOPTIONS="-c dashboard.user=$DASHBOARD_DB_USER -c dashboard.password=$DASHBOARD_DB_PASSWORD" \
psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" <<'"'"'SQL'"'"'
CREATE SCHEMA IF NOT EXISTS cpa_monitor;

DO $$
DECLARE
  dashboard_user text := current_setting('"'"'dashboard.user'"'"');
  dashboard_password text := current_setting('"'"'dashboard.password'"'"');
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = dashboard_user) THEN
    EXECUTE format('"'"'CREATE ROLE %I LOGIN PASSWORD %L'"'"', dashboard_user, dashboard_password);
  ELSE
    EXECUTE format('"'"'ALTER ROLE %I LOGIN PASSWORD %L'"'"', dashboard_user, dashboard_password);
  END IF;
END $$;

DO $$
DECLARE
  dashboard_user text := current_setting('"'"'dashboard.user'"'"');
BEGIN
  EXECUTE format('"'"'GRANT CONNECT ON DATABASE %I TO %I'"'"', current_database(), dashboard_user);
  EXECUTE format('"'"'GRANT USAGE ON SCHEMA public TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'GRANT USAGE ON SCHEMA cpa_monitor TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'GRANT SELECT ON TABLE public.usage_logs TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'GRANT SELECT ON TABLE public.accounts TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'GRANT SELECT ON TABLE public.api_keys TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'GRANT SELECT ON ALL TABLES IN SCHEMA cpa_monitor TO %I'"'"', dashboard_user);
  EXECUTE format('"'"'ALTER DEFAULT PRIVILEGES IN SCHEMA cpa_monitor GRANT SELECT ON TABLES TO %I'"'"', dashboard_user);
END $$;
SQL
'

echo "Dashboard database role is ready: $DASHBOARD_DB_USER"
