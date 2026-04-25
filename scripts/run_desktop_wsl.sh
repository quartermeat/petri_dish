#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

view_arg=()
if [[ "${1:-}" == "settings" ]]; then
  view_arg=(-view settings)
fi

BUILD_VERSION="$(git describe --always --dirty 2>/dev/null || date +%Y%m%d-%H%M%S)"

XDG_RUNTIME_DIR=/mnt/wslg/runtime-dir \
LIBGL_ALWAYS_SOFTWARE=1 \
go run -ldflags "-X main.Version=${BUILD_VERSION}" . "${view_arg[@]}"
