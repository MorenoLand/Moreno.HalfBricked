# Cross-platform 2D remake foundation

This project is an independent, fan-created Go/Ebitengine foundation for a cross-platform 2D action game recreation and clean-room recompilation. It contains newly written source code for an interactive frontend, generated-content loader, and diagnostic map viewer.

## Run

```powershell
go run . --assets="path-to-your-content"
```

The `--assets` value may point to generated content or a local content directory. The first run prepares the local ignored cache automatically.

The desktop window can be resized or maximized; the 480×320 scene is stretched to the full drawable window.

```powershell
go run . --assets=.\bin\data-cache
```

Normal level selection enters the playable map slice. Use the arrow keys or WASD to move Barry; on desktop the mouse continuously controls facing and a left click fires. Mobile WASM uses the circular virtual-stick HUD automatically, or it can be forced with `--mobile`. The diagnostic map viewer and its layer, grid, zoom, pan, and inspection controls are available only with `--debug`:

```powershell
go run . --assets=.\bin\data-cache --debug
```

Build the browser host bundle with `.\scripts\build.ps1 -Target wasm`; serve `bin\web` over HTTP.

Original content, native libraries, and generated private caches are not part of this source tree. See `LEGAL_NOTICE.md` for the project boundary.
