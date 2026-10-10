package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Player vitals recovered from libmortargame.so (v7) in the 2026-10-09 parity
// audit (Research/native/parity-audit-2026-10-09.md):
//
//   - FUN_00096818 (player update, lives >= 0 branch): while +0x98 > 0 it counts
//     down by dt; once it is <= 0 and 0 < health < 1 the health regrows by
//     dt * 0.75 (DAT_00096b78) up to 1.0. FUN_00094c6c sets +0x98 to
//     (1.0 - health) + 0.5 on every damaging hit, so the regeneration waits
//     0.5 s (full health) to 1.5 s (almost dead) after the last hit.
//   - FUN_000962f4 (respawn) sets +0x94 = 1500 and FUN_00094c6c ignores all damage
//     while +0x94 > 0; FUN_00096818 lowers it by (int)(dt * 1000) per frame (the
//     counter only runs down while +0x271 == 0, which the constructor and the
//     respawn clear and nothing else was found to set).
//   - The p_hover pickup (FUN_000928f4 type 0x25 -> FUN_0009625c) sets +0x3e8 = 2.0
//     (the int at 0x005cf1a4); FUN_00096818 lowers it by dt and scales the walk
//     speed by 2 * clamp(t + 0.5, 0.5, 1) = 1..2 (ramping down over the last
//     half second).
const (
	playerRegenRate      = .75 // health per second, DAT_00096b78
	playerRegenBaseDelay = .5  // DAT_00095044: delay = (1 - health) + 0.5
	playerRespawnGraceMS = 1500
	playerSpeedBoostTime = 2.0
	playerSpeedBoostMax  = 2.0
)

// playerVitals is the per-player part of that state (lives in bodyState too).
type playerVitals struct {
	regenDelay float64 // +0x98 while alive
	speedBoost float64 // +0x3e8
	invulnMS   int     // +0x94
}

// stepVitals runs once per frame for the active body.
func (p *playState) stepVitals(dt float64) {
	v := &p.vitals
	if v.invulnMS > 0 {
		v.invulnMS = max(0, v.invulnMS+int(dt*1000*-1))
	}
	if p.health <= 0 {
		v.speedBoost = 0
		return
	}
	v.speedBoost = math.Max(0, v.speedBoost-dt)
	if v.regenDelay > 0 {
		v.regenDelay -= dt
		return
	}
	limit := p.maxHealth
	if limit <= 0 {
		limit = 1
	}
	if p.health < limit {
		p.health = math.Min(limit, p.health+dt*playerRegenRate)
	}
}

// noteHit is the tail of FUN_00094c6c for a damaging hit.
func (v *playerVitals) noteHit(healthAfter float64) {
	v.regenDelay = (1 - healthAfter) + playerRegenBaseDelay
}

// playerWalkStick is the 0.81 scale of the stick input in the native player update
// (DAT_00096bfc; the same literal in both builds).
const playerWalkStick = 0.81

// playerWalkBase is the base walk speed of the native player update. v7 (SD cache,
// 1.2.1) reads DAT_00096bf0 = 200, giving 162 px/s; 1.2.5 (HD cache, the default)
// reads DAT_000f3f40 = 180, giving 145.8 px/s. The build comes from the level's cache.
func playerWalkBase(build string) float64 {
	if build == content.WaveBuildV7 {
		return playerWalkStick * 200
	}
	return playerWalkStick * 180
}

// walkSpeedFactor is fVar18 of FUN_00096818.
func (v *playerVitals) walkSpeedFactor() float64 {
	return playerSpeedBoostMax * math.Max(1/playerSpeedBoostMax, math.Min(1, v.speedBoost+1/playerSpeedBoostMax))
}

// collectHover is the p_hover pickup.
func (p *playState) collectHover() { p.vitals.speedBoost = playerSpeedBoostTime }

// startRespawnGrace is the +0x94 write of FUN_000962f4.
func (p *playState) startRespawnGrace() {
	p.vitals.invulnMS, p.vitals.regenDelay = playerRespawnGraceMS, 0
}

