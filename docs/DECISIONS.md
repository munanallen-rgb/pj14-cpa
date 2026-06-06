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
- Status: Accepted
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
