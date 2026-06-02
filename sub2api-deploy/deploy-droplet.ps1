param(
  [Parameter(Mandatory = $true)]
  [string]$Server,
  [string]$User = "root",
  [int]$Port = 22,
  [string]$RemoteDir = "/opt/cpa-sub2api",
  [string]$BundleDir = "temp/cpa-sub2api-cloud",
  [string]$RemoteEnvFile = "",
  [switch]$IncludeAuth,
  [switch]$StartRemote,
  [switch]$DryRun
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$bundlePath = if ([System.IO.Path]::IsPathRooted($BundleDir)) {
  $BundleDir
} else {
  Join-Path $root $BundleDir
}
$bundleFullPath = [System.IO.Path]::GetFullPath($bundlePath)
$resolvedRemoteEnvFile = ""
$archivePath = Join-Path ([System.IO.Path]::GetTempPath()) ("cpa-sub2api-bundle-" + [System.Guid]::NewGuid().ToString("N") + ".tar.gz")

$exportArgs = @(
  "-ExecutionPolicy", "Bypass",
  "-File", (Join-Path $PSScriptRoot "export-cloud-bundle.ps1"),
  "-OutputDir", $bundleFullPath
)
if ($IncludeAuth) {
  $exportArgs += "-IncludeAuth"
}

$sshTarget = "$User@$Server"
$remoteTmp = "$RemoteDir.upload"

if (-not [string]::IsNullOrWhiteSpace($RemoteEnvFile)) {
  if ([System.IO.Path]::IsPathRooted($RemoteEnvFile)) {
    $envPath = $RemoteEnvFile
  } else {
    $envPath = Join-Path $root $RemoteEnvFile
  }
  $envFullPath = [System.IO.Path]::GetFullPath($envPath)
  if (-not (Test-Path -LiteralPath $envFullPath -PathType Leaf)) {
    throw "RemoteEnvFile does not exist: $envFullPath"
  }

  $resolvedRemoteEnvFile = Join-Path ([System.IO.Path]::GetTempPath()) ("cpa-sub2api-env-" + [System.Guid]::NewGuid().ToString("N") + ".env")
  Copy-Item -LiteralPath $envFullPath -Destination $resolvedRemoteEnvFile -Force
}

function Show-Command {
  param([string]$Command)
  Write-Host $Command
}

function Invoke-LoggedCommand {
  param(
    [string]$FilePath,
    [string[]]$ArgumentList,
    [string]$Display
  )

  Show-Command $Display
  if (-not $DryRun) {
    & $FilePath @ArgumentList
    if ($LASTEXITCODE -ne 0) {
      throw "Command failed with exit code $LASTEXITCODE`: $Display"
    }
  }
}

Write-Host "Exporting cloud bundle..."
Show-Command "powershell.exe $($exportArgs -join ' ')"
if (-not $DryRun) {
  & powershell.exe @exportArgs
  if ($LASTEXITCODE -ne 0) {
    throw "Bundle export failed with exit code $LASTEXITCODE"
  }
}

$remotePrepare = "rm -rf '$remoteTmp' && mkdir -p '$remoteTmp'"
$remoteArchive = "$remoteTmp.tar.gz"
$remoteUnpack = "rm -rf '$remoteTmp' && mkdir -p '$remoteTmp' && tar -xzf '$remoteArchive' -C '$remoteTmp' && rm -f '$remoteArchive'"
$remotePromote = "rm -rf '$RemoteDir' && mv '$remoteTmp' '$RemoteDir'"
$remoteStart = "cd '$RemoteDir' && if [ ! -f .env ]; then cp sub2api-deploy/.env.cloud.example .env; fi && docker compose -f docker-compose.cloud.yml --env-file .env up -d && docker compose -f docker-compose.cloud.yml --env-file .env ps"

Invoke-LoggedCommand -FilePath "tar" -ArgumentList @("-czf", $archivePath, "-C", $bundleFullPath, ".") -Display "tar -czf `"$archivePath`" -C `"$bundleFullPath`" ."

Invoke-LoggedCommand -FilePath "scp" -ArgumentList @("-P", [string]$Port, $archivePath, "$sshTarget`:$remoteArchive") -Display "scp -P $Port `"$archivePath`" $sshTarget`:$remoteArchive"

Invoke-LoggedCommand -FilePath "ssh" -ArgumentList @("-p", [string]$Port, $sshTarget, $remoteUnpack) -Display "ssh -p $Port $sshTarget `"$remoteUnpack`""
Invoke-LoggedCommand -FilePath "ssh" -ArgumentList @("-p", [string]$Port, $sshTarget, $remotePromote) -Display "ssh -p $Port $sshTarget `"$remotePromote`""

if (-not [string]::IsNullOrWhiteSpace($RemoteEnvFile)) {
  Invoke-LoggedCommand -FilePath "scp" -ArgumentList @("-P", [string]$Port, $resolvedRemoteEnvFile, "$sshTarget`:$RemoteDir/.env") -Display "scp -P $Port `"$resolvedRemoteEnvFile`" $sshTarget`:$RemoteDir/.env"
}

if ($StartRemote) {
  Invoke-LoggedCommand -FilePath "ssh" -ArgumentList @("-p", [string]$Port, $sshTarget, $remoteStart) -Display "ssh -p $Port $sshTarget `"$remoteStart`""
} else {
  Write-Host ""
  Write-Host "Bundle uploaded to $sshTarget`:$RemoteDir"
  Write-Host "Next on the droplet:"
  Write-Host "  cd $RemoteDir"
  if ([string]::IsNullOrWhiteSpace($RemoteEnvFile)) {
    Write-Host "  cp -n sub2api-deploy/.env.cloud.example .env"
    Write-Host "  nano .env"
  } else {
    Write-Host "  # .env was uploaded from $RemoteEnvFile"
  }
  Write-Host "  docker compose -f docker-compose.cloud.yml --env-file .env up -d"
}
