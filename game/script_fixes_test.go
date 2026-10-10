package game

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
)

// Regression tests for the script / level-flow bugs the headless playthrough (playthrough_test.go) found. Each one
// quotes the native evidence (libmortargame.so 1.2.5 unless noted) next to the behaviour it pins.

// The position getters push 0 and the position setters skip their store for an id with no entity (closures at
// 0x0013d6b0, 0x0013d730, 0x0013d7b0, 0x0013d830). world1_level2_entry keeps calling its car updater after
// DestroyEntity(sprite), and its Wait() helper runs that updater.
func TestEntityPositionCallbacksTolerateDestroyedEntities(t *testing.T) {
	play := &playState{scriptEntities: map[int]*scriptEntity{}, scriptNextEntity: 1}
	host := &playScriptHost{play: play}
	for _, name := range []string{"GetSpriteXPosition", "GetSpriteYPosition", "GetEntityXPos", "GetEntityYPos"} {
		result, err := host.Call(name, []scripting.Value{7.0})
		if err != nil || len(result.Values) != 1 || result.Values[0] != 0.0 {
			t.Fatalf("%s(destroyed) = %v, %v; want 0", name, result.Values, err)
		}
	}
	for name, args := range map[string][]scripting.Value{"SetSpriteXPosition": {7.0, 5.0}, "SetSpriteYPosition": {7.0, 5.0}, "SetEntityPos": {7.0, 1.0, 2.0}} {
		if result, err := host.Call(name, args); err != nil || len(result.Values) != 0 {
			t.Fatalf("%s(destroyed) = %v, %v; want a silent no-op", name, result.Values, err)
		}
	}
	// the same holds for the zombie callbacks: a boss intro script calls SetZombieSpeed(boss, speed) even when the
	// player already shot the boss dead, and GetFirstEntityOfType pushes the null handle 0 when nothing matches
	for _, call := range []struct {
		name string
		args []scripting.Value
	}{{"SetZombieSpeed", []scripting.Value{7.0, 90.0}}, {"SetZombieAlpha", []scripting.Value{7.0, 0.5}}, {"MakeZombieInvulnerable", []scripting.Value{7.0, true}}, {"WalkZombieTo", []scripting.Value{7.0, 1.0, 2.0, 3.0}}, {"KillZombie", []scripting.Value{7.0}}, {"SetEntityRotation", []scripting.Value{7.0, 90.0}}, {"DestroyEntity", []scripting.Value{7.0}}} {
		if result, err := host.Call(call.name, call.args); err != nil || len(result.Values) != 0 {
			t.Fatalf("%s(stale handle) = %v, %v; want a silent no-op", call.name, result.Values, err)
		}
	}
	if result, err := host.Call("GetZombieSpeed", []scripting.Value{7.0}); err != nil || result.Values[0] != 0.0 {
		t.Fatalf("GetZombieSpeed(stale handle) = %v, %v", result.Values, err)
	}
	if result, err := host.Call("GetFirstEntityOfType", []scripting.Value{"boss_rex"}); err != nil || len(result.Values) != 1 || result.Values[0] != 0 {
		t.Fatalf("GetFirstEntityOfType with no match = %v, %v; want the null handle 0", result.Values, err)
	}
	if len(play.scriptEntities) != 0 {
		t.Fatalf("a setter on a destroyed id created an entity: %v", play.scriptEntities)
	}
	play.scriptEntities[3] = &scriptEntity{id: 3, kind: "sprite", x: 10, y: 20}
	if result, err := host.Call("GetSpriteXPosition", []scripting.Value{3.0}); err != nil || result.Values[0] != 10.0 {
		t.Fatalf("GetSpriteXPosition of a live entity = %v, %v", result.Values, err)
	}
}

