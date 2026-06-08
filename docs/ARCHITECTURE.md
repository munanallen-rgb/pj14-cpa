# Architecture

## Overview
CLIProxyAPI is a Go proxy service. Clients call OpenAI/Gemini/Claude/Codex/Grok-compatible endpoints, the API layer routes and normalizes requests, provider executors perform upstream work, translators adapt protocol shapes, and supporting modules handle auth, config, registry, storage, logging, and management.

The architecture should stay pragmatic: extend existing modules first and add new layers only when there is a repeated, current need.

## Main Module Responsibilities
- `cmd/server/`: process startup, CLI flags, server wiring.
- `cmd/quota_collector/`: standalone CPA quota collection process for cloud deployments.
- `cmd/portal_api/`: standalone user Portal API for the PJ14 customer MVP.
- `internal/api/`: Gin server, protocol multiplexing, request middleware, management endpoints, module registration.
- `internal/api/modules/amp/`: Amp-specific route support and proxy behavior.
- `internal/auth/`: provider OAuth/token acquisition and auth file handling.
- `internal/runtime/executor/`: provider execution against upstream services.
- `internal/runtime/executor/helps/`: executor support helpers.
- `internal/translator/`: internal protocol translators between provider/client protocol shapes.
- `internal/thinking/`: canonical thinking/reasoning config parsing, validation, conversion, and provider application.
- `internal/config/`: config parsing and defaults.
- `internal/watcher/`: hot reload, diffing, event dispatch, and config synthesis.
- `internal/registry/`: model definitions, client-visible models, and updater.
- `internal/store/`: persistence backends and secret resolution.
- `internal/cache/`: request signature caching.
- `internal/cpa_dashboard/`: Postgres reads, quota efficiency calculations, reusable report service methods, and capacity report types used by Portal admin routes.
- `internal/quota_collector/`: collector config, scheduling, CPA management report fetches, quota parsing, and Postgres writes.
- `internal/portal/`: user registration/login, Portal schema migrations, Sub2API user/key adapter, scoped usage reads, manual recharge orders, ledger entries, admin-only capacity dashboard routes, and embedded MVP static UI.
- `internal/wsrelay/`: WebSocket relay lifecycle.
- `internal/tui/`: terminal UI.
- `sdk/`: embeddable public-facing API, auth, config, translation, logging, and access packages.
- `test/`: cross-module and compatibility tests.

## Typical Data Flow
1. Client request enters `internal/api/`.
2. Middleware handles logging, protocol detection, request wrapping, or management routing.
3. Config, auth, registry, and access modules resolve provider/account/model behavior.
4. `internal/thinking/` applies reasoning configuration when applicable.
5. Translators convert request/response shapes when protocols differ.
6. Runtime executor calls the selected upstream provider.
7. Stream, non-stream, or WebSocket response is returned to the client.
8. Logging, cache, usage, watcher, or management side effects occur through their owning packages.

## User Portal MVP Flow
1. Browser calls the standalone Portal API.
2. Portal authenticates users with its own session cookie and stores user state under the `portal` Postgres schema.
3. Portal maps each Portal user to a Sub2API user through a server-side adapter.
4. Portal creates Sub2API API keys as the mapped Sub2API user, then stores only the Sub2API key id and a preview.
5. End-user inference traffic goes directly to Sub2API `/v1`, not through Portal.
6. Portal reads Sub2API `public.usage_logs` only for mapped key ids.
7. Admin-confirmed recharge orders create immutable Portal ledger entries and call Sub2API to add balance.
8. Portal admins can view capacity, quota efficiency, account health, usage trend, and cleanup-candidate reports through Portal routes that reuse `internal/cpa_dashboard` service logic.

## Boundaries
- API handlers should not own provider-specific upstream details; delegate to executors, translators, auth, config, or registry packages.
- Executors should not become general utility containers. Put shared executor support in `internal/runtime/executor/helps/`.
- Translators should focus on protocol shape conversion and should not own auth, storage, routing, or business policy.
- SDK packages should expose stable embeddable behavior and should not import internal packages unless an existing pattern already does so and the boundary is understood.
- Config files describe configuration; they must not hide business logic.

## Restricted Cross-Layer Calls
- Do not call `internal/translator/` directly from unrelated features as a shortcut around API/runtime boundaries.
- Do not make management handlers mutate low-level storage or auth details without using the existing owning package patterns.
- Do not make watcher/diff code depend on API handler internals.
- Do not make TUI code the source of behavior used by non-TUI paths.
- Do not expose Sub2API admin credentials or CPA management credentials through Portal frontend routes.

## Future Extension Points
- New provider auth: add under `internal/auth/<provider>` and SDK auth only when needed.
- New executor: add to `internal/runtime/executor/` with tests; helpers go under `internal/runtime/executor/helps/`.
- New protocol translation: extend the existing translator registry/layout; follow the translator permission rule in `AGENTS.md`.
- New management capability: add under `internal/api/handlers/management/` and reuse existing management response/error patterns.
- New config behavior: update `internal/config/`, config examples, docs, and watcher behavior as needed.
- Future undecided modules: keep placeholders in docs as TBD (待确认) until the concrete design is approved and recorded.

## Avoid Premature Abstraction
- Do not introduce generic provider frameworks until at least two concrete implementations need the same behavior.
- Do not create broad `utils` or `common` packages without a narrow documented responsibility.
- Do not split files only to make the tree look layered.
- Do not add plugin systems, lifecycle managers, or registries unless current code needs them.
