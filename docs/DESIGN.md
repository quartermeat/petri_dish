# Petri Dish — Design

What the code currently does, how it's put together, and the current product direction.

## North star

Petri Dish is a phone-first, Mewgenics-inspired tactics game prototype. The project should preserve two core layers:

- **Strategic view.** The rotating globe remains the higher-level map, campaign surface, and place for broad-world decisions.
- **Tactical view.** The local hex/tile view remains the lower-level play space and should evolve into a turn-based tactical battlefield.

For the current prototype, a tactical tile can zoom one level deeper into a living cell grid. Working name: **Petri dish view**; alternate name to consider later: **Culture view**. This view is a small dish of cells that runs mostly automated. The player does not directly command every cell. Instead, they poke, prod, seed, stress, feed, isolate, or otherwise perturb the dish to influence its evolution.

The dish should become a bridge between experimentation and tactics. Successful cultures produce new traits, mutations, or perks that can later be applied to units used in the tactical battlefield. The design goal is for the player to feel like they are cultivating strange biological possibilities, then weaponizing or field-testing them in tactical encounters.

The long-term shape is not a factory game or a pure globe toy. The old resource/mining/build loop has been removed from the active prototype, and future design choices should bend toward readable mobile tactics: clear unit state, sharp turn structure, compact battlefields, fast touch interactions, and decisions that feel good in short phone sessions.

Mewgenics is the reference point for tone and structure: strange little organisms, authored-feeling tactical situations, inherited/quirky traits, and battles where positioning and emergent unit behavior matter. Petri Dish should find its own cellular/biome identity rather than copying the theme directly.

## Current implementation

## What it is

A rotating Goldberg-polyhedron globe rendered with Ebitengine. Topology is built once from an icosphere dual. A `Ruleset` drives per-cell state and per-frame styling. The demo ruleset paints a blue/green biome globe and auto-rotates it; the user can drag to tumble it.

## Package layout

```
core/        geometry, math, ruleset interface + demo
petridish/    Ebiten Game — loop, projection, input, draw
mobile/      gomobile bridge (mobile.SetGame)
android/     Gradle app consuming the generated AAR
main.go      !android desktop entry
scripts/     WSL build / install helpers
```

`core/` has no Ebiten imports. `petridish/` is the only package that touches pixels.

## Geometry

`core.NewGlobe(radius, subdivisions)` returns a **Goldberg polyhedron** — the dual of a subdivided icosahedron. Construction in `core/globe.go`:

1. Seed 12 icosahedron vertices from the golden ratio; 20 triangular faces.
2. Subdivide each triangle into 4 smaller triangles per level, projecting midpoints back onto the unit sphere. `midpointIndex` caches shared edges.
3. Build the dual: each original vertex becomes a **cell**, each triangle it belongs to contributes one **corner** (the face centroid). Corners are sorted counter-clockwise around the cell's local up vector via a tangent-frame `atan2`.
4. Tag pentagons. The 12 original icosahedral vertices stay as 5-sided cells; everything else is a 6-sided hexagon. Exactly 12 pentagons at every subdivision level — enforced by `TestNewGlobeProducesHexSphereTopology`.

| Subdivisions | Cells |
| --- | --- |
| 0 | 12 |
| 1 | 42 |
| 2 | 162 |
| 3 | 642 (current default in `petridish.NewGame`) |
| 4 | 2562 |

## Cell model

```go
type Cell struct {
    ID          int
    Center      Vec3          // on sphere surface, scaled by radius
    Corners     []Vec3        // 5 or 6, CCW around local up
    Neighbors   []int          // cell IDs, sorted
    Pentagon    bool
    Elevation   float64
    Moisture    float64
    Temperature float64        // pre-seeded to 1 - abs(Center.Y)
    Ocean       bool
    BaseColor   color.RGBA
    Data        map[string]float64
    Tags        map[string]bool
}
```

