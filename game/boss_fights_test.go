package game

import (
	"fmt"
	"math"
	"testing"
)

func runScriptToEnd(t *testing.T, p *playState, limit int) {
	t.Helper()
	for frame := 0; frame < limit && p.scriptRuntime != nil && !p.scriptRuntime.Done(); frame++ {
		if p.dialogueIndex < len(p.dialogue) && frame%60 == 0 {
			p.dialogueIndex++
		}
		p.aimActive = true // a scripted wait for "the player shoots" is satisfied
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
		p.updateZombies()
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		t.Fatalf("script still running after %d frames at %s", limit, p.scriptLastCallback)
	}
}

func TestRexBossIntroRunsAndBossDeathEndsTheLevel(t *testing.T) {
	h := script125CachedHost(t, "world0_level2")
	p := h.play
	p.scriptRuntime.Close()
	p.scriptRuntime = nil
	p.hudVisible = true
	p.waveIndex = 3
	var rexID int
	for frame := 0; frame < 400 && rexID == 0; frame++ {
		p.Update(0, 0, false, false, false)
		for id, entity := range p.scriptEntities {
			if entity.entityType == "boss_rex" {
				rexID = id
			}
		}
	}
	if rexID == 0 {
		t.Fatal("boss_rex never spawned")
	}
	if len(p.bossScripts) == 0 || p.bossScripts[0] != "dino_boss.script" {
		t.Fatalf("boss intro not queued: %v", p.bossScripts)
	}
	if err := h.app.updateBossScripts(); err != nil {
		t.Fatal(err)
	}
	rex := h.findZombie(rexID)
	speed := rex.speed
	if speed <= 0 {
		t.Fatalf("rex spawned with speed %v", speed)
	}
	runScriptToEnd(t, p, 6000)
	if rex.speed != speed {
		t.Fatalf("rex speed after intro %v, want the original %v", rex.speed, speed)
	}
	if !p.hudVisible {
		t.Fatal("intro left the HUD hidden")
	}

	// Killing the boss stops the waves, clears the field and queues the end script.
	// (The landing shockwave may have compacted the zombie list: look the rex up again.)
	rex = h.findZombie(rexID)
	rex.health, rex.dying, rex.deathAge = 0, true, 10
	for frame := 0; frame < 5; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if !p.bossDefeated || !p.wavesFinished {
		t.Fatalf("boss death ignored: defeated=%v wavesFinished=%v", p.bossDefeated, p.wavesFinished)
	}
	if len(p.bossScripts) == 0 || p.bossScripts[0] != "dino_boss_end.script" {
		t.Fatalf("boss end script not queued: %v", p.bossScripts)
	}
	if err := h.app.updateBossScripts(); err != nil {
		t.Fatal(err)
	}
	runScriptToEnd(t, p, 3000)
	if p.scriptPaused {
		t.Fatal("PauseGame(true) left the game paused")
	}
	if p.paused {
		t.Fatal("PauseGame must not open the player pause menu")
	}
	for frame := 0; frame < 400 && len(p.zombies) > 0; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if !p.readyForExitScript() {
		t.Fatalf("level never became ready to exit: zombies=%d", len(p.zombies))
	}
}

func TestBossScriptsAreMappedPerBoss(t *testing.T) {
	for entity, texture := range map[string]string{"boss_rex": "rexwalk_1", "boss_egyptian": "pharoh", "boss_samurai": "samurai"} {
		if bossIntroScript(entity, texture) == "" {
			t.Fatalf("%s has no intro script", entity)
		}
	}
	if bossIntroScript("boss_rex", "characters/georgewashington") != "president_boss.script" {
		t.Fatal("president rex uses the president intro")
	}
	if bossEndScript("boss_rex", "characters/georgewashington") != "" {
		t.Fatal("only the dinosaur has a boss-end script")
	}
}

