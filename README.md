# Helios

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

## Current Testing Cheat Sheet

This section is the current MVP testing reference. Update it as interaction or recipes change.

### Strategic View

- Drag horizontally to wrap around the globe.
- Drag vertically to pan north/south within the allowed camera bounds.
- Pinch or mouse wheel to zoom.
- Tap a globe cell to select it.
- `ENTER HEX` opens the tactical map for the selected strategic cell.
- `GEAR` opens the settings view.
- `REGENERATE MAP` rebuilds the world and clears tactical-region state from the previous world.

### Tactical View

- Drag to pan the local map.
- Pinch or mouse wheel to zoom the local map.
- Tap a tile to select it.
- Tap `BUILD` to open the device builder for the selected tile.
- Tap a built miner tile to crank it if it contains a hand crank.

### Tactical Resource Glyphs

- No glyph: `stone` or no remaining deposit.
- Square glyph: `iron ore`
- Round glyph: `copper ore`
- Capsule glyph: `coal`
- Diamond glyph: `crystal`

When a deposit is exhausted, its resource glyph disappears.

### Miner State

Built miner tint shows remaining deposit on that tile:

- Green: healthy deposit remaining
- Yellow: running low
- Red: depleted or effectively empty

The miner only produces while it has power in its local buffer.

### Current Miner Recipe

The active MVP miner pattern is:

```text
. . . . .
. . M . .
. F D F .
. . O . .
. . H . .
```

Legend:

- `M` = `MOTOR`
- `D` = `DRILL`
- `F` = `FRAME`
- `O` = `OUTPUT`
- `H` = `CRANK`

The pattern can appear anywhere on the `5x5` build grid.

### Current Build / Run Loop

1. Select a tactical tile with a visible deposit glyph.
2. Open `BUILD`.
3. Place the miner pattern.
4. Press `CREATE` if you have enough inventory.
5. Return to tactical view.
6. Tap the built miner to crank it.
7. Watch the miner tint and the inventory panel as the deposit depletes.

### Current Starter Inventory

The current bootstrap inventory is intentionally small:

- `stone`
- `iron ore`
- `copper ore`

It is only meant to be enough to reach one self-bootstrapping miner.
