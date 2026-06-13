# AGENTS.md

This is the highest-priority entry document for AI Agents working in this repository.

CLIProxyAPI is a Go 1.26+ proxy server that provides OpenAI/Gemini/Claude/Codex/Grok-compatible APIs with OAuth, round-robin account routing, provider translation, management APIs, and an embeddable SDK.

## Repository
- PJ14 fork origin: https://github.com/munanallen-rgb/pj14-cpa
- Official upstream: https://github.com/router-for-me/CLIProxyAPI
- Long-lived PJ14 branch: `pj14-cpa`
- Module: `github.com/router-for-me/CLIProxyAPI/v7`

## PJ14 Scope
- This repository is the CPA source fork only. Keep CPA source code, SDK code, tests, source-level docs, and source-level packaging assets here.
- `pj14-deploy` is the PJ14 project control and deployment repository. Put Compose files, cloud instance configs, env templates, deployment runbooks, image tags, backup/deploy scripts, and cross-repository operational docs there.
- `pj14-sub2api` is the Sub2API source fork. Put user-facing product, billing, payment, subscription, recharge, balance, concurrency, and quota-distribution customizations there.
- Pull official CPA updates from `upstream` into `pj14-cpa`, then resolve PJ14-specific CPA changes here. Do not push to `upstream`; its push URL should stay disabled.
- Portal is not the current PJ14 product surface. Current user and admin operations are owned by Sub2API. Do not add, restore, or expand Portal behavior unless the user explicitly asks for Portal work.
- Do not reintroduce deployment-only assets, Sub2API runtime data, auth files, `.env`, logs, backups, database data, or project operational skills into this source fork.

## Required Reading Before Every Task
Before changing code or docs, every Agent must read:

1. `AGENTS.md`
2. `docs/PROJECT_OVERVIEW.md`
3. `docs/ARCHITECTURE.md`
4. `docs/DIRECTORY_STRUCTURE.md`
5. `docs/CODING_STANDARDS.md`
6. `docs/REVIEW_CHECKLIST.md`
7. For PJ14 cross-repository, deployment, or product-surface work, read `../pj14-deploy/AGENTS.md` and `../pj14-deploy/docs/PJ14_PROJECT_GOVERNANCE.md` when available.
8. Source files and docs directly related to the current task

Do not modify code before understanding the project structure, current conventions, and the relevant implementation.

## Agent Development Principles
- Prefer modifying existing code over creating parallel implementations.
- Do not create unnecessary abstractions, directories, framework layers, or wrappers.
- Do not add temporary code, hidden logic, or bypasses just to make a task pass.
- Before adding a file, confirm that no suitable existing location exists.
- Before adding a dependency, explain why it is needed and check whether an existing dependency or standard library option is enough.
- Each function, component, package, and module must have a clear responsibility.
- Code must be readable, testable, removable, and replaceable.
- Do not generate large unused blocks, dead code, duplicate code, or speculative scaffolding.
- Do not design complex architecture without a current requirement.
- Use simple implementations for simple requirements; every increase in complexity needs a reason.
- Record all significant changes in `docs/CHANGELOG.md`.
- Record architecture-level, technology-choice, directory-structure, and standards decisions in `docs/DECISIONS.md`.
- Run a self-review before finishing every task.

## Required Agent Workflow
For every development task:

1. Read the required standards and relevant code.
2. Restate the task goal in your own working notes or response when helpful.
3. Decide whether standards or docs need updates.
4. Design the smallest change that satisfies the task.
5. Modify code and docs.
6. Remove unused, duplicate, temporary, or debug code.
7. Run available checks, tests, or builds.
8. Update related documentation when behavior, commands, structure, or contracts changed.
9. Update `docs/CHANGELOG.md` for meaningful changes.
10. If architecture, dependencies, standards, or directory structure changed, update `docs/DECISIONS.md` and the relevant standards doc.
11. Review the result using `docs/REVIEW_CHECKLIST.md`.

## Required Final Output From Agents
Every completed task response must include:

- Files changed.
- Why the change was made.
- Whether dependencies were added.
- Whether directory structure was affected.
- Whether documentation was updated.
- Whether `docs/CHANGELOG.md` was updated.
- Any unfinished work or TBD (待确认) items.
- Checks/tests/builds run and their result.

## Commands
```bash
gofmt -w . # Format after Go changes
go build -o cli-proxy-api ./cmd/server # Build server
go run ./cmd/server # Run dev server
go test ./... # Run all tests
go test -v -run TestName ./path/to/pkg # Run one test
go build -o test-output ./cmd/server && rm test-output # Required compile verification after code changes
```

