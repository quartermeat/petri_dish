#!/usr/bin/env python3
"""Install and launch the debug APK using adb from Windows, WSL, or Linux."""

from __future__ import annotations

import argparse
import os
import platform
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path


DEFAULT_PACKAGE = "com.quartermeat.petridish"
DEFAULT_ACTIVITY = "com.quartermeat.petridish.MainActivity"
DEFAULT_LOG_FILTER = r"Petri Dish|AndroidRuntime|Go|ebiten|FATAL|panic"


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def is_windows() -> bool:
    return platform.system().lower().startswith("win")


def find_adb() -> str | None:
    if os.environ.get("ADB_EXE"):
        return os.environ["ADB_EXE"]

    for name in ("adb.exe", "adb"):
        found = shutil.which(name)
        if found:
            return found

    for env_name in ("ANDROID_HOME", "ANDROID_SDK_ROOT"):
        sdk = os.environ.get(env_name)
        if not sdk:
            continue
        candidate = Path(sdk) / "platform-tools" / ("adb.exe" if is_windows() else "adb")
        if candidate.exists():
            return str(candidate)
    return None


def maybe_windows_path(path: Path, adb: str) -> str:
    resolved = str(path.resolve())
    if not adb.lower().endswith(".exe") or is_windows():
        return resolved
    try:
        return subprocess.check_output(["wslpath", "-w", resolved], text=True).strip()
    except (FileNotFoundError, subprocess.CalledProcessError):
        return resolved


def run_capture(cmd: list[str], check: bool = False, timeout: float | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=check, timeout=timeout)


def adb_cmd(adb: str, serial: str | None, *args: str) -> list[str]:
    cmd = [adb]
    if serial:
        cmd.extend(["-s", serial])
    cmd.extend(args)
    return cmd


def adb_devices_with_retry(adb: str, wait_secs: int) -> str:
    deadline = time.time() + wait_secs
    last = ""
    while True:
        last = run_capture([adb, "devices"]).stdout
        if parse_devices(last):
            return last
        if time.time() >= deadline:
            return last
        time.sleep(1)


def parse_devices(output: str) -> list[str]:
    devices: list[str] = []
    for line in output.replace("\r", "").splitlines()[1:]:
        parts = line.split()
        if len(parts) >= 2 and parts[1] == "device":
            devices.append(parts[0])
    return devices


def select_device(devices: list[str]) -> str | None:
    if os.environ.get("ANDROID_SERIAL"):
        serial = os.environ["ANDROID_SERIAL"]
        print(f"Using device: {serial} (from ANDROID_SERIAL)")
        return serial
    if len(devices) <= 1:
        return None
    for serial in devices:
        if not serial.startswith("emulator-"):
            print(f"Multiple devices detected. Selecting physical device: {serial}")
            return serial
    print(f"Multiple devices detected. Selecting first: {devices[0]}")
    return devices[0]


def install_apk(adb: str, serial: str | None, apk_path: str, package_name: str, wait_secs: int) -> bool:
    print("Preparing app for install...")
    subprocess.run(adb_cmd(adb, serial, "logcat", "-c"), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    subprocess.run(adb_cmd(adb, serial, "shell", "am", "force-stop", package_name), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    print(f"Installing {apk_path}...")
    out = run_capture(adb_cmd(adb, serial, "install", "-r", apk_path)).stdout
    if re.search(r"device offline|device still authorizing|unauthorized", out, re.I):
        print("adb reported device offline; retrying once after wait...")
        subprocess.run(adb_cmd(adb, serial, "wait-for-device"), timeout=wait_secs, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        out = run_capture(adb_cmd(adb, serial, "install", "-r", apk_path)).stdout

    if re.search(r"INSTALL_FAILED_UPDATE_INCOMPATIBLE|INSTALL_FAILED_VERSION_DOWNGRADE", out, re.I):
        print(f"Existing {package_name} install is incompatible; uninstalling and retrying...")
        subprocess.run(adb_cmd(adb, serial, "uninstall", package_name), stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        out = run_capture(adb_cmd(adb, serial, "install", "-r", apk_path)).stdout

    print(out.rstrip())
    return "success" in out.lower()


def show_logs(adb: str, serial: str | None, seconds: int, pattern: str) -> None:
    print(f"Collecting first {seconds}s of app logs...")
    regex = re.compile(pattern, re.I)
    proc = subprocess.Popen(
        adb_cmd(adb, serial, "logcat"),
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    assert proc.stdout is not None
    deadline = time.time() + seconds
    try:
        while time.time() < deadline:
            line = proc.stdout.readline()
            if not line:
                break
            if regex.search(line):
                print(line.rstrip())
    finally:
        proc.terminate()
        try:
            proc.wait(timeout=2)
        except subprocess.TimeoutExpired:
            proc.kill()


def main() -> int:
    root = repo_root()
    parser = argparse.ArgumentParser(description="Install and launch the Petri Dish debug APK.")
    parser.add_argument("--apk", default=str(root / "android/app/build/outputs/apk/debug/app-debug.apk"))
    parser.add_argument("--package", default=DEFAULT_PACKAGE)
    parser.add_argument("--activity", default=DEFAULT_ACTIVITY)
    parser.add_argument("--show-logs-secs", type=int, default=int(os.environ.get("SHOW_LOGS_SECS", "6")))
    parser.add_argument("--adb-wait-secs", type=int, default=int(os.environ.get("ADB_WAIT_SECS", "20")))
    parser.add_argument("--log-filter-regex", default=os.environ.get("LOG_FILTER_REGEX", DEFAULT_LOG_FILTER))
    args = parser.parse_args()

    adb = find_adb()
    if not adb:
        print("ERROR: adb not found. Set ADB_EXE or ANDROID_HOME/ANDROID_SDK_ROOT.", file=sys.stderr)
        return 1

    apk = Path(args.apk)
    if not apk.exists():
        print(f"ERROR: APK not found at {apk}", file=sys.stderr)
        return 1

    devices_out = adb_devices_with_retry(adb, args.adb_wait_secs)
    print(devices_out.rstrip())
    devices = parse_devices(devices_out)
    if not devices:
        print(f"ERROR: No running Android emulator/device found by {adb}", file=sys.stderr)
        return 1

    serial = select_device(devices)
    install_path = maybe_windows_path(apk, adb)
    if not install_apk(adb, serial, install_path, args.package, args.adb_wait_secs):
        print("ERROR: APK install failed.", file=sys.stderr)
        return 1

    subprocess.run(adb_cmd(adb, serial, "shell", "am", "start", "-n", f"{args.package}/{args.activity}"), check=True)
    if args.show_logs_secs > 0:
        show_logs(adb, serial, args.show_logs_secs, args.log_filter_regex)
    print("Done.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
