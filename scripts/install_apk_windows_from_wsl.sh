#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APK_PATH="${APK_PATH:-${ROOT_DIR}/android/app/build/outputs/apk/debug/app-debug.apk}"
PACKAGE_NAME="${1:-com.hexglobe}"
LAUNCH_ACTIVITY="${2:-com.hexglobe.MainActivity}"
SHOW_LOGS_SECS="${SHOW_LOGS_SECS:-6}"
ADB_WAIT_SECS="${ADB_WAIT_SECS:-20}"
LOG_FILTER_REGEX="${LOG_FILTER_REGEX:-HexGlobe|AndroidRuntime|Go|ebiten|FATAL|panic}"

source "/home/jerem/work/rewind/CODEX_rewind/scripts/lib_local_dev.sh"

ADB_EXE="$(rewindecho_resolve_adb_exe "/home/jerem/work/rewind/CODEX_rewind" || true)"

if [[ -z "${ADB_EXE}" ]]; then
  echo "ERROR: adb not found." >&2
  exit 1
fi

RUNNABLE_ADB="$(rewindecho_resolve_runnable_adb "/home/jerem/work/rewind/CODEX_rewind" "${ADB_EXE}" || true)"
if [[ -n "${RUNNABLE_ADB}" ]] && [[ "${RUNNABLE_ADB}" != "${ADB_EXE}" ]]; then
  echo "Resolved adb '${ADB_EXE}' is not runnable here; falling back to '${RUNNABLE_ADB}'." >&2
  ADB_EXE="${RUNNABLE_ADB}"
elif [[ -z "${RUNNABLE_ADB}" ]]; then
  echo "ERROR: adb is not runnable at ${ADB_EXE}" >&2
  echo "Run this from your interactive WSL shell, or point ADB_EXE at a runnable adb binary." >&2
  exit 1
fi

if [[ ! -f "${APK_PATH}" ]]; then
  echo "ERROR: APK not found at ${APK_PATH}" >&2
  exit 1
fi

INSTALL_APK_PATH="${APK_PATH}"
if [[ "${ADB_EXE,,}" == *.exe ]] && command -v wslpath >/dev/null 2>&1; then
  INSTALL_APK_PATH="$(wslpath -w "${APK_PATH}")"
fi

DEVICES_OUT="$(rewindecho_adb_devices_with_retry "${ADB_EXE}" "${ADB_WAIT_SECS}" || true)"
echo "${DEVICES_OUT}"
if ! rewindecho_adb_has_device "${DEVICES_OUT}"; then
  echo "ERROR: No running Android emulator/device found by ${ADB_EXE}" >&2
  exit 1
fi

ADB_SERIAL_FLAG=()
if [[ -n "${ANDROID_SERIAL:-}" ]]; then
  ADB_SERIAL_FLAG=(-s "${ANDROID_SERIAL}")
  echo "Using device: ${ANDROID_SERIAL} (from ANDROID_SERIAL)"
else
  DEVICE_COUNT=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" {count++} END{print count+0}')
  if [[ "${DEVICE_COUNT}" -gt 1 ]]; then
    PHYSICAL=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" && $1 !~ /^emulator-/ {print $1; exit}')
    if [[ -n "${PHYSICAL}" ]]; then
      ADB_SERIAL_FLAG=(-s "${PHYSICAL}")
      echo "Multiple devices detected. Selecting physical device: ${PHYSICAL}"
    else
      FIRST=$(printf '%s\n' "${DEVICES_OUT}" | tr -d '\r' | awk 'NR>1 && $2=="device" {print $1; exit}')
      ADB_SERIAL_FLAG=(-s "${FIRST}")
      echo "Multiple devices detected. Selecting first: ${FIRST}"
    fi
  fi
fi

adb_cmd() { "${ADB_EXE}" "${ADB_SERIAL_FLAG[@]}" "$@"; }

echo "Preparing app for install..."
adb_cmd logcat -c || true
adb_cmd shell am force-stop "${PACKAGE_NAME}" || true

echo "Installing ${INSTALL_APK_PATH}..."
INSTALL_OUT="$(adb_cmd install -r "${INSTALL_APK_PATH}" 2>&1)" || true
if printf '%s\n' "${INSTALL_OUT}" | grep -Eqi "device offline|device still authorizing|unauthorized"; then
  echo "adb reported device offline; retrying once after wait..."
  timeout "${ADB_WAIT_SECS}" "${ADB_EXE}" "${ADB_SERIAL_FLAG[@]}" wait-for-device >/dev/null 2>&1 || true
  INSTALL_OUT="$(adb_cmd install -r "${INSTALL_APK_PATH}" 2>&1)" || true
fi
printf '%s\n' "${INSTALL_OUT}"
if ! printf '%s\n' "${INSTALL_OUT}" | grep -qi "Success"; then
  echo "ERROR: APK install failed." >&2
  exit 1
fi

adb_cmd shell am start -n "${PACKAGE_NAME}/${LAUNCH_ACTIVITY}"

echo "Collecting first ${SHOW_LOGS_SECS}s of app logs..."
timeout "${SHOW_LOGS_SECS}" "${ADB_EXE}" "${ADB_SERIAL_FLAG[@]}" logcat | grep -Ei "${LOG_FILTER_REGEX}" || true

echo "Done."
