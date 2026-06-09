param(
  [string]$OutputPath = "",
  [string]$Sub2APIImage = "",
  [string]$CpaRemote = "cpa-official",
  [string]$CpaRef = "main",
  [string]$CpaBuildStatus = "pending",
  [string]$CpaDirectModelsStatus = "pending",
  [string]$FocusedGoTestsStatus = "pending",
  [string]$Sub2APIHealthStatus = "pending",
  [string]$Sub2APIUserFlowStatus = "pending",
  [string]$DeploymentStatus = "not_started"
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

if ([string]::IsNullOrWhiteSpace($OutputPath)) {
  $stamp = Get-Date -Format "yyyyMMdd"
  $OutputPath = Join-Path $root "temp/upgrade-manifest-$stamp.json"
} elseif (-not [System.IO.Path]::IsPathRooted($OutputPath)) {
  $OutputPath = Join-Path $root $OutputPath
}

if ([string]::IsNullOrWhiteSpace($Sub2APIImage)) {
  $Sub2APIImage = if ($env:SUB2API_IMAGE) { $env:SUB2API_IMAGE } else { "weishaw/sub2api:latest" }
}

function Invoke-Text {
  param([string[]]$CommandArgs)

  $command = $CommandArgs[0]
  $arguments = @()
  if ($CommandArgs.Count -gt 1) {
    $arguments = $CommandArgs[1..($CommandArgs.Count - 1)]
  }

  $output = & $command @arguments 2>$null
  if ($LASTEXITCODE -ne 0) {
    return ""
  }
  return ($output -join "`n").Trim()
}

function Read-DockerImage {
  param([string]$Image)

  $raw = Invoke-Text -CommandArgs @("docker", "image", "inspect", $Image)
  if ([string]::IsNullOrWhiteSpace($raw)) {
    return [PSCustomObject]@{
      image = $Image
      image_id = ""
      repo_digests = @()
      created = ""
      inspected = $false
    }
  }

  $items = $raw | ConvertFrom-Json
  $item = @($items)[0]
  return [PSCustomObject]@{
    image = $Image
    image_id = $item.Id
    repo_digests = @($item.RepoDigests)
    created = $item.Created
    inspected = $true
  }
}

$status = Invoke-Text -CommandArgs @("git", "status", "--short")
$head = Invoke-Text -CommandArgs @("git", "rev-parse", "HEAD")
$branch = Invoke-Text -CommandArgs @("git", "branch", "--show-current")
$upstreamCommit = Invoke-Text -CommandArgs @("git", "rev-parse", "$CpaRemote/$CpaRef")

$manifest = [PSCustomObject]@{
  schema_version = 1
  generated_at = (Get-Date).ToString("o")
  workspace = [PSCustomObject]@{
    branch = $branch
    head_commit = $head
    dirty = -not [string]::IsNullOrWhiteSpace($status)
  }
  cpa = [PSCustomObject]@{
    upstream_remote = $CpaRemote
    upstream_ref = $CpaRef
    upstream_commit = $upstreamCommit
    tested_commit = $head
  }
  sub2api = Read-DockerImage -Image $Sub2APIImage
  verification = [PSCustomObject]@{
    cpa_build = $CpaBuildStatus
    cpa_direct_models = $CpaDirectModelsStatus
    focused_go_tests = $FocusedGoTestsStatus
    sub2api_health = $Sub2APIHealthStatus
    sub2api_user_flow = $Sub2APIUserFlowStatus
    deployment = $DeploymentStatus
  }
  notes = @(
    "This manifest records non-secret version and verification metadata only.",
    "Generated manifests belong under temp/ and must not be committed."
  )
}

$parent = Split-Path -Parent $OutputPath
New-Item -ItemType Directory -Force -Path $parent | Out-Null
$manifest | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $OutputPath -Encoding UTF8

Write-Host "Upgrade manifest written:"
Write-Host "  $OutputPath"
