# Cross-platform 2D remake foundation

This project is an independent, fan-created Go/Ebitengine foundation for a cross-platform 2D action game recreation and clean-room recompilation. It contains newly written source code for an interactive frontend, generated-content loader, and diagnostic map viewer.

## Run

```powershell
go run . --assets="path-to-your-content"
```

The `--assets` value may point to a generated cache, a local content directory, or a native Android package containing the game content. When an APK is supplied on desktop, the runtime reads it into the ignored disposable cache under `bin/` and reuses that cache by source hash; the package and extracted files are never part of the repository. The first run prepares the local ignored cache automatically, including the shipped level XML, frontend variables, and story scripts.

The desktop window can be resized or maximized; the 480×320 scene is stretched to the full drawable window.

Source icon assets live under `resources/`. The Windows build regenerates the multi-resolution icon resource from `resources/icon.ico` and stages the linker input only for that build, so the project root stays free of icon files while Explorer can select the correct size.

```powershell
go run . --assets=.\bin\data-cache
```

Normal progression is title/start → main menu → Story/Survival ShopFront → unlocked level launch. The generic world catalog is not used in the normal path while its native UI definition is being recovered. Use the arrow keys or WASD to move Barry; on desktop the mouse continuously controls facing and a left click fires. Mobile WASM uses the circular virtual-stick HUD automatically, or it can be forced with `--mobile`. Menu music, menu selection sounds, and pistol fire sound use the supplied content references when available. The diagnostic map viewer and its layer, grid, zoom, pan, and inspection controls are available only with `--debug`:

```powershell
go run . --assets=.\bin\data-cache --debug
```

For unattended visual verification, save the actual rendered window surface on state changes or at a fixed interval:

```powershell
go run . --assets=.\bin\data-cache --capture-dir=.\bin\captures --capture-every=30
```

Captures are disposable PNGs named with their frame and state; the capture directory is ignored by Git.

For a bounded background probe of a specific state, add `--capture-state=title`, `main-menu`, `world-select`, `level-select`, `play`, or `debug-viewer` with `--capture-frames=1`. For menu hover-state QA, add `--capture-selection=0` through `3` with `--capture-state=main-menu`.

Build the browser host bundle with `.\scripts\build.ps1 -Target wasm`; serve `bin\web` over HTTP.

Original content, native libraries, and generated private caches are not part of this source tree. See `LEGAL_NOTICE.md` for the project boundary.
