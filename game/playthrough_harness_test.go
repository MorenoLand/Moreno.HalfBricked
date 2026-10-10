package game

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

// Headless level playthrough harness. A scripted bot drives the real app.Update path
// (script runtime, waves, boss scripts, exit script, results screen, continueStoryLevel)
// through the injected primary-player input (app.inputHook). Nothing here opens a window.

type ptCache struct{ name, root string }

func ptCaches() []ptCache {
	var caches []ptCache
	for _, c := range []ptCache{{"hd-1.2.5", "bin/data-cache"}, {"sd-1.2.1", "bin/web/data/data-cache"}} {
		if _, err := os.Stat(filepath.Join(c.root, "pack.json")); err == nil {
			caches = append(caches, c)
		}
	}
	return caches
}

// newHeadlessApp mirrors newApp without the audio context (only one may exist per process),
// the profile file (the harness must not read or write the player's bin/profile.json) and fonts.
func newHeadlessApp(t testing.TB, root string) *app {
	t.Helper()
	pack, err := content.NewPack(content.NewSource(root))
	if err != nil {
		t.Fatal(err)
	}
	weaponCatalog, err := pack.Weapons()
	if err != nil {
		t.Fatal(err)
	}
	zombieWeapons, err := pack.ZombieWeapons()
	if err != nil {
		t.Fatal(err)
	}
	sprites, err := pack.Sprites()
	if err != nil {
		t.Fatal(err)
	}
	weapon, ok := weaponCatalog.Find("PISTOL")
	if !ok {
		t.Fatal("no pistol")
	}
	a := &app{pack: pack, levels: pack.List(), variables: pack.Variables(), silent: true, titleScreen: false, weapon: weapon, weapons: weaponCatalog, zombieWeapons: zombieWeapons, sprites: sprites, unlocked: initialUnlocks(pack.List()), images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}, frontendScaleX: 1, frontendScaleY: 1, outputWidth: logicalWidth, outputHeight: logicalHeight}
	a.options = newOptionsMenu(false, false)
	a.statistics = stats.NewStatsData()
	a.achievements, err = pack.Achievements()
	if err != nil {
		t.Fatal(err)
	}
	rng := weapons.NewNativeRNG()
	a.rng = &rng
	a.profileWritable = false
	return a
}

type ptOutcome struct {
	Cache, ID, Mode string
	Pass            bool
	God             bool // the level only completed with god mode
	Frames          int
	Seconds         float64
	Waves, WavesLen int
	Kills           int
	Bosses          []string
	EntryDoneFrame  int
	ExitStarted     int
	ResultsOpened   string
	NextLoaded      string
	Issues          []string
	Notes           []string
}

func (o *ptOutcome) fail(format string, args ...any) {
	o.Pass = false
	o.Issues = append(o.Issues, fmt.Sprintf(format, args...))
}
func (o *ptOutcome) note(format string, args ...any) {
	o.Notes = append(o.Notes, fmt.Sprintf(format, args...))
}

type ptBot struct {
	a        *app
	frame    int
	stuck    int
	lastX    float64
	lastY    float64
	detour   int
	detourDX float64
	detourDY float64
	grenade  int
	kills    int
	killAt   int
	path     [][2]int
	last     playerInput
	pathAt   int
	pathGoal [2]int
}

func (b *ptBot) nearestZombie() (zombieState, float64, bool) {
	p := b.a.play
	best, bestDistance, found := zombieState{}, math.Inf(1), false
	for _, z := range p.zombies {
		if z.dying || z.spawnAway || z.health <= 0 || z.alpha <= 0 {
			continue
		}
		if d := math.Hypot(z.x-p.x, z.y-p.y); d < bestDistance {
			best, bestDistance, found = z, d, true
		}
	}
	return best, bestDistance, found
}

