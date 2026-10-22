# Cross-platform 2D remake foundation

This project is an independent, fan-created Go/Ebitengine foundation for a cross-platform 2D action game recreation and clean-room recompilation. It contains newly written source code for an interactive frontend, generated-content loader, and diagnostic map viewer.

## Run

```powershell
go run . --assets="path-to-your-content"
```

The `--assets` value may point to a generated cache, a local content directory, or a native Android package containing the game content. When an APK is supplied on desktop, the runtime reads it into the ignored disposable cache under `bin/` and reuses that cache by source hash; the package and extracted files are never part of the repository. The first run prepares the local ignored cache automatically, including the shipped level XML, frontend variables, and story scripts.

The desktop window can be resized or maximized. Title, loading, and frontend menu states render directly to the full drawable output at the forced 16:9 aspect, using nearest sampling with no linear interpolation; gameplay and the diagnostic viewer retain the 480×320 logical scene.

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

For a bounded background probe of a specific state, add `--capture-state=title`, `main-menu`, `level-select`, `play`, or `debug-viewer` with `--capture-frames=1`. For menu hover-state QA, add `--capture-selection=0` through `3` with `--capture-state=main-menu`.

Run `.\scripts\build.ps1` to build the Windows executable and browser host bundle; use `-Target wasm` for only the WASM bundle. Serve `bin\web` over HTTP. `-WebMode apk` builds `bin\web-apk` instead: a bundle with no game data whose visitors pick their own APK (see `web/README.md`).

On the first launch without `--assets`, the desktop build asks for your Age of Zombies APK with a file dialog and remembers where it is (a small file in your user config directory); `--choose-apk` asks again, and F2 in the program chooses another APK. The window opens first and shows the files being prepared. They are made into a `data` folder next to the program (the working directory under `go run`, whose program lives in a temporary folder); an existing `data` folder that is not a game cache is never overwritten. Capture runs (`--capture-dir`) are never asked, and `--assets` skips the question.

Original content, native libraries, and generated private caches are not part of this source tree. See `LEGAL_NOTICE.md` for the project boundary.

## Level editor (port addition)

F4 opens a level editor from the menus (`-level-editor=false` turns it off). Pick any level of the cache, or press N for a
new empty one: you choose its size and tileset, and it takes the waves of the level the list is on. Paint the five tile layers (1-5: ground,
decoration, tall objects, objects behind the player, collision and spawn markers), place and move props (P), resize the map
(Ctrl+arrows), change the tileset from a menu with previews (Ctrl+T), undo/redo (Ctrl+Z / Ctrl+Y), and play the result at once (T, quitting the level
returns to the editor).

- Ctrl+S saves the level as a JSON file anywhere; Ctrl+O opens one again.
- Ctrl+Shift+S saves over the level's own file in the cache (the original is kept next to it as `.orig`).
- E opens the wave editor: waves, their spawners (zone, count, start time) and the kinds each drops (kind, look, weapon, weight, health, speed). It warns when a spawner's zone is not painted on the collision layer.
- In the browser the editor saves as a download and cannot open a file.

More in the editor (all in the toolbar and the File menu; F1 shows every shortcut):

- **File menu:** open or save a level file, save over a cache level, **Add to the game** (the level is listed in level select,
  in the world you chose, and kept in a `custom/` folder next to the game's `data/` folder), copy or paste a **level code** (the
  whole level as one line of text, to send to someone), recover the autosave, and set the keyboard shortcuts.
- **Level details:** name, description, id, story or survival, world, music, level card picture, the next level, and an opening
  conversation with portraits.
- **Problems:** a list of what would stop a level working (no player spawn, a wave zone that is not painted or is walled off, and
  so on); a click goes to the place. **Test options** (the gear): god mode, a starting wave, start where the map view is.
- **Wave editor numbers:** zombies, health and when they appear (a chart), per wave.
- **Tools:** paint, line, rectangle, fill, pick, select (copy, cut, paste, mirror), pickups (crates placed at fixed spots, at the
  start or when a wave begins), props (including any texture of the game, from the prop library) and stamps (saved pieces you
  can place again). Layers can be hidden (Alt+click shows one alone) and locked. A minimap, undo history and recent tiles help
  on big maps.
- **Auto-edge (experimental):** after you paint, the cells around it follow what the game's own levels put there. Measured on
  the game's levels with one level left out each time: on ground tiles beside a change it is confident about 55% of cells and right
  about 89% of the time; the other layers are 90 to 95%. It can still be wrong, so it is an option and undo undoes it.
