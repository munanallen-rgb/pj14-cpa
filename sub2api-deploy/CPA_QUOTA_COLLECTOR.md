# CPA Quota Collector

The cloud bundle includes a lightweight `cpa-quota-collector` service. It queries CPA1/CPA2/CPA3 management quota reports and writes quota snapshots to the existing Postgres container under the `cpa_monitor` schema.

## Required Environment

Set the shared CPA management key in `.env`:

```bash
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY=<cpa-management-key>
```

If CPA instances use different management keys, set per-instance overrides:

```bash
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA1=<cpa1-key>
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA2=<cpa2-key>
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA3=<cpa3-key>
```

The default internal instance list is:

```bash
CPA_QUOTA_COLLECTOR_INSTANCES=cpa1=http://cpa1:8317,cpa2=http://cpa2:8317,cpa3=http://cpa3:8317
```

## Run

The collector image can be built before the key is filled, but the service needs the key before it can start collecting.

```bash
docker compose -f docker-compose.cloud.yml --env-file .env up -d cpa-quota-collector
docker logs cpa-quota-collector
```

## Query Recent Snapshots

```bash
docker exec sub2api-postgres psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c \
  "select collected_at, cpa_source, auth_file, account_plan, status, five_hour_remaining_percent, weekly_remaining_percent from cpa_monitor.cpa_quota_snapshots order by collected_at desc limit 20;"
```