func (b *ptBot) input() playerInput {
	b.frame++
	p := b.a.play
	var in playerInput
	if p == nil || p.health <= 0 {
		return in
	}
	if math.Hypot(p.x-b.lastX, p.y-b.lastY) < .05 {
		b.stuck++
	} else {
		b.stuck = 0
	}
	b.lastX, b.lastY = p.x, p.y
	var moveX, moveY float64
	zombie, distance, haveZombie := b.nearestZombie()
	if haveZombie && distance < 420 {
		dx, dy := zombie.x-p.x, zombie.y-p.y
		length := math.Hypot(dx, dy)
		if length > .001 {
			in.aimX, in.aimY = dx/length, dy/length
		}
		// keep a little distance from melee range, strafe around rather than hug the zombie
		hold := 220.0
		if p.scriptRuntime != nil && !p.scriptRuntime.Done() && !p.exitScriptStarted && strings.HasPrefix(p.scriptEntities[zombie.scriptID].entityTypeOrEmpty(), "boss_") {
			// a boss intro waits for Barry to walk up to the boss (the japan_boss range circle sits 150 px below it)
			hold = 80
		}
		switch {
		case distance < 60:
			moveX, moveY = -dx/length, -dy/length
		case distance > hold:
			moveX, moveY = b.steer(zombie.x, zombie.y)
		}
	} else if haveZombie {
		moveX, moveY = b.steer(zombie.x, zombie.y)
	}
	// A boss intro that waits for Barry to come into range of a boss that was shot dead meanwhile: walk to the corpse.
	if scriptRunning := p.scriptRuntime != nil && !p.scriptRuntime.Done(); scriptRunning && !haveZombie && !p.exitScriptStarted {
		ids := make([]int, 0, len(p.scriptEntities))
		for id := range p.scriptEntities {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		for _, id := range ids {
			if entity := p.scriptEntities[id]; entity != nil && strings.HasPrefix(entity.entityType, "boss_") && math.Hypot(entity.x-p.x, entity.y-p.y) > 120 {
				moveX, moveY = b.steer(entity.x, entity.y)
				break
			}
		}
	}
	// Weapon / secondary pickups when nothing is close.
	scriptRunning := p.scriptRuntime != nil && !p.scriptRuntime.Done()
	if (!haveZombie || distance > 150) && (!scriptRunning || b.frame%360 < 100) {
		bestDistance := 600.0
		ids := make([]int, 0, len(p.scriptEntities))
		for id := range p.scriptEntities {
			ids = append(ids, id)
		}
		sort.Ints(ids) // map order would make the bot, and so every run, nondeterministic
		for _, id := range ids {
			entity := p.scriptEntities[id]
			if entity == nil || entity.kind != "pickup" {
				continue
			}
			if d := math.Hypot(entity.x-p.x, entity.y-p.y); d < bestDistance {
				bestDistance = d
				moveX, moveY = b.steer(entity.x, entity.y)
			}
		}
	}
	if b.stuck > 25 && (moveX != 0 || moveY != 0) {
		// Blocked by a wall: sidestep for a while (deterministic perpendicular detour).
		b.detour = 45
		sign := 1.0
		if (b.frame/90)%2 == 1 {
			sign = -1
		}
		b.detourDX, b.detourDY = -moveY*sign, moveX*sign
		b.stuck = 0
	}
	if b.detour > 0 {
		b.detour--
		moveX, moveY = b.detourDX, b.detourDY
	}
	if moveX == 0 && moveY == 0 && (p.scriptRuntime != nil && !p.scriptRuntime.Done()) && b.frame%360 < 100 {
		// A tutorial prompt waits for the player to move / aim / use the secondary: do what a human would
		// (walk around, aim and fire in changing directions, press the secondary), but only in short bursts:
		// cutscenes that chase the player's position (sprites stepping 2 px/frame) need Barry to stand still.
		angle := float64(b.frame/45) * 1.3
		moveX, moveY = math.Cos(angle), math.Sin(angle)
		if in.aimX == 0 && in.aimY == 0 {
			in.aimX, in.aimY = math.Cos(angle+2), math.Sin(angle+2)
		}
		if b.frame%240 < 40 {
			in.secondary = true
			if b.frame%240 == 0 {
				in.secondaryHit = true
			}
		}
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() && p.grenades > 0 && b.frame%120 < 20 {
		// scripted "throw a grenade" prompts (tutorial): tap the secondary button now and then
		in.secondary = true
		in.secondaryHit = b.frame%120 == 0
	}
	if p.levelKills != b.kills {
		b.kills, b.killAt = p.levelKills, b.frame
	}
	if haveZombie && distance < 120 && b.frame-b.killAt > 300 {
		// Pinned against a wall by zombies standing inside the muzzle offset: a human would run for it.
		angle := float64(b.frame/50) * 1.7
		moveX, moveY = math.Cos(angle), math.Sin(angle)
	}
	if p.moveControl && !p.exitScriptStarted {
		// (the exit script is a cutscene that walks Barry itself: a human lets it play, and sprites that chase his
		// position at 2 px/frame never catch a Barry who keeps walking away)
		in.moveX, in.moveY = moveX, moveY
	}
	if p.exitScriptStarted {
		in.secondary, in.secondaryHit = false, false
	}
	if b.grenade > 0 {
		b.grenade--
	}
	if p.grenades > 0 && b.grenade == 0 {
		near := 0
		for _, z := range p.zombies {
			if !z.dying && !z.spawnAway && z.health > 0 && math.Hypot(z.x-p.x, z.y-p.y) < 140 {
				near++
			}
		}
		if near >= 4 {
			in.secondaryHit, in.secondary = true, true
			b.grenade = 120
		}
	}
	b.last = in
	return in
}

type ptOptions struct {
	maxFrames int
	god       bool
	seed      int // shifts the bot's wander phase (the RNG stream is advanced by the caller)
}

// ptTexturesResolve reports script texture / zombie texture names that do not resolve with
// the same fallbacks the draw paths use.
func (a *app) ptTextureOK(name string) bool {
	if name == "" {
		return true
	}
	for _, candidate := range []string{commonSDTexture(name), name, name + "_SD"} {
		if _, err := a.Texture(candidate); err == nil {
			return true
		}
	}
	return false
}

func (a *app) ptZombieTextureOK(name string) bool {
	if name == "" {
		return true
	}
	if animation, ok := a.spriteAnimation(name, ""); ok {
		_, err := a.Texture(animation.Texture)
		return err == nil
	}
	_, err := a.Texture(commonSDTexture(name))
	return err == nil
}

// ptStaticAudit checks the level data without running it.
func ptStaticAudit(a *app, info formats.LevelInfo, out *ptOutcome) {
	level, err := a.pack.Load(info.ID)
	if err != nil {
		out.fail("load: %v", err)
		return
	}
	if err := level.Validate(); err != nil {
		out.fail("validate: %v", err)
	}
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		out.fail("tileset %q missing", level.Tileset)
		return
	}
	if _, err := a.Texture(tileset.Texture); err != nil {
		out.fail("tileset texture: %v", err)
	}
	missing := map[string]bool{}
	for _, prop := range level.Props {
		if _, err := a.Texture(prop.Texture); err != nil {
			if _, err := a.Texture(prop.Texture + "_SD"); err != nil {
				missing[prop.Texture] = true
			}
		}
	}
	for _, prop := range level.AnimatedProps {
		if _, err := a.Texture(prop.Texture); err != nil {
			if _, err := a.Texture(prop.Texture + "_SD"); err != nil {
				missing[prop.Texture] = true
			}
		}
	}
	for name := range missing {
		out.fail("prop texture %q missing", name)
	}
	// spawn point: valid and not inside a wall
	layerC := level.Layers[formats.LayerC]
	spawnTile := -1
	for index, value := range layerC {
		if value == 2 {
			spawnTile = index
			break
		}
	}
	if spawnTile < 0 {
		out.fail("no player spawn marker (layer C value 2); the port falls back to the map centre")
	}
	probe := &playState{world: nil}
	_ = probe
	markers := map[uint32]int{}
	for _, value := range layerC {
		markers[value]++
	}
	out.WavesLen = len(level.Waves)
	if len(level.Waves) == 0 {
		out.fail("no waves")
	}
	reportedSpawner := map[int]bool{}
	reportedType := map[string]bool{}
	for waveIndex, wave := range level.Waves {
		if wave.NextWave >= len(level.Waves) {
			out.fail("wave %d nextWave %d out of range", waveIndex, wave.NextWave)
		}
		if wave.RunTime <= 0 && len(wave.Spawners) > 0 {
			out.note("wave %d runTime %v", waveIndex, wave.RunTime)
		}
		for _, spawner := range wave.Spawners {
			if spawner.Count <= 0 || len(spawner.Types) == 0 {
				if spawner.Count < 0 {
					out.fail("wave %d spawner %d: count=%d", waveIndex, spawner.Index, spawner.Count)
				}
				continue // authored-but-unused spawner slot (count 0): nothing to spawn
			}
			if spawner.Index < 1 || spawner.Index > 13 {
				out.fail("wave %d spawner index %d outside 1..13 (never spawns in the port)", waveIndex, spawner.Index)
				continue
			}
			if markers[uint32(spawner.Index+2)] == 0 && !reportedSpawner[spawner.Index] {
				reportedSpawner[spawner.Index] = true
				pickupsOnly := true
				for _, entry := range spawner.Types {
					if !strings.HasPrefix(strings.ToLower(entry.Name), "p_") {
						pickupsOnly = false
					}
				}
				if pickupsOnly {
					out.note("wave %d pickup spawner %d has no spawn marker (its pickups never appear)", waveIndex, spawner.Index)
				} else {
					out.fail("wave %d spawner index %d has no spawn marker on the map (zombies never spawn)", waveIndex, spawner.Index)
				}
			}
			for _, entry := range spawner.Types {
				if entry.Chance < 0 {
					out.fail("wave %d type %s negative chance", waveIndex, entry.Name)
				}
				lower := strings.ToLower(entry.Name)
				if strings.HasPrefix(lower, "p_") {
					upper := strings.ToUpper(entry.Name)
					_, random := randomPickupGroup(upper)
					_, _, secondary := secondaryForPickup(upper)
					_, weapon := a.weapons.Find(catalogWeaponName(strings.TrimPrefix(upper, "P_")))
					if !random && !secondary && !weapon && upper != "P_SHIELD" && upper != "P_HEALTH" && upper != "P_HOVER" {
						out.fail("pickup %s is not handled by collectPickup", entry.Name)
					}
					continue
				}
				if entry.Name == "train" || isBossType(entry.Name) {
					continue
				}
				if _, known := nativeZombieTypes[entry.Name]; !known {
					out.fail("spawn type %q is not a known zombie type", entry.Name)
				}
				if !reportedType[entry.Texture] && !a.ptZombieTextureOK(entry.Texture) {
					reportedType[entry.Texture] = true
					out.fail("spawn type %s texture %q does not resolve", entry.Name, entry.Texture)
				}
			}
		}
	}
	// A story level has to be able to end: its waves either run out or loop only while a boss is still to be killed
	// (the six boss levels and president_story_2 loop their last waves until the boss dies).
	if !hasLevelFlag(info, "SURVIVAL") {
		loops, boss := false, false
		for waveIndex, wave := range level.Waves {
			next := waveIndex + 1
			if wave.NextWave > 0 {
				next = wave.NextWave
			}
			if next <= waveIndex {
				loops = true
			}
			for _, spawner := range wave.Spawners {
				for _, entry := range spawner.Types {
					if isBossType(entry.Name) {
						boss = true
					}
				}
			}
		}
		if loops && !boss {
			out.fail("story waves loop forever and no boss spawner can end the level")
		}
	}
	// scripts must exist
	paths := []string{entryScriptPath(info)}
	if !hasLevelFlag(info, "SURVIVAL") {
		paths = append(paths, exitScriptPath(info))
	}
	for _, path := range paths {
		if _, err := a.pack.ScriptSource(path); err != nil {
			if path == entryScriptPath(info) && strings.Contains(err.Error(), "not found") {
				out.note("no entry script (%s): level starts directly", path)
				continue
			}
			out.fail("script %s: %v", path, err)
		}
	}
}

var ptCallRE = regexp.MustCompile(`(?:^|[^A-Za-z0-9_.:])([A-Za-z_][A-Za-z0-9_]*)\s*\(`)
var ptDefRE = regexp.MustCompile(`function\s+([A-Za-z_][A-Za-z0-9_]*)`)

var ptLuaBuiltins = map[string]bool{"if": true, "while": true, "for": true, "function": true, "and": true, "or": true, "not": true, "return": true, "elseif": true, "until": true, "repeat": true, "print": true, "type": true, "tostring": true, "tonumber": true, "pairs": true, "ipairs": true, "assert": true, "error": true, "pcall": true, "select": true, "unpack": true, "require": true, "setmetatable": true, "getmetatable": true, "rawget": true, "rawset": true, "next": true, "collectgarbage": true, "loadstring": true, "dofile": true, "then": true, "do": true, "in": true}

// ptScriptCalls returns the host/global names a script source calls but neither defines
// itself nor is registered in scriptCallbacks.
func ptScriptCalls(source string, shared ...string) (used []string, unknown []string) {
	registered := map[string]bool{}
	for _, name := range scriptCallbacks {
		registered[name] = true
	}
	defined := map[string]bool{}
	for _, m := range ptDefRE.FindAllStringSubmatch(source, -1) {
		defined[m[1]] = true
	}
	for _, other := range shared {
		for _, m := range ptDefRE.FindAllStringSubmatch(other, -1) {
			defined[m[1]] = true
		}
	}
	seen := map[string]bool{}
	var stripped []string
	for _, line := range strings.Split(source, "\n") {
		if i := strings.Index(line, "--"); i >= 0 {
			line = line[:i]
		}
		stripped = append(stripped, line)
	}
	for _, m := range ptCallRE.FindAllStringSubmatch(strings.Join(stripped, "\n"), -1) {
		name := m[1]
		if ptLuaBuiltins[name] || defined[name] || seen[name] {
			continue
		}
		seen[name] = true
		used = append(used, name)
		if !registered[name] {
			unknown = append(unknown, name)
		}
	}
	sort.Strings(used)
	sort.Strings(unknown)
	return used, unknown
}

// ptRun plays one level from a fresh play state. When the level was entered through the
// story chain (a.play already is this level) pass reuse=true.
func ptRun(t testing.TB, a *app, cache string, info formats.LevelInfo, opt ptOptions, reuse bool) *ptOutcome {
	out := &ptOutcome{Cache: cache, ID: info.ID, Pass: true, God: opt.god, Mode: "story"}
	survival := hasLevelFlag(info, "SURVIVAL")
	if survival {
		out.Mode = "survival"
	}
	if !reuse || a.play == nil || !strings.EqualFold(a.play.levelInfo.ID, info.ID) {
		a.resultsScreen, a.page = nil, 2
		a.play = nil
		if err := a.selectCaptureLevel(info.ID); err != nil {
			out.fail("select: %v", err)
			return out
		}
		if err := a.openPlay(); err != nil {
			out.fail("openPlay: %v", err)
			return out
		}
	}
	bot := &ptBot{a: a, frame: opt.seed * 53}
	a.inputHook = bot.input
	defer func() { a.inputHook = nil }()
	p := a.play
	if opt.god {
		p.cheats.god = true
	}
	p.cheats.infiniteAmmo = true
	out.WavesLen = len(p.world.Level.Waves)
	tileSize := p.tileSize
	mapW, mapH := float64(p.world.Level.Width*tileSize), float64(p.world.Level.Height*tileSize)
	// spawn point must not be inside a wall
	if p.isSolid(p.x, p.y) && !p.playerUnspawned {
		out.fail("player spawn %.0f,%.0f is inside a solid tile", p.x, p.y)
	}
	if p.world.Zoom != 1 {
		out.fail("level opened with zoom %v", p.world.Zoom)
	}
	missingTex := map[string]bool{}
	lastProgress, lastSig := 0, ""
	seenLines := map[int]bool{}
	stallLimit := 60 * 90
	entryDone := p.scriptRuntime == nil
	if entryDone {
		out.EntryDoneFrame = 0
	}
	maxBosses := map[string]bool{}
	resultsHandled := false
	var prevX, prevY float64
	scriptWasActive := ptScriptActive(p)
	postScriptCheckAt := 0
	lastBossScript := ""
	for frame := 0; frame < opt.maxFrames; frame++ {
		out.Frames = frame
		p = a.play
		if p == nil {
			out.fail("play state vanished at frame %d", frame)
			return out
		}
		if p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue) && frame%25 == 0 {
			p.dialogueIndex++
			p.dialogueAge = 0
		}
		before := a.play
		if len(p.bossScripts) > 0 {
			lastBossScript = p.bossScripts[0]
		}
		prevX, prevY = p.x, p.y
		if err := a.Update(); err != nil {
			if survival || a.play == nil || !strings.EqualFold(before.levelInfo.ID, info.ID) {
				out.fail("update error at frame %d (callback %s): %v", frame, before.scriptLastCallback, err)
				return out
			}
			out.fail("update error at frame %d (callback %s): %v", frame, before.scriptLastCallback, err)
			return out
		}
		// results screen
		if a.page == 5 && a.resultsScreen != nil && !resultsHandled {
			resultsHandled = true
			dead := a.resultsScreen.Data.Dead
			switch {
			case dead:
				out.ResultsOpened = "death"
			case a.resultsScreen.Data.Survival:
				out.ResultsOpened = "survival"
			default:
				out.ResultsOpened = "level-complete"
			}
			out.Seconds = float64(frame) / 60
			out.Waves, out.Kills = p.waveIndex, p.levelKills
			if dead && !survival {
				out.fail("player died (lives exhausted) at %.0fs wave %d/%d", out.Seconds, p.waveIndex, out.WavesLen)
				out.Waves = p.waveIndex
				out.Kills = p.levelKills
				return out
			}
			if survival {
				out.Waves, out.Kills = p.waveIndex, p.levelKills
				return out
			}
			// story completion: press Continue and check where it leads.
			info := p.levelInfo
			var err error
			activated := false
			for i := 0; i < 3000 && a.resultsScreen != nil && err == nil; i++ {
				if !activated {
					activated = a.resultsScreen.activate(resultsContinue)
				}
				err = a.Update()
			}
			if !activated {
				out.fail("results screen never became ready for Continue")
			}
			ptCheckContinue(a, info, out, err)
			return out
		}
		// level changed by continueStoryLevel (no results screen: flags without ENDWORLD)
		if a.play != nil && !strings.EqualFold(a.play.levelInfo.ID, info.ID) {
			out.ResultsOpened = "none"
			out.ExitStarted = frame
			out.Waves, out.Kills = p.waveIndex, p.levelKills
			ptCheckContinue(a, p.levelInfo, out, nil)
			return out
		}
		p = a.play
		// Every script that ends (entry, boss intro / outro) must hand the game back: HUD on, controls restored,
		// no leftover text / fade / alpha. Checked a few frames after the script finishes.
		active := p.scriptRuntime != nil && !p.scriptRuntime.Done()
		if scriptWasActive && !active && !p.exitScriptStarted {
			postScriptCheckAt = frame + 10
		}
		scriptWasActive = active
		if postScriptCheckAt != 0 && frame >= postScriptCheckAt {
			postScriptCheckAt = 0
			if !active && !p.exitScriptStarted && p.health > 0 {
				ptCheckPostScriptState(p, frame, out)
			}
		}
		if !entryDone && (p.scriptRuntime == nil || p.scriptRuntime.Done()) {
			entryDone = true
			out.EntryDoneFrame = frame
			if p.scriptRuntime != nil && p.scriptRuntime.Err() != nil {
				out.fail("entry script error: %v", p.scriptRuntime.Err())
			}
		}
		if p.exitScriptStarted && out.ExitStarted == 0 {
			out.ExitStarted = frame
		}
		// sanity checks
		if math.IsNaN(p.x) || math.IsNaN(p.y) || math.IsInf(p.x, 0) || math.IsInf(p.y, 0) {
			out.fail("player position NaN/Inf at frame %d (previous %.1f,%.1f input %+v health %v)", frame, prevX, prevY, p.input, p.health)
			for index, z := range p.zombies {
				if math.Hypot(z.x-prevX, z.y-prevY) < 90 {
					out.note("near zombie %d: %+v", index, z)
				}
			}
			return out
		}
		if (p.x < -64 || p.y < -64 || p.x > mapW+64 || p.y > mapH+64) && !p.playerUnspawned {
			out.fail("player outside the map (%.0f,%.0f) at frame %d", p.x, p.y, frame)
			return out
		}
		if frame%30 == 0 {
			for _, z := range p.zombies {
				if math.IsNaN(z.x) || math.IsNaN(z.y) || math.IsNaN(z.health) {
					out.fail("zombie NaN at frame %d", frame)
					return out
				}
				if z.health > 0 && !z.spawnAway && (z.x < -200 || z.y < -200 || z.x > mapW+200 || z.y > mapH+200) {
					out.fail("zombie %s outside the map (%.0f,%.0f) at frame %d", p.scriptEntities[z.scriptID].entityTypeOrEmpty(), z.x, z.y, frame)
					return out
				}
				if !missingTex[z.texture] && !a.ptZombieTextureOK(z.texture) {
					missingTex[z.texture] = true
					out.fail("zombie texture %q missing", z.texture)
				}
			}
			for _, entity := range p.scriptEntities {
				if entity != nil && entity.kind != "pickup" && entity.texture != "" && !missingTex["entity:"+entity.texture] && !a.ptZombieTextureOK(entity.texture) {
					missingTex["entity:"+entity.texture] = true
					out.fail("script entity texture %q does not resolve", entity.texture)
				}
			}
			for _, tex := range p.scriptTextures {
				// A loaded slot that is never shown (character sheets loaded for CreateEntity) is harmless.
				if tex.visible && !missingTex[tex.name] && !a.ptTextureOK(tex.name) && !a.ptZombieTextureOK(strings.TrimPrefix(tex.name, "Characters/")) {
					missingTex[tex.name] = true
					out.fail("script texture %q missing", tex.name)
				}
			}
		}
		for _, entity := range p.scriptEntities {
			if entity != nil && strings.HasPrefix(entity.entityType, "boss_") && !maxBosses[entity.entityType] {
				maxBosses[entity.entityType] = true
				out.Bosses = append(out.Bosses, entity.entityType)
			}
		}
		// progress signature
		scriptLine := 0
		if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
			scriptLine = p.scriptRuntime.CurrentLine()
		}
		live := 0
		for _, z := range p.zombies {
			if !z.dying && z.health > 0 {
				live++
			}
		}
		sig := fmt.Sprintf("%d|%d|%d|%d|%d|%v|%v|%v|%d", p.waveIndex, p.levelKills, live, len(p.zombies), p.score, p.wavesFinished, p.exitScriptStarted, scriptLine != 0, p.dialogueIndex)
		if sig != lastSig {
			lastSig, lastProgress = sig, frame
			seenLines = map[int]bool{}
		}
		// A script that keeps revisiting the same few lines (a `while cond do Idle() end` loop) is not progressing.
		if scriptLine != 0 && !seenLines[scriptLine] {
			seenLines[scriptLine] = true
			lastProgress = frame
		}
		if frame-lastProgress > stallLimit {
			out.fail("softlock: no progress for %ds at frame %d (wave %d/%d finished=%v live=%d zombies=%d script=%v cb=%s line=%d lastBossScript=%q exitStarted=%v pos=%.0f,%.0f)", stallLimit/60, frame, p.waveIndex, out.WavesLen, p.wavesFinished, live, len(p.zombies), p.scriptRuntime != nil && !p.scriptRuntime.Done(), p.scriptLastCallback, scriptLine, lastBossScript, p.exitScriptStarted, p.x, p.y)
			out.Waves, out.Kills = p.waveIndex, p.levelKills
			ptDiagnoseStall(a, out)
			return out
		}
	}
	p = a.play
	out.Seconds = float64(out.Frames) / 60
	if p != nil {
		out.Waves, out.Kills = p.waveIndex, p.levelKills
	}
	if survival {
		// Survival never "completes"; capped without a softlock counts as playable.
		out.note("survived the %d-frame cap at wave %d (kills %d)", opt.maxFrames, out.Waves, out.Kills)
		return out
	}
	out.fail("level did not complete within %d frames (%.0fs): wave %d/%d finished=%v zombies=%d exitStarted=%v", opt.maxFrames, out.Seconds, p.waveIndex, out.WavesLen, p.wavesFinished, len(p.zombies), p.exitScriptStarted)
	return out
}