Common flags: `--config <path>`, `--tui`, `--standalone`, `--local-model`, `--no-browser`, `--oauth-callback-port <port>`.

## Config
- Default config: `config.yaml`; template: `config.example.yaml`.
- `.env` is auto-loaded from the working directory.
- Auth material defaults under `auths/`.
- Storage backends: file-based default; optional Postgres/git/object store via `PGSTORE_*`, `GITSTORE_*`, and `OBJECTSTORE_*`.

## Architecture Map
- `cmd/server/`: server entrypoint.
- `cmd/fetch_*`: model-fetching utilities.
- `internal/api/`: Gin HTTP API, routes, middleware, protocol multiplexing, and management handlers.
- `internal/api/modules/amp/`: Amp integration, Amp-style routes, reverse proxy, model mapping, and fallback handling.
- `internal/thinking/`: main thinking/reasoning pipeline. `ApplyThinking()` parses suffixes, normalizes config into canonical `ThinkingConfig`, validates centrally, then applies provider-specific output through `ProviderApplier`. Do not break the canonical representation to provider translation architecture.
- `internal/runtime/executor/`: per-provider runtime executors, including Codex WebSocket behavior.
- `internal/runtime/executor/helps/`: helper/support code for executors.
- `internal/translator/`: provider protocol translators and shared translator helpers.
- `internal/registry/`: model registry and remote updater; `--local-model` disables remote updates.
- `internal/store/`: storage implementations and secret resolution.
- `internal/managementasset/`: config snapshots and management assets.
- `internal/cache/`: request signature caching.
- `internal/watcher/`: config hot reload, diffing, event dispatch, and synthesizers.
- `internal/wsrelay/`: WebSocket relay sessions.
- `internal/usage/` or related usage packages: usage and token accounting when present.
- `internal/tui/`: Bubbletea terminal UI for `--tui` and `--standalone`.
- `sdk/cliproxy/`: embeddable SDK service, builder, provider runtime, watchers, and pipeline.
- `sdk/api/`, `sdk/auth/`, `sdk/config/`, `sdk/translator/`: SDK-facing API, auth, config, and translation packages.
- `test/`: cross-module integration and compatibility tests.
- `docs/`: project and SDK documentation.

## Code Conventions
- Keep changes small and simple.
- Comments in code must be English.
- If editing code that already contains non-English comments, translate those comments to English when touching the area.
- User-visible strings should keep the existing language used in that file or feature area.
- New Markdown docs should be English unless the file is explicitly language-specific, such as `README_CN.md`.
- As a rule, do not make standalone changes to `internal/translator/`.
- If a task requires changing only `internal/translator/`, run `gh repo view --json viewerPermission -q .viewerPermission` to confirm `WRITE`, `MAINTAIN`, or `ADMIN`. If allowed, proceed. Otherwise, file a GitHub issue with the goal, rationale, and intended implementation code, then stop further work.
- `internal/runtime/executor/` should contain executors and their unit tests only. Put executor helper/support files under `internal/runtime/executor/helps/`.
- Follow `gofmt`; keep imports goimports-style.
- Wrap errors with context where helpful.
- Do not use `log.Fatal` or `log.Fatalf`; return errors and log through logrus instead.
- For shadowed variables, use method suffixes such as `errStart := server.Start()`.
- Wrap defer errors: `defer func() { if err := f.Close(); err != nil { log.Errorf(...) } }()`.
- Use logrus structured logging and never leak secrets or tokens.
- Avoid panics in HTTP handlers; prefer logged errors and meaningful HTTP status codes.
- Timeouts are allowed only during credential acquisition. After an upstream connection is established, do not set timeouts for subsequent network behavior. Allowed intentional exceptions are Codex websocket liveness deadlines in `internal/runtime/executor/codex_websockets_executor.go`, wsrelay session deadlines in `internal/wsrelay/session.go`, the management APICall timeout in `internal/api/handlers/management/api_tools.go`, and `cmd/fetch_antigravity_models` utility timeouts.

## Traceability Rules
- Update `docs/CHANGELOG.md` for meaningful code, docs, config, test, command, behavior, or standards changes.
- Update `docs/DECISIONS.md` for architecture, dependency, standards, directory, or technology decisions.
- Leave future modules as placeholders until a concrete design is accepted.
- If standards and real code disagree, treat current code as source of truth, update the standards, record the change, and mention it in the final response.
