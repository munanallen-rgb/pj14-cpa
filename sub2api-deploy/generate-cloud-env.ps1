param(
  [string]$OutputPath = "temp/cpa-sub2api-cloud.env",
  [string]$AdminEmail = "admin@sub2api.local",
  [string]$BindHost = "0.0.0.0",
  [int]$Port = 18080,
  [switch]$Force
)

$ErrorActionPreference = "Stop"

$root = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")).Path
$targetPath = if ([System.IO.Path]::IsPathRooted($OutputPath)) {
  $OutputPath
} else {
  Join-Path $root $OutputPath
}
$targetFullPath = [System.IO.Path]::GetFullPath($targetPath)
$rootFullPath = [System.IO.Path]::GetFullPath($root)

if (-not $targetFullPath.StartsWith($rootFullPath, [System.StringComparison]::OrdinalIgnoreCase)) {
  throw "OutputPath must be inside the repository workspace: $rootFullPath"
}

if ((Test-Path -LiteralPath $targetFullPath) -and -not $Force) {
  throw "Refusing to overwrite existing env file: $targetFullPath. Use -Force to overwrite."
}

function New-RandomHex {
  param([int]$ByteCount = 32)

  $bytes = New-Object byte[] $ByteCount
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try {
    $rng.GetBytes($bytes)
  } finally {
    $rng.Dispose()
  }
  return -join ($bytes | ForEach-Object { $_.ToString("x2") })
}

$adminPassword = New-RandomHex -ByteCount 32
$postgresPassword = New-RandomHex -ByteCount 32
$jwtSecret = New-RandomHex -ByteCount 32
$totpKey = New-RandomHex -ByteCount 32

$content = @"
SUB2API_BIND_HOST=$BindHost
SUB2API_PORT=$Port
ADMIN_EMAIL=$AdminEmail
ADMIN_PASSWORD=$adminPassword
POSTGRES_USER=sub2api
POSTGRES_PASSWORD=$postgresPassword
POSTGRES_DB=sub2api
JWT_SECRET=$jwtSecret
TOTP_ENCRYPTION_KEY=$totpKey
TZ=Asia/Shanghai
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY=replace-with-cpa-management-key
CPA_QUOTA_COLLECTOR_INSTANCES=cpa1=http://cpa1:8317,cpa2=http://cpa2:8317,cpa3=http://cpa3:8317
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA1=
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA2=
CPA_QUOTA_COLLECTOR_MANAGEMENT_KEY_CPA3=

SECURITY_URL_ALLOWLIST_ENABLED=false
SECURITY_URL_ALLOWLIST_ALLOW_INSECURE_HTTP=true
SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS=true
"@

$parent = Split-Path -Parent $targetFullPath
New-Item -ItemType Directory -Force -Path $parent | Out-Null
Set-Content -LiteralPath $targetFullPath -Value $content -Encoding UTF8

Write-Host "Generated cloud env:"
Write-Host "  $targetFullPath"
Write-Host ""
Write-Host "Admin login:"
Write-Host "  email:    $AdminEmail"
Write-Host "  password: $adminPassword"
