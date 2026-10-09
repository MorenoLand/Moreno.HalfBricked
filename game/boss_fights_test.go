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

// The dino_boss intro never recentres on the rex natively: the script's own camera
// writes only SetCamera(camX, camY) from a stale value (the RexWalk lerp lines are
// commented out), auto-tracking is off while a script runs (FUN_0013c170 gate in
// FUN_000c35b0 / FUN_00096030 follows nothing), and CameraShake only adds a decaying
// offset. GetCameraX/Y return centre + shake offset (FUN_00095aa8). See
// Research/native/rex-intro-camera-2026-10-09.md.
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
	if err := h.app.updateBossScripts(); err != nil {
		t.Fatal(err)
	}
	startX, startY := p.scriptCameraCenterX(), p.scriptCameraCenterY()
	maxDrift := 0.0
	for frame := 0; frame < 6000 && p.scriptRuntime != nil && !p.scriptRuntime.Done(); frame++ {
		if p.dialogueIndex < len(p.dialogue) && frame%60 == 0 {
			p.dialogueIndex++
		}
		p.aimActive = true
		if err := p.updateScript(); err != nil {
			t.Fatal(err)
		}
		p.shakeOffX, p.shakeOffY = p.updateShake(1.0 / 60.0)
		p.updateZombies()
		p.updateCamera()
		drift := math.Hypot(p.scriptCameraCenterX()-startX, p.scriptCameraCenterY()-startY)
		maxDrift = math.Max(maxDrift, drift)
		// GetCameraX/Y are the centre plus the live shake offset.
		if got := h.play.scriptCameraCenterX() + p.shake.currentX; math.Abs(got-(p.world.CameraX+float64(logicalWidth)/(2*p.world.Zoom)+p.shake.currentX)) > 1e-9 {
			t.Fatalf("GetCameraX = %v", got)
		}
	}
	if p.scriptRuntime != nil && !p.scriptRuntime.Done() {
		t.Fatal("intro never finished")
	}
	// Native: the camera is not pulled to the rex (only the shake feedback moves it a little).
	if maxDrift > 40 {
		t.Fatalf("camera drifted %.1f px from Barry during the intro; native has no pan toward the rex (rex at %.0f,%.0f)", maxDrift, rex.x, rex.y)
	}
	// After the script the normal follow resumes and settles on the player.
	for frame := 0; frame < 300; frame++ {
		p.updateCamera()
	}
	if math.Hypot(p.scriptCameraCenterX()-p.x, p.scriptCameraCenterY()-p.y) > 1 {
		t.Fatalf("camera did not return to Barry after the intro: centre %.1f,%.1f player %.1f,%.1f", p.scriptCameraCenterX(), p.scriptCameraCenterY(), p.x, p.y)
	}
}
