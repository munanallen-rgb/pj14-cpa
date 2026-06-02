# Documentation Standards

## When Documentation Is Required
- New user-visible behavior.
- New or changed commands, flags, configuration, environment variables, or deployment steps.
- New public SDK APIs or changed SDK contracts.
- Architecture, directory, dependency, or standards changes.
- Significant bug fixes where future maintainers need context.

## When Documentation Must Be Updated
- Code behavior no longer matches an existing doc.
- A config example changes.
- A module moves or its responsibility changes.
- A command, test, build, or run step changes.
- A previously TBD (待确认) item becomes known.

## Writing Rules
- Write facts, not slogans.
- Keep docs short and actionable.
- Keep documentation consistent with current code.
- Mark uncertain information as TBD (待确认).
- Do not invent capabilities that do not exist.
- Do not add template filler unrelated to this project.
- Prefer links to existing docs over copying long content.
- New Markdown docs should be English unless the file is language-specific.

## Traceability
- Update `docs/CHANGELOG.md` for meaningful documentation or standards changes.
- Update `docs/DECISIONS.md` for architecture, technology, dependency, directory, or standards decisions.
- If documentation standards were wrong or stale, update them in the same task that discovers the mismatch.

## Minimal Burden Rule
Do not create process-heavy documentation for small code changes. Update the smallest relevant doc that keeps future Agents from misunderstanding the project.
