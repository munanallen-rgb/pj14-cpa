# Directory Structure

## Current Top-Level Tree
```text
.
├── .github/
├── assets/
├── auths/
├── backups/
├── cmd/
├── docs/
├── examples/
├── instances/
├── internal/
├── logs/
├── sdk/
├── skills/
├── sub2api-deploy/
├── temp/
├── test/
├── config.example.yaml
├── config.yaml
├── docker-compose*.yml
├── Dockerfile
├── go.mod
├── go.sum
├── README*.md
└── AGENTS.md
```

Some local/runtime directories such as `auths/`, `backups/`, `instances/`, `logs/`, `skills/`, and `temp/` may contain machine-specific files. Do not treat local generated content as product source unless the task explicitly targets it.

## PJ14 Repository Layout
PJ14 uses this repository root as the single local project root for the CPA source tree plus deployment assets. Do not move the CPA Go source into a nested `cpa/` directory during the first-stage cleanup; `cmd/`, `internal/`, `sdk/`, `go.mod`, `Dockerfile`, and the existing compose files stay at the root so current build and deployment scripts continue to work.

Track deployment definitions and reusable deployment helpers, including `docker-compose.cloud.yml`, `instances/*/config.yaml`, `sub2api-deploy/` scripts, docs, and skills. Do not track runtime secrets or generated state such as `.env`, auth JSON files, logs, `backups/`, Sub2API data, Postgres data, Redis data, cookies, or temporary output.

The cloud deployment path remains `/opt/cpa-sub2api`. First-stage deployments continue to use the existing local bundle/upload workflow instead of turning the cloud directory into a Git checkout.

## Directory Responsibilities
- `.github/`: GitHub workflows and repository automation.
- `assets/`: README and documentation images/assets.
- `auths/`: default local auth material location; do not commit secrets.
- `cmd/`: executable entrypoints and small command utilities.
- `cmd/quota_collector/`: cloud quota collector entrypoint for recording CPA auth quota snapshots.
- `cmd/portal_api/`: user-facing Portal API entrypoint for the PJ14 MVP.
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
- `internal/cpa_dashboard/`: read-only capacity report queries, quota efficiency metrics, reusable report service methods, and report types used by Portal admin routes.
- `internal/portal/`: Portal configuration, schema migrations, user/session logic, Sub2API adapter, user-scoped usage queries, manual recharge ledger, admin-only capacity dashboard routes, and embedded MVP static UI.
- `sdk/`: public embeddable packages.
- `sub2api-deploy/`: deployment helper scripts and runbooks.
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
- New PJ14 cloud deployment runbooks: `sub2api-deploy/` when they directly describe the CPA + Sub2API cloud bundle.

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