// Native GetCurrentDialogSpeech (0x0013ca6c) returns -1 while no speech is active; world5_level1_exit loops
// `while GetCurrentDialogSpeech() >= 1` inside its speech loop and never left it with the finished index.
func TestGetCurrentDialogSpeechIsMinusOneWhenNoSpeechRuns(t *testing.T) {
	play := &playState{dialogue: []dialogueLine{{text: "a"}, {text: "b"}}}
	host := &playScriptHost{play: play}
	for index, want := range []any{0, 1, -1} {
		play.dialogueIndex = index
		result, err := host.Call("GetCurrentDialogSpeech", nil)
		if err != nil || len(result.Values) != 1 || result.Values[0] != want {
			t.Fatalf("index %d: GetCurrentDialogSpeech = %v, %v; want %v", index, result.Values, err, want)
		}
	}
}

// The exit scripts step a sprite toward a target with a WalkTo helper that finishes on exact equality
// (`return ((ypos == y) and (xpos == x))`) and moves AWAY from a target to its left once less than `speed` is
// left (the helper's `dx < 0` branch uses +ax). With arbitrary fractional coordinates the remaining distance
// then falls into a permanent 2-cycle (-3.99999999999994 / -1.99999999999994, a rounding residue just below the
// speed) and the cutscene never ends; single precision sprite storage (the native getters / setters) breaks it
// for about 7 % of the boss death positions of world2_level2_exit. Coordinates handed to scripts are therefore
// snapped to a binary grid, which keeps all of the helper's arithmetic exact so the doubling pattern of the
// wrong-sign branch always ends on exactly zero.
func TestScriptWalkToHelperTerminatesForAnyTarget(t *testing.T) {
	const helper = `
function ABS(v) if v < 0 then return -v end return v end
function WalkTo(s, x, y, speed)
  local xpos = GetSpriteXPosition(s)
  local ypos = GetSpriteYPosition(s)
  local dx, dy = x - xpos, y - ypos
  local ax, ay = ABS(dx), ABS(dy)
  local speedX, speedY
  if dy < 0 then if ay < speed then speedY = ay else speedY = -speed end else if ay < speed then speedY = ay else speedY = speed end end
  if dx < 0 then if ax < speed then speedX = ax else speedX = -speed end else if ax < speed then speedX = ax else speedX = speed end end
  xpos = xpos + speedX
  ypos = ypos + speedY
  SetSpriteXPosition(s, xpos)
  SetSpriteYPosition(s, ypos)
  return ((ypos == y) and (xpos == x))
end
Mummy = GetFirstEntityOfType("boss_egyptian")
MummyPosX = GetEntityXPos(Mummy)
MummyPosY = GetEntityYPos(Mummy)
sprite = CreateEntity(0, 0, "Characters/egyptprince", 5, 4, true)
SetSpriteXPosition(sprite, MummyPosX + 350)
SetSpriteYPosition(sprite, MummyPosY + 24)
while WalkTo(sprite, MummyPosX + 25, MummyPosY + 24, 2) == false do Idle() end
arrived = true
`
	for _, boss := range [][2]float64{{491.9200134277344, 480}, {498.7699890136719, 525.5}, {500.1400146484375, 507.3}, {300.3, 300.7}, {1000.01, 40.5}, {485.07, 480}, {485.07, 489.1}, {333.33, 111.11}} {
		play := &playState{scriptEntities: map[int]*scriptEntity{7: {id: 7, kind: "zombie", entityType: "boss_egyptian", x: boss[0], y: boss[1]}}, scriptNextEntity: 8}
		runtime, err := scripting.New(helper, &playScriptHost{play: play}, scriptCallbacks)
		if err != nil {
			t.Fatal(err)
		}
		steps := 0
		for ; steps < 3000 && !runtime.Done(); steps++ {
			if err := runtime.Step(); err != nil {
				t.Fatal(err)
			}
		}
		runtime.Close()
		if steps >= 3000 {
			t.Fatalf("WalkTo toward boss %.4f,%.4f never finished", boss[0], boss[1])
		}
	}
}