func (e *scriptEntity) entityTypeOrEmpty() string {
	if e == nil {
		return "?"
	}
	return e.entityType
}

// ptCheckContinue verifies that completing info leads to its declared next level.
func ptCheckContinue(a *app, info formats.LevelInfo, out *ptOutcome, err error) {
	if err == nil && a.credits != nil {
		// HD native Continue enters the Credits state first (final level, or a SHOWCREDITS level); leave it the
		// way Back does and check the destination.
		if a.page != creditsPage {
			out.fail("level %s: Credits open but page is %d", info.ID, a.page)
		}
		out.note("HD Credits state entered after %s", info.ID)
		err = a.finishCredits()
	}
	if info.NextLevel == "" {
		// terminal story level: Continue must not fail the frame; it leaves for the main menu
		if err != nil {
			out.fail("final level %s: Continue failed: %v", info.ID, err)
		} else if a.play != nil || a.page != 0 {
			out.fail("final level %s: Continue did not return to the main menu (page %d)", info.ID, a.page)
		}
		out.note("final story level: Continue returns to the main menu (SD: Credits state not decoded; HD: through Credits)")
		return
	}
	if err != nil {
		out.fail("continue to %s: %v", info.NextLevel, err)
		return
	}
	if a.play == nil || !strings.EqualFold(a.play.levelInfo.ID, info.NextLevel) {
		got := "<nil>"
		if a.play != nil {
			got = a.play.levelInfo.ID
		}
		out.fail("continue loaded %s, want %s", got, info.NextLevel)
		return
	}
	out.NextLoaded = a.play.levelInfo.ID
	if !a.unlocked[info.NextLevel] {
		out.fail("next level %s not unlocked after completion", info.NextLevel)
	}
	for _, id := range info.UnlockLevels {
		if !a.unlocked[id] {
			out.fail("level %s not unlocked after completion", id)
		}
	}
	// state that must not leak from the previous level
	p := a.play
	if p.world.Zoom != 1 {
		out.fail("zoom %v leaked into %s", p.world.Zoom, info.NextLevel)
	}
	if !p.hudVisible && p.scriptRuntime == nil {
		out.fail("HUD hidden in %s", info.NextLevel)
	}
	if p.scriptAlpha != 1 && p.scriptRuntime == nil {
		out.fail("script alpha %v leaked into %s", p.scriptAlpha, info.NextLevel)
	}
}

