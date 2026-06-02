[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)]
  [ValidateSet("cpa1", "cpa2", "cpa3", "all")]
  [string]$Target,

  [string]$Server = "159.65.7.65",
  [string]$User = "root",
  [int]$Port = 22,
  [string]$RemoteRoot = "/opt/cpa-sub2api"
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

function Get-RepoRoot {
  $scriptDir = Split-Path -Parent $PSCommandPath
  return (Resolve-Path -LiteralPath (Join-Path $scriptDir "..\..\..")).Path
}

function Get-TargetSpecs {
  param([string]$RemoteRoot)

  return @{
    cpa1 = @{
      LocalRelative = "auths"
      RemoteDir = "$RemoteRoot/auths"
      Container = "cpa1"
    }
    cpa2 = @{
      LocalRelative = "instances/cpa2/auths"
      RemoteDir = "$RemoteRoot/instances/cpa2/auths"
      Container = "cpa2"
    }
    cpa3 = @{
      LocalRelative = "instances/cpa3/auths"
      RemoteDir = "$RemoteRoot/instances/cpa3/auths"
      Container = "cpa3"
    }
  }
}

function Invoke-Remote {
  param(
    [string]$SshTarget,
    [int]$Port,
    [string]$Command
  )

  & ssh -p $Port $SshTarget $Command
  if ($LASTEXITCODE -ne 0) {
    throw "Remote command failed with exit code $LASTEXITCODE"
  }
}

function Invoke-RemoteAllowFailure {
  param(
    [string]$SshTarget,
    [int]$Port,
    [string]$Command
  )

  & ssh -p $Port $SshTarget $Command
}

function Parse-Sha256Lines {
  param([string[]]$Lines)

  $map = @{}
  foreach ($line in $Lines) {
    if ($line -match "^\s*([a-fA-F0-9]{64})\s+(.+?)\s*$") {
      $name = Split-Path -Leaf $Matches[2]
      $map[$name] = $Matches[1].ToLowerInvariant()
    }
  }
  return $map
}

function Deploy-OneTarget {
  param(
    [string]$Name,
    [hashtable]$Spec,
    [string]$RepoRoot,
    [string]$SshTarget,
    [int]$Port
  )

  $localDir = Join-Path $RepoRoot $Spec.LocalRelative
  if (-not (Test-Path -LiteralPath $localDir)) {
    throw "Local auth directory does not exist for ${Name}: $localDir"
  }

  $files = @(Get-ChildItem -LiteralPath $localDir -File -Filter "*.json" | Sort-Object Name)
  if ($files.Count -eq 0) {
    throw "No local JSON auth files found for ${Name}: $localDir"
  }

  Write-Host "==> $Name"
  Write-Host "Local source: $localDir"
  Write-Host "Remote dir: $($Spec.RemoteDir)"
  Write-Host "Files:"
  foreach ($file in $files) {
    Write-Host ("  {0} {1} bytes {2}" -f $file.Name, $file.Length, $file.LastWriteTime.ToString("yyyy-MM-ddTHH:mm:ss"))
  }

  $remoteDir = $Spec.RemoteDir
  $backupCommand = "set -eu; mkdir -p '$remoteDir'; ts=`$(date +%Y%m%d%H%M%S); backup='$remoteDir.backup-'`$ts; if ls '$remoteDir'/*.json >/dev/null 2>&1; then mkdir -p `"`$backup`"; cp -p '$remoteDir'/*.json `"`$backup`"/; printf 'backup=%s\n' `"`$backup`"; else printf 'backup=none\n'; fi"
  Invoke-Remote -SshTarget $SshTarget -Port $Port -Command $backupCommand

  $scpArgs = @("-P", [string]$Port)
  foreach ($file in $files) {
    $scpArgs += $file.FullName
  }
  $scpArgs += "${SshTarget}:$remoteDir/"

  & scp @scpArgs
  if ($LASTEXITCODE -ne 0) {
    throw "SCP upload failed for $Name with exit code $LASTEXITCODE"
  }

  Invoke-Remote -SshTarget $SshTarget -Port $Port -Command "chmod 600 '$remoteDir'/*.json"

  $localHashes = @{}
  foreach ($file in $files) {
    $localHashes[$file.Name] = (Get-FileHash -LiteralPath $file.FullName -Algorithm SHA256).Hash.ToLowerInvariant()
  }

  $remoteHashLines = @(Invoke-Remote -SshTarget $SshTarget -Port $Port -Command "cd '$remoteDir' && sha256sum *.json | sort")
  $remoteHashes = Parse-Sha256Lines -Lines $remoteHashLines

  foreach ($file in $files) {
    $remoteHash = $remoteHashes[$file.Name]
    if ([string]::IsNullOrWhiteSpace($remoteHash)) {
      throw "Remote hash missing for $Name/$($file.Name)"
    }
    if ($remoteHash -ne $localHashes[$file.Name]) {
      throw "SHA256 mismatch for $Name/$($file.Name)"
    }
  }
  Write-Host "SHA256 verified for $($files.Count) uploaded file(s)."

  Write-Host "Remote permissions:"
  Invoke-Remote -SshTarget $SshTarget -Port $Port -Command "stat -c '%n %s %a' '$remoteDir'/*.json | sort"

  $container = $Spec.Container
  Write-Host "Container auth directory:"
  Invoke-RemoteAllowFailure -SshTarget $SshTarget -Port $Port -Command "docker exec '$container' sh -c 'ls -l /root/.cli-proxy-api/*.json 2>/dev/null || true'"

  Write-Host "Recent auth-related logs:"
  Invoke-RemoteAllowFailure -SshTarget $SshTarget -Port $Port -Command "docker logs '$container' --since 5m 2>&1 | grep -i -E 'auth file changed|processing incrementally|auth|credential|error' | tail -50 || true"

  Write-Host "Completed $Name without container restart."
}

$repoRoot = Get-RepoRoot
$sshTarget = "$User@$Server"
$allSpecs = Get-TargetSpecs -RemoteRoot $RemoteRoot

$targets = if ($Target -eq "all") {
  @("cpa1", "cpa2", "cpa3")
} else {
  @($Target)
}

foreach ($name in $targets) {
  Deploy-OneTarget -Name $name -Spec $allSpecs[$name] -RepoRoot $repoRoot -SshTarget $sshTarget -Port $Port
}

Write-Host "Done. Restart is normally unnecessary when the container logs show incremental auth processing."
