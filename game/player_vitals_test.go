package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"testing"
)

// FUN_00094c6c sets +0x98 = (1 - health) + 0.5 on a hit; FUN_00096818 then waits
// that long and regrows the health by 0.75 per second.
func TestPlayerHealthRegeneratesAfterTheNativeDelay(t *testing.T) {
	p := &playState{health: 1, maxHealth: 1}
	p.damagePlayer(0, .5)
	if p.health != .5 || math.Abs(p.vitals.regenDelay-1.0) > 1e-9 {
		t.Fatalf("health %v delay %v, want .5 and 1.0 (= 0.5 + 0.5)", p.health, p.vitals.regenDelay)
	}
	for i := 0; i < 59; i++ {
		p.stepVitals(1.0 / 60)
	}
	if p.health != .5 {
		t.Fatalf("regenerated during the delay: %v", p.health)
	}
	for i := 0; i < 60; i++ { // one more second of regrowth
		p.stepVitals(1.0 / 60)
	}
	if p.health < .5+.7 && p.health < 1 {
		t.Fatalf("health after delay+1s = %v, want about 0.5 + 0.75 capped at 1", p.health)
	}
	for i := 0; i < 120; i++ {
		p.stepVitals(1.0 / 60)
	}
	if p.health != 1 {
		t.Fatalf("health must cap at 1, got %v", p.health)
	}
	// A new hit restarts the wait, longer when lower.
	p.damagePlayer(0, .9)
	if math.Abs(p.vitals.regenDelay-1.4) > 1e-9 {
		t.Fatalf("delay at health .1 = %v, want 1.4", p.vitals.regenDelay)
	}
	// The dead do not regenerate.
	p.health = 0
	p.vitals.regenDelay = 0
	p.stepVitals(1)
	if p.health != 0 {
		t.Fatal("dead players do not heal")
	}
}

// FUN_000962f4 sets +0x94 = 1500 on respawn; FUN_00094c6c ignores hits until it
// has run down (16 ms per 60 Hz frame, 94 frames).
func TestRespawnGraceBlocksDamageFor1500Milliseconds(t *testing.T) {
	p := &playState{health: 0, maxHealth: 1, lives: 3, spawnX: 100, spawnY: 100}
	for frame := 0; frame < 60*3 && p.health <= 0; frame++ {
		p.updatePlayerDeath()
	}
	if p.health != 1 || p.vitals.invulnMS != playerRespawnGraceMS {
		t.Fatalf("respawn: health %v grace %d", p.health, p.vitals.invulnMS)
	}
	p.damagePlayer(0, .5)
	if p.health != 1 {
		t.Fatal("damage during the grace period must be ignored")
	}
	frames := 0
	for p.vitals.invulnMS > 0 {
		p.stepVitals(1.0 / 60)
		frames++
	}
	if frames != 94 {
		t.Fatalf("grace lasted %d frames, want 94 (1500 ms in 16 ms steps)", frames)
	}
	p.damagePlayer(0, .5)
	if p.health != .5 {
		t.Fatal("damage works again after the grace period")
	}
}

// The native counter starts at 3 and only a negative value ends the run: four deaths.
func TestPlayerHasFourDeathsLikeTheNativeLivesCounter(t *testing.T) {
	p := &playState{health: 1, maxHealth: 1, lives: 3, spawnX: 50, spawnY: 50}
	respawns := 0
	for death := 0; death < 4; death++ {
		p.health = 0
		p.deathStarted = false
		for frame := 0; frame < 60*4; frame++ {
			p.updatePlayerDeath()
			if p.health > 0 {
				respawns++
				break
			}
		}
		if p.storyGameOver(0) && death < 3 {
			t.Fatalf("game over after only %d deaths", death+1)
		}
	}
	if respawns != 3 || p.lives != -1 || !p.outOfLives() {
		t.Fatalf("respawns %d lives %d", respawns, p.lives)
	}
	p.deathTimer = 0
	if !p.storyGameOver(0) {
		t.Fatal("the fourth death ends the story run")
	}
}

func TestDeathPutsThePistolBack(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.collectPickup("p_shotgun")
	p.health, p.deathStarted = 0, false
	p.updatePlayerDeath()
	if p.weapon.GunType != "PISTOL" {
		t.Fatalf("weapon after death %s", p.weapon.GunType)
	}
}