// ptDiagnoseStall steps a few more frames and records what the live zombies and the script are doing.
func ptDiagnoseStall(a *app, out *ptOutcome) {
	p := a.play
	describe := func(label string) {
		for _, z := range p.zombies {
			if z.dying || z.spawnAway || z.health <= 0 {
				continue
			}
			line := fmt.Sprintf("%s zombie %s tex=%s at %.1f,%.1f speed=%.0f player %.1f,%.1f dist=%.0f", label, p.scriptEntities[z.scriptID].entityTypeOrEmpty(), z.texture, z.x, z.y, z.speed, p.x, p.y, math.Hypot(z.x-p.x, z.y-p.y))
			line += fmt.Sprintf(" native state=%d rise=%.0f speedFactor=%.2f rounds=%d kind=%d", z.native.ai.state, z.native.ai.riseMS, z.native.ai.speedFactor, z.native.ai.gun.rounds, z.native.kind)
			if e := p.scriptEntities[z.scriptID]; e != nil && e.walking {
				line += fmt.Sprintf(" walking to %.1f,%.1f range %.0f (distance %.1f)", e.targetX, e.targetY, e.targetRange, math.Hypot(e.targetX-z.x, e.targetY-z.y))
			}
			out.note("%s", line)
		}
	}
	describe("stall")
	for i := 0; i < 3; i++ {
		_ = a.Update()
	}
	describe("+3 frames")
}

