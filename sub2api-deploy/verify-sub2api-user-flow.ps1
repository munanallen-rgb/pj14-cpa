param(
  [string]$BaseUrl = "http://127.0.0.1:18080",
  [string]$EnvFile = "sub2api-deploy/.env",
  [string]$GroupName = "cpa-openai",
  [string]$Model = "gpt-5.4-mini",
  [string]$UserEmail = "",
  [string]$UserPassword = "UpgradeSmoke-Local-Only-2026!",
  [double]$UserBalance = 5,
  [double]$KeyQuota = 1
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

function Invoke-Sub2API {
  param(
    [ValidateSet("GET", "POST", "PUT", "DELETE")]
    [string]$Method,
    [string]$Path,
    [hashtable]$Headers = @{},
    [object]$Body = $null,
    [int]$TimeoutSec = 90
  )

  $uri = "$BaseUrl$Path"
  if ($null -eq $Body) {
    return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -TimeoutSec $TimeoutSec
  }

  $json = $Body | ConvertTo-Json -Depth 20 -Compress
  return Invoke-RestMethod -Method $Method -Uri $uri -Headers $Headers -ContentType "application/json" -Body $json -TimeoutSec $TimeoutSec
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

function Login {
  param(
    [string]$Email,
    [string]$Password
  )

  $response = Invoke-Sub2API -Method POST -Path "/api/v1/auth/login" -Body @{
    email = $Email
    password = $Password
  }
  return $response.data.access_token
}

if ([string]::IsNullOrWhiteSpace($UserEmail)) {
  $UserEmail = "upgrade-smoke-$(Get-Date -Format 'yyyyMMddHHmmss')@local.test"
}

$envValues = Read-DotEnv -Path $EnvFile
$adminEmail = if ($env:ADMIN_EMAIL) { $env:ADMIN_EMAIL } elseif ($envValues.ContainsKey("ADMIN_EMAIL")) { $envValues["ADMIN_EMAIL"] } else { "admin@sub2api.local" }
$adminPassword = if ($env:ADMIN_PASSWORD) { $env:ADMIN_PASSWORD } elseif ($envValues.ContainsKey("ADMIN_PASSWORD")) { $envValues["ADMIN_PASSWORD"] } else { "" }

if ([string]::IsNullOrWhiteSpace($adminPassword)) {
  throw "ADMIN_PASSWORD is required through environment or $EnvFile."
}

$adminToken = Login -Email $adminEmail -Password $adminPassword
$adminHeaders = @{ Authorization = "Bearer $adminToken" }

$groups = Get-DataItems (Invoke-Sub2API -Method GET -Path "/api/v1/admin/groups" -Headers $adminHeaders)
$group = $groups | Where-Object { $_.name -eq $GroupName } | Select-Object -First 1
if ($null -eq $group) {
  throw "Sub2API group '$GroupName' was not found. Run bootstrap-openai-pool.ps1 first."
}

$createdUser = (Invoke-Sub2API -Method POST -Path "/api/v1/admin/users" -Headers $adminHeaders -Body @{
  email = $UserEmail
  password = $UserPassword
  groups = @([int]$group.id)
  status = "active"
  balance = $UserBalance
  rate_limit = 0
}).data

$userToken = Login -Email $UserEmail -Password $UserPassword
$userHeaders = @{ Authorization = "Bearer $userToken" }

$key = (Invoke-Sub2API -Method POST -Path "/api/v1/keys" -Headers $userHeaders -Body @{
  name = "upgrade-smoke"
  group_id = [int]$group.id
  quota = $KeyQuota
  status = "active"
}).data

$adminDenied = $false
try {
  Invoke-Sub2API -Method GET -Path "/api/v1/admin/users" -Headers $userHeaders | Out-Null
} catch {
  $statusCode = [int]$_.Exception.Response.StatusCode
  $adminDenied = $statusCode -eq 401 -or $statusCode -eq 403
}

$chatOK = $false
$chatError = ""
try {
  $chat = Invoke-Sub2API -Method POST -Path "/v1/chat/completions" -Headers @{ Authorization = "Bearer $($key.key)" } -Body @{
    model = $Model
    messages = @(
      @{
        role = "user"
        content = "Say hello in one short sentence."
      }
    )
  } -TimeoutSec 120
  $chatOK = $null -ne $chat.id
} catch {
  $chatError = if ($_.ErrorDetails.Message) { $_.ErrorDetails.Message } else { $_.Exception.Message }
}

$summary = [PSCustomObject]@{
  ok = $adminDenied -and $chatOK
  base_url = $BaseUrl
  user_email = $UserEmail
  user_id = $createdUser.id
  group = $GroupName
  model = $Model
  api_key_created = -not [string]::IsNullOrWhiteSpace($key.key)
  admin_api_denied_for_user = $adminDenied
  chat_ok = $chatOK
  chat_error = $chatError
}

$summary | ConvertTo-Json -Depth 10

if (-not $summary.ok) {
  exit 1
}
