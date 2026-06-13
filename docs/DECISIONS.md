# Decisions

This file records important engineering decisions, architecture decisions, technology choices, and standards changes.

## DEC-0001: Initialize Engineering Standards System

- Date: 2026-05-29
- Status: Accepted
- Background: The project is an active Go proxy server with multiple provider integrations, runtime executors, translators, SDK packages, tests, and deployment scripts. Future modules may be added by AI Agents, so the repository needs a lightweight system that prevents duplicate, speculative, or untraceable work.
- Decision: Establish root `AGENTS.md` as the mandatory Agent entrypoint and add a focused standards set under `docs/`: project overview, architecture, directory structure, coding standards, documentation standards, changelog, decisions, review checklist, and Agent workflow.
- Reason: A small but explicit standards system gives future Agents enough context to make minimal, traceable changes without introducing heavy process or invented architecture.
- Impact: Future significant changes must update `docs/CHANGELOG.md`; architecture, dependency, directory, technology, or standards decisions must add a DEC entry; Agents must self-review before finishing.
- Follow-up: Replace TBD placeholders only when project facts or accepted designs are available.

## DEC-0002: Manage PJ14 as a Single Local Project Root

- Date: 2026-06-02
- Status: Superseded by DEC-0008
- Background: PJ14 combines the CPA source tree, multiple CPA instance configs, Sub2API deployment assets, cloud deployment scripts, project skills, and local recovery backups. Moving CPA source into a nested directory would force broad build, Docker, and deployment path changes.
- Decision: Keep CPA source at the repository root and manage PJ14 on the long-lived local `pj14-main` branch. Rename the official CPA remote to `cpa-official`; leave `origin` unset until a private PJ14 remote exists. Track deployment definitions, scripts, docs, skills, and CPA config files in this repository, but keep `.env`, auth JSON files, logs, backups, Sub2API runtime data, Postgres data, Redis data, cookies, and temporary output out of Git.
- Reason: This preserves the working local and cloud deployment layout while making future project ownership and update flows clearer.
- Impact: First-stage deployment continues to produce the existing `/opt/cpa-sub2api` bundle shape. The cloud directory is not converted into a Git checkout in this stage.
- Follow-up: Add a private `origin` remote later when the user creates one, then push `pj14-main` there.

## DEC-0003: Use a Go Quota Collector with Existing Postgres

- Date: 2026-06-05
- Status: Accepted
- Background: PJ14 needs durable CPA auth quota history for future dashboards while Sub2API already runs a Postgres container on the cloud server.
- Decision: Add a standalone Go `cpa-quota-collector` command that reads CPA management quota reports and writes snapshots under a dedicated `cpa_monitor` Postgres schema in the existing Sub2API Postgres instance. Deploy it as a Docker Compose service built from a locally compiled Linux binary in the cloud bundle.
- Reason: This avoids a second database, keeps Sub2API business tables untouched, and keeps collector code versioned locally while the cloud bundle only needs the runtime binary.
- Impact: Cloud deployments include a new `cpa-quota-collector` service, new collector environment variables, and new `cpa_monitor` database tables.
- Follow-up: Future dashboard work can read `cpa_monitor.cpa_quota_snapshots` together with Sub2API usage tables.

## DEC-0004: Add a Standalone Read-Only CPA Dashboard

- Date: 2026-06-06
- Status: Accepted
- Background: PJ14 needs a product-facing view that aligns CPA weekly quota movement with Sub2API token and cost usage, while keeping the proxy services and quota collector focused on their existing jobs.
- Decision: Add a standalone Go `cpa-dashboard` command that serves an embedded static dashboard and reads from the existing Sub2API Postgres instance. Deploy it as a separate Docker Compose service with a simple password gate and a dedicated read-only database role.
- Reason: A separate service keeps dashboard iteration independent from request proxying and quota collection, avoids adding a frontend build toolchain, and preserves the existing single-Postgres deployment model.
- Impact: Cloud bundles now include a dashboard binary, Dockerfile, compose service, environment variables, and a runbook for creating the read-only database role.
- Follow-up: Request-level attribution from Sub2API usage to internal CPA auth emails remains out of scope until the proxy/usage pipeline records that relationship.

## DEC-0005: Add a Decoupled Portal API for the User MVP

- Date: 2026-06-08
- Status: Superseded by DEC-0009
- Superseded note: Portal is retained only as legacy source unless the user explicitly asks for Portal work. Current PJ14 user/admin product operations happen in Sub2API.
- Background: PJ14 needs a user-facing minimum closed loop for registration/login, Sub2API user mapping, API key creation, usage/cost display, manual recharge confirmation, Portal ledger entries, and Sub2API balance updates. The frontend can remain simple, but the product boundary should not be Sub2API's own admin frontend because future membership work may move to Supabase or another identity/data layer.
- Decision: Add a standalone Go `portal-api` command and `internal/portal` package. Store Portal-owned product state in a dedicated `portal` schema inside the existing Sub2API Postgres database for the MVP. Use a Sub2API adapter for user/key/balance operations, while end-user inference traffic continues to call Sub2API `/v1` directly.
- Reason: This keeps Sub2API as the execution gateway while making Portal the product boundary. It avoids adding a new runtime stack or database now, and preserves a clear migration path for future Supabase-based membership features.
- Impact: Cloud deployments include a new `portal-api` service, Dockerfile, environment variables, bundle export support, Portal schema migrations, and an embedded minimal control panel.
- Follow-up: Future work can replace Portal's identity, billing, or usage providers without changing the user-facing frontend contract.

