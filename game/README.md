# game/ - map of the code

`game` is the whole playable application: menus, level play, scripting host, co-op and rendering, driven from
`game.Run()` (called by the repository's `main.go`). Go allows one package per directory, so everything that could be
carved out without an import cycle lives in a subpackage below; the rest is one large package that is **grouped by
file name** in the tables further down.

## Subpackages (leaf packages, no import of `game`)

| Directory | What it is | Used from |
|---|---|---|
| `platform/` | Machine glue with a native and a js/wasm twin per file: clipboard (`ClipboardWrite/Request/Take`), profile storage (`ReadProfile/WriteProfile`: `bin/profile.json` vs `localStorage`), `QuitGame`, and the browser switches `PerfOverlay` / `AudioDisabled` (`?perf`, `?noaudio`) with the perfmark hook. | `net_lobby.go`, `net_session.go`, `player_profile.go`, `pause_menu.go`, `perf_overlay.go`, `main.go` |
| `discord/` | Discord Rich Presence over the local IPC socket (`RPC`, `DefaultAppID`, `SetAppID`). Has its own unit test. | `main.go`, `extras.go` |
| `netwire/` | The online co-op wire format: `Msg`, `Snapshot`, `Player`, `Zombie`, `Bullet`, ... plus `Encode`/`Decode`. Plain data with exported fields; the game converts to/from it in `net_wire.go`. | `net_wire.go`, `net_session.go`, `combo*.go` |
| `focusgraph/` | Parser of the native `Controller/*.txt` focus graphs (`Parse`, `Graph`, `Neighbour`). | `ui_controller.go`, `credits.go` |
| `nativemath/` | Small pure ports of native arithmetic: `CameoY`, `ScoreTextGeometry`, `ZombieSpriteScale`, the portal fade/rotation steps. | `main.go`, `script_host.go` |

Reusable engine code stays in `../engine/*` (formats, content, scripting, weapons, netplay, ...).

## Files that stay in package `game`, by area

### Boot, loop, scene switching
- `main.go` - the `app` struct, `newApp`, `Run`, `Update` / `Draw` / `Layout`, the capture states, and (historically)
  the level-select, title, menu-zombie, play-field, HUD and bullet drawing and the `playState` update loop,
  spawning, firing and camera. It is the big file; the sections are, in order: app/play state types and constants,
  `newApp` + music, `Update`, level select, `Draw`, capture, menus/title/level cards, text/font helpers, `openPlay`,
  `playState.Update` (walk, bullets, waves, zombies, portals, explosions), weapon fire, camera/collision, textures.
- `loading_gate.go` - browser loading screen that holds a click until the needed files are downloaded.
- `level_prefetch.go`, `level_music.go` - background download of the selected level, per-level music choice.
- `perf_overlay.go` - frame-time readout drawing (the flags live in `platform`).

### Menus and front end
- `main_menu_groups.go` - the 1.2.5 main-menu zombie groups parsed from `MainScreen.uiscreen`.
- `menu_hover.go` - hover easing of menu text.
- `options_menu.go`, `options_control.go` - options page and control-layout settings.
- `pause_menu.go`, `confirm_dialog.go` - pause screen and the "are you sure" box.
- `extras.go` - the port's EXTRA page (FOV, Discord toggle).
- `stats_menu.go` - statistics page.
- `credits.go` - credits roll.
- `last_played.go`, `level_badges.go`, `level_completion.go`, `survival_gate.go` - level-select memory, badges,
  completion/exit-script flow, story-gated survival maps.
- `sandbox.go` - the F1 sandbox/cheat menu.
- `ui_controller.go`, `gamepad_menu.go` - focus-graph loading and gamepad menu navigation + "controller attached" notice.

### Results and profile
- `results_menu.go`, `results_screen.go`, `results_count.go`, `results_runtime.go`, `player_dead.go` - TV results
  screen, count-up, death/results flow.
- `player_profile.go` - profile.json (de)serialisation (storage backend is `platform`).

### Achievements and stats
- `achievements.go`, `achievement_events.go`, `achievement_progress.go`, `achievement_toast.go`.

### HUD and presentation
- `hud_hearts.go`, `hud_progress.go`, `hud_weapon_button.go`, `play_controls.go` (touch/stick buttons),
  `wave_banner.go`, `combo.go`, `combo_draw.go`, `combo_kill_pop.go` (score multiplier and pop-ups),
  `offscreen_markers.go`, `random_box_mark.go`, `life_crate.go`, `effects.go`, `slowmo.go`.

### Input
- `coop_input.go` (gamepad sticks for co-op), `gamepad_menu.go` (menu pads), `play_controls.go`.

### Zombies and AI
- `zombie_model.go` (spawn record, health/damage), `zombie_ai.go`, `zombie_navigation.go`, `zombie_tiles.go`,
  `zombie_unstick.go`, `zombie_weapons.go`, `zombie_gib.go`, `zombie_trail.go`, `zombie_shadows.go`,
  `zombie_zoo.go`, `exploding_zombie.go`, `entity_grid.go`.

### Bosses
- `boss_fights.go`, `rex_ai.go`, `rex_shockwave.go`, `samurai.go`, `samurai_blade.go`, `gangster_car.go`,
  `egypt_boss.go`, `scene_car.go`.

### Weapons and projectiles
- `weapon_playback.go`, `weapon_render.go`, `gun_visual.go`, `secondary_slot.go`, `sentry.go`, `shield.go`,
  `thrown_bomb.go`, `blast.go`, `grenade_explosion.go`, `flame_flight.go`, `saw_blade.go`, `saw_audio.go`,
  `projectile_tiles.go`.

### Pickups
- `pickup_drop.go`, `random_pickups.go`, `life_crate.go`, `shield.go`.

### Waves, survival and level flow
- `wave_timing.go` (wave-advance rule), `survival.go` (escalation), `level_completion.go`, `train_hazard.go`
  (western train hazard).

### Rendering and depth
- `depth_band.go`, `depth_sort.go`, `camera_build.go`, `camera_lookahead.go`, `camera_shake.go`, `cameo` math is in
  `nativemath/`.

### Scripting
- `script_host.go` - the `playScriptHost` that backs the level scripts (entities, textures, callbacks).
- `script125_host.go` - the 1.2.5-only script calls (BrickUI, analytics).
- `script_position.go` - GetPositionWithinRadius search.

### Networking / co-op
- `coop.go`, `coop_draw.go`, `coop_input.go` - local co-op players.
- `net_session.go`, `net_lobby.go`, `net_wire.go` - online co-op (WebRTC session, lobby UI, snapshot conversion;
  the wire types are `netwire/`).

## Blocked on (why the rest is still one package)

Everything above is welded together by three types whose fields are all unexported and read directly everywhere:

- `app` (about 100 methods across 60 files: textures, sound, fonts, profile, menu state),
- `playState` (the running level: bodies, zombies, bullets, camera, wave state, combo, script entities),
- `zombieState` / `bodyState` / `bullet` and friends (read and written field by field by AI, bosses, drawing,
  gibs, snapshots and about 100 tests).

A file such as `zombie_ai.go` or `samurai.go` is "a few methods on `*playState`", so it cannot leave package `game`
without either exporting those structs wholesale or giving it an interface. Subsystems that look separable
(`combo`, `sentry`, `survival`, `wave_timing`, `credits`, `results_*`, `net_session`) were checked: their state
structs are small, but their methods call `a.text`, `a.drawImage`, `a.sound`, `p.zombies`, and the ~100 test files
reach into their unexported fields, so extracting them now would only move the coupling behind a pile of exported
names. They were therefore left in place.

### Follow-up refactor that would unlock the core

1. Move the pure data structs (`zombieState`, `zombieNative`, `bodyState`, `bullet`, `explosionState`, `portalState`,
   `sentryState`, `mineState`, `comboSystem`, `survivalState`, ...) into `game/entities` with exported fields.
   Mechanical, no behaviour change; `playState` keeps holding slices of them.
2. Define a small `Env` interface next to the AI code (`game/zombies`): `Solid(x, y) bool`,
   `CollisionValue(tx, ty) uint32`, `TileSize() int`, `Player() Target`, `RNG() *weapons.NativeRNG`,
   `Build() content.WaveBuild`, `PlaySound(path, volume)`, `Spawn(...)`. `*playState` implements it. Zombie AI,
   navigation, unstick, bosses and gibs then take `Env` plus `*entities.Zombie` instead of `*playState`.
3. Do the same for drawing with a `Renderer` interface (`DrawImage`, `Text`, `Texture(name)`), which lets the
   HUD, combo draw, markers and menus leave package `game`; `app` implements it.
4. Only then split `main.go`'s `playState.Update` into per-system `Update(env)` calls (`waves`, `projectiles`,
   `zombies`, `player`), and move the matching tests next to each package.

## Notes

- Tests for package `game` live outside this directory at the moment (`../tests/`); they must be placed back in
  `game/` to compile because they use unexported identifiers.
- wasm: `GOOS=js GOARCH=wasm go vet ./game/...` must pass. Platform files are paired by build tag inside `platform/`.
