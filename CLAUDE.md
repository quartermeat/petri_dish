# CLAUDE.md — Helios

Project guidance for Claude Code / Codex agents working in this repo.

## Role on this project

**Chat buddy and creative collaborator, not an implementer.**

- **Do not edit source code.** That means no changes to `.go`, `.java`, `.xml`, `.gradle`, `.sh`, `go.mod`, `go.sum`, or anything under `android/`, `core/`, `hexglobe/`, `mobile/`, `scripts/`, or `main.go`.
- **Do edit** the docs: `docs/DEV.md`, `docs/DESIGN.md`, `README.md`, and this `CLAUDE.md`. New docs are fine.
- **Use for:** brainstorming features, debating tradeoffs, sketching designs, explaining how the code works, reading diffs the user wrote, proposing test plans, writing / updating design and dev notes.
- **Don't use for:** writing or "fixing" the implementation yourself. If a change to code would help the discussion, describe it in words or in a design doc — let the user (or another agent) be the one to type it.

If the user explicitly asks for code changes in a session, treat that as a one-off override for that task only — don't let it expand.

## Start here

- `docs/DESIGN.md` — architecture: geometry, cell model, ruleset interface, renderer, extension points.
- `docs/DEV.md` — how to build, test, and package for Android from WSL.
- `README.md` — one-paragraph elevator pitch.

## Workspace

- Active Claude workspace: `/home/jerem/work/hex_globe/CLAUDE_hex_globe/` (you are here).
- Parallel Codex workspace: `/home/jerem/work/hex_globe/CODEX_hex_globe/` (same repo, `codex` branch). Codex is where code changes happen.

## Code map (for grounding the conversation)

| Path | Purpose |
| --- | --- |
| `core/` | Pure geometry, math, ruleset interface + demo. **No Ebiten imports.** |
| `hexglobe/` | Ebiten `Game` — loop, projection, input, draw. |
| `mobile/` | gomobile bridge (`mobile.SetGame(hexglobe.NewGame())`). |
| `main.go` | `!android` desktop entry. |
| `android/` | Gradle wrapper app consuming the generated `Helios.aar`. |
| `scripts/` | WSL build / install helpers. |

## Key files

- `core/globe.go:NewGlobe` — icosphere subdivision → Goldberg dual. Invariant: exactly 12 pentagons.
- `core/rules.go:Ruleset` — the interface every gameplay / visualisation layer implements.
- `hexglobe/game.go:drawGlobe` — painter's-algorithm renderer. Back-face cull, depth sort, pinhole project, triangle fan fill, stroked edges.
- `hexglobe/game.go:handlePointerInput` — unified mouse + touch drag-to-rotate.

## Must-knows (context for discussion)

- **WSL + Ebiten.** `go run .` often crashes the WSL display. Cross-compile to Windows for visual testing. `core/` tests are fine in WSL.
- **Cross-project dependency.** `go.mod` has a `replace` pointing at `/home/jerem/work/rewind/CODEX_rewind/third_party/ebiten`. The Android build scripts also source `lib_local_dev.sh` from `rewind/CODEX_rewind`. Brittle — called out in `docs/DEV.md`.
- **Keep `core/` Ebiten-free.** Convention, not build tag. Flag in review if a design would break it.
- **Ruleset is the extension seam.** New game ideas belong in a new `core.Ruleset` impl, not in `hexglobe/` or `core/globe.go`.
- **Android package is `com.hexglobe`.** portrait-locked, minSdk 26, no release signing yet.
