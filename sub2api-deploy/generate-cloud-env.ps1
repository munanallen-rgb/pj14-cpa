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
$portalSessionSecret = New-RandomHex -ByteCount 32
$portalAdminEmail = "portal-admin@sub2api.local"
$portalAdminPassword = New-RandomHex -ByteCount 32
$portalSub2APIAdminEmail = "portal-service@sub2api.local"
$portalSub2APIAdminPassword = New-RandomHex -ByteCount 32

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

PORTAL_BIND_HOST=0.0.0.0
PORTAL_PORT=18100
PORTAL_PUBLIC_SUB2API_BASE_URL=http://<server-ip>:$Port
PORTAL_SESSION_SECRET=$portalSessionSecret
PORTAL_SESSION_TTL_HOURS=24
PORTAL_COOKIE_SECURE=false
PORTAL_ALLOWED_ORIGINS=
PORTAL_BOOTSTRAP_ADMIN_EMAIL=$portalAdminEmail
PORTAL_BOOTSTRAP_ADMIN_PASSWORD=$portalAdminPassword
PORTAL_SUB2API_ADMIN_EMAIL=$portalSub2APIAdminEmail
PORTAL_SUB2API_ADMIN_PASSWORD=$portalSub2APIAdminPassword
PORTAL_SUB2API_DEFAULT_GROUP_NAME=cpa-openai
PORTAL_SUB2API_DEFAULT_KEY_QUOTA=0

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
Write-Host ""
Write-Host "Portal API:"
Write-Host "  url:      http://<server-ip>:18100"
Write-Host "  email:    $portalAdminEmail"
Write-Host "  password: $portalAdminPassword"
Write-Host "  sub2api service email:    $portalSub2APIAdminEmail"
Write-Host "  sub2api service password: $portalSub2APIAdminPassword"
