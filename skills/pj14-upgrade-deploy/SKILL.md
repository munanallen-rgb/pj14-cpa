---
name: pj14-upgrade-deploy
description: Update, locally test, cloud deploy, and verify the PJ14 CPA + Sub2API stack. Use when the user asks to pull the latest CLIProxyAPI/CPA from cpa-official/main, pull the latest Sub2API image, record non-secret upgrade manifests, run local validation, back up root@159.65.7.65:/opt/cpa-sub2api, deploy tested images to cloud, or validate the production CPA/Sub2API/Portal services after an upgrade.
---

# PJ14 Upgrade Deploy

## Purpose

Run the complete PJ14 upgrade path for CPA and Sub2API: update local candidates, test locally, record a non-secret manifest, wait for user deployment approval, back up production, deploy only tested versions, and verify cloud health.

## Constants

- Workspace: `D:\WorkSpace\Ai-vibe-coding-2\pj14-cliproxyapi`
- Working branch: `pj14-main`
- Official CPA remote: `cpa-official/main`
- Cloud target: `root@159.65.7.65`
- Cloud root: `/opt/cpa-sub2api`
- Cloud compose file: `docker-compose.cloud.yml`
- Manifest example: `sub2api-deploy/upgrade-manifest.example.json`
- Generated manifests: `temp/upgrade-manifest-YYYYMMDD.json`
- CPA image tag pattern: `cli-proxy-api:pj14-<shortsha>`
- Sub2API cloud image: use the tested repo digest, not floating `latest`.

## Safety Rules

- Do not deploy until the user explicitly approves cloud deployment after local validation.
- Do not use `sub2api-deploy/deploy-droplet.ps1` for upgrades; it can clear `/opt/cpa-sub2api`.
- Never overwrite or delete cloud runtime state: `.env`, `auths/`, `instances/*/auths/`, `logs/`, `instances/*/logs/`, `sub2api-deploy/data/`, `sub2api-deploy/postgres_data/`, or `sub2api-deploy/redis_data/`.
- Do not commit secrets, auth JSON, backups, Docker image tar files, generated manifests under `temp/`, or cloud `.env` files.
- Do not print API keys, OAuth token contents, database credentials, or full auth JSON.
- If SSH/SCP/Docker fails due to sandbox restrictions, retry the same needed command with `sandbox_permissions: "require_escalated"` and a narrow justification.

## Preflight

1. Read `AGENTS.md` and the required project docs before editing or deploying.
2. Check local status:

```powershell
git status --short --branch
git branch -vv
git fetch cpa-official
git rev-list --left-right --count main...cpa-official/main
```

3. If local `main` has `0 N` against `cpa-official/main`, it has no local-only commits and can be fast-forwarded with `git branch -f main cpa-official/main` when the user wants the branch list cleaned.
4. If unrelated user changes exist, leave them alone. If they conflict with upgrade files, ask before proceeding.

## Local Update

1. Update CPA on `pj14-main`:

```powershell
git checkout pj14-main
git merge --no-edit cpa-official/main
```

2. Pull and inspect Sub2API:

```powershell
docker pull weishaw/sub2api:latest
docker image inspect weishaw/sub2api:latest
```

Record the image ID and repo digest.

3. Build the tested CPA image:

```powershell
$short = (git rev-parse --short HEAD)
docker build -t "cli-proxy-api:pj14-$short" .
```

## Local Validation

Run validation before any cloud deployment:

```powershell
go build -o test-output ./cmd/server
Remove-Item -LiteralPath test-output -Force
```

Run focused tests for touched areas. When local Go is unavailable, use Docker Go 1.26.

Verify Sub2API health and normal-user behavior:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\verify-sub2api-user-flow.ps1
```

Verify CPA model availability when local auth exists:

```powershell
python sub2api-deploy\cloud_tools.py verify --sub2api-url http://127.0.0.1:18080 --require-all-cpa-models
```

If a local CPA instance has no auth files, mark that validation as partial and make cloud `--require-all-cpa-models` the final gate.

Write or update the non-secret manifest:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\write-upgrade-manifest.ps1 `
  -OutputPath temp\upgrade-manifest-YYYYMMDD.json `
  -Sub2APIImage weishaw/sub2api:latest `
  -CpaBuildStatus passed `
  -CpaDirectModelsStatus passed_or_partial_reason `
  -FocusedGoTestsStatus passed `
  -Sub2APIHealthStatus passed `
  -Sub2APIUserFlowStatus passed `
  -DeploymentStatus not_started
```

Report the CPA commit, Sub2API digest, local test results, and manifest path to the user. Stop for deployment approval.

## Production Backup

After approval, run the `pj14-autobackup` skill or its bundled script before touching cloud services:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\skills\pj14-autobackup\scripts\backup-pj14-cloud.ps1
```

Require all backup validations before deploying: SHA256 match, `contents.txt`, `dumps/sub2api.dump`, all CPA auth directories, and remote temp cleanup.

## Package And Upload

Export the cloud bundle without local auth:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\export-cloud-bundle.ps1 -OutputDir temp\cpa-sub2api-cloud-upgrade
docker save -o temp\cli-proxy-api-pj14-<shortsha>.tar cli-proxy-api:pj14-<shortsha>
tar -czf temp\cpa-sub2api-cloud-upgrade.tar.gz -C temp cpa-sub2api-cloud-upgrade
scp temp\cli-proxy-api-pj14-<shortsha>.tar root@159.65.7.65:/tmp/
scp temp\cpa-sub2api-cloud-upgrade.tar.gz root@159.65.7.65:/tmp/
```