// 1.2.5 Player::Update (FUN_000f3280): with the walk flag set the position advances straight along the target
// vector and the tile collision loop (FUN_0012122c) is skipped, so a scripted walk ends even when the target lies
// inside a wall. Level 2 of the 1.2.1 data starts Barry at a wall edge and walks him 64 px into it.
func TestScriptedPlayerWalkIgnoresTilesAndIsNotPushedBack(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 5 })
	p.x, p.y, p.radius = 3*32+16, 4*32+16, playerCollisionRadius
	host := &playScriptHost{play: p}
	targetX := 5*32 + 16.0 // the middle of the wall column
	if _, err := host.Call("WalkPlayerTo", []scripting.Value{targetX, p.y}); err != nil {
		t.Fatal(err)
	}
	farthest := p.x
	for frame := 0; frame < 120 && p.scriptWalking; frame++ {
		p.updateScriptWalk()
		farthest = math.Max(farthest, p.x)
		p.stepBody(0, 0, false, false, true)
	}
	if p.scriptWalking {
		t.Fatalf("the scripted walk never arrived: player at %.1f,%.1f", p.x, p.y)
	}
	if farthest < 5*32 {
		t.Fatalf("the walk stopped at x=%.1f in front of the wall column starting at 160", farthest)
	}
	// once the walk flag is clear the normal tile push moves Barry out of the wall again
	p.stepBody(0, 0, false, false, true)
	if p.isSolid(p.x, p.y) {
		t.Fatalf("Barry was left inside the wall at %.1f,%.1f", p.x, p.y)
	}
}

// FUN_000bf120 drops a spawner whose count is below 1 right after its first FUN_000bec48 call, and that call still
// spawns when the delay timer is already due: president_story_2's boss (delay 0, count 0) must appear, a delayed
// count 0 spawner never does.
func TestCountZeroSpawnerSpawnsOnceWhenDue(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.world.Level.Layers[formats.LayerC][4*16+4] = 3 // spawn marker of spawner index 1
	pickup := []formats.SpawnType{{Name: "p_shotgun", Chance: 1}}
	p.world.Level.Waves = []formats.Wave{{RunTime: 5000, EndWaveTime: 5000, Spawners: []formats.Spawner{
		{Index: 1, Count: 0, DelayTime: 0, Types: pickup},
		{Index: 1, Count: 0, DelayTime: 5000, Types: pickup},
	}}}
	for frame := 0; frame < 30; frame++ {
		p.updateWaves()
	}
	pickups := 0
	for _, entity := range p.scriptEntities {
		if entity.kind == "pickup" {
			pickups++
		}
	}
	if pickups != 1 {
		t.Fatalf("%d pickups spawned, want exactly the one of the undelayed count 0 spawner", pickups)
	}
}

// FUN_001013c4 ends a zombie walk when the squared distance is below the squared range and otherwise steps the full
// speed * dt. The port clamped the step to (distance - range), which stalled forever on a remainder that float
// rounding could not add (world2_level1 / world3_level2 / world5_level0 exit scripts waited on IsZombieWalking).
func TestScriptedZombieWalkArrivesWhenTheRangeEdgeIsAHairAway(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y = 450, 250
	size := nativeSpawnRenderSize(formats.Vec2{X: 29, Y: 31})
	p.scriptEntities = map[int]*scriptEntity{}
	p.scriptEntities[1] = &scriptEntity{id: 1, kind: "zombie", entityType: "zombie", x: 200.0000001, y: 100, scaleX: 1, scaleY: 1, alpha: 1, texture: "cavezombie", speed: 160, targetX: 120, targetY: 100, targetRange: 80, walking: true}
	p.zombies = append(p.zombies, zombieState{x: 200.0000001, y: 100, speed: 160, health: 100, size: formats.Vec2{X: size, Y: size}, texture: "cavezombie", scriptID: 1, alpha: 1, scriptControlled: true})
	for frame := 0; frame < 4; frame++ {
		p.updateZombies()
	}
	if p.scriptEntities[1].walking {
		t.Fatalf("the walk never ended; zombie at %.6f,%.6f", p.zombies[0].x, p.zombies[0].y)
	}
}

