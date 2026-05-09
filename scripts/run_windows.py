#!/usr/bin/env python3
"""Build and launch the native Windows desktop runner."""

from __future__ import annotations

import argparse
import subprocess
import sys
from datetime import datetime
from pathlib import Path


def repo_root() -> Path:
    return Path(__file__).resolve().parents[1]


def git_version(root: Path) -> str:
    try:
        return subprocess.check_output(
            ["git", "describe", "--always", "--dirty"],
            cwd=root,
            text=True,
            stderr=subprocess.DEVNULL,
        ).strip()
    except (FileNotFoundError, subprocess.CalledProcessError):
        return datetime.now().strftime("%Y%m%d-%H%M%S")


def main() -> int:
    root = repo_root()
    parser = argparse.ArgumentParser(description="Build and launch Petri Dish on Windows.")
    parser.add_argument("--view", choices=["settings"], help="Optional startup view.")
    parser.add_argument("--screenshot", help="Save a screenshot and exit.")
    parser.add_argument("--no-build", action="store_true", help="Launch the existing debug executable.")
    args = parser.parse_args()

    exe = root / "debug" / "petridish_windows.exe"
    exe.parent.mkdir(exist_ok=True)

    if not args.no_build:
        version = git_version(root)
        subprocess.run(
            ["go", "build", "-ldflags", f"-X main.Version={version}", "-o", str(exe), "."],
            cwd=root,
            check=True,
        )

    cmd = [str(exe)]
    if args.view:
        cmd.extend(["-view", args.view])
    if args.screenshot:
        cmd.extend(["-screenshot", args.screenshot])

    subprocess.run(cmd, cwd=root, check=True)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except subprocess.CalledProcessError as exc:
        raise SystemExit(exc.returncode)
