package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Wave advance rule, recovered from the native libraries (Research/native/wave-advance-2026-10-09.md).
// Each data cache follows the build it was imported from: the HD cache (1.2.5) and the SD cache
// (1.2.1, libmortargame.so v7) differ in the enemy limit below; the rest is the same in both.
//
// Gate: the level update (FUN_000c0cec in v7, FUN_00123180 in 1.2.5) runs the wave block only while
// the living enemy counter is below the limit (waveZombieLimit). The block holds the spawners
// (FUN_000bec48 / FUN_00121b50), the wave timer and the wave advance, so all of them pause at the limit.
//
// Wave end, FUN_000bf120 (v7) and FUN_00122058 (1.2.5), identical in both builds: the wave record keeps a
// millisecond timer that starts at end_wave_time - 10 (FUN_000c39c0 / FUN_001263e8). While it is positive
// it is lowered every frame by (int)(timer - dt * 1000) (the cast truncates, so a 60 Hz frame costs 17 ms);
// when it runs out the wave is over if every spawner has delivered its count, otherwise the timer is
// reloaded with 100 ms and the check repeats. The alive zombie count is not read on this path. Only a wave
// whose timer was not positive to begin with (end_wave_time <= 10, usually 0) ends once the spawners are
// empty AND alive zombies < end_wave_zombies.
//
// Advance: the next wave is next_wave (index + 1 when next_wave < 1). When that index is past the last
// wave no wave follows and the level ends (builder FUN_000c0b70 / FUN_00123008, push FUN_000c0c68 /
// FUN_001230fc). The banner and the wave counter move on the same frame as the advance.
//
// Spawn placement, FUN_000bec48: after the type roll the spawner picks a tile of its
// marker list with rnd(n - 1) when it has two or more (the last one can never be
// picked: FUN_00079634(rng, n) is exclusive), then a point inside that tile:
// x = (tileX + 0.1 + rnd(0.8)) * 32, y likewise.
const (
	waveEndTimerOffsetMS = 10    // FUN_000c39c0: end_wave_time + -10
	waveEndRecheckMS     = 100   // FUN_000bf120 reload while spawners remain
	waveSpawnInset       = .1    // DAT_000bf038
	waveSpawnSpan        = .8    // DAT_000bf050
	waveSpawnTile        = 32.0  // DAT_000bf03c
	waveFrameMS          = 1000. // DAT_000bf034 (dt is seconds)
)

// waveEndRuleKind chooses the end test of a wave.
type waveEndRuleKind int

const (
	// waveEndAllDead is a port option and the default: a wave ends once every spawner has delivered and no
	// living zombie is left. It replaces the timer and the alive limit. It is not the recovered native rule.
	waveEndAllDead waveEndRuleKind = iota
	// waveEndNative is FUN_000bf120 as recovered (the timer, then the alive limit of untimed waves).
	waveEndNative
)

// waveEndRule is the active end test. The -wave-end flag selects it ("all-dead" or "native").
var waveEndRule = waveEndAllDead

// waveEndStep is FUN_000bf120's end test for one frame (after the spawners ran).
// spawnersLeft is "the spawner list is not empty".
func (p *playState) waveEndStep(wave formats.Wave, spawnersLeft bool, alive int) bool {
	if waveEndRule == waveEndAllDead {
		return !spawnersLeft && alive == 0
	}
	if !p.waveEndInit || p.waveElapsed < p.waveEndStamp {
		p.waveEndInit = true
		p.waveEndTimer = int(wave.EndWaveTime) - waveEndTimerOffsetMS
	}
	p.waveEndStamp = p.waveElapsed
	if p.waveEndTimer > 0 {
		p.waveEndTimer = int(float32(p.waveEndTimer) - float32(1.0/60.0)*float32(waveFrameMS))
		if p.waveEndTimer > 0 {
			return false
		}
		if !spawnersLeft {
			return true
		}
		p.waveEndTimer = waveEndRecheckMS
	}
	if spawnersLeft {
		return false
	}
	return alive < wave.EndWaveZombies
}