// steer returns the unit direction to walk from the player toward (tx, ty), following a tile path
// around walls when the straight line is blocked (a human would not walk into a wall either).
func (b *ptBot) steer(tx, ty float64) (float64, float64) {
	p := b.a.play
	dx, dy := tx-p.x, ty-p.y
	length := math.Hypot(dx, dy)
	if length < 1 {
		return 0, 0
	}
	if !p.segmentBlocked(p.x, p.y, tx, ty) && !p.segmentBlocked(p.x+dy/length*10, p.y-dx/length*10, tx, ty) && !p.segmentBlocked(p.x-dy/length*10, p.y+dx/length*10, tx, ty) {
		return dx / length, dy / length
	}
	ts := float64(p.tileSize)
	goal := [2]int{int(tx / ts), int(ty / ts)}
	if len(b.path) == 0 || b.frame-b.pathAt > 20 || goal != b.pathGoal {
		b.path, b.pathAt, b.pathGoal = ptFindPath(p, [2]int{int(p.x / ts), int(p.y / ts)}, goal), b.frame, goal
	}
	// aim at the farthest path tile that is still in a clear straight line (up to 6 tiles ahead)
	for index := min(len(b.path)-1, 6); index >= 0; index-- {
		wx, wy := (float64(b.path[index][0])+.5)*ts, (float64(b.path[index][1])+.5)*ts
		if index == 0 || !p.segmentBlocked(p.x, p.y, wx, wy) {
			ddx, ddy := wx-p.x, wy-p.y
			if l := math.Hypot(ddx, ddy); l > 1 {
				return ddx / l, ddy / l
			}
			if index+1 < len(b.path) {
				b.path = b.path[index+1:]
			}
		}
	}
	return dx / length, dy / length
}