The exported bundle contains empty auth/data directories. Do not copy directories wholesale into the cloud root.

## Apply Cloud Bundle Safely

On the cloud server:

```bash
mkdir -p /opt/cpa-sub2api.releases/upgrade-YYYYMMDD-HHMM
tar -xzf /tmp/cpa-sub2api-cloud-upgrade.tar.gz -C /opt/cpa-sub2api.releases/upgrade-YYYYMMDD-HHMM --strip-components=1
docker load -i /tmp/cli-proxy-api-pj14-<shortsha>.tar
cp /opt/cpa-sub2api/.env /opt/cpa-sub2api/.env.bak-YYYYMMDD-HHMM
```

Update `/opt/cpa-sub2api/.env` without printing values:

```text
CLI_PROXY_IMAGE=cli-proxy-api:pj14-<shortsha>
CLI_PROXY_PULL_POLICY=never
SUB2API_IMAGE=weishaw/sub2api@sha256:<tested-digest>
```

Copy only actual files from staging into `/opt/cpa-sub2api`; avoid copying empty runtime directories:

```bash
cd /opt/cpa-sub2api.releases/upgrade-YYYYMMDD-HHMM
find . -maxdepth 3 -type f -print0 | tar --null -cf /tmp/cpa-sub2api-selected-YYYYMMDD.tar -T -
tar -xf /tmp/cpa-sub2api-selected-YYYYMMDD.tar -C /opt/cpa-sub2api
```

Before restarting, verify cloud runtime directories still contain files:

```bash
find /opt/cpa-sub2api/auths -mindepth 1 -maxdepth 1 | wc -l
find /opt/cpa-sub2api/instances/cpa2/auths -mindepth 1 -maxdepth 1 | wc -l
find /opt/cpa-sub2api/instances/cpa3/auths -mindepth 1 -maxdepth 1 | wc -l
find /opt/cpa-sub2api/sub2api-deploy/postgres_data -mindepth 1 -maxdepth 1 | wc -l
find /opt/cpa-sub2api/sub2api-deploy/redis_data -mindepth 1 -maxdepth 1 | wc -l
find /opt/cpa-sub2api/sub2api-deploy/data -mindepth 1 -maxdepth 1 | wc -l
```

Pull the tested Sub2API digest and check Compose:

```bash
docker pull weishaw/sub2api@sha256:<tested-digest>
cd /opt/cpa-sub2api
docker compose -f docker-compose.cloud.yml --env-file .env config --quiet
```

## Deploy

Start the upgraded services:

```bash
cd /opt/cpa-sub2api
docker compose -f docker-compose.cloud.yml --env-file .env up -d --build cpa1 cpa2 cpa3 sub2api cpa-quota-collector portal-api
```

Do not recreate Postgres or Redis unless the user explicitly asks and a backup exists.

## Cloud Validation

Run these checks after deployment:

```bash
cd /opt/cpa-sub2api
docker compose -f docker-compose.cloud.yml --env-file .env ps
curl -fsS http://127.0.0.1:18080/health
curl -fsS http://127.0.0.1:18100/health
python3 sub2api-deploy/cloud_tools.py verify --sub2api-url http://127.0.0.1:18080 --require-all-cpa-models
docker compose -f docker-compose.cloud.yml --env-file .env logs --no-color --tail=80 sub2api
docker compose -f docker-compose.cloud.yml --env-file .env logs --no-color --tail=80 cpa1 cpa2 cpa3
docker compose -f docker-compose.cloud.yml --env-file .env logs --no-color --tail=80 portal-api cpa-quota-collector
```

Pass criteria:

- `cpa1`, `cpa2`, `cpa3` run the tested CPA image.
- `sub2api` runs the tested digest and is healthy.
- `portal-api` is healthy.
- `cloud_tools.py verify --require-all-cpa-models` returns all CPA instances ready.
- Recent logs do not show deployment-caused crashes or missing runtime data.

If an existing production API key is available through a secret-safe path, run a `/v1/chat/completions` or `/v1/responses` smoke test without printing the key. If Sub2API admin credentials or group permissions block creating a smoke key, do not force production user changes; report the limitation and use direct CPA checks plus live traffic logs as evidence.

Update the generated manifest:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\write-upgrade-manifest.ps1 `
  -OutputPath temp\upgrade-manifest-YYYYMMDD.json `
  -Sub2APIImage weishaw/sub2api:latest `
  -CpaBuildStatus passed `
  -CpaDirectModelsStatus passed_cloud_all_cpa_models `
  -FocusedGoTestsStatus passed `
  -Sub2APIHealthStatus passed_cloud `
  -Sub2APIUserFlowStatus passed_local_cloud_live_traffic_observed `
  -DeploymentStatus deployed_cloud_validated
```

## Git Cleanup

Commit reusable project changes, not generated artifacts:

```powershell
git status --short
git add -- <reusable files only>
git commit -m "chore: record pj14 upgrade workflow"
```

Keep `temp/upgrade-manifest-YYYYMMDD.json`, backups, Docker image tar files, `.env`, auth files, and logs out of Git. Do not push to `cpa-official` unless the user explicitly requests and the remote is appropriate.

## Final Report

Include:

- CPA commit and image tag.
- Sub2API tested digest.
- Backup archive path.
- Manifest path.
- Local tests and cloud checks run with pass/fail results.
- Any partial validation or TBD items.
- Files changed, dependency status, directory-structure impact, docs/changelog status, and checks run per `AGENTS.md`.
