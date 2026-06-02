# Review Checklist

Run this checklist before finishing every task.

## Required Checks
- Did I read `AGENTS.md`?
- Did I read the relevant docs under `docs/`?
- Did I read the source files related to this task?
- Did I follow the directory structure rules?
- Did I reuse existing code where possible?
- Did I avoid duplicate implementations?
- Did I remove unused code?
- Did I add a dependency? If yes, is it necessary and documented?
- Did I add an abstraction? If yes, is it necessary now?
- Did I handle errors instead of swallowing them?
- Did I avoid leaking secrets or tokens?
- Did I add or update tests, or explain why tests were not needed/possible?
- Did I run relevant checks, tests, or builds?
- Did I update related documentation?
- Did I update `docs/CHANGELOG.md`?
- Do I need to update `docs/DECISIONS.md`?
- Did I avoid temporary code, debug code, and dead code?
- Is there an obviously simpler implementation?
- Did I avoid changes outside the requested scope?
- Did I preserve the existing architecture boundaries?
- If touching `internal/translator/`, did I follow the `AGENTS.md` permission rule?
- If touching network behavior, did I respect the timeout rule in `AGENTS.md`?

## Final Response Checklist
- List changed files.
- Explain why the changes were made.
- State whether dependencies were added.
- State whether directory structure changed.
- State whether docs and changelog were updated.
- State any unfinished or TBD (待确认) items.
- State checks/tests/builds run and results.
