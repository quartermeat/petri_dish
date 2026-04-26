#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${ROOT_DIR}"

APK_PATH="${ROOT_DIR}/android/app/build/outputs/apk/debug/app-debug.apk"
PACKAGE_NAME="com.hexglobe"
LAUNCH_ACTIVITY="com.hexglobe.MainActivity"

LOG_DIR="${ROOT_DIR}/logs"
mkdir -p "${LOG_DIR}"
LOG_FILE="${LOG_FILE:-${LOG_DIR}/build_install_$(date +%Y%m%d_%H%M%S).txt}"

exec > >(tee "${LOG_FILE}") 2>&1
echo "Logging to ${LOG_FILE}"
echo "Building, installing, and launching Helios..."

/bin/bash "${ROOT_DIR}/scripts/build_apk_wsl.sh"
APK_PATH="${APK_PATH}" /bin/bash "${ROOT_DIR}/scripts/install_apk_windows_from_wsl.sh" "${PACKAGE_NAME}" "${LAUNCH_ACTIVITY}"

echo "Completed. Log saved at ${LOG_FILE}"