// ptFindPath is a breadth-first search over passable tiles (8-connected, no corner cutting).
func ptFindPath(p *playState, from, to [2]int) [][2]int {
	w, h := p.world.Level.Width, p.world.Level.Height
	passable := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < w && y < h && !playerCollisionBlocks(p.collisionValue(x, y))
	}
	// the goal may be inside a blocked tile (zombie against a wall): accept its nearest passable neighbour
	if !passable(to[0], to[1]) {
		best, found := to, false
		for r := 1; r <= 3 && !found; r++ {
			for dy := -r; dy <= r && !found; dy++ {
				for dx := -r; dx <= r; dx++ {
					if passable(to[0]+dx, to[1]+dy) {
						best, found = [2]int{to[0] + dx, to[1] + dy}, true
						break
					}
				}
			}
		}
		to = best
	}
	prev := map[[2]int][2]int{from: from}
	queue := [][2]int{from}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		if cur == to {
			break
		}
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			next := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if _, seen := prev[next]; seen || !passable(next[0], next[1]) {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (!passable(cur[0]+d[0], cur[1]) || !passable(cur[0], cur[1]+d[1])) {
				continue
			}
			prev[next] = cur
			queue = append(queue, next)
		}
	}
	if _, ok := prev[to]; !ok {
		// the target sits in an alcove or behind a gate the player cannot enter (the pharaoh's niche in
		// world2_level2): walk to the reachable tile closest to it, which is where a human would stop too
		best, bestDistance := from, math.Inf(1)
		for tile := range prev {
			if d := math.Hypot(float64(tile[0]-to[0]), float64(tile[1]-to[1])); d < bestDistance || (d == bestDistance && (tile[1] < best[1] || (tile[1] == best[1] && tile[0] < best[0]))) {
				best, bestDistance = tile, d
			}
		}
		to = best
	}
	var path [][2]int
	for cur := to; cur != from; cur = prev[cur] {
		path = append(path, cur)
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	return path
}

// ptDeathFlow loses every life on a fresh start of the level and checks the game-over path: story shows the
// Dead results (Main Menu / Retry), survival its results screen; Retry reopens the same level with Barry alive and
// Main Menu leaves the play state.
func ptDeathFlow(a *app, info formats.LevelInfo) []string {
	var issues []string
	survival := hasLevelFlag(info, "SURVIVAL")
	open := func() bool {
		a.resultsScreen, a.page, a.play = nil, 2, nil
		if err := a.selectCaptureLevel(info.ID); err != nil {
			issues = append(issues, fmt.Sprintf("death flow select: %v", err))
			return false
		}
		if err := a.openPlay(); err != nil {
			issues = append(issues, fmt.Sprintf("death flow open: %v", err))
			return false
		}
		a.play.closeScript()
		a.play.dialogueIndex = len(a.play.dialogue)
		a.play.moveControl, a.play.shootControl = true, true
		return true
	}
	kill := func(label string) bool {
		a.play.health, a.play.lives = 0, 0
		for frame := 0; frame < 1800; frame++ {
			if err := a.Update(); err != nil {
				issues = append(issues, fmt.Sprintf("%s: update error %v", label, err))
				return false
			}
			if a.page == 5 && a.resultsScreen != nil {
				data := a.resultsScreen.Data
				if survival && !data.Survival && !data.Dead {
					issues = append(issues, fmt.Sprintf("%s: survival death opened level-complete results", label))
				}
				if !survival && !data.Dead {
					issues = append(issues, fmt.Sprintf("%s: story death results are not the Dead variant", label))
				}
				return true
			}
		}
		issues = append(issues, fmt.Sprintf("%s: results never opened after the last life was lost", label))
		return false
	}
	press := func(label string, action resultsAction) bool {
		for frame := 0; frame < 3000; frame++ {
			if a.resultsScreen != nil && a.resultsScreen.activate(action) {
				break
			}
			if err := a.Update(); err != nil {
				issues = append(issues, fmt.Sprintf("%s: update error %v", label, err))
				return false
			}
		}
		for frame := 0; frame < 3000 && a.resultsScreen != nil; frame++ {
			if err := a.Update(); err != nil {
				issues = append(issues, fmt.Sprintf("%s: update error %v", label, err))
				return false
			}
		}
		if a.resultsScreen != nil {
			issues = append(issues, fmt.Sprintf("%s: results screen never closed", label))
			return false
		}
		return true
	}
	if !open() || !kill("death") || !press("retry", resultsReplay) {
		return issues
	}
	if a.play == nil || !strings.EqualFold(a.play.levelInfo.ID, info.ID) || a.play.health <= 0 {
		issues = append(issues, "retry did not reopen the level with Barry alive")
		return issues
	}
	a.play.closeScript()
	if !kill("second death") || !press("main menu", resultsMainMenu) {
		return issues
	}
	if a.play != nil || a.page != 0 {
		issues = append(issues, fmt.Sprintf("Main Menu left page %d / play %v", a.page, a.play != nil))
	}
	return issues
}

