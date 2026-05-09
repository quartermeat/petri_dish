#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p debug/log

timestamp="$(date +%Y%m%d_%H%M%S)"
exe_wsl="$(pwd)/debug/log/petridish_windows.exe"
png_wsl="$(pwd)/debug/log/settings_${timestamp}.png"

GOOS=windows GOARCH=amd64 go build -o "$exe_wsl" .

exe_win="$(wslpath -w "$exe_wsl")"
png_win="$(wslpath -w "$png_wsl")"
ps1_win="$(wslpath -w "$(pwd)/scripts/windows_settings_screenshot.ps1")"

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$ps1_win" -ExePath "$exe_win" -PngPath "$png_win"

mapfile -t screenshots < <(ls -1t debug/log/settings_*.png 2>/dev/null || true)
if (( ${#screenshots[@]} > 5 )); then
    for old in "${screenshots[@]:5}"; do
        rm -f "$old"
    done
fi

printf 'Saved screenshot: %s\n' "$png_wsl"
