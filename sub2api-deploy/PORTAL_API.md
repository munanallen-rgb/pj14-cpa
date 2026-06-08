# Portal API

The cloud bundle includes a user-facing `portal-api` service. It owns the product boundary for the first customer MVP while Sub2API remains the execution gateway for API keys, balances, usage logs, and routing to the CPA pool.

## Scope

The first version supports this closed loop:

1. User registers or logs in to Portal.
2. Portal creates a user in the `portal` Postgres schema.
3. Portal creates or maps a matching Sub2API user.
4. User creates a Sub2API API key from Portal.
5. User calls Sub2API `/v1` directly with that key.
6. Sub2API records usage in `public.usage_logs`.
7. Portal shows usage and cost for the user's keys.
8. User submits a recharge order.
9. Admin manually confirms the recharge.
10. Portal records a ledger entry and calls Sub2API to add balance.

No payment provider is integrated in this version.

## Embedded Console UI

The embedded MVP UI uses a Sub2API-inspired left-sidebar console layout but keeps the Portal product boundary:

- Normal users see only account overview, API keys, usage, billing, recharge, and integration information.
- Normal users do not see Sub2API, group, channel, or other gateway-internal labels.
- Portal admins see a separate operations workspace for recharge review, order status, and configuration-boundary guidance.
- Frontend role-based navigation is only a usability layer; route authorization remains enforced by the Portal API.

## Runtime Shape

```text
Cloudflare Pages or embedded static UI
  -> portal-api
  -> portal schema in the existing Sub2API Postgres
  -> Sub2API HTTP API
  -> CPA pool
```

The Portal API does not expose Sub2API admin credentials to the browser. It derives an internal Sub2API password per Portal user from `PORTAL_SESSION_SECRET`, creates the Sub2API user with that password, and uses it only server-side when creating API keys.

## Environment

Required values:

```bash
PORTAL_PUBLIC_SUB2API_BASE_URL=http://<server-ip>:18080
PORTAL_SESSION_SECRET=<long-random-secret>
PORTAL_DATABASE_PASSWORD=<POSTGRES_PASSWORD>
PORTAL_SUB2API_ADMIN_EMAIL=portal-service@sub2api.local
PORTAL_SUB2API_ADMIN_PASSWORD=<dedicated Sub2API admin service password>
```

Use a dedicated Sub2API admin service account for `PORTAL_SUB2API_ADMIN_EMAIL` and `PORTAL_SUB2API_ADMIN_PASSWORD`. Do not rely on the initial `ADMIN_PASSWORD` value after Sub2API has already been set up, because the runtime admin password can diverge from the bootstrap environment value.

Optional bootstrap admin:

```bash
PORTAL_BOOTSTRAP_ADMIN_EMAIL=portal-admin@sub2api.local
PORTAL_BOOTSTRAP_ADMIN_PASSWORD=<long-random-password>
```

Prefer a Portal-specific admin email that does not already exist as a Sub2API user. Portal creates Sub2API users with server-derived passwords, so sharing the bootstrap Portal admin email with an existing Sub2API admin can make that Portal account unsuitable for customer-style API key creation.

When using Cloudflare Pages or another separate frontend origin, set:

```bash
PORTAL_ALLOWED_ORIGINS=https://console.example.com
PORTAL_COOKIE_SECURE=true
```

For plain HTTP testing, keep `PORTAL_COOKIE_SECURE=false`.

## Run

The service starts with the default Compose stack:

```bash
docker compose -f docker-compose.cloud.yml --env-file .env up -d portal-api
docker logs portal-api
```

Open:

```text
http://<server-ip>:18100
```

## Main API

- `POST /api/auth/register`
- `POST /api/auth/login`
- `POST /api/auth/logout`
- `GET /api/me`
- `GET /api/api-keys`
- `POST /api/api-keys`
- `GET /api/usage/summary`
- `GET /api/usage/records`
- `POST /api/recharge-orders`
- `GET /api/recharge-orders`
- `GET /api/billing/ledger`
- `GET /api/admin/recharge-orders`
- `POST /api/admin/recharge-orders/{id}/confirm`
- `POST /api/admin/recharge-orders/{id}/cancel`

The full API key is returned only once from `POST /api/api-keys`; Portal stores only the Sub2API key id and a preview string.
