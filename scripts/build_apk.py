#!/usr/bin/env python3
"""Build the Android debug APK from Windows or WSL.

The current Android toolchain for this workspace lives in WSL, so this script
uses Python as the Windows-friendly entrypoint and runs the build payload inside
WSL when launched from Windows.
"""

from __future__ import annotations

import os
import platform
import shlex
import subprocess
import sys
from pathlib import Path
from pathlib import PureWindowsPath


REWIND_ROOT = os.environ.get("REWIND_ROOT", "/home/jerem/work/rewind/CODEX_rewind")
EBITEN_SOURCE_ROOT = os.environ.get("EBITEN_SOURCE_ROOT", f"{REWIND_ROOT}/third_party/ebiten")
DEFAULT_GOMODCACHE = os.environ.get(
    "GOMODCACHE", "/home/jerem/work/rewind/CLAUDE_rewind/.localdev/gopath/pkg/mod"
)


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def is_windows() -> bool:
    return platform.system().lower().startswith("win")


def wsl_path(path: Path) -> str:
    if not is_windows():
        return str(path)
    win_path = PureWindowsPath(path)
    drive = win_path.drive.rstrip(":").lower()
    if drive:
        parts = [part for part in win_path.parts[1:] if part not in ("\\", "/")]
        return "/mnt/" + drive + "/" + "/".join(parts)
    return subprocess.check_output(["wsl.exe", "wslpath", "-a", str(path)], text=True).strip()


def run_bash(script: str, root: Path, root_wsl: str) -> None:
    tools_dir = root / ".tools"
    tools_dir.mkdir(exist_ok=True)
    payload = tools_dir / "build_apk_payload.sh"
    payload.write_text(script, encoding="utf-8", newline="\n")
    if is_windows():
        cmd = ["wsl.exe", "bash", f"{root_wsl}/.tools/build_apk_payload.sh"]
    else:
        cmd = ["bash", str(payload)]
    subprocess.run(cmd, check=True)


def main() -> int:
    root = repo_root()
    root_wsl = wsl_path(root)
    android_dir = f"{root_wsl}/android"
    aar_path = f"{android_dir}/app/libs/PetriDish.aar"
    ebitenmobile_bin = f"{root_wsl}/.tools/ebitenmobile"

    script = f"""
set -euo pipefail

ROOT_DIR={shlex.quote(root_wsl)}
ANDROID_DIR={shlex.quote(android_dir)}
LOCAL_EBITEN_DIR={shlex.quote(EBITEN_SOURCE_ROOT)}
LOCAL_EBITENMOBILE_BIN={shlex.quote(ebitenmobile_bin)}
AAR_PATH={shlex.quote(aar_path)}
REWIND_ROOT={shlex.quote(REWIND_ROOT)}

source "${{REWIND_ROOT}}/scripts/lib_local_dev.sh"
rewindecho_setup_local_build_env "${{ROOT_DIR}}"
rewindecho_activate_go_toolchain "${{REWIND_ROOT}}"
rewindecho_activate_java_toolchain "${{REWIND_ROOT}}"

ANDROID_SDK_DIR="${{ANDROID_HOME:-${{ANDROID_SDK_ROOT:-$(rewindecho_resolve_android_sdk "${{REWIND_ROOT}}" || true)}}}}"
ANDROID_NDK_DIR="${{ANDROID_NDK_HOME:-${{ANDROID_NDK_ROOT:-$(rewindecho_resolve_android_ndk "${{REWIND_ROOT}}" "${{ANDROID_SDK_DIR}}" || true)}}}}"

if [[ -z "${{ANDROID_SDK_DIR}}" || -z "${{ANDROID_NDK_DIR}}" ]]; then
  echo "ERROR: Android SDK/NDK not found." >&2
  exit 1
fi

export ANDROID_HOME="${{ANDROID_SDK_DIR}}"
export ANDROID_SDK_ROOT="${{ANDROID_SDK_DIR}}"
export ANDROID_NDK_HOME="${{ANDROID_NDK_DIR}}"
export ANDROID_NDK_ROOT="${{ANDROID_NDK_DIR}}"

mkdir -p "${{ANDROID_DIR}}/app/libs" "${{ROOT_DIR}}/.tools"
rm -f "${{ANDROID_DIR}}/app/libs/PetriDish.aar" "${{ANDROID_DIR}}/app/libs/PetriDish-sources.jar"

if [[ -f "${{LOCAL_EBITEN_DIR}}/go.mod" ]]; then
  echo "Building local ebitenmobile from ${{LOCAL_EBITEN_DIR}}..."
  (cd "${{LOCAL_EBITEN_DIR}}" && GOCACHE=/tmp/petri_dish-go-build go build -o "${{LOCAL_EBITENMOBILE_BIN}}" ./cmd/ebitenmobile)
  EBITENMOBILE_BIN="${{LOCAL_EBITENMOBILE_BIN}}"
elif command -v ebitenmobile >/dev/null 2>&1; then
  EBITENMOBILE_BIN="$(command -v ebitenmobile)"
else
  echo "ERROR: ebitenmobile not found." >&2
  exit 1
fi

BUILD_VERSION="$(cd "${{ROOT_DIR}}" && git describe --always --dirty 2>/dev/null || date +%Y%m%d-%H%M%S)"
echo "Build version: ${{BUILD_VERSION}}"

echo "Binding Go mobile library (.aar)..."
GOCACHE=/tmp/petri_dish-go-build GOMODCACHE="${{GOMODCACHE:-{shlex.quote(DEFAULT_GOMODCACHE)}}}" \\
  "${{EBITENMOBILE_BIN}}" bind \\
  -target android \\
  -javapkg com.quartermeat.petridish \\
  -ldflags "-X petri_dish/mobile.Version=${{BUILD_VERSION}}" \\
  -o "${{AAR_PATH}}" \\
  ./mobile

echo "Building debug APK..."
cd "${{ANDROID_DIR}}"
/bin/bash ./gradlew assembleDebug --no-daemon

APK_PATH="${{ANDROID_DIR}}/app/build/outputs/apk/debug/app-debug.apk"
if [[ ! -f "${{APK_PATH}}" ]]; then
  echo "APK not found at ${{APK_PATH}}" >&2
  exit 1
fi

echo "Built APK: ${{APK_PATH}}"
"""

    try:
        run_bash(script, root, root_wsl)
    except FileNotFoundError as exc:
        print(f"ERROR: required command not found: {exc.filename}", file=sys.stderr)
        return 1
    except subprocess.CalledProcessError as exc:
        return exc.returncode
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
