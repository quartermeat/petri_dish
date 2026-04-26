# Helios — Dev Guide

How to build, run, test, and ship Helios from WSL. See `docs/DESIGN.md` for architecture.

## Prereqs

| Thing | Version / Notes |
| --- | --- |
| Go | 1.24.5 (module declares this — matching toolchain is reused from `CODEX_rewind/scripts/lib_local_dev.sh`) |
| Ebitengine | v2.9.8, pulled from a **local fork** at `/home/jerem/work/rewind/CODEX_rewind/third_party/ebiten` via a `replace` directive in `go.mod` |
| Android SDK / NDK | Resolved automatically by the build scripts via `rewindecho_resolve_android_sdk` / `rewindecho_resolve_android_ndk`. Override with `ANDROID_HOME`, `ANDROID_SDK_ROOT`, `ANDROID_NDK_HOME`, `ANDROID_NDK_ROOT`. |
| Java | 17 (compileOptions). Activated via `rewindecho_activate_java_toolchain`. |
| `adb` | Windows-side `adb.exe` reached from WSL; resolver in `lib_local_dev.sh` picks it up. |

> **Cross-project dependency:** Helios currently leans on scripts and a vendored Ebiten fork inside the `rewind` project. If you move or delete `/home/jerem/work/rewind/CODEX_rewind`, the Android build and the desktop Ebiten linking both break. See *Decoupling* below.

## Desktop

```bash
cd /home/jerem/work/hex_globe/CLAUDE_hex_globe
go run .                                       # launches Ebiten window
go build -o bin/hex_globe .                    # optional: build binary
```

Entry point: `main.go` → `hexglobe.NewGame()`. The window is opened at 2x the logical size (`432x768` → `864x1536`).

### WSL caveat

Ebiten / GLFW crashes the WSLg display when run directly from WSL on many machines — the app boots, then the VM hangs. If that happens, cross-compile and run from Windows:

```bash
GOOS=windows GOARCH=amd64 go build -o bin/hex_globe.exe .
# then run bin/hex_globe.exe from PowerShell / Explorer
```

Tests that import Ebiten/GLFW should **not** run under native WSL Go for the same reason — build them as a Windows test binary (`go test -c`) when needed. The current tests in `core/` are pure Go and safe in WSL.

## Tests

```bash
go test ./...                                  # full suite
go test ./core/ -run TestNewGlobeProducesHexSphereTopology -v
```

`core/globe_test.go` asserts the Goldberg topology invariants: every cell has 5 or 6 corners, every cell has ≥5 neighbors, and exactly **12** pentagons exist at the icosahedral vertices. Add new topology invariants here when adding subdivision levels or alternate globe builders.

## Android APK

### One-shot build + install + launch

```bash
./scripts/build_and_install_from_wsl.sh
```

This calls, in order:
1. `scripts/build_apk_wsl.sh` — binds the Go mobile package into an AAR and runs `gradlew assembleDebug`.
2. `scripts/install_apk_windows_from_wsl.sh` — installs via `adb`, force-stops any previous instance, launches the activity, and tails `logcat` for ~6 s filtered by `Helios|AndroidRuntime|Go|ebiten|FATAL|panic`.

Logs are written to `logs/build_install_<timestamp>.txt`.

### Just build the APK

```bash
./scripts/build_apk_wsl.sh
# → android/app/build/outputs/apk/debug/app-debug.apk
```

What this does:
- Builds `ebitenmobile` from the local Ebiten fork (output: `.tools/ebitenmobile`).
- Runs `ebitenmobile bind -target android -javapkg com.hexglobe -o android/app/libs/Helios.aar ./mobile`.
- Runs `gradlew assembleDebug --no-daemon` against `android/`.

### Just install an already-built APK