func ptScriptActive(p *playState) bool {
	return p != nil && p.scriptRuntime != nil && !p.scriptRuntime.Done()
}

// ptCheckPostScriptState reports state a finished script left behind that would break normal play.
func ptCheckPostScriptState(p *playState, frame int, out *ptOutcome) {
	var left []string
	if !p.hudVisible {
		left = append(left, "HUD hidden")
	}
	if !p.moveControl || !p.shootControl {
		left = append(left, fmt.Sprintf("controls off (move=%v shoot=%v)", p.moveControl, p.shootControl))
	}
	if p.scriptTextVisible {
		left = append(left, "script text still shown")
	}
	for id, texture := range p.scriptTextures {
		if texture.visible {
			left = append(left, fmt.Sprintf("script texture %d (%s) still visible", id, texture.name))
		}
	}
	if p.scriptFadeBlack {
		left = append(left, "screen left faded to black")
	}
	if p.scriptAlpha != 1 {
		left = append(left, fmt.Sprintf("script alpha %v", p.scriptAlpha))
	}
	if p.scriptShowSkip {
		left = append(left, "skip prompt still shown")
	}
	if p.scriptPaused {
		left = append(left, "game still paused by PauseGame")
	}
	if p.playerUnspawned {
		left = append(left, "Barry never spawned")
	}
	if p.world.Zoom != 1 {
		out.note("zoom %.2f after a script at frame %d", p.world.Zoom, frame)
	}
	if len(left) > 0 {
		out.fail("script left the game in a bad state at frame %d: %s", frame, strings.Join(left, ", "))
	}
}

// ptExitFuzz plays one level's entry script, then replaces the end state by a random one (boss corpse and Barry on random
// open tiles, every zombie dead, waves finished) and runs the exit script, reporting what the script is waiting on if it never
// finishes. Scripts that chase positions (WalkTo helpers, IsPlayerWalking / IsZombieWalking loops) behave differently for every
// boss death position, which a single natural playthrough cannot cover.
func ptExitFuzz(a *app, info formats.LevelInfo, sample int) string {
	a.resultsScreen, a.play, a.page = nil, nil, 2
	for draw := 0; draw < sample*131+7; draw++ {
		a.rng.Bounded(1 << 30)
	}
	if err := a.selectCaptureLevel(info.ID); err != nil {
		return fmt.Sprintf("select: %v", err)
	}
	if err := a.openPlay(); err != nil {
		return fmt.Sprintf("open: %v", err)
	}
	p := a.play
	bot := &ptBot{a: a, frame: sample * 53}
	a.inputHook = bot.input
	defer func() { a.inputHook = nil }()
	frame := 0
	step := func() error {
		if p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue) && frame%25 == 0 {
			p.dialogueIndex++
		}
		frame++
		return a.Update()
	}
	for p.scriptRuntime != nil && !p.scriptRuntime.Done() && frame < 8000 {
		if err := step(); err != nil {
			return fmt.Sprintf("entry script: %v", err)
		}
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		return "entry script never finished"
	}
	level := p.world.Level
	var open [][2]float64
	reachable := ptReachableTiles(p, p.x, p.y)
	for y := 2; y < level.Height-2; y++ {
		for x := 2; x < level.Width-2; x++ {
			if !reachable[[2]int{x, y}] {
				continue
			}
			free := true
			for dy := -1; dy <= 1 && free; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if playerCollisionBlocks(p.collisionValue(x+dx, y+dy)) {
						free = false
						break
					}
				}
			}
			if free {
				open = append(open, [2]float64{(float64(x) + .1 + .8*float64(a.rng.Bounded(1000))/1000) * 32, (float64(y) + .1 + .8*float64(a.rng.Bounded(1000))/1000) * 32})
			}
		}
	}
	if len(open) == 0 {
		return "no open tile"
	}
	pick := func() [2]float64 { return open[int(a.rng.Bounded(uint32(len(open))))] }
	bossType, bossTexture := "", ""
	for _, wave := range level.Waves {
		for _, spawner := range wave.Spawners {
			for _, entry := range spawner.Types {
				if isBossType(entry.Name) && bossType == "" {
					bossType, bossTexture = entry.Name, entry.Texture
				}
			}
		}
	}
	found := false
	for index := range p.zombies {
		if entity := p.scriptEntities[p.zombies[index].scriptID]; entity != nil && strings.HasPrefix(entity.entityType, "boss_") {
			found = true
			at := pick()
			p.zombies[index].x, p.zombies[index].y = at[0], at[1]
			entity.x, entity.y = at[0], at[1]
		}
	}
	if !found && bossType != "" {
		at := pick()
		p.spawnZombieAt(formats.SpawnType{Name: bossType, Chance: 1, Speed: formats.Vec2{X: 90, Y: 90}, Strength: 20000, Size: formats.Vec2{X: 60, Y: 60}, Texture: bossTexture}, formats.Vec2{X: at[0], Y: at[1]})
		p.bossScripts = nil
	}
	for index := range p.zombies {
		p.zombies[index].health, p.zombies[index].dying, p.zombies[index].deathAge = 0, true, 10
	}
	at := pick()
	p.x, p.y = at[0], at[1]
	p.waveIndex, p.wavesFinished = len(level.Waves), true
	p.bossDefeated = bossType != ""
	for end := frame + 5000; frame < end; {
		if err := step(); err != nil {
			return fmt.Sprintf("exit: %v", err)
		}
		if a.page == 5 || a.play == nil || !strings.EqualFold(a.play.levelInfo.ID, info.ID) {
			return ""
		}
	}
	line := 0
	if p.scriptRuntime != nil {
		line = p.scriptRuntime.CurrentLine()
	}
	bossAt := "none"
	for _, entity := range p.scriptEntities {
		if entity != nil && strings.HasPrefix(entity.entityType, "boss_") {
			bossAt = fmt.Sprintf("%.4f,%.4f", entity.x, entity.y)
		}
	}
	stuck := fmt.Sprintf("exit script stuck: player %.4f,%.4f boss %s callback %s line %d exitStarted=%v", p.x, p.y, bossAt, p.scriptLastCallback, line, p.exitScriptStarted)
	for _, z := range p.zombies {
		if z.dying || z.health <= 0 || z.spawnAway {
			continue
		}
		stuck += fmt.Sprintf("; zombie %s at %.2f,%.2f speed %.0f", z.texture, z.x, z.y, z.speed)
		if e := p.scriptEntities[z.scriptID]; e != nil && e.walking {
			stuck += fmt.Sprintf(" walking to %.2f,%.2f range %.0f (distance %.2f)", e.targetX, e.targetY, e.targetRange, math.Hypot(e.targetX-z.x, e.targetY-z.y))
		}
	}
	return stuck
}

