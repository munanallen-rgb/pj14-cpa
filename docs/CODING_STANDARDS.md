# Coding Standards

## General
- Keep changes small, direct, and easy to remove.
- Reuse existing packages, helpers, config parsing, logging, and test patterns before adding new ones.
- Prefer explicit code over hidden behavior.
- Do not implement features that are not requested or needed by current code.

## Naming
- Use clear Go names that describe behavior, not implementation tricks.
- Package names should be short, lowercase, and stable.
- Exported names require a clear reason and should have Go doc comments when part of a public package.
- Avoid abbreviations unless already common in the codebase, such as API, HTTP, OAuth, SDK, TUI.

## Function and File Size
- Keep functions focused on one job. If a function is hard to test or explain, split by responsibility.
- Prefer small files grouped by behavior. Large files are acceptable when they hold cohesive protocol mappings or tables.
- Do not split files just to satisfy a line count. Split when navigation, testing, or responsibility improves.

## Module Boundaries
- API packages coordinate HTTP behavior; provider-specific upstream behavior belongs in executors/translators/auth packages.
- Executors execute provider requests and should not become routing or config policy owners.
- Translators translate protocol shapes and should not own auth, storage, or routing policy.
- SDK packages expose reusable behavior and should preserve stable public contracts.
- Config packages parse/default config; business logic should live in code, not config files.

## Error Handling
- Return errors with enough context for callers to diagnose failures.
- Do not swallow errors. If an error is intentionally ignored, state why in code.
- Avoid panics in HTTP/server paths.
- Do not use `log.Fatal` or `log.Fatalf`; return errors and log through logrus.
- Wrap deferred close/write errors when they can matter.

## Logging
- Use logrus and structured fields where useful.
- Never log secrets, OAuth tokens, API keys, auth file contents, or full sensitive headers.
- Log at the owning layer. Avoid duplicate logs for the same error path unless they add context.

## Configuration
- Keep `config.example.yaml` aligned with supported config.
- Use `.env` for environment overrides already supported by the project.
- Do not put business logic in config files.
- Document new config keys in relevant docs and examples.

## Types
- Prefer typed structs and constants over unstructured maps when the shape is known.
- Use constants for repeated protocol values, magic strings, and magic numbers.
- Keep JSON/YAML tags consistent with existing API/config contracts.

## Comments
- Code comments must be in English.
- Comment why something is non-obvious, not what obvious code does.
- Public exported Go APIs should follow normal Go doc style when they are part of SDK or reusable packages.

## Testing
- Add or update tests when changing core logic, protocol translation, auth behavior, executor behavior, config parsing, watcher behavior, or SDK contracts.
- Use focused tests for narrow changes.
- Run broader tests for shared behavior changes.
- If tests cannot be run, explain why and provide the most relevant verification performed.

## Dependency Management
- Prefer the Go standard library or existing dependencies.
- Add a dependency only when it materially reduces risk or complexity.
- Explain new dependencies in the final response and record significant choices in `docs/DECISIONS.md`.
- Keep `go.mod` and `go.sum` changes intentional.

## Required Formatting
- Run `gofmt -w` on changed Go files.
- Keep imports goimports-style.
- Use repository command patterns from `AGENTS.md`.

## Prohibited
- Do not duplicate existing utility functions.
- Do not create meaningless wrappers.
- Do not create empty abstractions to make the architecture look cleaner.
- Do not create garbage-bag `utils` directories unless a standards doc defines a narrow responsibility.
- Do not put business logic in configuration files.
- Do not scatter magic numbers or magic strings.
- Do not swallow errors.
- Do not change core logic without tests or an explicit explanation.
- Do not leave unused code.
- Do not commit temporary debug code.
- Do not leak secrets or tokens in logs, docs, tests, or examples.
- Do not add standalone `internal/translator/` changes unless the `AGENTS.md` permission rule is satisfied.
- Do not set network timeouts after upstream connections are established except for documented exceptions.