// bossLevelSweep drives a boss level through spawn -> intro script -> boss death.
func bossLevelSweep(t *testing.T, base, bossType string, spawnedByWave bool) {
	t.Helper()
	h := script125CachedHost(t, base)
	p := h.play
	if !spawnedByWave {
		// The entry script stages this boss (AddRobotBossZombie / AddWesternBossZombie).
		runScriptToEnd(t, p, 60000)
	} else {
		p.closeScript()
		for index, wave := range p.world.Level.Waves {
			for _, spawner := range wave.Spawners {
				for _, entry := range spawner.Types {
					if entry.Name == bossType {
						p.waveIndex = index
					}
				}
			}
		}
	}
	p.hudVisible = true
	var bossID int
	for frame := 0; frame < 600 && bossID == 0; frame++ {
		p.Update(0, 0, false, false, false)
		for id, entity := range p.scriptEntities {
			if entity.entityType == bossType {
				bossID = id
			}
		}
	}
	if bossID == 0 {
		t.Fatalf("%s: %s never spawned", base, bossType)
	}
	if boss := h.findZombie(bossID); boss != nil {
		// Intro scripts wait for Barry to walk up to the boss.
		p.x, p.y = boss.x, boss.y+100
	}
	for guard := 0; guard < 4 && len(p.bossScripts) > 0; guard++ {
		name := p.bossScripts[0]
		if err := h.app.updateBossScripts(); err != nil {
			t.Fatalf("%s %s: %v", base, name, err)
		}
		runScriptToEnd(t, p, 20000)
	}
	boss := h.findZombie(bossID)
	if boss == nil {
		t.Fatalf("%s: boss vanished", base)
	}
	if boss.speed < 0 && bossType != "boss_robot" {
		t.Fatalf("%s: boss left with negative speed %v", base, boss.speed)
	}
	boss.health, boss.dying, boss.deathAge = 0, true, 10
	for frame := 0; frame < 5; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if !p.bossDefeated {
		t.Fatalf("%s: boss death ignored", base)
	}
	for guard := 0; guard < 4 && len(p.bossScripts) > 0; guard++ {
		if err := h.app.updateBossScripts(); err != nil {
			t.Fatalf("%s end script: %v", base, err)
		}
		runScriptToEnd(t, p, 20000)
	}
	for frame := 0; frame < 600 && len(p.zombies) > 0; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if !p.readyForExitScript() {
		left := ""
		for _, zombie := range p.zombies {
			left += fmt.Sprintf(" [%s health=%.0f dying=%v away=%v age=%.1f]", p.scriptEntities[zombie.scriptID].entityType, zombie.health, zombie.dying, zombie.spawnAway, zombie.deathAge)
		}
		t.Fatalf("%s: not ready for the exit script (zombies %d, health %v, finished %v, script done %v):%s", base, len(p.zombies), p.health, p.wavesFinished, p.scriptRuntime == nil || p.scriptRuntime.Done(), left)
	}
}

func TestEgyptianBossFight(t *testing.T) { bossLevelSweep(t, "world2_level2", "boss_egyptian", true) }
func TestSamuraiBossFight(t *testing.T)  { bossLevelSweep(t, "world3_level2", "boss_samurai", true) }
func TestGangsterBossFight(t *testing.T) { bossLevelSweep(t, "world1_level2", "boss_gangster", true) }
func TestRobotBossFight(t *testing.T)    { bossLevelSweep(t, "world4_level2", "boss_robot", false) }
func TestWesternBossFight(t *testing.T)  { bossLevelSweep(t, "world5_level2", "boss_west", false) }

