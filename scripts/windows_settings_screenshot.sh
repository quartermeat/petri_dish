#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

mkdir -p debug/log

timestamp="$(date +%Y%m%d_%H%M%S)"
exe_wsl="$(pwd)/debug/log/hexglobe_windows.exe"
png_wsl="$(pwd)/debug/log/settings_${timestamp}.png"

GOOS=windows GOARCH=amd64 go build -o "$exe_wsl" .

exe_win="$(wslpath -w "$exe_wsl")"
png_win="$(wslpath -w "$png_wsl")"

powershell.exe -NoProfile -Command "& '$exe_win' -view settings -screenshot '$png_win'"

printf 'Saved screenshot: %s\n' "$png_wsl"
