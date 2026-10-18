# scripts/check-versions.ps1
# Validates that the application version is synchronized across all configuration files.

$ErrorActionPreference = "Stop"

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir

# 1. internal/updater/updater.go
$updaterPath = Join-Path $repoRoot "internal/updater/updater.go"
$updaterContent = Get-Content $updaterPath -Raw
if ($updaterContent -match 'CurrentVersion\s*=\s*"([^"]+)"') {
    $verUpdater = $matches[1].TrimStart("v").Trim()
} else {
    Write-Error "Could not find CurrentVersion in $updaterPath"
    exit 1
}

# 2. frontend/package.json (or package.json)
$pkgPath = Join-Path $repoRoot "frontend/package.json"
if (-not (Test-Path $pkgPath)) {
    $pkgPath = Join-Path $repoRoot "package.json"
}
$pkgJson = Get-Content $pkgPath -Raw | ConvertFrom-Json
$verPkg = $pkgJson.version.TrimStart("v").Trim()

# 3. wails.json
$wailsPath = Join-Path $repoRoot "wails.json"
$wailsJson = Get-Content $wailsPath -Raw | ConvertFrom-Json
$verWails = ""
if ($wailsJson.version) {
    $verWails = $wailsJson.version
} elseif ($wailsJson.info -and $wailsJson.info.productVersion) {
    $verWails = $wailsJson.info.productVersion
}
$verWails = $verWails.TrimStart("v").Trim()

Write-Host "Checking application versions:"
Write-Host "  internal/updater/updater.go : $verUpdater"
Write-Host "  frontend/package.json       : $verPkg"
Write-Host "  wails.json                  : $verWails"

if ($verUpdater -ne $verPkg -or $verUpdater -ne $verWails) {
    Write-Error "Version mismatch! All versions must be synchronized across internal/updater/updater.go, package.json, and wails.json."
    exit 1
}

Write-Host "[OK] Versions are properly synchronized: $verUpdater"
