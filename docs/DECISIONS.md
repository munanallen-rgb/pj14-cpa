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
