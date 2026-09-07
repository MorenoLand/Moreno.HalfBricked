# Cross-platform 2D remake foundation

This project is an independent, fan-created Go/Ebitengine foundation for a cross-platform 2D action game recreation. It contains newly written source code for an interactive frontend, generated-content loader, and diagnostic map viewer.

## Reference import

```powershell
go run . --assets="G:\Development\Go\Moreno.AgeofZombies\Age of Zombies"
```

## Run

```powershell
go run . --assets=.aoz-cache
```

## Inspect

```powershell
go run ../tools/aoz-inspect --reference "G:\Development\Go\Moreno.AgeofZombies\Age of Zombies" --level World0Level0
```

Original content, native libraries, and generated private caches are not part of this source tree. See `LEGAL_NOTICE.md` for the project boundary.
