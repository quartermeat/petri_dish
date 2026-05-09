#!/usr/bin/env python3
"""Build, install, launch, and log the Petri Dish Android debug APK."""

from __future__ import annotations

import subprocess
import sys
import argparse
from datetime import datetime
from pathlib import Path


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


class Tee:
    def __init__(self, path: Path) -> None:
        self.file = path.open("w", encoding="utf-8")

    def write(self, data: str) -> int:
        sys.__stdout__.write(data)
        return self.file.write(data)

    def flush(self) -> None:
        sys.__stdout__.flush()
        self.file.flush()

    def close(self) -> None:
        self.file.close()


def run_python(script: Path, *args: str) -> None:
    proc = subprocess.Popen(
        [sys.executable, str(script), *args],
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
    )
    assert proc.stdout is not None
    for line in proc.stdout:
        print(line, end="")
    return_code = proc.wait()
    if return_code != 0:
        raise subprocess.CalledProcessError(return_code, [sys.executable, str(script), *args])


def main() -> int:
    root = repo_root()
    parser = argparse.ArgumentParser(description="Build, install, launch, and log the Petri Dish Android debug APK.")
    parser.add_argument("--skip-build", action="store_true", help="Install the existing debug APK without rebuilding it.")
    parser.add_argument("--install-arg", action="append", default=[], help="Extra argument forwarded to install_apk.py. May be repeated.")
    args = parser.parse_args()

    log_dir = root / "logs"
    log_dir.mkdir(exist_ok=True)
    log_file = log_dir / f"build_install_{datetime.now():%Y%m%d_%H%M%S}.txt"

    tee = Tee(log_file)
    old_stdout = sys.stdout
    old_stderr = sys.stderr
    sys.stdout = tee
    sys.stderr = tee
    try:
        print(f"Logging to {log_file}")
        print("Building, installing, and launching Petri Dish...")
        if not args.skip_build:
            run_python(root / "scripts" / "build_apk.py")
        run_python(root / "scripts" / "install_apk.py", *args.install_arg)
        print(f"Completed. Log saved at {log_file}")
    except subprocess.CalledProcessError as exc:
        print(f"ERROR: command failed with exit code {exc.returncode}", file=sys.stderr)
        return exc.returncode
    finally:
        sys.stdout = old_stdout
        sys.stderr = old_stderr
        tee.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
