# Project layout

- Root: entry point only (`main.go` embeds the window icons and calls `game.Run()`), plus `go.mod`/`go.sum` and the non-code folders (`bin`, `build`, `resources`, `scripts`, `tools`, `web`).
- `game/`: the game itself (`package game`): `app`, `playState`, menus, the script host, co-op and networking glue. It is one package because those pieces share `app`/`playState`. Platform files (`*_native.go`/`*_wasm.go`) keep their build tags. Tests run from the repository root (see `TestMain`), so relative paths such as `bin/data-cache` resolve.
- `engine/*`: reusable packages.

Packages extracted from the game (self-contained, no dependency on `app`/`playState`):

- `engine/weapons`: weapon firing maths, projectiles, weapon audio queue, secondary (grenade) weapons, pickup parity, grenade explosion frame/vertex maths, and the native RNG.
- `engine/achievements`: stationary / no-kill achievement tracking.
- `engine/stats`: player statistics data and number/time formatting.
- `engine/carousel`: level-select carousel catalogue and finite scroll core.
- `engine/menumotion`: main-menu zombie motion state machine.

Existing engine packages: `formats`, `netplay`, `scripting`, `ui`, `viewer`.
