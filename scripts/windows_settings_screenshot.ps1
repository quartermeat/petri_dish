Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

$timestamp = Get-Date -Format "yyyyMMdd_HHmmss"
$logDir = Join-Path $repoRoot "debug/log"
New-Item -ItemType Directory -Force -Path $logDir | Out-Null

$exePath = Join-Path $logDir "hexglobe_windows.exe"
$pngPath = Join-Path $logDir ("settings_{0}.png" -f $timestamp)

go build -o $exePath .
& $exePath -view settings -screenshot $pngPath

Write-Host ("Saved screenshot: {0}" -f $pngPath)
