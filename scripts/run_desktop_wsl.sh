#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

view_arg=()
if [[ "${1:-}" == "settings" ]]; then
  view_arg=(-view settings)
fi

XDG_RUNTIME_DIR=/mnt/wslg/runtime-dir \
LIBGL_ALWAYS_SOFTWARE=1 \
go run . "${view_arg[@]}"
