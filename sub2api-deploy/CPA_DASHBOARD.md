# CPA Dashboard

The cloud bundle includes a lightweight `cpa-dashboard` service. It reads CPA quota snapshots from `cpa_monitor` and Sub2API usage rows from `public`, then serves a password-protected dashboard.

The first version is read-only:

- It does not modify CPA auth files.
- It does not delete or move cleanup candidates.
- It does not expose a SQL console.
- It does not attribute one Sub2API request to one internal CPA auth email.

## Required Environment

Set these values in the cloud `.env` file:

```bash
CPA_DASHBOARD_BIND_HOST=0.0.0.0
CPA_DASHBOARD_PORT=18090
CPA_DASHBOARD_LOGIN_PASSWORD=<long-random-dashboard-password>
CPA_DASHBOARD_DATABASE_USER=sub2api_dashboard
CPA_DASHBOARD_DATABASE_PASSWORD=<long-random-read-only-db-password>
```

The service connects to the existing Postgres container:

```bash
CPA_DASHBOARD_DATABASE_HOST=sub2api-postgres
CPA_DASHBOARD_DATABASE_PORT=5432
CPA_DASHBOARD_DATABASE_NAME=${POSTGRES_DB}
CPA_DASHBOARD_DATABASE_SSLMODE=disable
```

## Create The Read-Only Database Role

Run this once on the cloud server after `.env` is ready and Sub2API has created its tables:

```bash
bash sub2api-deploy/create-dashboard-db-role.sh
```

The script creates or updates the role from `CPA_DASHBOARD_DATABASE_USER` and `CPA_DASHBOARD_DATABASE_PASSWORD`.

Manual SQL is also possible. Replace the password placeholder with `CPA_DASHBOARD_DATABASE_PASSWORD` from `.env`, and replace `<POSTGRES_DB>` with `POSTGRES_DB`.

```sql
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'sub2api_dashboard') THEN
    CREATE ROLE sub2api_dashboard LOGIN PASSWORD '<CPA_DASHBOARD_DATABASE_PASSWORD>';
  ELSE
    ALTER ROLE sub2api_dashboard LOGIN PASSWORD '<CPA_DASHBOARD_DATABASE_PASSWORD>';
  END IF;
END $$;

GRANT CONNECT ON DATABASE <POSTGRES_DB> TO sub2api_dashboard;
GRANT USAGE ON SCHEMA public TO sub2api_dashboard;
GRANT USAGE ON SCHEMA cpa_monitor TO sub2api_dashboard;
GRANT SELECT ON public.usage_logs TO sub2api_dashboard;
GRANT SELECT ON public.accounts TO sub2api_dashboard;
GRANT SELECT ON public.api_keys TO sub2api_dashboard;
GRANT SELECT ON ALL TABLES IN SCHEMA cpa_monitor TO sub2api_dashboard;
ALTER DEFAULT PRIVILEGES IN SCHEMA cpa_monitor GRANT SELECT ON TABLES TO sub2api_dashboard;
```

One way to open `psql` on the server is:

```bash
docker exec -it sub2api-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB"
```

## Run

The dashboard service uses the `dashboard` Compose profile so a plain full-stack start does not fail before the read-only role is ready.

```bash
bash sub2api-deploy/create-dashboard-db-role.sh
docker compose -f docker-compose.cloud.yml --env-file .env up -d cpa-dashboard
docker logs cpa-dashboard
```

Open:

```text
http://<server-ip>:18090
```

## What The Dashboard Shows

- Overview: current account health, latest quota collection time, today usage, and 7-day normalized quota efficiency.
- Quota efficiency: CPA instance, model, API key, and time-range filters. The default range is the last 7 days.
- Account health: current CPA accounts grouped by account email, with the selected auth file and quota reset times.
- Usage trend: hourly Sub2API usage buckets with input, output, cache-read tokens, `total_cost`, and `actual_cost`.
- Cleanup candidates: failed or stale auth files that may need manual review.

## Main Metric

For each CPA instance, the dashboard sums only decreases in `weekly_remaining_percent`. If multiple accounts are active, the total can exceed `100%`.

Then it aligns that quota decrease with Sub2API usage from the same CPA instance in the same effective time range. The effective range is clipped to the successful quota collection window for each CPA instance:

```text
effective_start = max(requested_start, first_successful_quota_collection_at)
effective_end = min(requested_end, latest_successful_quota_collection_at)
```

This prevents usage or cost from before the quota collector existed from being normalized against newer quota samples.

```text
per 100% weekly quota = usage / observed_weekly_quota_decrease * 100
monthly estimate = per_100_percent * 4.3
estimated actual quota RMB = monthly_estimated_quota_cost * browser_input_factor
```

Token values are displayed in millions (`M`). Cost values are rounded to whole-dollar display values in the UI. The estimated actual quota RMB panel is a browser-only calculation with a default factor of `0.2`; it is not stored in Postgres and is not returned by the dashboard API.

Model and API-key filters apply to Sub2API usage. Quota consumption is still measured at CPA-instance level, so filtered efficiency is directional unless the selected traffic represents all traffic in that window.