// The dino_boss intro moves the camera centre only through its own SetCamera calls (1.2.5):
//   - SetCamera stores the clamped centre at once (FUN_0013de0c -> FUN_00095dd8, param 0).
//   - The per-camera updater FUN_00096030 leaves the centre alone here: no follow entity
//     (cam+0x4c, cleared by the level entry script's SetCameraFollow(0)), no pan (timer cam+0x58
//     and flag cam+0x78 are cleared once no script runs), and auto-tracking FUN_000c35b0 is gated
//     off while the script flag is set (FUN_0013c170).
//   - GetCameraX = centre + shake offset (FUN_00095aa8 = cam+0x24 + cam+0x18), so the boss lerp
//     centre' = clamp(0.95*(centre + shake) + 0.05*Barry) feeds the shake back into the centre.
//   - The shake offset is bounded by its targets: X by 9*ampX = 13.5 px, Y by 9*ampY = 9 px
//     (FUN_00095d38 / FUN_00095c14), and it stops stepping when the 1.5 s timer ends.
//   - WaitInit(1000) is 60 ticks; RexWalk then reuses the stale camX, so the centre stays put.
//
// See Research/native/rex-intro-camera-2026-10-09.md.
func TestRexIntroCameraStaysOnBarryAndReturnsAfterTheScript(t *testing.T) {
	h := script125CachedHost(t, "world0_level2")
	p := h.play
	p.scriptRuntime.Close()
	p.scriptRuntime = nil
	p.hudVisible = true
	p.waveIndex = 3
	var rexID int
	for frame := 0; frame < 400 && rexID == 0; frame++ {
		p.Update(0, 0, false, false, false)
		for id, entity := range p.scriptEntities {
			if entity.entityType == "boss_rex" {
				rexID = id
			}
		}
	}
	if rexID == 0 {
		t.Fatal("boss_rex never spawned")
	}
	rex := h.findZombie(rexID)
	p.world.SetZoom(.65)
	p.setScriptCamera(p.x, p.y)
	// dino_boss.script: desiredCamX = GetPlayerX() before the first Update. Script numbers returned by
	// the host are float32 (native callbacks push a float), so the desired point is float32 as well.
	desiredX, desiredY := float64(float32(p.x)), float64(float32(p.y))
	if err := h.app.updateBossScripts(); err != nil {
		t.Fatal(err)
	}
	startX, startY := p.scriptCameraCenterX(), p.scriptCameraCenterY()
	if math.Abs(startX-p.x) > 1e-9 || math.Abs(startY-p.y) > 1e-9 {
		t.Fatalf("camera starts at %.3f,%.3f, want Barry at %.3f,%.3f", startX, startY, p.x, p.y)
	}
	const (
		shakeMaxX  = 9 * 1.5 // CameraShake(rexX, rexY, 1.5, 1.5): ampX 1.5, first target 9*ampX
		shakeMaxY  = 9 * 1.0 // ampY 1.0
		lerpFrames = 61      // WaitInit(1000): 60 ticks of 1000/60 ms, plus one for rounding
	)
	clampX := func(x float64) float64 {
		half := float64(logicalWidth) / (2 * p.world.Zoom)
		return math.Max(half, math.Min(math.Max(half, float64(p.world.Level.Width*p.tileSize)-half), x))
	}
	clampY := func(y float64) float64 {
		half := float64(logicalHeight) / (2 * p.world.Zoom)
		return math.Max(half, math.Min(math.Max(half, float64(p.world.Level.Height*p.tileSize)-half), y))
	}
	cameraValue := func(name string) float64 {
		t.Helper()
		result, err := h.Call(name, nil)
		if err != nil || len(result.Values) != 1 {
			t.Fatalf("%s: %v (%#v)", name, err, result)
		}
		value, ok := result.Values[0].(float64)
		if !ok {
			t.Fatalf("%s returned %#v", name, result.Values[0])
		}
		return value
	}
	motion, frozen := 0, false
	maxFeedback := 0.0
	for frame := 0; frame < 6000 && p.scriptRuntime != nil && !p.scriptRuntime.Done(); frame++ {
		if p.dialogueIndex < len(p.dialogue) && frame%60 == 0 {
			p.dialogueIndex++
		}
		p.aimActive = true
		beforeX, beforeY := p.scriptCameraCenterX(), p.scriptCameraCenterY()
		// GetCameraX/Y as the script reads them this tick: centre plus the shake offset.
		seenX, seenY := cameraValue("GetCameraX"), cameraValue("GetCameraY")
		if math.Abs(seenX-(beforeX+p.shake.currentX)) > 1e-3 || math.Abs(seenY-(beforeY+p.shake.currentY)) > 1e-3 {
			t.Fatalf("GetCameraX/Y = %.6f,%.6f, want centre %.6f,%.6f plus shake %.6f,%.6f (float32)", seenX, seenY, beforeX, beforeY, p.shake.currentX, p.shake.currentY)
		}
		maxFeedback = math.Max(maxFeedback, math.Max(math.Abs(seenX-beforeX), math.Abs(seenY-beforeY)))
		if err := p.updateScript(); err != nil {
			t.Fatal(err)
		}
		afterX, afterY := p.scriptCameraCenterX(), p.scriptCameraCenterY()
		if math.Abs(afterX-beforeX) > 1e-9 || math.Abs(afterY-beforeY) > 1e-9 {
			if frozen {
				t.Fatalf("frame %d: centre moved to %.3f,%.3f after the lerp had stopped (rex at %.0f,%.0f)", frame, afterX, afterY, rex.x, rex.y)
			}
			// dino_boss.script lerp: camX = GetCameraX + (desiredCamX - GetCameraX) * 0.05.
			wantX := clampX(seenX + (desiredX-seenX)*0.05)
			wantY := clampY(seenY + (desiredY-seenY)*0.05)
			if math.Abs(afterX-wantX) > 1e-6 || math.Abs(afterY-wantY) > 1e-6 {
				t.Fatalf("frame %d: centre %.6f,%.6f, want the lerp %.6f,%.6f (GetCameraX %.6f,%.6f, Barry %.3f,%.3f; rex at %.0f,%.0f)", frame, afterX, afterY, wantX, wantY, seenX, seenY, desiredX, desiredY, rex.x, rex.y)
			}
			motion++
		} else if motion > 0 {
			frozen = true
		}
		p.updateZombies()
		p.updateCamera()
		p.stepShakeTick()
		if math.Abs(p.shake.currentX) > shakeMaxX+1e-9 || math.Abs(p.shake.currentY) > shakeMaxY+1e-9 {
			t.Fatalf("frame %d: shake offset %.3f,%.3f exceeds the native targets %.1f,%.1f", frame, p.shake.currentX, p.shake.currentY, shakeMaxX, shakeMaxY)
		}
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		t.Fatal("intro never finished")
	}
	if motion < lerpFrames-3 || motion > lerpFrames {
		t.Fatalf("the lerp moved the centre on %d ticks, want about 60 (WaitInit(1000))", motion)
	}
	if maxFeedback < 1 {
		t.Fatalf("shake feedback reached GetCameraX by at most %.3f px; the lerp coupling was not exercised", maxFeedback)
	}
	// Feedback envelope: u' = 0.95*u + 0.95*C with |C| <= the targets, so |u| <= 0.95*C_max*(1-0.95^n)/0.05.
	envX := 0.95 * shakeMaxX * (1 - math.Pow(0.95, lerpFrames)) / 0.05
	envY := 0.95 * shakeMaxY * (1 - math.Pow(0.95, lerpFrames)) / 0.05
	if dx, dy := p.scriptCameraCenterX()-desiredX, p.scriptCameraCenterY()-desiredY; math.Abs(dx) > envX || math.Abs(dy) > envY {
		t.Fatalf("centre %.1f,%.1f is %.1f,%.1f from Barry, beyond the shake-feedback envelope %.1f,%.1f (rex at %.0f,%.0f)", p.scriptCameraCenterX(), p.scriptCameraCenterY(), dx, dy, envX, envY, rex.x, rex.y)
	}
	// After the script the normal follow resumes and settles on the player.
	for frame := 0; frame < 300; frame++ {
		p.updateCamera()
	}
	if math.Hypot(p.scriptCameraCenterX()-p.x, p.scriptCameraCenterY()-p.y) > 1 {
		t.Fatalf("camera did not return to Barry after the intro: centre %.1f,%.1f player %.1f,%.1f", p.scriptCameraCenterX(), p.scriptCameraCenterY(), p.x, p.y)
	}
}
