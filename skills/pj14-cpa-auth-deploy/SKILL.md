---
name: pj14-cpa-auth-deploy
description: Deploy prepared CPA auth JSON files from the PJ14 local workspace to the matching PJ14 cloud CPA project. Use when the user asks to deploy or sync auth JSON files for cpa1, cpa2, cpa3, or all CPA instances on root@159.65.7.65. If the user does not clearly specify cpa1, cpa2, cpa3, or all, stop and ask which target to deploy before running any upload.
---

# PJ14 CPA Auth Deploy

## Purpose

Deploy only prepared auth JSON files from the local PJ14 workspace to the matching cloud CPA auth directory, then verify the upload without printing token contents or restarting containers.

## Target Map

Use this exact mapping unless the user explicitly overrides it:

| Target | Local source | Remote directory | Container |
| --- | --- | --- | --- |
| `cpa1` | `auths/` | `/opt/cpa-sub2api/auths/` | `cpa1` |
| `cpa2` | `instances/cpa2/auths/` | `/opt/cpa-sub2api/instances/cpa2/auths/` | `cpa2` |
| `cpa3` | `instances/cpa3/auths/` | `/opt/cpa-sub2api/instances/cpa3/auths/` | `cpa3` |

Remote SSH target:

```text
root@159.65.7.65
```

## Required User Intent

Before uploading, identify exactly one deployment target from the user's request:

- `cpa1`
- `cpa2`
- `cpa3`
- `all` or a clear request to deploy every CPA instance

If the target is missing or ambiguous, ask the user which target to deploy and stop. Do not infer a target from existing files.

## Quick Start

Run the bundled script from the repository root:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\skills\pj14-cpa-auth-deploy\scripts\deploy-cpa-auths.ps1 -Target cpa3
```

Deploy all three:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\skills\pj14-cpa-auth-deploy\scripts\deploy-cpa-auths.ps1 -Target all
```

Use parameters if the server changes:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\skills\pj14-cpa-auth-deploy\scripts\deploy-cpa-auths.ps1 `
  -Target cpa2 `
  -Server 159.65.7.65 `
  -User root `
  -Port 22 `
  -RemoteRoot /opt/cpa-sub2api
```

## Workflow

1. Confirm the requested target is explicit.
2. List local JSON files by filename, size, and mtime only. Never print auth JSON contents.
3. Run the script for the requested target.
4. Verify that each uploaded local file has a matching remote SHA256.
5. Check that the cloud container sees the files through its mounted auth directory.
6. Check recent container logs for auth file change processing.
7. Tell the user whether a restart is needed. Normally it is not needed when logs show incremental auth processing.

## Safety Rules

- Do not restart, stop, recreate, or remove containers during this deployment.
- Do not delete remote auth JSON files. Uploads may overwrite files with the same name.
- Do not print token contents, OAuth credentials, API keys, or full JSON payloads.
- Keep checks to filenames, sizes, permissions, hashes, mount metadata, and log lines.
- Treat `auths/`, `instances/*/auths/`, and remote auth directories as secret material.
- If SSH or SCP fails because of permissions or network restrictions, request the required approval and continue only after approval.

## Script Behavior

The script:

- Requires `-Target cpa1|cpa2|cpa3|all`.
- Creates a timestamped remote backup directory beside each target auth directory when remote JSON files already exist.
- Uploads local `*.json` files to the mapped remote auth directory.
- Sets remote JSON permissions to `600`.
- Compares SHA256 for each uploaded file.
- Confirms the Docker container can see `/root/.cli-proxy-api/*.json`.
- Shows recent auth-related container log lines without requiring a restart.

The script intentionally does not remove extra remote JSON files that are not present locally.