## DEC-0006: Reuse CPA Dashboard Reports Inside Portal Admin

- Date: 2026-06-08
- Status: Superseded by DEC-0007
- Background: PJ14 needs Portal admins to access the same capacity and quota-efficiency data that the standalone `cpa-dashboard` currently exposes on port `18090`, while preserving Portal as the operator-facing product boundary.
- Decision: Keep `cpa-dashboard` running unchanged for now, but expose its report calculations through reusable service methods and attach that service to `portal-api`. Portal admin routes under `/api/admin/cpa-dashboard/*` enforce Portal admin authorization and power a new embedded admin capacity dashboard.
- Reason: This lets Portal develop and validate a replacement dashboard without coupling to the standalone `18090` frontend or its separate password gate.
- Impact: `portal-api` opens a read connection to the same Postgres database and serves admin-only capacity views.
- Follow-up: Completed by DEC-0007 after the Portal dashboard was validated.

## DEC-0007: Retire the Standalone `18090` CPA Dashboard

- Date: 2026-06-08
- Status: Superseded by DEC-0009
- Superseded note: Portal is no longer the current PJ14 dashboard or product surface. Keep report code only where it remains useful to current CPA/Sub2API operations.
- Background: The Portal admin capacity dashboard has been validated as the supported operator surface for quota efficiency, account health, usage trend, and cleanup-candidate reports.
- Decision: Remove the standalone `cpa-dashboard` command, embedded standalone frontend, Dockerfile, Compose service, dashboard runbook, generated environment variables, bundle export build step, and dashboard-specific read-only role setup script. Keep `internal/cpa_dashboard` as the reusable report package used by Portal admin routes.
- Reason: This removes the public `18090` surface and its separate password gate without deleting the report queries that Portal needs.
- Impact: At that historical stage, cloud deployments no longer started or exposed `cpa-dashboard`, and Portal carried the dashboard UI. DEC-0009 supersedes that product-surface decision.
- Follow-up: A later cleanup may rename `internal/cpa_dashboard` to a more neutral report package name after Portal behavior is stable and the extra churn is worth it.

## DEC-0008: Split PJ14 Into Two Source Forks and One Deploy Repository

- Date: 2026-06-13
- Status: Accepted
- Background: PJ14 initially kept CPA source, Sub2API deployment assets, cloud scripts, instance configs, and project-specific operational skills in one CPA fork. That made first-stage deployment fast, but it mixed two upstream projects and deployment orchestration in one Git history.
- Decision: Maintain CPA code in the `pj14-cpa` fork, Sub2API code in a separate `pj14-sub2api` fork, and cloud orchestration in a separate `pj14-deploy` repository. Keep `/opt/cpa-sub2api` runtime compatibility in exported deploy bundles so production state paths do not change.
- Reason: This keeps upstream merges for CPA and Sub2API independent while preserving a single deployment source of truth for Compose, env templates, runbooks, and image tags.
- Impact: PJ14 deployment assets, CPA instance configs, project operational skills, and Sub2API runtime directories are removed from the CPA source fork and owned by `pj14-deploy`. CPA source packaging assets such as `Dockerfile` and source-level Compose files remain here.
- Follow-up: Completed. The target repositories are `munanallen-rgb/pj14-cpa`, `munanallen-rgb/pj14-sub2api`, and `munanallen-rgb/pj14-deploy`.

## DEC-0009: Use Sub2API as the PJ14 Product Surface and `pj14-deploy` as the Project Control Repository

- Date: 2026-06-13
- Status: Accepted
- Background: After validating the split-repository workflow, PJ14 no longer uses the custom Portal as the active user/admin surface. Product operations now happen in Sub2API, while deployment and cross-repository coordination need a stable entrypoint that is independent of either source fork.
- Decision: Treat `pj14-deploy` as the PJ14 project control repository and deployment source of truth. Keep CPA source changes in `pj14-cpa`, Sub2API product changes in `pj14-sub2api`, and all deployment orchestration plus cross-repository governance in `pj14-deploy`. Treat Portal code in this CPA fork as legacy unless the user explicitly asks to revive or modify it.
- Reason: This gives future agents a clear default: synchronize official CPA and Sub2API upstreams into their own forks, build/publish images from those forks, then deploy from `pj14-deploy`. It also prevents new work from drifting back into the retired Portal path.
- Impact: Agent instructions must point PJ14-wide work to `pj14-deploy`; user-facing product features such as recharge, subscription, balance, concurrency, and quota distribution should default to Sub2API; this CPA fork should not regain deployment-only files or Portal-first product behavior.
- Follow-up: Keep `pj14-deploy` documentation authoritative for cross-repository workflow and update the source-fork `AGENTS.md` files when ownership boundaries change.
