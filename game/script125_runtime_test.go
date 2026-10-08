package game

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"math"
	"strings"
	"testing"
)

func script125CachedHost(t *testing.T, base string) *playScriptHost {
	t.Helper()
	pack, err := content.NewPack(content.NewSource("bin/data-cache"))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{pack: pack, silent: true, images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}}
	a.weapons, err = pack.Weapons()
	if err != nil {
		t.Fatal(err)
	}
	a.zombieWeapons, err = pack.ZombieWeapons()
	if err != nil {
		t.Fatal(err)
	}
	a.sprites, err = pack.Sprites()
	if err != nil {
		t.Fatal(err)
	}
	for _, info := range pack.List() {
		if !strings.EqualFold(info.BaseFile, base) {
			continue
		}
		level, err := pack.Load(info.ID)
		if err != nil {
			t.Fatal(err)
		}
		tileset, ok := pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
		if !ok {
			t.Fatalf("missing tileset %s", level.Tileset)
		}
		atlas, err := a.Texture(tileset.Texture)
		if err != nil {
			t.Fatal(err)
		}
		tileSize := tileSizeFor(tileset)
		x, y := spawnPosition(level, tileSize)
		rng := weapons.NewNativeRNG()
		p := &playState{world: viewer.New(level, tileset, atlas, a), levelInfo: info, x: x, y: y, spawnX: x, spawnY: y, radius: playerCollisionRadius, tileSize: tileSize, health: 1, maxHealth: 1, lives: 3, multiplier: 1, weapons: a.weapons, sprites: a.sprites, rng: &rng, scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}, scriptTextures: map[int]*scriptTexture{}, scriptAlpha: 1, hudVisible: true}
		a.play = p
		h := &playScriptHost{app: a, play: p}
		source, err := pack.ScriptSource(entryScriptPath(info))
		if err == nil {
			p.scriptRuntime, err = scripting.New(source, h, scriptCallbacks)
		}
		if err != nil && !strings.Contains(err.Error(), "not found") {
			t.Fatal(err)
		}
		t.Cleanup(p.closeScript)
		return h
	}
	t.Fatalf("missing cached level %s", base)
	return nil
}
func TestScript125World0Level2EntryCompletes(t *testing.T) {
	h := script125CachedHost(t, "world0_level2")
	p := h.play
	if err := p.updateScript(); err != nil {
		t.Fatal(err)
	}
	if p.scriptRuntime.Status() != scripting.StatusSuspended || !p.scriptWaitActive || p.scriptWaitRemaining != 50 {
		t.Fatalf("entry did not reach its real initial wait: status=%v wait=%v", p.scriptRuntime.Status(), p.scriptWaitRemaining)
	}
	for frame := 0; frame < 600 && !p.scriptRuntime.Done(); frame++ {
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
		p.updateCamera()
	}
	if p.scriptRuntime.Status() != scripting.StatusComplete || p.scriptWaitStarts != 4 || !p.hudVisible || p.scriptTextVisible || len(p.scriptTextures) != 0 || !p.enterAfterScript() || !p.moveControl || !p.shootControl {
		t.Fatalf("entry did not complete: status=%v waits=%d last=%s", p.scriptRuntime.Status(), p.scriptWaitStarts, p.scriptLastCallback)
	}
}
func TestScript125TutorialEntryReachesRealSpeech(t *testing.T) {
	h := script125CachedHost(t, "world0_level0")
	p := h.play
	for frame := 0; frame < 1500 && len(p.dialogue) == 0; frame++ {
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
		p.updateCamera()
	}
	if len(p.dialogue) == 0 || p.scriptRuntime.Status() != scripting.StatusSuspended || len(h.analyticsEvents) != 1 || p.scriptLevelToLoad != "lab" {
		t.Fatalf("tutorial did not reach real speech: status=%v last=%s position=%v,%v", p.scriptRuntime.Status(), p.scriptLastCallback, p.x, p.y)
	}
}
func TestScript125OfflineAnalyticsPayload(t *testing.T) {
	h := &playScriptHost{play: &playState{levelInfo: formats.LevelInfo{ID: "World0Level0"}}}
	for _, event := range []string{"tutorialStart", "tutorialMoveShoot", "tutorialSecondary", "tutorialFinish"} {
		if _, err := h.Call("Analytics_SendEvent", []scripting.Value{event}); err != nil {
			t.Fatal(err)
		}
	}
	if len(h.analyticsEvents) != 4 {
		t.Fatalf("events=%v", h.analyticsEvents)
	}
	data, err := json.Marshal(h.analyticsEvents[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["event"] != "tutorialStart" || payload["PLAYER_COUNT"] != float64(1) || payload["level"] != "World0Level0" {
		t.Fatalf("payload=%s", data)
	}
	for _, args := range [][]scripting.Value{nil, {1}, {false}, {"event", "extra"}} {
		if _, err := h.Call("Analytics_SendEvent", args); err == nil || len(h.analyticsEvents) != 4 {
			t.Fatalf("invalid analytics args=%v error=%v", args, err)
		}
	}
}
func TestScript125SecondaryWaitOnlyChangesSecondaryState(t *testing.T) {
	h := &playScriptHost{play: &playState{moveControl: true, shootControl: true}}
	for _, wait := range []bool{true, false} {
		if _, err := h.Call("SecondaryPlayersPleaseWait", []scripting.Value{wait}); err != nil {
			t.Fatal(err)
		}
		if h.secondaryPlayersWait != wait || !h.play.moveControl || !h.play.shootControl {
			t.Fatal("secondary wait changed the sole primary player controls")
		}
	}
	if _, err := h.Call("SecondaryPlayersPleaseWait", []scripting.Value{"true"}); err == nil {
		t.Fatal("accepted invalid wait flag")
	}
}
func TestScript125ParentHooksReportRequiredIntegration(t *testing.T) {
	h := &playScriptHost{play: &playState{}}
	for _, call := range []struct {
		name string
		args []scripting.Value
	}{
		{"BrickUI_DisplayScreen", []scripting.Value{"Tutorial_Controller"}},
		{"BrickUI_PlayAnimation", []scripting.Value{"Tutorial_Controller", "AnimateIn"}},
		{"BrickUI_GetPropertyBool", []scripting.Value{"Tutorial_Controller", "IsDisplayed", true}},
		{"BrickUI_RemoveScreen", []scripting.Value{"Tutorial_Controller"}},
		{"BrickUI_SetupTutorialScreen", nil},
		{"Controller_OnEnterScreen", []scripting.Value{"Tutorial_Controller"}},
		{"Controller_OnLeaveScreen", []scripting.Value{"Tutorial_Controller"}},
	} {
		result, err := h.Call(call.name, call.args)
		if err == nil || !strings.Contains(err.Error(), "requires the parent") || len(result.Values) != 0 {
			t.Fatalf("%s invented a result: %+v, %v", call.name, result, err)
		}
	}
	for _, value := range []scripting.Value{nil, true, "1", .5, math.NaN(), math.Inf(1)} {
		if _, err := h.Call("SetZombiesActiveDuringScripts", []scripting.Value{value}); err == nil || strings.Contains(err.Error(), "requires the parent") {
			t.Fatalf("invalid activity=%v error=%v", value, err)
		}
	}
}
func TestScript125UIGetterDoesNotFabricateLuaFallback(t *testing.T) {
	h := &playScriptHost{play: &playState{}}
	runtime, err := scripting.New(`BrickUI_GetPropertyBool("Tutorial_Controller", "IsDisplayed", true)`, h, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	err = runtime.Step()
	if err == nil || !strings.Contains(err.Error(), "requires the parent BrickUI") || strings.Contains(err.Error(), "non-function") {
		t.Fatalf("missing UI host produced %v", err)
	}
}

func TestScript125EntryGuardsWalkIntoPortalAndSpawnAway(t *testing.T) {
	h := script125CachedHost(t, "world0_level0")
	p := h.play
	for frame := 0; frame < 700; frame++ {
		if p.dialogueIndex < len(p.dialogue) && frame%120 == 0 {
			p.dialogueIndex++
		}
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
		p.updateZombies()
	}
	live := 0
	for _, zombie := range p.zombies {
		if zombie.health > 0 && !zombie.spawnAway {
			live++
		}
	}
	if live != 0 || len(p.zombies) > 1 {
		t.Fatalf("entry guards left in the lab: live=%d total=%d, want all sent through the portal", live, len(p.zombies))
	}
}

func TestScript125ZombieQueriesReturnBooleansAndTolerateNilHandles(t *testing.T) {
	h := script125CachedHost(t, "world0_level0")
	for _, name := range []string{"ZombieExists", "IsZombieWalking"} {
		result, err := h.Call(name, []scripting.Value{nil})
		if err != nil || len(result.Values) != 1 || result.Values[0] != false {
			t.Fatalf("%s(nil) = %v, %v; want false", name, result.Values, err)
		}
	}
	for _, name := range []string{"WalkZombieTo", "SpawnAwayZombie"} {
		if _, err := h.Call(name, []scripting.Value{nil, 1.0, 2.0, 3.0}); err != nil {
			t.Fatalf("%s(nil) native no-op returned %v", name, err)
		}
	}
}

func TestScript125TutorialBarryHiddenOnlyUntilPortalSpawn(t *testing.T) {
	h := script125CachedHost(t, "world0_level0")
	p := h.play
	sawLabVisible, sawHidden, sawSpawned := false, false, false
	for frame := 0; frame < 2400 && !sawSpawned; frame++ {
		if p.dialogueIndex < len(p.dialogue) && frame%120 == 0 {
			p.dialogueIndex++
		}
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
		switch {
		case p.scriptLevelToLoad == "lab" && !p.playerUnspawned:
			sawLabVisible = true
		case p.scriptLevelToLoad == "world0_level0" && p.playerUnspawned:
			sawHidden = true
		case p.scriptLevelToLoad == "world0_level0" && sawHidden && !p.playerUnspawned:
			sawSpawned = true
		}
	}
	if !sawLabVisible || !sawHidden || !sawSpawned {
		t.Fatalf("lab visible=%v hidden before portal spawn=%v spawned=%v", sawLabVisible, sawHidden, sawSpawned)
	}
}