// ptBossIntroFuzz spawns the level's wave boss (the ones whose intro lives in a separate boss script: rex, egyptian, samurai) on a
// random open tile with Barry somewhere else, lets the bot play, and reports an intro script that never finishes. The intros wait for
// the player to come into range of the boss, so they depend on where both stand.
var ptFuzzTrace = os.Getenv("AOZ_FUZZ_TRACE") != "" // prints the boss intro fuzz progress (debugging aid)
var ptFuzzTraceEvery = func() int {
	if value, err := strconv.Atoi(os.Getenv("AOZ_FUZZ_TRACE")); err == nil && value > 1 {
		return value
	}
	return 300
}()

func ptBossIntroFuzz(a *app, info formats.LevelInfo, sample int) string {
	a.resultsScreen, a.play, a.page = nil, nil, 2
	for draw := 0; draw < sample*173+11; draw++ {
		a.rng.Bounded(1 << 30)
	}
	if err := a.selectCaptureLevel(info.ID); err != nil {
		return fmt.Sprintf("select: %v", err)
	}
	if err := a.openPlay(); err != nil {
		return fmt.Sprintf("open: %v", err)
	}
	p := a.play
	p.cheats.god, p.cheats.infiniteAmmo = true, true
	bot := &ptBot{a: a, frame: sample * 53}
	a.inputHook = bot.input
	defer func() { a.inputHook = nil }()
	frame := 0
	step := func() error {
		if p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue) && frame%25 == 0 {
			p.dialogueIndex++
		}
		frame++
		return a.Update()
	}
	for p.scriptRuntime != nil && !p.scriptRuntime.Done() && frame < 8000 {
		if err := step(); err != nil {
			return fmt.Sprintf("entry script: %v", err)
		}
	}
	bossType, bossTexture := "", ""
	for _, wave := range p.world.Level.Waves {
		for _, spawner := range wave.Spawners {
			for _, entry := range spawner.Types {
				if bossIntroScript(entry.Name, entry.Texture) != "" && bossType == "" {
					bossType, bossTexture = entry.Name, entry.Texture
				}
			}
		}
	}
	if bossType == "" {
		return ""
	}
	var open [][2]float64
	reachable := ptReachableTiles(p, p.x, p.y)
	for y := 2; y < p.world.Level.Height-2; y++ {
		for x := 2; x < p.world.Level.Width-2; x++ {
			if reachable[[2]int{x, y}] && !playerCollisionBlocks(p.collisionValue(x, y)) && !playerCollisionBlocks(p.collisionValue(x+1, y)) && !playerCollisionBlocks(p.collisionValue(x, y+1)) && !playerCollisionBlocks(p.collisionValue(x-1, y)) && !playerCollisionBlocks(p.collisionValue(x, y-1)) {
				open = append(open, [2]float64{(float64(x) + .5) * 32, (float64(y) + .5) * 32})
			}
		}
	}
	pick := func() [2]float64 { return open[int(a.rng.Bounded(uint32(len(open))))] }
	bossAt, playerAt := pick(), pick()
	p.x, p.y = playerAt[0], playerAt[1]
	p.spawnZombieAt(formats.SpawnType{Name: bossType, Chance: 1, Speed: formats.Vec2{X: 90, Y: 90}, Strength: 25000, Size: formats.Vec2{X: 80, Y: 80}, Texture: bossTexture}, formats.Vec2{X: bossAt[0], Y: bossAt[1]})
	ran := false
	for end := frame + 9000; frame < end; {
		if err := step(); err != nil {
			return fmt.Sprintf("boss intro: %v", err)
		}
		if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
			ran = true
		}
		if ptFuzzTrace && frame%ptFuzzTraceEvery == 0 {
			boss := "gone"
			for _, z := range p.zombies {
				if entity := p.scriptEntities[z.scriptID]; entity != nil && entity.entityType == bossType {
					boss = fmt.Sprintf("%.0f,%.0f hp=%.0f speed=%.0f", z.x, z.y, z.health, z.speed)
				}
			}
			line := 0
			if p.scriptRuntime != nil {
				line = p.scriptRuntime.CurrentLine()
			}
			fmt.Printf("  trace f%d player %.0f,%.0f move=%v boss %s script=%v line %d bossScripts=%v cb=%s botIn=%.2f,%.2f stuck=%d detour=%d path=%d\n", frame, p.x, p.y, p.moveControl, boss, p.scriptRuntime != nil && !p.scriptRuntime.Done(), line, p.bossScripts, p.scriptLastCallback, bot.last.moveX, bot.last.moveY, bot.stuck, bot.detour, len(bot.path))
		}
		if ran && len(p.bossScripts) == 0 && (p.scriptRuntime == nil || p.scriptRuntime.Done()) {
			return ""
		}
	}
	return fmt.Sprintf("boss intro never finished: boss %s at %.0f,%.0f, player %.0f,%.0f now %.1f,%.1f, callback %s", bossType, bossAt[0], bossAt[1], playerAt[0], playerAt[1], p.x, p.y, p.scriptLastCallback)
}

// ptReachableTiles floods the open tiles Barry can walk to from the start tile (8-connected, no corner cutting).
func ptReachableTiles(p *playState, startX, startY float64) map[[2]int]bool {
	w, h := p.world.Level.Width, p.world.Level.Height
	passable := func(x, y int) bool {
		return x >= 0 && y >= 0 && x < w && y < h && !playerCollisionBlocks(p.collisionValue(x, y))
	}
	start := [2]int{int(startX / 32), int(startY / 32)}
	seen := map[[2]int]bool{start: true}
	queue := [][2]int{start}
	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		for _, d := range [][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
			next := [2]int{cur[0] + d[0], cur[1] + d[1]}
			if seen[next] || !passable(next[0], next[1]) {
				continue
			}
			if d[0] != 0 && d[1] != 0 && (!passable(cur[0]+d[0], cur[1]) || !passable(cur[0], cur[1]+d[1])) {
				continue
			}
			seen[next] = true
			queue = append(queue, next)
		}
	}
	return seen
}