// 1.2.5 FUN_00122828: a random point of the ring minRadius..maxRadius around the centre whose straight line to the
// centre crosses no solid cell and whose clearance circle (default 32) overlaps no solid tile.
func TestGetPositionWithinRadiusStaysInTheRingOnTheCentresSideOfAWall(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 3 })
	host := &playScriptHost{play: p}
	const centreX, centreY = 8.5 * 32, 4.5 * 32
	for call := 0; call < 60; call++ {
		result, err := host.Call("GetPositionWithinRadius", []scripting.Value{centreX, centreY, 150.0, 200.0})
		if err != nil || len(result.Values) != 2 {
			t.Fatalf("GetPositionWithinRadius = %v, %v", result.Values, err)
		}
		x, y := result.Values[0].(float64), result.Values[1].(float64)
		if distance := math.Hypot(x-centreX, y-centreY); distance < 149 || distance > 201 {
			t.Fatalf("call %d: (%.1f,%.1f) is %.1f from the centre, want 150..200", call, x, y, distance)
		}
		if x < 4*32+31 || y < 31 || y > 9*32-31 {
			t.Fatalf("call %d: (%.1f,%.1f) overlaps the wall column / map border or lies behind the wall", call, x, y)
		}
	}
}

// A gamepad has no pointer: AimControlActive / IsSecondaryButtonDown must see the injected player input too
// (the tutorial waits for them).
func TestAimAndSecondaryScriptQueriesSeeGamepadInput(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y, p.radius = 100, 100, playerCollisionRadius
	p.shootControl, p.moveControl = true, true
	host := &playScriptHost{play: p}
	if result, _ := host.Call("AimControlActive", nil); result.Values[0] != false {
		t.Fatal("AimControlActive true without any aiming")
	}
	p.input = playerInput{aimX: 1}
	p.Update(0, 0, false, false, false)
	if result, _ := host.Call("AimControlActive", nil); result.Values[0] != true {
		t.Fatal("AimControlActive false while the aim stick is pushed")
	}
	p.input = playerInput{secondary: true}
	if result, _ := host.Call("IsSecondaryButtonDown", nil); result.Values[0] != true {
		t.Fatal("IsSecondaryButtonDown false while the pad's secondary button is held")
	}
}

