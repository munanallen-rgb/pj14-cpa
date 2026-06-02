param(
  [string]$OutputDir = "temp/cpa-sub2api-cloud",
  [string]$EnvOutputPath = "temp/cpa-sub2api-cloud.env",
  [switch]$SkipEnvGeneration
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
Set-Location $root

function Invoke-Step {
  param(
    [string]$Title,
    [scriptblock]$Body
  )

  Write-Host ""
  Write-Host "==> $Title"
  & $Body
}

Invoke-Step -Title "Checking all three local CPA instances" -Body {
  & powershell.exe -ExecutionPolicy Bypass -File ".\sub2api-deploy\verify-cpa-pool.ps1" -RequireAllCpaModels
  $code = $LASTEXITCODE
  if ($code -eq 2) {
    Write-Host ""
    Write-Host "CPA2/CPA3 are not ready yet. Finish Codex OAuth here:"
    Write-Host "  CPA2: http://127.0.0.1:8318/management.html"
    Write-Host "  CPA3: http://127.0.0.1:8319/management.html"
    exit 2
  }
  if ($code -ne 0) {
    exit $code
  }
}

Invoke-Step -Title "Refreshing local Sub2API CPA pool" -Body {
  & powershell.exe -ExecutionPolicy Bypass -File ".\sub2api-deploy\bootstrap-openai-pool.ps1" `
    -BaseUrl "http://127.0.0.1:18080" `
    -EnvFile ".\sub2api-deploy\.env" `
    -UpstreamMode local
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
}

if (-not $SkipEnvGeneration) {
  Invoke-Step -Title "Generating cloud .env" -Body {
    & powershell.exe -ExecutionPolicy Bypass -File ".\sub2api-deploy\generate-cloud-env.ps1" `
      -OutputPath $EnvOutputPath `
      -Force
    if ($LASTEXITCODE -ne 0) {
      exit $LASTEXITCODE
    }
  }
}

Invoke-Step -Title "Exporting final cloud bundle with OAuth auth files" -Body {
  & powershell.exe -ExecutionPolicy Bypass -File ".\sub2api-deploy\export-cloud-bundle.ps1" `
    -OutputDir $OutputDir `
    -IncludeAuth
  if ($LASTEXITCODE -ne 0) {
    exit $LASTEXITCODE
  }
}

Write-Host ""
Write-Host "Final local bundle is ready:"
Write-Host "  $OutputDir"
Write-Host ""
Write-Host "Deploy command template:"
Write-Host "  powershell.exe -ExecutionPolicy Bypass -File .\sub2api-deploy\deploy-droplet.ps1 -Server <server-ip> -User root -IncludeAuth -RemoteEnvFile $EnvOutputPath"
