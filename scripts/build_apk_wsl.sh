#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ANDROID_DIR="${ROOT_DIR}/android"
LOCAL_EBITEN_DIR="/home/jerem/work/rewind/CODEX_rewind/third_party/ebiten"
LOCAL_EBITENMOBILE_BIN="${ROOT_DIR}/.tools/ebitenmobile"
AAR_PATH="${ANDROID_DIR}/app/libs/PetriDish.aar"

source "/home/jerem/work/rewind/CODEX_rewind/scripts/lib_local_dev.sh"
rewindecho_setup_local_build_env "${ROOT_DIR}"
rewindecho_activate_go_toolchain "/home/jerem/work/rewind/CODEX_rewind"
rewindecho_activate_java_toolchain "/home/jerem/work/rewind/CODEX_rewind"

ANDROID_SDK_DIR="${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$(rewindecho_resolve_android_sdk /home/jerem/work/rewind/CODEX_rewind || true)}}"
ANDROID_NDK_DIR="${ANDROID_NDK_HOME:-${ANDROID_NDK_ROOT:-$(rewindecho_resolve_android_ndk /home/jerem/work/rewind/CODEX_rewind "${ANDROID_SDK_DIR}" || true)}}"

if [[ -z "${ANDROID_SDK_DIR}" || -z "${ANDROID_NDK_DIR}" ]]; then
  echo "ERROR: Android SDK/NDK not found." >&2
  exit 1
fi

export ANDROID_HOME="${ANDROID_SDK_DIR}"
export ANDROID_SDK_ROOT="${ANDROID_SDK_DIR}"
export ANDROID_NDK_HOME="${ANDROID_NDK_DIR}"
export ANDROID_NDK_ROOT="${ANDROID_NDK_DIR}"

mkdir -p "${ANDROID_DIR}/app/libs" "${ROOT_DIR}/.tools"
rm -f "${ANDROID_DIR}/app/libs/PetriDish.aar" "${ANDROID_DIR}/app/libs/PetriDish-sources.jar"

if [[ -f "${LOCAL_EBITEN_DIR}/go.mod" ]]; then
  echo "Building local ebitenmobile from ${LOCAL_EBITEN_DIR}..."
  (cd "${LOCAL_EBITEN_DIR}" && GOCACHE=/tmp/petri_dish-go-build go build -o "${LOCAL_EBITENMOBILE_BIN}" ./cmd/ebitenmobile)
  EBITENMOBILE_BIN="${LOCAL_EBITENMOBILE_BIN}"
elif command -v ebitenmobile >/dev/null 2>&1; then
  EBITENMOBILE_BIN="$(command -v ebitenmobile)"
else
  echo "ERROR: ebitenmobile not found." >&2
  exit 1
fi

BUILD_VERSION="$(cd "${ROOT_DIR}" && git describe --always --dirty 2>/dev/null || date +%Y%m%d-%H%M%S)"
echo "Build version: ${BUILD_VERSION}"

echo "Binding Go mobile library (.aar)..."
GOCACHE=/tmp/petri_dish-go-build GOMODCACHE="${GOMODCACHE:-/home/jerem/work/rewind/CLAUDE_rewind/.localdev/gopath/pkg/mod}" \
  "${EBITENMOBILE_BIN}" bind \
  -target android \
  -javapkg com.quartermeat.petridish \
  -ldflags "-X petri_dish/mobile.Version=${BUILD_VERSION}" \
  -o "${AAR_PATH}" \
  ./mobile

echo "Building debug APK..."
cd "${ANDROID_DIR}"
/bin/bash ./gradlew assembleDebug --no-daemon

APK_PATH="${ANDROID_DIR}/app/build/outputs/apk/debug/app-debug.apk"
if [[ ! -f "${APK_PATH}" ]]; then
  echo "APK not found at ${APK_PATH}" >&2
  exit 1
fi

echo "Built APK: ${APK_PATH}"
