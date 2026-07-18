# AGENTS.md

This is the highest-priority entry document for agents working in the PJ14 CPA source fork.

## Repository Boundary

- Origin: `munanallen-rgb/pj14-cpa`; branch: `pj14-cpa`; upstream: `router-for-me/CLIProxyAPI`.
- This repository is the CPA source fork only. It owns CPA source, SDK code, tests, source-local docs, and packaging assets.
- `pj14-deploy` owns deployment orchestration and PJ14-wide governance; this repository does not own deployment orchestration.
- Put cloud Compose, environment templates, image locks, runtime state, backups, and deployment runbooks in `pj14-deploy`.
- Put user/admin billing, payments, subscriptions, balances, keys, concurrency, and quota distribution in `pj14-sub2api`.
- Portal is not the current PJ14 product surface. Do not expand it unless the user explicitly requests Portal work.
- Never push to upstream or commit secrets, auth material, `.env`, logs, backups, databases, runtime data, or generated `temp/` content.

## Reading Route

Read `AGENTS.md` first. Use `docs/PROJECT_OVERVIEW.md` only when the task has no
clear route below, then follow the task:

- Architecture or new modules: `docs/ARCHITECTURE.md` and `docs/DIRECTORY_STRUCTURE.md`.
- Go implementation or review: `docs/CODING_STANDARDS.md`; use `docs/REVIEW_CHECKLIST.md` when reviewing or finishing a broad change.
- PJ14-wide scope or ownership: `../pj14-deploy/AGENTS.md` and `../pj14-deploy/docs/PJ14_PROJECT_GOVERNANCE.md` when available.
- Read source files and focused docs directly related to the request.

## Development Rules

- Prefer existing packages and patterns; avoid parallel implementations, speculative layers, and vague utility packages.
- Keep changes easy to merge with upstream. Pull official updates from `upstream` into `pj14-cpa`; never push upstream.
- Keep `internal/runtime/executor/` for executors and tests; shared executor support belongs in `internal/runtime/executor/helps/`.
- Preserve the canonical `internal/thinking/` config-to-provider translation flow.
- Do not make a standalone `internal/translator/` change without confirming repository write permission as documented by the existing project policy.
- Do not set timeouts after an upstream connection is established, except for existing documented liveness, session, management, and utility cases.
- Use logrus, return contextual errors, avoid handler panics and `log.Fatal`, and never log secrets or tokens.
- Add focused tests for behavior changes. Run broader tests when shared contracts change.
- Update `docs/CHANGELOG.md` for meaningful source behavior changes and `docs/DECISIONS.md` only for lasting architecture, dependency, or standards decisions.
