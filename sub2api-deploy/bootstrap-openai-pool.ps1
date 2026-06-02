param(
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$EnvFile = "sub2api-deploy/.env",
  [ValidateSet("local", "cloud")]
  [string]$UpstreamMode = "local",
  [string]$GroupName = "cpa-openai",
  [string]$ChannelName = "cpa-openai-channel",
  [string]$KeyName = "local-cpa-pool",
  [double]$UserBalance = 1000,
  [double]$KeyQuota = 100,
  [string]$Cpa1ConfigPath = "config.yaml",
  [string]$Cpa2ConfigPath = "instances/cpa2/config.yaml",
  [string]$Cpa3ConfigPath = "instances/cpa3/config.yaml",
  [string]$Cpa1Key = "",
  [string]$Cpa2Key = "",
  [string]$Cpa3Key = ""
)

$ErrorActionPreference = "Stop"

function Read-DotEnv {
  param([string]$Path)

  $values = @{}
  if (-not (Test-Path -LiteralPath $Path)) {
    return $values
  }

  foreach ($line in Get-Content -LiteralPath $Path) {
    $trimmed = $line.Trim()
    if ($trimmed.Length -eq 0 -or $trimmed.StartsWith("#")) {
      continue
    }

    $index = $trimmed.IndexOf("=")
    if ($index -lt 1) {
      continue
    }

    $key = $trimmed.Substring(0, $index).Trim()
    $value = $trimmed.Substring($index + 1).Trim()
    if (($value.StartsWith('"') -and $value.EndsWith('"')) -or ($value.StartsWith("'") -and $value.EndsWith("'"))) {
      $value = $value.Substring(1, $value.Length - 2)
    }
    $values[$key] = $value
  }

  return $values
}

function Invoke-Sub2Api {
  param(
    [ValidateSet("GET", "POST", "PUT", "DELETE")]
    [string]$Method,
    [string]$Path,
    [hashtable]$Headers,
    [object]$Body = $null
  )

  $uri = "$BaseUrl$Path"
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -TimeoutSec 60
  }

  $json = $Body | ConvertTo-Json -Depth 20 -Compress
  return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -ContentType "application/json" -Body $json -TimeoutSec 60
}

function Get-DataItems {
  param([object]$Response)

  if ($null -eq $Response.data) {
    return @()
  }
  if ($null -ne $Response.data.items) {
    return @($Response.data.items)
  }
  return @($Response.data)
}

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

$envValues = Read-DotEnv -Path $EnvFile

$adminEmail = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } elseif ($envValues.ContainsKey("ADMIN_EMAIL")) { $envValues["ADMIN_EMAIL"] } else { "admin@sub2api.local" }
$adminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } elseif ($envValues.ContainsKey("ADMIN_PASSWORD")) { $envValues["ADMIN_PASSWORD"] } else { "" }

if ([string]::IsNullOrWhiteSpace($adminPassword)) {
  throw "ADMIN_PASSWORD is required. Set it in the environment or in $EnvFile."
}

$login = Invoke-RestMethod -Method Post -Uri "$BaseUrl/api/v1/auth/login" -ContentType "application/json" -Body (@{
  email = $adminEmail
  password = $adminPassword
} | ConvertTo-Json -Compress) -TimeoutSec 60

$headers = @{ Authorization = "Bearer $($login.data.access_token)" }

$groups = Get-DataItems (Invoke-Sub2Api -Method GET -Path "/api/v1/admin/groups" -Headers $headers)
$group = $groups | Where-Object { $_.name -eq $GroupName } | Select-Object -First 1
if ($null -eq $group) {
  $group = (Invoke-Sub2Api -Method POST -Path "/api/v1/admin/groups" -Headers $headers -Body @{
    name = $GroupName
    description = "CPA OpenAI-compatible upstream group"
    platform = "openai"
    rate_multiplier = 1
    is_exclusive = $false
    status = "active"
    subscription_type = "standard"
    allow_image_generation = $true
  }).data
  Write-Host "Created group: $($group.name) (#$($group.id))"
} else {
  Write-Host "Using existing group: $($group.name) (#$($group.id))"
}

$groupId = [int]$group.id

$cpaKeys = @{
  cpa1 = if ($Cpa1Key) { $Cpa1Key } else { Read-CpaApiKey -Path $Cpa1ConfigPath }
  cpa2 = if ($Cpa2Key) { $Cpa2Key } else { Read-CpaApiKey -Path $Cpa2ConfigPath }
  cpa3 = if ($Cpa3Key) { $Cpa3Key } else { Read-CpaApiKey -Path $Cpa3ConfigPath }
}