// Native speed: 0.81 * 200 = 162 px/s in v7 (SD cache), 0.81 * 180 = 145.8 px/s in
// 1.2.5 (HD cache, the default); radius 0.2 * 64 = 12.8.
func TestPlayerSpeedAndRadiusAreTheNativeValues(t *testing.T) {
	if math.Abs(playerWalkBase(content.WaveBuildV7)-162) > 1e-9 || math.Abs(playerWalkBase(content.WaveBuild125)-145.8) > 1e-9 || math.Abs(playerCollisionRadius-12.8) > 1e-9 {
		t.Fatalf("speed v7 %v 1.2.5 %v radius %v", playerWalkBase(content.WaveBuildV7), playerWalkBase(content.WaveBuild125), playerCollisionRadius)
	}
	for _, c := range []struct {
		build string
		want  float64
	}{{content.WaveBuildV7, 162}, {content.WaveBuild125, 145.8}} {
		p := tileRig(t, func(x, y int) bool { return false })
		p.world.Level.WaveBuild = c.build
		p.x, p.y, p.radius = 100, 144, playerCollisionRadius
		for i := 0; i < 60; i++ {
			p.stepBody(1, 0, false, false, true)
		}
		if math.Abs(p.x-100-c.want) > .5 {
			t.Fatalf("build %s: one second of walking moved %.2f px, want %v", c.build, p.x-100, c.want)
		}
	}
}

func TestPlayerStopsAgainstWallsAtTheNativeRadius(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return x == 8 })
	p.x, p.y, p.radius = 4*32+16, 4*32+16, playerCollisionRadius
	for i := 0; i < 240; i++ {
		p.stepBody(1, 0, false, false, true)
	}
	face := 8.0 * 32
	if math.Abs(p.x-(face-playerCollisionRadius)) > .6 {
		t.Fatalf("player rests at x=%.2f, want %.2f", p.x, face-playerCollisionRadius)
	}
}

// Zombies never hold Barry back: the native separation pass only shoves them (see
// separatePlayers); Barry is pushed only inside a script (tests below).
func TestZombiesDoNotBlockThePlayerLikeTheOriginal(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y, p.radius = 200, 144, playerCollisionRadius
	z := spawnRisen(p, "zombie", 210, 144)
	z.speed = 0
	for i := 0; i < 30; i++ {
		p.stepBody(1, 0, false, false, true)
	}
	if p.x < 200+40 {
		t.Fatalf("the player was held back by a zombie (x=%.1f)", p.x)
	}
	if playerBlockedByZombies {
		t.Fatal("the original has no player-zombie body collision")
	}
}

// FUN_000a1b08 never stops Barry: a zombie standing in his path is shoved ahead of him
// (overlap * dt * 10 per frame) and he keeps walking at the full native speed.
func TestZombieInBarrysPathIsShovedAsideNotBlocking(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y, p.radius = 200, 144, playerCollisionRadius
	spawnRisen(p, "zombie", 210, 144)
	p.zombies[0].speed = 0
	p.zombies[0].native.ai.state = 1 // risen: the rise (state 0) takes no separation (FUN_000a1b08)
	for i := 0; i < 60; i++ {
		p.stepBody(1, 0, false, false, true)
		p.separateZombie(0, 1.0/60)
	}
	if want := playerWalkBase(p.waveBuild()); math.Abs(p.x-200-want) > 1 {
		t.Fatalf("Barry walked %.2f px past a zombie in his path, want %.1f", p.x-200, want)
	}
	if p.zombies[0].x <= p.x {
		t.Fatalf("zombie at %.1f was not shoved ahead of Barry at %.1f", p.zombies[0].x, p.x)
	}
}

// FUN_000a1b08 pushes the zombie away from Barry by overlap * dt * 10 (DAT_000a2090)
// and leaves Barry where he is.
func TestZombieYieldsToBarryAtTheNativeRate(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y = 200, 144
	spawnRisen(p, "zombie", 210, 144)
	p.zombies[0].speed = 0
	p.zombies[0].native.ai.state = 1 // risen: the rise (state 0) takes no separation (FUN_000a1b08)
	z := &p.zombies[0]
	before := z.x
	reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*entityGridPlayerBody
	p.separateZombie(0, 1.0/60)
	want := (reach - 10) * (1.0 / 60) * zombiePushScale
	if math.Abs(z.x-before-want) > 1e-9 || z.y != 144 || p.x != 200 {
		t.Fatalf("zombie moved %.4f (want %.4f), Barry at x=%.1f", z.x-before, want, p.x)
	}
}