```bash
./scripts/install_apk_windows_from_wsl.sh com.hexglobe com.hexglobe.MainActivity
# Env knobs:
#   APK_PATH          default android/app/build/outputs/apk/debug/app-debug.apk
#   ANDROID_SERIAL    target a specific device when multiple are attached
#   SHOW_LOGS_SECS    seconds of logcat to collect after launch (default 6)
#   LOG_FILTER_REGEX  grep filter for logcat (default includes Helios|ebiten|FATAL)
```

### Android app facts

- Package / applicationId: `com.hexglobe`
- minSdk 26, targetSdk 35, compileSdk 35
- Portrait-locked, immersive / fullscreen, GLES 2.0 required
- `MainActivity` hosts `com.hexglobe.mobile.EbitenView` and calls `Mobile.dummy()` to pull the Go bindings in; `Seq.setContext(...)` wires the Android context into gomobile.
- No release signing is configured — release builds would need a keystore.

## Go module layout

```
hex_globe/
├── core/      pure logic — geometry, math, rulesets (no Ebiten imports)
├── hexglobe/  Ebiten game loop, rendering, input
├── mobile/    gomobile entry (calls hexglobe.NewGame via ebiten/v2/mobile)
├── main.go    desktop entry (build tag !android)
├── android/   Gradle wrapper app; consumes Helios.aar
└── scripts/   build/install helpers
```

The `!android` build tag on `main.go` keeps the desktop entry point out of the mobile AAR. Keep it on any future desktop-only files.

## Common tasks

### Add a new ruleset

1. Implement `core.Ruleset` in `core/`.
2. Wire it in `hexglobe/game.go:NewGame` (currently hard-codes `core.NewDemoRuleset()`).
3. Restart desktop; rebuild AAR for Android.

### Change globe density

`core.NewGlobe(radius, subdivisions)` is called in `hexglobe/game.go:NewGame`. `subdivisions=3` → ~642 cells. Each extra level quadruples face count. **Before you bump it**: the renderer draws every front-facing cell with `DrawTriangles` + a per-edge `StrokeLine` — at subdiv 4 (~2562 cells) this is already noticeable on mid-range Androids.

### Change the icon / app label

- Label: `android/app/src/main/res/values/strings.xml` (`app_name`).
- Icon: no launcher icon is currently declared; add `android:icon` to `<application>` in `AndroidManifest.xml` and drop mipmaps into `android/app/src/main/res/mipmap-*` when ready.

## Troubleshooting

| Symptom | Cause / Fix |
| --- | --- |
| `ebitenmobile not found` | `/home/jerem/work/rewind/CODEX_rewind/third_party/ebiten/go.mod` missing. Either restore that checkout or `go install github.com/hajimehoshi/ebiten/v2/cmd/ebitenmobile@v2.9.8` (then the script falls through to `command -v ebitenmobile`). |
| `Android SDK/NDK not found` | Set `ANDROID_HOME` and `ANDROID_NDK_HOME` explicitly before running the script. |
| `adb` not runnable | The script auto-detects `adb.exe` via `wslpath`. If both WSL and Windows `adb` exist and conflict, set `ADB_EXE` to a specific path. |
| Device shown but install fails with "offline" | Unlock the phone, re-trust the host, then re-run — the script retries once after `wait-for-device`. |
| GLFW / OpenGL crash on `go run .` | Run on Windows via cross-compile (see *WSL caveat*). |
| AAR rebuilds every time, slowly | Normal — gomobile has no incremental cache. Consider running from the `go test` layer during iteration and only binding when you need to test on-device. |

## Decoupling from `rewind`

The shared `lib_local_dev.sh`, vendored Ebiten, and shared Go/Java/Android toolchain activation are pragmatic but brittle. When Helios grows past prototype, the clean-up worth doing:

- Vendor Ebiten into `third_party/ebiten` under this project (or drop the `replace` once upstream releases include the features this fork relies on).
- Copy the minimum needed helpers out of `lib_local_dev.sh` into `scripts/lib_local_dev.sh` here.
- Parameterise the Gradle + NDK paths so other contributors don't need the `rewind` checkout at all.
