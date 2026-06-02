# Agent Workflow

## 1. Start by Reading Context
Before editing, read:

1. `AGENTS.md`
2. `docs/PROJECT_OVERVIEW.md`
3. `docs/ARCHITECTURE.md`
4. `docs/DIRECTORY_STRUCTURE.md`
5. `docs/CODING_STANDARDS.md`
6. `docs/REVIEW_CHECKLIST.md`
7. Relevant source files, tests, configs, and docs for the task

If the task concerns a specific module, inspect nearby tests and existing patterns before making changes.

## 2. Decide Whether Docs Need Updates
Update docs when behavior, commands, config, public SDK contracts, architecture, directory structure, or standards change. Do not update docs for tiny internal changes unless leaving docs unchanged would mislead the next Agent.

## 3. Decide Whether a New File Is Needed
Before adding a file:

- Search for an existing file or package with the same responsibility.
- Prefer extending the closest existing file for small cohesive changes.
- Add a file only when it improves responsibility, navigation, or testing.
- Add a directory only when it has a distinct responsibility and no equivalent directory exists.

## 4. Decide Whether a New Dependency Is Needed
Before adding a dependency:

- Check the standard library.
- Check existing dependencies in `go.mod`.
- Check local helpers and packages.
- Add the dependency only if it clearly reduces complexity or risk.
- Record significant dependency decisions in `docs/DECISIONS.md`.

## 5. Handle Uncertainty
- Use current code as the source of truth.
- Mark unknown facts as TBD (待确认).
- Do not invent future modules, capabilities, APIs, or architecture.
- If a design is not decided, create a placeholder note rather than scaffolding code.

## 6. Make the Minimal Change
- Keep the change limited to the task.
- Reuse existing patterns.
- Avoid speculative generalization.
- Delete temporary or unused code before finishing.
- Keep behavior in the owner package instead of crossing module boundaries for convenience.

## 7. Verify
Run the smallest meaningful verification:

- Go formatting after Go changes: `gofmt -w <changed-go-files>`.
- Focused tests for changed packages.
- `go test ./...` for broad/shared behavior changes.
- Compile check after code changes: `go build -o test-output ./cmd/server && rm test-output`.

If a command cannot run in the current environment, record the reason and any fallback verification.

## 8. Update Traceability
- Update `docs/CHANGELOG.md` for meaningful changes.
- Update `docs/DECISIONS.md` for architecture, dependency, standards, technology, or directory decisions.
- Update related docs when code and docs would otherwise disagree.

## 9. When Standards Are Stale
If existing standards conflict with real project code:

1. Treat current real code as the source of truth.
2. Update the corresponding standards document.
3. Record the change in `docs/CHANGELOG.md`.
4. If the change is architectural or standards-level, add a record to `docs/DECISIONS.md`.
5. Mention the standards adjustment in the final response.

## 10. Finish With Self-Review
Use `docs/REVIEW_CHECKLIST.md` before responding. The final response must include changed files, rationale, dependency impact, directory impact, docs/changelog status, unfinished items, and verification results.
