# Directory Structure

## Current Top-Level Tree
```text
.
├── .github/
├── assets/
├── backups/
├── cmd/
├── docs/
├── examples/
├── internal/
├── sdk/
├── temp/
├── test/
├── config.example.yaml
├── docker-compose*.yml
├── Dockerfile
├── go.mod
├── go.sum
├── README*.md
└── AGENTS.md
```

Some local/runtime directories such as `auths/`, `backups/`, `logs/`, and `temp/` may contain machine-specific files. Do not treat local generated content as product source unless the task explicitly targets it.

## PJ14 Repository Layout
PJ14 now uses two source forks plus one deployment repository:

- `pj14-cpa`: CPA source fork. Keep CPA source, SDK, tests, docs, and source-level packaging assets here. Local checkout directories may still use older names, but the canonical remote and branch are `munanallen-rgb/pj14-cpa` and `pj14-cpa`.
- `pj14-sub2api`: Sub2API source fork. Keep Sub2API product and billing customizations there.
- `pj14-deploy`: PJ14 project control and deployment repository. Keep cloud Compose, CPA instance configs, Nginx examples, env templates, runbooks, deploy scripts, image tags, and cross-repository governance docs there.

The cloud deployment path remains `/opt/cpa-sub2api`, but that runtime shape is exported by `pj14-deploy`, not this CPA source repository.

## Directory Responsibilities
- `.github/`: GitHub workflows and repository automation.
- `assets/`: README and documentation images/assets.
- `cmd/`: executable entrypoints and small command utilities.
- `cmd/quota_collector/`: cloud quota collector entrypoint for recording CPA auth quota snapshots.
- `cmd/portal_api/`: legacy Portal API entrypoint. It is not the current PJ14 product surface.
- `docs/`: engineering, SDK, and project documentation.
- `examples/`: runnable examples for SDK or provider usage.
- `internal/`: private application packages.
- `internal/api/`: HTTP API, middleware, protocol multiplexing, management handlers, modules.
- `internal/auth/`: provider auth flows and token handling.
- `internal/config/`: configuration parsing and defaults.
- `internal/runtime/executor/`: upstream execution implementations.
- `internal/runtime/executor/helps/`: executor helper/support files.
- `internal/thinking/`: reasoning config pipeline.
- `internal/translator/`: protocol translators.
- `internal/watcher/`: config hot reload and diff/synthesis logic.
- `internal/quota_collector/`: CPA quota collector configuration, scheduling, report parsing, and Postgres persistence.
- `internal/cpa_dashboard/`: read-only capacity report queries, quota efficiency metrics, reusable report service methods, and report types retained for legacy/reporting paths.
- `internal/portal/`: legacy Portal configuration, schema migrations, user/session logic, Sub2API adapter, user-scoped usage queries, manual recharge ledger, admin-only capacity dashboard routes, and embedded MVP static UI.
- `sdk/`: public embeddable packages.
- `test/`: cross-module integration tests.

## Where New Files Should Go
- New server commands: `cmd/<command>/`.
- New API route or handler: the closest existing package under `internal/api/`.
- New management endpoint: `internal/api/handlers/management/`.
- New API module: `internal/api/modules/<module>/` only when it is a real module with routes or proxy behavior.
- New provider executor: `internal/runtime/executor/`.
- Executor helpers: `internal/runtime/executor/helps/`.
- New provider auth: `internal/auth/<provider>/` and SDK auth only when required.
- New config behavior: `internal/config/`, plus examples/docs.
- New SDK-facing functionality: closest existing package under `sdk/`.
- New integration test: `test/`.
- New package test: same package as the code under test with `_test.go`.
- New docs: `docs/`, unless updating language-specific root README files.
- New PJ14 cloud deployment runbooks: the separate `pj14-deploy` repository.

## Directory Creation Rules
- Before creating a directory, search for an equivalent existing directory.
- Create a new directory only when it has a distinct responsibility and at least one real source/test/doc file.
- Do not create directories named only by vague concepts such as `utils`, `common`, `helpers`, `core`, or `services` unless the responsibility is narrow and documented.
- Do not create a new framework layer for a single feature.
- Prefer adding to an existing package when the behavior belongs to that package.

## Naming Rules
- Go package names should be short, lowercase, and responsibility-based.
- Go files should use lowercase names with underscores when helpful, e.g. `request_logging.go`.
- Tests should be named `*_test.go` and live near the code unless they are cross-module integration tests.
- Docs should use uppercase snake case for project standards, e.g. `CODING_STANDARDS.md`, and lowercase names for topic guides when already established.
- Config examples should be explicit, e.g. `.env.example`, `config.example.yaml`.

## File Splitting Rules
- Split a file when it has multiple unrelated responsibilities or has become hard to navigate.
- Keep related request/response translation logic together unless tests or provider differences justify a split.
- Merge new behavior into an existing file when the change is small and fits the existing responsibility.
- Do not create one-file packages only for appearance.
