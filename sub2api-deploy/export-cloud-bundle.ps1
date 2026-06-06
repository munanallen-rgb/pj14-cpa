param(
  [string]$OutputDir = "temp/cpa-sub2api-cloud",
  [switch]$IncludeAuth
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$outputPath = if ([System.IO.Path]::IsPathRooted($OutputDir)) {
  $OutputDir
} else {
  Join-Path $root $OutputDir
}

$outputFullPath = [System.IO.Path]::GetFullPath($outputPath)
$rootFullPath = [System.IO.Path]::GetFullPath($root)

if (-not $outputFullPath.StartsWith($rootFullPath, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "OutputDir must be inside the repository workspace: $rootFullPath"
}

if ((Split-Path -Leaf $outputFullPath) -eq "" -or $outputFullPath -eq $rootFullPath) {
  throw "Refusing to export directly into the repository root."
}

function Copy-RequiredFile {
  param(
    [string]$RelativePath,
    [string]$DestinationRelativePath = $RelativePath
  )

  $source = Join-Path $root $RelativePath
  if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
    throw "Required file is missing: $RelativePath"
  }

  $destination = Join-Path $outputFullPath $DestinationRelativePath
  $destinationParent = Split-Path -Parent $destination
  New-Item -ItemType Directory -Force -Path $destinationParent | Out-Null
  Copy-Item -LiteralPath $source -Destination $destination -Force
}

function Ensure-Directory {
  param([string]$RelativePath)
  New-Item -ItemType Directory -Force -Path (Join-Path $outputFullPath $RelativePath) | Out-Null
}

function Copy-DirectoryContents {
  param(
    [string]$RelativePath,
    [string]$DestinationRelativePath = $RelativePath
  )

  $source = Join-Path $root $RelativePath
  if (-not (Test-Path -LiteralPath $source -PathType Container)) {
    throw "Required directory is missing: $RelativePath"
  }

  $destination = Join-Path $outputFullPath $DestinationRelativePath
  New-Item -ItemType Directory -Force -Path $destination | Out-Null

  Get-ChildItem -LiteralPath $source -Force | ForEach-Object {
    Copy-Item -LiteralPath $_.FullName -Destination $destination -Recurse -Force
  }
}

function Build-LinuxGoBinary {
  param(
    [string]$CommandPath,
    [string]$OutputBinary,
    [string]$DisplayName
  )

  if (Get-Command go -ErrorAction SilentlyContinue) {
    $previousGOOS = $env:GOOS
    $previousGOARCH = $env:GOARCH
    $previousCGO = $env:CGO_ENABLED
    try {
      $env:GOOS = "linux"
      $env:GOARCH = "amd64"
      $env:CGO_ENABLED = "0"
      & go build -o $OutputBinary $CommandPath
      if ($LASTEXITCODE -ne 0) {
        throw "Failed to build $DisplayName binary"
      }
    } finally {
      $env:GOOS = $previousGOOS
      $env:GOARCH = $previousGOARCH
      $env:CGO_ENABLED = $previousCGO
    }
    return
  }

  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    throw "Neither go nor docker is available. Install Go 1.26+ or start Docker Desktop before exporting the cloud bundle."
  }

  $relativePath = $OutputBinary.Substring($rootFullPath.Length).TrimStart("\", "/").Replace("\", "/")
  $dockerBuildCommand = "CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ""/app/$relativePath"" $CommandPath"
  & docker run --rm -v "$($rootFullPath):/app" -w /app golang:1.26-alpine sh -c $dockerBuildCommand
  if ($LASTEXITCODE -ne 0) {
    throw "Failed to build $DisplayName binary with Docker"
  }
}

if (Test-Path -LiteralPath $outputFullPath) {
  Remove-Item -LiteralPath $outputFullPath -Recurse -Force
}
New-Item -ItemType Directory -Force -Path $outputFullPath | Out-Null

Copy-RequiredFile -RelativePath "docker-compose.cloud.yml"
Copy-RequiredFile -RelativePath "config.yaml"
Copy-RequiredFile -RelativePath "instances/cpa2/config.yaml"
Copy-RequiredFile -RelativePath "instances/cpa3/config.yaml"
Copy-RequiredFile -RelativePath "sub2api-deploy/.env.cloud.example"
Copy-RequiredFile -RelativePath "sub2api-deploy/CLOUD_RUNBOOK_CN.md"
Copy-RequiredFile -RelativePath "sub2api-deploy/CPA_UPSTREAMS.md"
Copy-RequiredFile -RelativePath "sub2api-deploy/CPA_QUOTA_COLLECTOR.md"
Copy-RequiredFile -RelativePath "sub2api-deploy/bootstrap-openai-pool.ps1"
Copy-RequiredFile -RelativePath "sub2api-deploy/verify-cpa-pool.ps1"
Copy-RequiredFile -RelativePath "sub2api-deploy/deploy-droplet.ps1"
Copy-RequiredFile -RelativePath "sub2api-deploy/generate-cloud-env.ps1"
Copy-RequiredFile -RelativePath "sub2api-deploy/cloud_tools.py"
Copy-RequiredFile -RelativePath "sub2api-deploy/cloud-start.sh"
Copy-RequiredFile -RelativePath "sub2api-deploy/create-dashboard-db-role.sh"
Copy-RequiredFile -RelativePath "sub2api-deploy/finalize-local.ps1"
Copy-RequiredFile -RelativePath "sub2api-deploy/quota-collector.Dockerfile"
Copy-RequiredFile -RelativePath "sub2api-deploy/cpa-dashboard.Dockerfile"
Copy-RequiredFile -RelativePath "sub2api-deploy/CPA_DASHBOARD.md"

Ensure-Directory -RelativePath "logs"
Ensure-Directory -RelativePath "instances/cpa2/logs"
Ensure-Directory -RelativePath "instances/cpa3/logs"
Ensure-Directory -RelativePath "sub2api-deploy/data"
Ensure-Directory -RelativePath "sub2api-deploy/postgres_data"
Ensure-Directory -RelativePath "sub2api-deploy/redis_data"
Ensure-Directory -RelativePath "sub2api-deploy/quota-collector"
Ensure-Directory -RelativePath "sub2api-deploy/cpa-dashboard"

$collectorBinary = Join-Path $outputFullPath "sub2api-deploy/quota-collector/quota-collector"
$dashboardBinary = Join-Path $outputFullPath "sub2api-deploy/cpa-dashboard/cpa-dashboard"

Build-LinuxGoBinary -CommandPath "./cmd/quota_collector" -OutputBinary $collectorBinary -DisplayName "quota collector"
Build-LinuxGoBinary -CommandPath "./cmd/cpa_dashboard" -OutputBinary $dashboardBinary -DisplayName "CPA dashboard"

if ($IncludeAuth) {
  Copy-DirectoryContents -RelativePath "auths"
  Copy-DirectoryContents -RelativePath "instances/cpa2/auths"
  Copy-DirectoryContents -RelativePath "instances/cpa3/auths"
} else {
  Ensure-Directory -RelativePath "auths"
  Ensure-Directory -RelativePath "instances/cpa2/auths"
  Ensure-Directory -RelativePath "instances/cpa3/auths"
}

$manifest = [PSCustomObject]@{
  created_at = (Get-Date).ToString("o")
  include_auth = [bool]$IncludeAuth
  output_dir = $outputFullPath
  next_steps = @(
    "Copy this directory to the droplet.",
    "On the droplet, run: cp sub2api-deploy/.env.cloud.example .env",
    "Edit .env secrets, including CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY.",
    "Run: docker compose -f docker-compose.cloud.yml --env-file .env up -d",
    "Create the read-only CPA dashboard database role: bash sub2api-deploy/create-dashboard-db-role.sh",
    "Run: docker compose -f docker-compose.cloud.yml --env-file .env up -d cpa-dashboard",
    "Run bootstrap-openai-pool.ps1 from a machine that can reach Sub2API.",
    "Run verify-cpa-pool.ps1 as the final gate."
  )
}

$manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $outputFullPath "bundle-manifest.json") -Encoding UTF8

Write-Host "Cloud bundle exported:"
Write-Host "  $outputFullPath"
Write-Host ""
if ($IncludeAuth) {
  Write-Host "Auth directories were included. Treat this bundle as sensitive."
} else {
  Write-Host "Auth directories were created empty. Re-run with -IncludeAuth after CPA2/CPA3 OAuth is complete."
}
