param(
  [string]$Server = "159.65.7.65",
  [string]$User = "root",
  [int]$Port = 22,
  [string]$RemoteRoot = "/opt/cpa-sub2api",
  [string]$LocalBackupRoot = "backups",
  [int]$TransferTimeoutSeconds = 1800,
  [switch]$SkipRedis
)

$ErrorActionPreference = "Stop"

$repoRoot = (Get-Location).Path
$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$localRoot = if ([System.IO.Path]::IsPathRooted($LocalBackupRoot)) {
  $LocalBackupRoot
} else {
  Join-Path $repoRoot $LocalBackupRoot
}
$backupDir = Join-Path $localRoot "cloud-$stamp"
New-Item -ItemType Directory -Force -Path $backupDir | Out-Null

$sshTarget = "$User@$Server"
$remoteScript = @"
set -euo pipefail
stamp=`$(date -u +%Y%m%d-%H%M%S)
root="$RemoteRoot"
work="/tmp/cpa-cloud-backup-`$stamp"
archive="/tmp/cpa-cloud-backup-`$stamp.tar.gz"
mkdir -p "`$work"
cd "`$root"

copy_path() {
  src="`$1"
  if [ -e "`$src" ]; then
    mkdir -p "`$work/`$(dirname "`$src")"
    cp -a "`$src" "`$work/`$src"
  fi
}

copy_path docker-compose.cloud.yml
copy_path .env
copy_path config.yaml
copy_path config.yaml.bak-commercial
copy_path auths
copy_path logs
copy_path instances/cpa2/config.yaml
copy_path instances/cpa2/config.yaml.bak-commercial
copy_path instances/cpa2/auths
copy_path instances/cpa2/logs
copy_path instances/cpa3/config.yaml
copy_path instances/cpa3/config.yaml.bak-commercial
copy_path instances/cpa3/auths
copy_path instances/cpa3/logs
copy_path sub2api-deploy/.env
copy_path sub2api-deploy/data/config.yaml
copy_path sub2api-deploy/data/model_pricing.json
copy_path sub2api-deploy/data/model_pricing.sha256
copy_path sub2api-deploy/data/logs
"@

if (-not $SkipRedis) {
  $remoteScript += "`ncopy_path sub2api-deploy/redis_data`n"
}

$remoteScript += @"
mkdir -p "`$work/dumps"
docker exec sub2api-postgres pg_dump -U sub2api -d sub2api -Fc > "`$work/dumps/sub2api.dump"
{
  echo "created_at_utc=`$(date -u --iso-8601=seconds)"
  echo "source_host=`$(hostname)"
  echo "source_root=`$root"
  echo "include_redis=$(-not $SkipRedis)"
  echo "compose_services=`$(docker compose -f docker-compose.cloud.yml --env-file .env ps --services | tr '\n' ' ')"
} > "`$work/backup-manifest.txt"
tar -C "`$work" -czf "`$archive" .
sha256sum "`$archive" > "`$archive.sha256"
du -h "`$archive" "`$work/dumps/sub2api.dump"
echo "ARCHIVE=`$archive"
echo "SHA256=`$archive.sha256"
"@

$encoded = [Convert]::ToBase64String([System.Text.Encoding]::UTF8.GetBytes($remoteScript))
$sshBaseArgs = @(
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=60",
  "-o", "ConnectionAttempts=3",
  "-o", "ServerAliveInterval=20",
  "-o", "ServerAliveCountMax=6",
  "-p", [string]$Port,
  $sshTarget
)

function Invoke-TransferCommand {
  param(
    [Parameter(Mandatory = $true)][string]$FilePath,
    [Parameter(Mandatory = $true)][string[]]$ArgumentList,
    [Parameter(Mandatory = $true)][int]$TimeoutSeconds
  )

  $process = Start-Process -FilePath $FilePath -ArgumentList $ArgumentList -NoNewWindow -PassThru
  if (-not $process.WaitForExit($TimeoutSeconds * 1000)) {
    & taskkill.exe /PID $process.Id /T /F | Out-Null
    return $false
  }

  return $process.ExitCode -eq 0
}

Write-Host "Creating remote backup archive on $sshTarget..."
$remoteOutput = & ssh @sshBaseArgs "printf '%s' '$encoded' | base64 -d | bash"
if ($LASTEXITCODE -ne 0) {
  throw "Remote backup creation failed."
}
$remoteOutput | ForEach-Object { Write-Host $_ }

$archiveLine = $remoteOutput | Where-Object { $_ -like "ARCHIVE=*" } | Select-Object -Last 1
$shaLine = $remoteOutput | Where-Object { $_ -like "SHA256=*" } | Select-Object -Last 1
if (-not $archiveLine -or -not $shaLine) {
  throw "Remote backup output did not include archive paths."
}
$remoteArchive = $archiveLine.Substring("ARCHIVE=".Length)
$remoteSha = $shaLine.Substring("SHA256=".Length)
$archiveName = Split-Path -Leaf $remoteArchive
$localArchive = Join-Path $backupDir $archiveName
$localSha = "$localArchive.sha256"

Write-Host "Downloading archive to $backupDir..."
$scpArgs = @(
  "-o", "BatchMode=yes",
  "-o", "ConnectTimeout=60",
  "-o", "ConnectionAttempts=3",
  "-o", "ServerAliveInterval=20",
  "-o", "ServerAliveCountMax=6",
  "-P", [string]$Port,
  "$sshTarget`:$remoteArchive",
  "$sshTarget`:$remoteSha",
  $backupDir
)
$downloaded = Invoke-TransferCommand -FilePath "scp" -ArgumentList $scpArgs -TimeoutSeconds $TransferTimeoutSeconds
if (-not $downloaded) {
  if ((Test-Path -LiteralPath $localArchive) -and (Test-Path -LiteralPath $localSha)) {
    Write-Warning "scp returned a non-zero exit code after writing local files; continuing with SHA256 validation."
  } else {
    throw "Download failed. Remote temporary files may remain: $remoteArchive"
  }
}

$expected = (Get-Content -LiteralPath $localSha).Split(" ")[0].Trim().ToLowerInvariant()
$actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $localArchive).Hash.ToLowerInvariant()
if ($expected -ne $actual) {
  throw "SHA256 mismatch. Expected $expected, got $actual"
}

$contentsPath = Join-Path $backupDir "contents.txt"
& tar -tzf $localArchive | Set-Content -LiteralPath $contentsPath
if ($LASTEXITCODE -ne 0) {
  throw "Archive listing failed."
}
$contents = Get-Content -LiteralPath $contentsPath
foreach ($required in @("./dumps/sub2api.dump", "./auths/", "./instances/cpa2/auths/", "./instances/cpa3/auths/")) {
  if (-not ($contents | Where-Object { $_ -eq $required -or $_.StartsWith($required) })) {
    throw "Archive is missing required path: $required"
  }
}

Write-Host "Cleaning remote temporary files..."
$remoteWork = $remoteArchive -replace "\.tar\.gz$", ""
$cleanupCommand = "rm -rf $remoteArchive $remoteSha $remoteWork"
& ssh @sshBaseArgs $cleanupCommand
if ($LASTEXITCODE -ne 0) {
  Write-Warning "Backup is valid locally, but remote cleanup failed. Check $remoteArchive on the server."
} else {
  Write-Host "Remote temporary files cleaned."
}

Write-Host "Backup complete: $backupDir"
Write-Host "Archive: $localArchive"
Write-Host "Contents: $contentsPath"
