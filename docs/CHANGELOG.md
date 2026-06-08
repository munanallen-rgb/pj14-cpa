# Changelog

All important changes are recorded in this file.

Format:

## YYYY-MM-DD

### Added
- ...

### Changed
- ...

### Fixed
- ...

### Removed
- ...

### Notes
- ...

## 2026-06-08

### Added
- Added a standalone `portal-api` command and `internal/portal` package for the PJ14 user MVP closed loop: registration/login, Sub2API user mapping, API key creation, user-scoped usage views, manual recharge orders, Portal ledger entries, and Sub2API balance updates.
- Added an embedded minimal Portal UI for local/cloud MVP validation.
- Added an admin-only Portal capacity dashboard that reuses CPA dashboard report logic for overview, quota efficiency, account health, usage trend, and cleanup-candidate views.
- Added cloud Docker Compose wiring, environment template entries, bundle export support, Dockerfile, and Portal runbook for the `portal-api` service.
- Added Portal tests for configuration loading, internal Sub2API password derivation, key previewing, Sub2API user/key creation, and balance updates.
- Added Portal admin dashboard authorization coverage.
- Added an authenticated Portal API key secret endpoint and copy buttons so users can copy existing API keys after creation without storing full keys in Portal-owned tables.

### Changed
- Documented the Portal architecture, directory responsibilities, and decision to keep Portal as a decoupled product boundary backed by the existing Sub2API Postgres database for the MVP.
- Changed cloud Portal deployment configuration to use a dedicated `PORTAL_SUB2API_ADMIN_EMAIL` and `PORTAL_SUB2API_ADMIN_PASSWORD` service account instead of relying on Sub2API bootstrap admin environment values.
- Changed generated Portal bootstrap admin defaults to use a Portal-specific email instead of reusing the Sub2API admin email.
- Restyled the embedded Portal UI with a Sub2API-inspired left-sidebar console layout, split normal-user and admin navigation, and hid Sub2API-internal labels from the normal user experience.
- Changed the CPA dashboard report service to expose reusable service methods so Portal can call the same report calculations without relying on the standalone `18090` frontend.
- Changed cloud startup and bundle export flows so Portal admin capacity reports are the supported dashboard surface.

### Fixed
- Nothing.

### Removed
- Removed the standalone `cpa-dashboard` command, embedded standalone dashboard frontend, `18090` Compose service, dashboard Dockerfile, dashboard runbook, and dashboard-specific read-only role setup script.

### Notes
- The MVP does not integrate a payment provider or migrate membership data to Supabase; recharge confirmation is manual.

## 2026-06-06

### Added
- Added a standalone `cpa-dashboard` command for viewing CPA quota efficiency, account health, Sub2API usage trends, and read-only auth cleanup candidates.
- Added cloud Docker Compose wiring, environment template entries, bundle export support, and a deployment runbook for the `cpa-dashboard` service.
- Added dashboard tests for weekly quota decrease calculations, same-email auth file grouping, normalization, range validation, and password rejection.

### Changed
- Added a configurable timeout to the PJ14 cloud backup `scp` download step and terminate the full process tree on timeout to avoid orphaned Windows OpenSSH child sessions.
- Let the PJ14 cloud backup continue to SHA256 validation when `scp` returns a non-zero exit code after writing both downloaded files.
- Documented PJ14 autobackup transfer pitfalls in the local skill, including slow Windows OpenSSH transfers, misleading `scp` exit codes, and avoiding public HTTP exposure for secret backup archives.
- Documented the dashboard directory responsibilities and the decision to keep the dashboard separate from proxying and quota collection.
- Updated the CPA dashboard monthly estimate multiplier to `4.3`, added monthly estimated quota cost, field help icons, million-token display units, and whole-dollar cost display.
- Added a browser-only CPA dashboard estimate panel for `预计实际额度RMB`, calculated from monthly estimated quota cost and a user-entered factor that defaults to `0.2`.

### Fixed
- Fixed CPA dashboard quota efficiency alignment so Sub2API usage and cost are clipped to each CPA instance's successful quota collection window instead of using pre-collector usage from the full selected range.

### Removed
- Nothing.

### Notes
- The backup still verifies SHA256 and `contents.txt` after download before cleaning remote temporary files.

## 2026-06-05

### Added
- Added a standalone CPA quota collector command that records Codex auth quota snapshots into a dedicated `cpa_monitor` Postgres schema.
- Added cloud Docker Compose wiring, environment template entries, and bundle export support for the `cpa-quota-collector` service.

### Changed
- Extended the management auth quota report account payload with an `auth_file` field for durable collector attribution.
- Documented the quota collector directory responsibilities and the decision to reuse the existing Sub2API Postgres instance with an isolated schema.
- Updated the cloud bundle exporter to build the quota collector through Docker when a local Go toolchain is unavailable.
- Allowed cloud Compose config to render before the collector management key is filled, while keeping the key required by the collector at runtime.
- Made the cloud CPA image pull policy configurable so locally loaded rollout images can be used safely.

### Fixed
- Nothing.

### Removed
- Nothing.

### Notes
- The collector does not modify CPA auth files or Sub2API business tables; dashboard work remains a future step.

## 2026-06-02

### Added
- Added the `pj14-cpa-auth-deploy` project skill for deploying prepared CPA auth JSON files from local CPA auth directories to the matching PJ14 cloud CPA instances.
- Added a PowerShell deployment helper that uploads auth JSON files, backs up existing remote JSON files, applies restrictive permissions, verifies SHA256 hashes, and checks container hot-load logs without restarting containers.
- Added a management auth quota report endpoint and a lightweight `/quota-report.html` page for querying selected CPA instances for Codex refresh validity and quota metadata.

### Changed
- Adopted `pj14-main` as the local long-lived PJ14 project branch and renamed the official CPA remote to `cpa-official`.
- Clarified PJ14 repository layout rules for keeping CPA source at the repository root while tracking deployment assets in the same local project.
- Updated Git ignore rules so `config.yaml` can be tracked while `backups/` remains local-only recovery material.
- Updated Sub2API bootstrap and verification helpers to read CPA API keys from tracked CPA config files and accept Sub2API verification keys from arguments or environment variables.
- Updated the auth quota report to show full management account emails and shorten the Codex quota status text.
- Updated the auth quota report to fetch Codex quota windows through the same ChatGPT wham/usage path used by the management panel quota refresh action.
- Improved the quota report page display with structured Codex quota bars, reset countdowns, and color-coded remaining quota states.
- Added Codex subscription remaining time to the auth quota report response and quota report page.

### Fixed
- Nothing.

### Removed
- Nothing.

### Notes
- The deployment skill requires an explicit `cpa1`, `cpa2`, `cpa3`, or `all` target before uploading.
- PJ14 continues to deploy with the existing local bundle/upload workflow and does not convert `/opt/cpa-sub2api` into a Git checkout in this stage.

## 2026-05-29

### Added
- Initialized the engineering standards documentation set for Agent collaboration.
- Added project overview, architecture, directory structure, coding standards, documentation standards, review checklist, workflow, changelog, and decision log documents.

### Changed
- Expanded `AGENTS.md` into the highest-priority Agent entrypoint while preserving existing Go commands, module map, and repository-specific constraints.

### Fixed
- Clarified maintenance rules for future AI Agent work to reduce duplicate code, unnecessary abstractions, and untracked changes.

### Removed
- Nothing.

### Notes
- Future undecided modules should remain documented as TBD (待确认) until a concrete design is accepted.