if ($UpstreamMode -eq "cloud") {
  $cpaUrls = @{
    cpa1 = "http://cpa1:8317/v1"
    cpa2 = "http://cpa2:8317/v1"
    cpa3 = "http://cpa3:8317/v1"
  }
} else {
  $cpaUrls = @{
    cpa1 = "http://host.docker.internal:8317/v1"
    cpa2 = "http://host.docker.internal:8318/v1"
    cpa3 = "http://host.docker.internal:8319/v1"
  }
}

$accounts = Get-DataItems (Invoke-Sub2Api -Method GET -Path "/api/v1/admin/accounts" -Headers $headers)
foreach ($name in @("cpa1", "cpa2", "cpa3")) {
  $payload = @{
    name = $name
    notes = "$($name.ToUpperInvariant()) $UpstreamMode upstream"
    platform = "openai"
    type = "apikey"
    credentials = @{
      base_url = $cpaUrls[$name]
      api_key = $cpaKeys[$name]
    }
    extra = @{
      openai_responses_supported = $true
    }
    concurrency = 10
    priority = 1
    rate_multiplier = 1
    status = "active"
    group_ids = @($groupId)
  }

  $existing = $accounts | Where-Object { $_.name -eq $name } | Select-Object -First 1
  if ($null -eq $existing) {
    $created = (Invoke-Sub2Api -Method POST -Path "/api/v1/admin/accounts" -Headers $headers -Body $payload).data
    Write-Host "Created account: $name (#$($created.id)) -> $($cpaUrls[$name])"
  } else {
    $updated = (Invoke-Sub2Api -Method PUT -Path "/api/v1/admin/accounts/$($existing.id)" -Headers $headers -Body $payload).data
    Write-Host "Updated account: $name (#$($updated.id)) -> $($cpaUrls[$name])"
  }
}

$channels = Get-DataItems (Invoke-Sub2Api -Method GET -Path "/api/v1/admin/channels" -Headers $headers)
$channel = $channels | Where-Object { $_.name -eq $ChannelName } | Select-Object -First 1
$channelPayload = @{
  name = $ChannelName
  description = "Routes OpenAI-compatible requests to the CPA pool"
  status = "active"
  billing_model_source = "channel_mapped"
  restrict_models = $false
  features = ""
  features_config = @{
    codex_image_generation_bridge = @{
      openai = $true
    }
  }
  group_ids = @($groupId)
  model_pricing = @()
  model_mapping = @{}
  apply_pricing_to_account_stats = $false
  account_stats_pricing_rules = @()
}

if ($null -eq $channel) {
  $channel = (Invoke-Sub2Api -Method POST -Path "/api/v1/admin/channels" -Headers $headers -Body $channelPayload).data
  Write-Host "Created channel: $ChannelName (#$($channel.id))"
} else {
  $channel = (Invoke-Sub2Api -Method PUT -Path "/api/v1/admin/channels/$($channel.id)" -Headers $headers -Body $channelPayload).data
  Write-Host "Updated channel: $ChannelName (#$($channel.id))"
}

$user = (Invoke-Sub2Api -Method GET -Path "/api/v1/admin/users/1" -Headers $headers).data
if ([double]$user.balance -lt $UserBalance) {
  $user = (Invoke-Sub2Api -Method POST -Path "/api/v1/admin/users/1/balance" -Headers $headers -Body @{
    balance = $UserBalance
    operation = "set"
    notes = "CPA pool bootstrap"
  }).data
  Write-Host "Set admin user balance: $($user.balance)"
} else {
  Write-Host "Admin user balance already sufficient: $($user.balance)"
}

$keys = Get-DataItems (Invoke-Sub2Api -Method GET -Path "/api/v1/keys" -Headers $headers)
$key = $keys | Where-Object { $_.name -eq $KeyName -and $_.group_id -eq $groupId } | Select-Object -First 1
if ($null -eq $key) {
  $key = (Invoke-Sub2Api -Method POST -Path "/api/v1/keys" -Headers $headers -Body @{
    name = $KeyName
    group_id = $groupId
    quota = $KeyQuota
    status = "active"
  }).data
  Write-Host "Created Sub2API key: $($key.key)"
} else {
  Write-Host "Using existing Sub2API key: $($key.key)"
}

Write-Host ""
Write-Host "OpenAI-compatible export:"
Write-Host "  Base URL: $BaseUrl/v1"
Write-Host "  API key:  $($key.key)"
Write-Host ""
Write-Host "Smoke test:"
Write-Host "curl.exe --% -s -X POST $BaseUrl/v1/chat/completions -H `"Authorization: Bearer $($key.key)`" -H `"Content-Type: application/json`" -d `"{\`"model\`":\`"gpt-5.4-mini\`",\`"messages\`":[{\`"role\`":\`"user\`",\`"content\`":\`"Say hello in one short sentence.\`"}]}`""
