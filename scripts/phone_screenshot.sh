#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${OUT_DIR:-${ROOT_DIR}/debug/log}"
KEEP_COUNT="${KEEP_COUNT:-5}"

source "/home/jerem/work/rewind/CODEX_rewind/scripts/lib_local_dev.sh"

ADB_EXE="$(rewindecho_resolve_adb_exe "/home/jerem/work/rewind/CODEX_rewind" || true)"
if [[ -z "${ADB_EXE}" ]]; then
  echo "ERROR: adb not found." >&2
  exit 1
fi

RUNNABLE_ADB="$(rewindecho_resolve_runnable_adb "/home/jerem/work/rewind/CODEX_rewind" "${ADB_EXE}" || true)"
if [[ -n "${RUNNABLE_ADB}" ]] && [[ "${RUNNABLE_ADB}" != "${ADB_EXE}" ]]; then
  ADB_EXE="${RUNNABLE_ADB}"
elif [[ -z "${RUNNABLE_ADB}" ]]; then
  echo "ERROR: adb is not runnable at ${ADB_EXE}" >&2
  echo "Run this from your interactive WSL shell, or point ADB_EXE at a runnable adb binary." >&2
  exit 1
fi

mkdir -p "${OUT_DIR}"

DEVICES_OUT="$(rewindecho_adb_devices_with_retry "${ADB_EXE}" 20 || true)"
echo "${DEVICES_OUT}"
if ! rewindecho_adb_has_device "${DEVICES_OUT}"; then
  echo "ERROR: No running Android emulator/device found by ${ADB_EXE}" >&2
  exit 1
fi

ADB_SERIAL_FLAG=()
if [[ -n "${ANDROID_SERIAL:-}" ]]; then
  ADB_SERIAL_FLAG=(-s "${ANDROID_SERIAL}")
else
  DEVICE_COUNT=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" {count++} END{print count+0}')
  if [[ "${DEVICE_COUNT}" -gt 1 ]]; then
    PHYSICAL=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" && $1 !~ /^emulator-/ {print $1; exit}')
    if [[ -n "${PHYSICAL}" ]]; then
      ADB_SERIAL_FLAG=(-s "${PHYSICAL}")
    else
      FIRST=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" {print $1; exit}')
      ADB_SERIAL_FLAG=(-s "${FIRST}")
    fi
  fi
fi

timestamp="$(date +%Y%m%d_%H%M%S)"
png_wsl="${OUT_DIR}/phone_${timestamp}.png"

"${ADB_EXE}" "${ADB_SERIAL_FLAG[@]}" exec-out screencap -p > "${png_wsl}"

mapfile -t screenshots < <(ls -1t "${OUT_DIR}"/phone_*.png 2>/dev/null || true)
if (( ${#screenshots[@]} > KEEP_COUNT )); then
  for old in "${screenshots[@]:KEEP_COUNT}"; do
    rm -f "${old}"
  done
fi

printf 'Saved screenshot: %s\n' "${png_wsl}"
