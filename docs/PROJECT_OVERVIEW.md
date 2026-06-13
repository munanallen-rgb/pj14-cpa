# Project Overview

## Name
CLIProxyAPI.

## Goal
Provide a local proxy server that exposes OpenAI/Gemini/Claude/Codex/Grok-compatible APIs for CLI clients and SDK users, with OAuth-based provider access, account routing, protocol translation, management APIs, and source-level packaging assets.

## Current Technology Stack
- Language: Go `1.26.0` from `go.mod`.
- HTTP framework: Gin.
- Terminal UI: Bubbletea, Bubbles, Lipgloss.
- Logging: logrus and lumberjack.
- Config: YAML plus `.env` loading.
- Auth/OAuth: provider-specific packages under `internal/auth/` and SDK auth packages.
- Storage: file-based default with optional Postgres, git, object store, Redis-related queue support.
- WebSocket: gorilla/websocket and internal wsrelay/runtime executors.
- Packaging: Dockerfile, Docker Compose files, GoReleaser config, PowerShell and shell scripts.
- Package manager/build tool: Go modules.

## Main Functional Areas
- Server entrypoint and CLI flags: `cmd/server/`.
- CPA quota collector entrypoint: `cmd/quota_collector/`.
- User Portal API entrypoint: `cmd/portal_api/`.
- HTTP API, protocol routing, middleware, and management handlers: `internal/api/`.
- Provider authentication: `internal/auth/` and `sdk/auth/`.
- Runtime provider execution: `internal/runtime/executor/`.
- Protocol translation: `internal/translator/` and `sdk/translator/`.
- Thinking/reasoning configuration pipeline: `internal/thinking/`.
- Model registry and updates: `internal/registry/`.
- Config loading and hot reload: `internal/config/`, `internal/watcher/`.
- Storage: `internal/store/`.
- CPA quota monitoring and capacity reports: `internal/quota_collector/` and `internal/cpa_dashboard/`.
- User Portal MVP: `internal/portal/`, backed by a dedicated `portal` schema in the existing Sub2API Postgres database, with an admin-only capacity dashboard that reuses CPA dashboard report logic.
- Embeddable SDK: `sdk/cliproxy/` plus related SDK packages.
- Integration and compatibility tests: `test/`.
- Packaging helpers: `Dockerfile` and source-level `docker-compose*.yml`.

## Run
```bash
go run ./cmd/server
```

Common flags include `--config <path>`, `--tui`, `--standalone`, `--local-model`, `--no-browser`, and `--oauth-callback-port <port>`.

## Build
```bash
go build -o cli-proxy-api ./cmd/server
```

After code changes, verify compile with:

```bash
go build -o test-output ./cmd/server && rm test-output
```

On Windows PowerShell, remove `test-output` with `Remove-Item -LiteralPath test-output` if needed.

## Test
```bash
go test ./...
go test -v -run TestName ./path/to/pkg
```

Run focused tests for the touched package when possible. Run `go test ./...` for broad or shared behavior changes.

## Current Project Status
Active Go service with existing production-facing modules, SDK packages, tests, docs, and Docker assets. PJ14 cloud deployment orchestration now lives in the separate `pj14-deploy` repository. The repository is not empty. Several future modules or changes may be undecided; record those as TBD (待确认) until a concrete design is accepted.

## Context New Agents Must Know
- Root `AGENTS.md` is mandatory reading before any task.
- Avoid standalone changes to `internal/translator/` unless the permission rule in `AGENTS.md` is satisfied.
- Preserve the `internal/thinking/` canonical config to provider-specific translation flow.
- `internal/runtime/executor/` is for executors and tests; shared executor helpers belong in `internal/runtime/executor/helps/`.
- Do not add network timeouts after upstream connections are established, except for documented liveness/session/management utility exceptions.
- Do not log secrets, tokens, OAuth credentials, or provider auth material.
- Keep new docs in English unless updating an explicitly language-specific document.