// With a script running (FUN_000d9ab4 != 0) the zombie is left alone and Barry is pushed
// by overlap * dt * 2.5 (DAT_000a207c) only while SetAllowThumbsticksDuringScripts is on.
func TestScriptedZombiePushesBarryOnlyWithThumbsticks(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y = 200, 144
	spawnRisen(p, "zombie", 210, 144)
	z := &p.zombies[0]
	z.speed = 0
	reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*entityGridPlayerBody
	const dt = 1.0 / 60
	if pushX, pushY := p.separatePlayers(z, dt, 0, 0, true); pushX != 0 || pushY != 0 || p.x != 200 {
		t.Fatalf("thumbsticks off: zombie push (%.3f,%.3f), Barry at %.3f", pushX, pushY, p.x)
	}
	p.scriptAllowThumbsticks = true
	pushX, pushY := p.separatePlayers(z, dt, 0, 0, true)
	want := 200 - (reach-10)*dt*playerScriptPushScale
	if pushX != 0 || pushY != 0 || math.Abs(p.x-want) > 1e-9 {
		t.Fatalf("thumbsticks on: zombie push (%.3f,%.3f), Barry at %.4f, want %.4f", pushX, pushY, p.x, want)
	}
}

// A candidate must share a grid cell with the zombie (FUN_000a1b08 walks the zombie's
// own cells). A 240 px wide zombie reaches 91.2 px, but Barry's +-0.45 * 64 box shares
// no cell with its +-0.3 * 64 box at 80 px, so he is not pushed; at 40 px he is.
func TestBarryIsOnlyACandidateInSharedGridCells(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.zombies = []zombieState{{x: 400, y: 144, health: 100, size: formats.Vec2{X: 240, Y: 240}, alpha: 1}}
	z := &p.zombies[0]
	p.x, p.y = 480, 144
	if pushX, pushY := p.separatePlayers(z, 1.0/60, 0, 0, false); pushX != 0 || pushY != 0 {
		t.Fatalf("Barry outside the shared cells pushed the zombie (%.3f,%.3f)", pushX, pushY)
	}
	p.x = 440
	if pushX, _ := p.separatePlayers(z, 1.0/60, 0, 0, false); pushX == 0 {
		t.Fatal("Barry inside the shared cells was not a candidate")
	}
}

// FUN_000962f4 takes a dead Barry out of the grid, so he is no candidate at all.
func TestDeadBarryIsNoZombieCandidate(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.x, p.y, p.health = 200, 144, 0
	spawnRisen(p, "zombie", 210, 144)
	z := &p.zombies[0]
	if pushX, pushY := p.separatePlayers(z, 1.0/60, 0, 0, false); pushX != 0 || pushY != 0 {
		t.Fatalf("dead Barry pushed a zombie (%.3f,%.3f)", pushX, pushY)
	}
}

// FUN_000bdf7c: respawn at the start marker with the fewest entities around it.
func TestRespawnPicksTheLeastCrowdedStartMarker(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	layer := p.world.Level.Layers[formats.LayerC]
	layer[2*16+2] = 2  // marker at tile (2,2)
	layer[2*16+12] = 2 // marker at tile (12,2)
	p.spawnX, p.spawnY = 2*32+16, 2*32+16
	if x, _ := p.respawnPoint(); x != 2*32+16 {
		t.Fatalf("empty level: first marker expected, got x=%v", x)
	}
	for i := 0; i < 6; i++ {
		z := spawnRisen(p, "zombie", 2*32+16+float64(i*3), 2*32+16)
		z.speed = 0
	}
	// FUN_000bdf7c counts entities through the grid (FUN_00091ac8), where a rising zombie is not listed.
	riseZombies(p)
	p.refreshEntityGrid()
	if x, y := p.respawnPoint(); x != 12*32+16 || y != 2*32+16 {
		t.Fatalf("crowded marker chosen: %v,%v", x, y)
	}
	// Death stores the spot in the spawn position the respawn then uses.
	p.health, p.lives, p.deathStarted = 0, 3, false
	for frame := 0; frame < 60*3 && p.health <= 0; frame++ {
		p.updatePlayerDeath()
	}
	if p.health <= 0 || p.x != 12*32+16 {
		t.Fatalf("respawned at %v,%v health %v", p.x, p.y, p.health)
	}
}