// outOfLives is native lives < 0 (FUN_00096818 tests +0x3ec < 0): the counter
// starts at 3, every death subtracts one and the player only stays down once it
// is negative, so a level allows four deaths. Co-op shares a pool that never goes
// below zero (port addition), so there zero means out.
func (p *playState) outOfLives() bool {
	return p.lives < 0 || (p.coopActive() && p.lives <= 0)
}

// playerBlockedByZombies would keep a hard body collision between Barry and zombies.
// The original has none: the player update (FUN_00096818, 1.2.5 FUN_000f3280) only
// collides with the level (FUN_000be3c0, 1.2.5 FUN_0012122c) and overwrites its
// velocity from the controls every frame. The zombie separation pass (FUN_000a1b08,
// 1.2.5 FUN_00100b80; separatePlayers in zombie_ai.go) pushes the zombie away from
// Barry, and pushes Barry only while a script runs with thumbsticks allowed.
// false = original behaviour.
var playerBlockedByZombies = false

// playerTileDisplacement is the player's level collision: the same FUN_000be3c0
// push the zombies use (zombie_tiles.go), radius = playerCollisionRadius.
func (p *playState) playerTileDisplacement(x, y, radius float64, tileSize int) (float64, float64, bool) {
	px, py := p.nativeTilePush(x, y, radius)
	return px, py, px != 0 || py != 0
}

// pickupReach is the collection distance of FUN_000928f4: the pickup is taken when
// (2 * |player - pickup|)^2 < size^2 * 1.25 (DAT_00092cf0 is the double 1.25), i.e.
// within size * sqrt(1.25) / 2 = 27.95 px for the 50 px crates (DAT_00092f88). The
// initialiser stores 80 (DAT_00092f84) instead when the special-crate flag +0x25d
// is set; its writer is UNRESOLVED (the initialiser itself always clears it), so the
// size the port draws the crate with (pickupDrawSize) decides. The port used 28 for
// every pickup.
func pickupReach(name string) float64 {
	return pickupDrawSize(name) * math.Sqrt(1.25) / 2
}

// respawnPoint is FUN_000bdf7c (called from FUN_00094c6c when the player dies): of
// all level start markers (collision value 3, scanned column by column) it picks
// the one with the least crowded surroundings, measured by FUN_00091ac8 as the sum
// of (entities + 1) over the entity-grid cells within 120 px (DAT_000be038); the
// first strictly lowest wins. Without a marker the port keeps the entry spawn.
func (p *playState) respawnPoint() (float64, float64) {
	if p.world == nil || p.world.Level.Layers[formats.LayerC] == nil || p.tileSize <= 0 {
		return p.spawnX, p.spawnY
	}
	level := p.world.Level
	layer := level.Layers[formats.LayerC]
	p.refreshEntityGrid()
	bestX, bestY, bestCrowd, found := p.spawnX, p.spawnY, 1_000_000_000, false
	for x := 0; x < level.Width; x++ {
		for y := 0; y < level.Height; y++ {
			if layer[y*level.Width+x] != 2 { // the port stores collision value 3 as 2
				continue
			}
			px, py := float64(x*p.tileSize+p.tileSize/2), float64(y*p.tileSize+p.tileSize/2)
			if crowd := p.respawnCrowd(px, py); crowd < bestCrowd {
				bestX, bestY, bestCrowd, found = px, py, crowd, true
			}
		}
	}
	if !found {
		return p.spawnX, p.spawnY
	}
	return bestX, bestY
}

func (p *playState) respawnCrowd(x, y float64) int {
	box := entityGridBox(x, y, respawnCrowdRadius)
	total := 0
	for cy := box.y0; cy <= box.y1; cy++ {
		for cx := box.x0; cx <= box.x1; cx++ {
			total++
			for index := range p.zombies {
				z := &p.zombies[index]
				if z.dying || z.health <= 0 || !z.grid.valid {
					continue
				}
				if cx >= z.grid.cells.x0 && cx <= z.grid.cells.x1 && cy >= z.grid.cells.y0 && cy <= z.grid.cells.y1 {
					total++
				}
			}
		}
	}
	return total
}

const respawnCrowdRadius = float32(120)
