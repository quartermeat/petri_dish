# Hex Globe

Isolated Ebiten prototype workspace for an Android-first rotating hex-cell globe.

The prototype is structured as a small framework:

- `hexglobe`: globe topology, reusable cell model, ruleset interfaces, and the touch-first Ebiten game loop
- `mobile`: gomobile bridge for Android packaging
- `main.go`: desktop runner for local iteration

The default ruleset is a biome demo intended to match the blue/green hex-globe vibe from the reference clip while keeping the cell graph reusable for later gameplay systems.

Android scaffold:

- `android/`: minimal Android wrapper app that hosts the Ebiten-generated `EbitenView`
- `scripts/build_apk_wsl.sh`: binds `./mobile` into an AAR and builds a debug APK
- `scripts/build_and_install_from_wsl.sh`: one-shot WSL command to build, install, and launch on a connected phone
- `scripts/install_apk_windows_from_wsl.sh`: install helper reused by the one-shot script

The intended Java package for the generated mobile bindings is `com.hexglobe`.