`Data` and `Tags` are initialised as empty maps in `buildDualCells` so rulesets can write without nil checks.

## Ruleset interface

```go
type Ruleset interface {
    Name() string
    Init(*Globe)
    Update(*Globe, float64)
    StyleCell(*Globe, *Cell) CellStyle
}

type CellStyle struct {
    Fill      color.RGBA
    Edge      color.RGBA
    Height    float64    // radial extrusion in units of radius
    Highlight float64    // 0..1, added to shade + edge strength
}
```

- `Init` runs once at start.
- `Update` runs per tick. May mutate cells, `globe.RotationY`, `globe.TiltX`, `globe.SelectedCell`.
- `StyleCell` runs per-cell per-frame, returning how the cell should look right now. It does not mutate.

`petridish.NewGame` hard-codes `core.NewDemoRuleset()`.

## Demo ruleset (`core/rules.go`)

- **Init.** Elevation and moisture from layered sines of `cell.Center`. `Ocean ≡ elevation < 0.5`. `coast` tag when `abs(elevation - 0.5) < 0.045`. `BaseColor` from `biomeColor`: deep/shallow ocean blends, polar caps by latitude, mountain/desert/temperate by elevation and moisture.
- **Update.** Rotates the globe at 0.32 rad/s (suppressed while dragging — that check lives in `Game.Update`). Re-picks `SelectedCell` as the front-facing land cell closest to a fixed screen-space target.
- **StyleCell.** Non-ocean cells extrude outward proportional to elevation. Coasts blend a sand tint and nudge up. The selected cell pulses with `sin(r.time * 2.2)`.

## Rendering pipeline (`petridish/game.go:drawGlobe`)

Per frame:

1. For each cell, compute a world-space center and corners extruded by `1 + style.Height`.
2. Apply `RotateY(globe.RotationY)` then `RotateX(globe.TiltX)`.
3. Back-face cull: drop the cell if `normal · viewDir ≤ 0` (view direction from cell to camera at `Z = 3.1`).
4. Depth sort by transformed `center.Z` ascending.
5. Pinhole project: `screen = c + v.xy * scale / (cameraZ - v.z)`.
6. Lambert-ish shade: `0.55 + 0.45 * max(0, normal · light)` with light `(-0.5, 0.6, 1).normalize()`. Add `style.Highlight`.
7. Triangle-fan fill via `DrawTriangles` against a 1×1 white source image. Per-edge `StrokeLine` at 1.4 px for the edge.

The backdrop is five alpha-stacked discs behind the globe.

### Current behavior notes

- Painter's-algorithm depth sort misorders at high extrusion deltas (not an issue at current demo heights).
- No edge anti-aliasing.
- Perspective is fixed; no zoom.
- Pentagon cells use the same fan triangulation; their centre is slightly off the true centroid of their corners.

## Input (`petridish/game.go:handlePointerInput`)

Unified mouse + touch. On `MouseButtonLeft` or first touch, start dragging. Moves integrate:

- Δx → `globe.RotationY += dx * 0.012`
- Δy → `globe.TiltX += dy * 0.006`, clamped to `[-0.9, 0.25]`

Touch is single-finger: the first touch ID wins, extra touches are ignored. Auto-rotation in `DemoRuleset.Update` is suppressed while dragging.

## Mobile integration

```go
// mobile/mobile.go
func init() { mobile.SetGame(petridish.NewGame()) }
func Dummy() {} // gomobile requires ≥1 exported symbol
```

`ebitenmobile bind -target android -javapkg com.quartermeat.petridish -o android/app/libs/PetriDish.aar ./mobile` produces a `com.quartermeat.petridish.mobile` Java package containing `EbitenView` and `Mobile`. `MainActivity` inflates `EbitenView` from XML, calls `Seq.setContext(applicationContext)` and `Mobile.dummy()` during `onCreate`, and proxies lifecycle into `suspendGame` / `resumeGame`.