// The native stick gates only apply while a script runs (FUN_000f3280: FUN_0013c170() == 0 || flag), so a script
// that finishes with SetPlayerMoveControl(false) (egypt_boss, japan_boss) must hand the controls back.
func TestControlsAreHandedBackWhenAScriptEnds(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	runtime, err := scripting.New("SetPlayerMoveControl(false)\nStopPlayerShootControl()", &playScriptHost{play: p}, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	p.scriptRuntime = runtime
	p.moveControl, p.shootControl = true, true
	if err := p.updateScript(); err != nil {
		t.Fatal(err)
	}
	if !p.scriptRuntime.Done() || p.moveControl || p.shootControl {
		t.Fatalf("after the script: done=%v move=%v shoot=%v, want a finished script that switched both off", p.scriptRuntime.Done(), p.moveControl, p.shootControl)
	}
	if err := p.updateScript(); err != nil {
		t.Fatal(err)
	}
	if !p.moveControl || !p.shootControl {
		t.Fatalf("controls not handed back: move=%v shoot=%v", p.moveControl, p.shootControl)
	}
	p.moveControl, p.shootControl, p.health = false, false, 0
	if err := p.updateScript(); err != nil || p.moveControl || p.shootControl {
		t.Fatalf("a dead player got its controls back: move=%v shoot=%v err=%v", p.moveControl, p.shootControl, err)
	}
}

// DLC2 levels (world index 6) have no entry in the per-world music table; the catalog's Music attribute names
// the track (Music_western for every president level).
func TestLevelMusicFollowsTheCatalogEntry(t *testing.T) {
	path, loop, ok := levelMusicTrack(formats.LevelInfo{ID: "president_story_0", WorldIndex: 6, Music: "Music_western"})
	want, wantLoop, _ := worldMusicTrack(5)
	if !ok || path != want || loop != wantLoop {
		t.Fatalf("president music = %q %d %v, want %q %d", path, loop, ok, want, wantLoop)
	}
	if path, _, ok := levelMusicTrack(formats.LevelInfo{ID: "World2Level0", WorldIndex: 2}); !ok || path != "audio/music/sound/Music_Egypt.ogg" {
		t.Fatalf("a level without a Music name falls back to its world track, got %q %v", path, ok)
	}
}

// A scripted zombie that cannot get closer (a wall between the ring point GetPositionWithinRadius chose and the target:
// found by the exit-script fuzz in world2_level1_exit) must not hold the cutscene's IsZombieWalking loop forever.
func TestScriptedZombieWalkEndsWhenTheTargetIsWalledOff(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 6 })
	p.x, p.y = 2*32+16, 2*32+16
	host := &playScriptHost{play: p}
	size := nativeSpawnRenderSize(formats.Vec2{X: 29, Y: 31})
	p.scriptEntities = map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "zombie", x: 4 * 32, y: 4 * 32, scaleX: 1, scaleY: 1, alpha: 1, texture: "cavezombie", speed: 100}}
	p.zombies = append(p.zombies, zombieState{x: 4 * 32, y: 4 * 32, speed: 100, health: 100, size: formats.Vec2{X: size, Y: size}, texture: "cavezombie", scriptID: 1, alpha: 1, scriptControlled: true})
	if _, err := host.Call("WalkZombieTo", []scripting.Value{1.0, 9*32 + 16.0, 4 * 32.0, 4.0}); err != nil {
		t.Fatal(err)
	}
	frames := 0
	for ; frames < 60*10 && p.scriptEntities[1].walking; frames++ {
		p.updateZombies()
	}
	if p.scriptEntities[1].walking {
		t.Fatalf("the walk toward a walled-off target never ended (zombie at %.1f,%.1f)", p.zombies[0].x, p.zombies[0].y)
	}
	if frames < 60*int(scriptWalkPatience) {
		t.Fatalf("the walk was given up after %d frames, before the %v s patience", frames, scriptWalkPatience)
	}
}

// Exit scripts that send a zombie to a point near Barry (world2_level1 prince, world3_level2 Tanaka, world5_level0 ruby:
// WalkZombieTo(z, GetPlayerX(), GetPlayerY(), range) followed by `while IsZombieWalking(z) == true do Idle() end`) hung
// whenever the zombie's last step landed on the range circle. Native FUN_001013c4 ends the walk when the squared distance
// is below the squared range and always steps speed * dt; the port clamped the last step to `distance - range` and compared
// with <=, so a leftover of one ulp (distance 80.00 against range 80) never closed and the cutscene never ended.
// Each case is a fixed seed (fresh headless app, ptExitFuzz sample index): with the legacy law every one of them stalled
// ("zombie ... walking to ... range 80 (distance 80.00)", checked by switching the law back), with the native law they finish.
func TestScriptedZombieWalkExitScriptsFinishFromTheSeedsThatStalled(t *testing.T) {
	cases := []struct {
		cache, level string
		sample       int
	}{
		{"hd", "World5Level0", 0}, {"hd", "World5Level0", 2}, {"hd", "World5Level0", 3},
		{"sd", "World5Level0", 0}, {"sd", "World5Level0", 2},
		{"hd", "World3Level2", 1}, {"hd", "World3Level2", 3}, {"sd", "World3Level2", 1},
		{"hd", "World2Level1", 4}, {"hd", "World2Level1", 7},
	}
	for _, c := range cases {
		var root string
		for _, cache := range ptCaches() {
			if strings.HasPrefix(cache.name, c.cache) {
				root = cache.root
			}
		}
		if root == "" {
			continue
		}
		t.Run(fmt.Sprintf("%s/%s/sample%d", c.cache, c.level, c.sample), func(t *testing.T) {
			a := newHeadlessApp(t, root)
			for _, info := range a.levels {
				if info.ID != c.level {
					continue
				}
				if problem := ptExitFuzz(a, info, c.sample); problem != "" {
					t.Errorf("%s %s sample %d: %s", c.cache, c.level, c.sample, problem)
				}
			}
		})
	}
}

