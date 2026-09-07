# Cross-platform 2D remake foundation

This project is an independent, fan-created Go/Ebitengine foundation for a cross-platform 2D action game recreation and clean-room recompilation. It contains newly written source code for an interactive frontend, generated-content loader, and diagnostic map viewer.

## Run

```powershell
go run . --assets="path-to-your-content"
```

The `--assets` value may point to generated content or a local content directory. The first run prepares the local ignored cache automatically.

```powershell
go run . --assets=.\bin\data-cache
```

Original content, native libraries, and generated private caches are not part of this source tree. See `LEGAL_NOTICE.md` for the project boundary.
