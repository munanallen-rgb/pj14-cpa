param(
  [string]$Sub2ApiUrl = "http://127.0.0.1:18080",
  [string]$Sub2ApiKey = "",
  [string]$Cpa1BaseUrl = "http://127.0.0.1:8317/v1",
  [string]$Cpa2BaseUrl = "http://127.0.0.1:8318/v1",
  [string]$Cpa3BaseUrl = "http://127.0.0.1:8319/v1",
  [string]$Cpa1Key = "",
  [string]$Cpa2Key = "",
  [string]$Cpa3Key = "",
  [string]$Cpa1ConfigPath = "config.yaml",
  [string]$Cpa2ConfigPath = "instances/cpa2/config.yaml",
  [string]$Cpa3ConfigPath = "instances/cpa3/config.yaml",
  [string]$Model = "gpt-5.4-mini",
  [switch]$RequireAllCpaModels,
  [switch]$SkipDirectCpaChecks
)

$ErrorActionPreference = "Stop"

function Read-CpaApiKey {
  param([string]$Path)

  if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
    throw "CPA config file is missing: $Path"
  }

  foreach ($line in Get-Content -LiteralPath $Path) {
    if ($line -match '^\s*-\s*"?([^"\s]+)"?\s*$') {
      return $Matches[1]
    }
  }

  throw "No api-keys entry found in $Path"
}

function Test-CpaModels {
  param(
    [string]$Name,
    [string]$BaseUrl,
    [string]$ApiKey
  )

  try {
    $response = Invoke-RestMethod -Method Get -Uri "$BaseUrl/models" -Headers @{ Authorization = "Bearer $ApiKey" } -TimeoutSec 30
    $items = @($response.data)
    return [PSCustomObject]@{
      instance = $Name
      ok = $true
      model_count = $items.Count
      first_model = if ($items.Count -gt 0) { $items[0].id } else { "" }
      error = ""
    }
  } catch {
    $message = if ($_.ErrorDetails.Message) { $_.ErrorDetails.Message } else { $_.Exception.Message }
    return [PSCustomObject]@{
      instance = $Name
      ok = $false
      model_count = 0
      first_model = ""
      error = $message
    }
  }
}

function Test-Sub2ApiHealth {
  try {
    $response = Invoke-RestMethod -Method Get -Uri "$Sub2ApiUrl/health" -TimeoutSec 30
    return [PSCustomObject]@{
      ok = $true
      status = $response.status
      error = ""
    }
  } catch {
    $message = if ($_.ErrorDetails.Message) { $_.ErrorDetails.Message } else { $_.Exception.Message }
    return [PSCustomObject]@{
      ok = $false
      status = ""
      error = $message
    }
  }
}

function Test-Sub2ApiChat {
  if ([string]::IsNullOrWhiteSpace($Sub2ApiKey)) {
    return [PSCustomObject]@{
      ok = $true
      status = 0
      model = $Model
      content = ""
      id = ""
      skipped = $true
      error = "Skipped because Sub2ApiKey was not provided. Set -Sub2ApiKey or SUB2API_API_KEY for chat verification."
    }
  }

  try {
    $body = @{
      model = $Model
      messages = @(
        @{
          role = "user"
          content = "Say hello in one short sentence."
        }
      )
    } | ConvertTo-Json -Depth 10 -Compress

    $response = Invoke-RestMethod -Method Post -Uri "$Sub2ApiUrl/v1/chat/completions" -Headers @{
      Authorization = "Bearer $Sub2ApiKey"
    } -ContentType "application/json" -Body $body -TimeoutSec 90

    return [PSCustomObject]@{
      ok = $true
      status = 200
      model = $response.model
      content = $response.choices[0].message.content
      id = $response.id
      skipped = $false
      error = ""
    }
  } catch {
    $message = if ($_.ErrorDetails.Message) { $_.ErrorDetails.Message } else { $_.Exception.Message }
    return [PSCustomObject]@{
      ok = $false
      status = 0
      model = $Model
      content = ""
      id = ""
      skipped = $false
      error = $message
    }
  }
}

$cpaResults = @()
if (-not $SkipDirectCpaChecks) {
  if (-not $Cpa1Key) { $Cpa1Key = Read-CpaApiKey -Path $Cpa1ConfigPath }
  if (-not $Cpa2Key) { $Cpa2Key = Read-CpaApiKey -Path $Cpa2ConfigPath }
  if (-not $Cpa3Key) { $Cpa3Key = Read-CpaApiKey -Path $Cpa3ConfigPath }
  $cpaResults = @(
    Test-CpaModels -Name "cpa1" -BaseUrl $Cpa1BaseUrl -ApiKey $Cpa1Key
    Test-CpaModels -Name "cpa2" -BaseUrl $Cpa2BaseUrl -ApiKey $Cpa2Key
    Test-CpaModels -Name "cpa3" -BaseUrl $Cpa3BaseUrl -ApiKey $Cpa3Key
  )
}

if (-not $Sub2ApiKey -and $env:SUB2API_API_KEY) {
  $Sub2ApiKey = $env:SUB2API_API_KEY
}

$health = Test-Sub2ApiHealth
$chat = Test-Sub2ApiChat

$allCpaReady = $true
if (-not $SkipDirectCpaChecks) {
  foreach ($result in $cpaResults) {
    if (-not $result.ok -or $result.model_count -le 0) {
      $allCpaReady = $false
    }
  }
}

$summary = [PSCustomObject]@{
  direct_cpa_checks_skipped = [bool]$SkipDirectCpaChecks
  all_cpa_ready = $allCpaReady
  sub2api_ready = ($health.ok -and $chat.ok)
  cpa = $cpaResults
  sub2api_health = $health
  sub2api_chat = $chat
}

$summary | ConvertTo-Json -Depth 20

if (-not $health.ok -or -not $chat.ok) {
  exit 1
}

if ($RequireAllCpaModels -and -not $allCpaReady) {
  exit 2
}
