# Local CPA upstreams for Sub2API

Use these OpenAI-compatible upstreams inside the Sub2API container:

| Instance | Local base URL | Cloud base URL | API key source |
| --- | --- | --- | --- |
| CPA1 | `http://host.docker.internal:8317/v1` | `http://cpa1:8317/v1` | `config.yaml` |
| CPA2 | `http://host.docker.internal:8318/v1` | `http://cpa2:8317/v1` | `instances/cpa2/config.yaml` |
| CPA3 | `http://host.docker.internal:8319/v1` | `http://cpa3:8317/v1` | `instances/cpa3/config.yaml` |

CPA2 and CPA3 need separate Codex OAuth auth files before they expose models.

Local Sub2API:

- URL: `http://127.0.0.1:18080`
- Admin credentials: `sub2api-deploy/.env`
- Exported OpenAI-compatible key: created by the Sub2API bootstrap flow and stored in Sub2API state

Repeatable bootstrap command:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\bootstrap-openai-pool.ps1 -BaseUrl http://127.0.0.1:18080 -EnvFile .\sub2api-deploy\.env -UpstreamMode local
```

Full readiness check:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\verify-cpa-pool.ps1
```

Final gate after CPA2/CPA3 OAuth:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\verify-cpa-pool.ps1 -RequireAllCpaModels
```

One-command final local handoff after CPA2/CPA3 OAuth:

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\finalize-local.ps1
```

CPA2 and CPA3 are already registered in Sub2API. Add separate Codex OAuth auth files to their management UIs before expecting them to carry traffic:

- CPA2 management: `http://127.0.0.1:8318/management.html`
- CPA3 management: `http://127.0.0.1:8319/management.html`