// Enemy limit of the wave gate (the living enemy count must be below it for the wave block to run).
//   - v7 (1.2.1): F(+0x51484) < 65, 65 is the table word at 0x005cf464 (FUN_000c0cec: 0x000c0d94 load, read only).
//   - 1.2.5: F(+0x450f8) < L, L = vtable slot 0x38 of the mode object at frame+0x45194 (FUN_00123180 0x00123208).
//     Story: 95 (0x00098984, mov r0,#0x5f). Survival: 0x0009962c gives 20 when FUN_0009788c(obj,10,16) > 0 (a
//     boss is in the entity lists 10..15), else 95. waveGateLimit applies the 20 (survival.go, survivalBossPresent).
const (
	waveLimitV7              = 65
	waveLimit125Story        = 95
	waveLimit125Survival     = 95
	waveLimit125SurvivalBoss = 20
)

// waveZombieLimit is the living-enemy limit of the wave gate for a build (see above).
func waveZombieLimit(build string, survival bool) int {
	if build == content.WaveBuildV7 {
		return waveLimitV7
	}
	if survival {
		return waveLimit125Survival
	}
	return waveLimit125Story
}

// waveGateLimit is the gate limit of the running level: waveZombieLimit, plus the 1.2.5 survival limit of 20 while a
// boss is on the field (FUN_0009962c). v7 keeps 65 in every mode, and story keeps 95.
func (p *playState) waveGateLimit() int {
	if p.isSurvival() && p.waveBuild() != content.WaveBuildV7 && p.survivalBossPresent() {
		return waveLimit125SurvivalBoss
	}
	return waveZombieLimit(p.waveBuild(), p.isSurvival())
}

// waveBuild is the build whose wave rule the level follows: the cache it was loaded from (1.2.5 by default).
func (p *playState) waveBuild() string {
	if p.world == nil || p.world.Level.WaveBuild == "" {
		return content.WaveBuild125
	}
	return p.world.Level.WaveBuild
}

// waveAliveForEnd is the living count the all-dead rule waits for: the zombies the spawners deliver. Script
// zombies and bosses are left out (they belong to the level script, and a boss fight would hold the wave open
// for ever), so only the wave's own zombies must be dead before the wave ends.
func (p *playState) waveAliveForEnd() int {
	count := 0
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.dying || z.health <= 0 || z.scriptControlled {
			continue
		}
		if entity := p.scriptEntities[z.scriptID]; entity != nil && isBossType(entity.entityType) {
			continue
		}
		count++
	}
	return count
}

// livingZombies is the port's enemy counter: zombies that are neither dying nor dead. The native counter goes up
// when a zombie is created and down when it is killed (FUN_000a0b80 / FUN_000a10fc in v7).
func (p *playState) livingZombies() int {
	count := 0
	for _, zombie := range p.zombies {
		if !zombie.dying && zombie.health > 0 {
			count++
		}
	}
	return count
}

// nativeSpawnPoint picks the spawn position of one zombie (see above). With no
// shared RNG (some tests) it falls back to the first tile centre.
func (p *playState) nativeSpawnPoint(points []formats.Vec2) formats.Vec2 {
	if len(points) == 0 {
		return formats.Vec2{}
	}
	if p.rng == nil {
		return points[0]
	}
	index := 0
	if len(points) >= 2 {
		index = int(zombieBounded(p.rng, uint32(len(points)-1)))
	}
	tile := float64(p.tileSize)
	if tile <= 0 {
		tile = waveSpawnTile
	}
	tileX, tileY := math.Floor(points[index].X/tile), math.Floor(points[index].Y/tile)
	x := (float32(tileX) + waveSpawnInset + zombieRandom(p.rng, waveSpawnSpan)) * waveSpawnTile
	y := (float32(tileY) + waveSpawnInset + zombieRandom(p.rng, waveSpawnSpan)) * waveSpawnTile
	return formats.Vec2{X: float64(x), Y: float64(y)}
}
