# CPA + Sub2API 云端部署手册

这份手册用于在一台 Linux Droplet 上部署 3 个 CLIProxyAPI 实例和 1 个 Sub2API 网关。

## 目标拓扑

外部客户端只需要调用：

- `http://<server-ip>:18080/v1`

Sub2API 在 Docker 内部网络里调用 3 个 CPA 上游：

| 实例 | 内部 Base URL | API key 来源 |
| --- | --- | --- |
| CPA1 | `http://cpa1:8317/v1` | `config.yaml` |
| CPA2 | `http://cpa2:8317/v1` | `instances/cpa2/config.yaml` |
| CPA3 | `http://cpa3:8317/v1` | `instances/cpa3/config.yaml` |

不要在 runbook 中写入真实 API key、OAuth token、管理员密码或数据库密码。真实密钥保存在配置文件、`.env`、auth 目录、Sub2API 数据库和备份包中。

## 准备文件

将这些路径复制到 Droplet：

- `docker-compose.cloud.yml`
- `config.yaml`
- `instances/cpa2/config.yaml`
- `instances/cpa3/config.yaml`
- `auths/`
- `instances/cpa2/auths/`
- `instances/cpa3/auths/`
- `sub2api-deploy/.env.cloud.example`

也可以在本地先导出一个干净 bundle：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\export-cloud-bundle.ps1
```

CPA1/CPA2/CPA3 都登录完成后，导出包含 auth 的最终包：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\export-cloud-bundle.ps1 -IncludeAuth
```

也可以直接跑本地收口脚本，它会先验证 3 个 CPA，再刷新 Sub2API、生成云端 `.env`、导出最终包：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\finalize-local.ps1
```

默认输出目录是 `temp/cpa-sub2api-cloud`。如果使用了 `-IncludeAuth`，这个目录包含 OAuth 文件，要当成敏感文件处理。

## 上传到服务器

先 dry-run 查看命令：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\deploy-droplet.ps1 -Server <server-ip> -User root -DryRun
```

上传包含 auth 的包：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\deploy-droplet.ps1 -Server <server-ip> -User root -IncludeAuth
```

如果想上传后立刻启动 compose，可以追加 `-StartRemote`。第一次启动前仍然要在服务器上准备真实 `.env`。

## 生成云端环境变量

到服务器后，将模板复制成 `.env`：

```bash
cp sub2api-deploy/.env.cloud.example .env
```

然后编辑 `.env`，把所有 `replace-with-*` 替换成强随机值。

也可以在本地生成强随机 `.env`：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\generate-cloud-env.ps1 -OutputPath temp\cpa-sub2api-cloud.env -AdminEmail admin@sub2api.local
```

配合部署脚本上传并作为远端 `.env`：

```powershell
powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\deploy-droplet.ps1 -Server <server-ip> -User root -IncludeAuth -RemoteEnvFile temp\cpa-sub2api-cloud.env
```

## 启动

```bash
docker compose -f docker-compose.cloud.yml --env-file .env up -d
docker compose -f docker-compose.cloud.yml --env-file .env ps
```

也可以在 Droplet 上使用一键启动脚本：

```bash
bash sub2api-deploy/cloud-start.sh
```

CPA1/CPA2/CPA3 的 auth 都就位后，可以开启最终检查：

```bash
REQUIRE_ALL=1 bash sub2api-deploy/cloud-start.sh
```

## 备份

日常备份使用 PJ14 自动备份 skill。备份产物放在本地 `backups/`，包含 `.env`、auth、配置、Redis 文件和 Sub2API 数据库逻辑 dump。`backups/` 是敏感资料，不提交 Git。