// The same law as a sweep: 300 deterministic start points around the target must all end their walk (the legacy clamped
// step stalled for a large share of approach angles because the last step lands on the range circle).
func TestScriptedZombieWalkEndsFromAnyApproach(t *testing.T) {
	seed := uint32(20261009)
	next := func() float64 { // small LCG: the sweep must not depend on the game RNG
		seed = seed*1664525 + 1013904223
		return float64(seed>>8) / float64(1<<24)
	}
	for trial := 0; trial < 300; trial++ {
		p := tileRig(t, func(x, y int) bool { return false })
		p.x, p.y = 5000, 5000 // far away: a walking zombie must not be pushed by Barry on its way
		targetX, targetY := 150+next()*200, 80+next()*130
		angle, distance := next()*2*math.Pi, 150+next()*250
		startX, startY := targetX+math.Cos(angle)*distance, targetY+math.Sin(angle)*distance*.4
		speed := 90 + next()*80
		rangeCheck := []float64{4, 32, 80}[trial%3]
		size := nativeSpawnRenderSize(formats.Vec2{X: 29, Y: 31})
		p.scriptEntities = map[int]*scriptEntity{1: {id: 1, kind: "zombie", entityType: "zombie", x: startX, y: startY, scaleX: 1, scaleY: 1, alpha: 1, texture: "cavezombie", speed: speed, targetX: targetX, targetY: targetY, targetRange: rangeCheck, walking: true}}
		p.zombies = append(p.zombies, zombieState{x: startX, y: startY, speed: speed, health: 100, size: formats.Vec2{X: size, Y: size}, texture: "cavezombie", scriptID: 1, alpha: 1, scriptControlled: true})
		frames := 0
		for ; frames < 60*20 && p.scriptEntities[1].walking; frames++ {
			p.updateZombies()
		}
		if p.scriptEntities[1].walking {
			z := p.zombies[0]
			t.Fatalf("trial %d: walk from %.3f,%.3f to %.3f,%.3f (range %.0f, speed %.1f) never ended; zombie at %.6f,%.6f, distance %.12f", trial, startX, startY, targetX, targetY, rangeCheck, speed, z.x, z.y, math.Hypot(targetX-z.x, targetY-z.y))
		}
		// Native law: full speed*dt steps, done when the distance is below the range, so the walk lasts
		// floor((D - range) / (speed*dt)) + 1 frames (+-1 for float rounding) and ends inside the range.
		wantFrames := int(math.Floor((distance*math.Hypot(math.Cos(angle), .4*math.Sin(angle))-rangeCheck)/(speed/60))) + 1
		if wantFrames < 1 { // it started inside the range: the first update ends the walk
			wantFrames = 1
		}
		z := p.zombies[0]
		if left := math.Hypot(targetX-z.x, targetY-z.y); left >= rangeCheck || frames < wantFrames-1 || frames > wantFrames+1 {
			t.Fatalf("trial %d: the walk ended after %d frames %.3f from the target (range %.0f), want about %d frames inside the range", trial, frames, left, rangeCheck, wantFrames)
		}
	}
}
