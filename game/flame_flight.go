package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Flame flight and gib flames (entity kind 0x16).
//
// Update FUN_000a6a58 (v7) / FUN_00106fe8 (1.2.5), both builds with the same literals:
//   - the direction vector is multiplied by 0.975 on every update (the speed +0x60 itself
//     only changes through a hit, x0.85, or a wall, 0), so the travelled distance is
//     speed * dt * sum(0.975^n), not speed * age;
//   - the height rises: height -= dt * vz where vz starts at 0 and gains -50 per second;
//   - while the speed is <= 0.1 the body grows by 5 px per second;
//   - the hit flags (+0x58 player, +0x59 zombies) are only live on every third update,
//     counter +0x80 cycles 2, 1, 0 starting at 0 (live updates 1, 4, 7, ...).
//
// Gib flames: a zombie in the gib hit phase (death state 2) counts a global countdown down
// once per frame; below zero it spawns one flame (v7 FUN_000a10fc 0x000a12b4.., 1.2.5
// FUN_001002bc) at its own position: heading Bounded(0xff3a), start height
// +0x2d4 * +0x30 (0.35 * body size, DAT_000a0e90), speed 100, hits the player and zombies,
// weapon class 5, owner the zombie. The countdown restarts at 50 + Bounded(20). The flame's
// life (0.7 + rnd 0.2) and width (20 + rnd 16) come from the shared post-init FUN_000a6cf8.
// The owner's direction vector added by that post-init (x0.5) is zero for a zombie and
// UNRESOLVED for the player (not ported for player flames either).
const (
	gibFlameHeightScale = float32(0.35) // DAT_000a0e90 (+0x2d4 default)
	flameFrameTime      = 1.0 / 60.0
)

// flameUpdateCount is the ordinal of the update of the flame that a stage of the bullet
// loop belongs to. age is the projectile's age counted in whole 60 Hz updates.
func flameUpdateCount(age float64) int { return int(math.Round(age / flameFrameTime)) }

// flameSpeed recovers the native speed (+0x60) from the decayed velocity.
func flameSpeed(b *bullet, updates int) float64 {
	decay := math.Pow(float64(weapons.FlameDirDecay), float64(updates))
	if decay <= 0 {
		return 0
	}
	return math.Hypot(b.vx, b.vy) / decay
}

// stepFlameFlight is the first half of FUN_000a6a58: direction decay and rise. It runs
// before the position step. The projectile's age still counts the previous updates.
func (p *playState) stepFlameFlight(b *bullet) {
	if b.projectile == nil || b.projectile.EntityType != 0x16 {
		return
	}
	decay := float64(weapons.FlameDirDecay)
	b.vx = float64(float32(b.vx) * float32(decay))
	b.vy = float64(float32(b.vy) * float32(decay))
	b.projectile.VX, b.projectile.VY = b.vx, b.vy
	b.projectile.Lift += float64(float32(flameFrameTime) * weapons.FlameRiseAccel * float32(b.projectile.Age))
}

// growFlame is the second half: a flame that has stopped (speed <= 0.1) widens by 5 px
// per second. updates is the count of updates including the current one.
func (p *playState) growFlame(b *bullet) {
	if b.projectile == nil || b.projectile.EntityType != 0x16 {
		return
	}
	if flameSpeed(b, flameUpdateCount(b.projectile.Age)) > float64(weapons.FlameStopSpeed) {
		return
	}
	b.projectile.Width += float64(float32(flameFrameTime) * weapons.FlameGrowRate)
	b.projectile.Height = -b.projectile.Width
}

// flameLive reports whether the flame's hit flags are set in the update that just ran
// (age already includes it).
func flameLive(b bullet) bool {
	if b.projectile == nil || b.projectile.EntityType != 0x16 {
		return true
	}
	return (flameUpdateCount(b.projectile.Age)-1)%weapons.FlameHitPeriod == 0
}

// flameHitPlayers is the player branch of FUN_000a5f74 for a flame that has its player
// flag set: ellipse dx^2 + (dy/0.666)^2 < (0.3 * 64 + 0.5 * width)^2, then FUN_000a4bbc
// FUN_00094c6c(player, frame delta, 0) per live update. The flame is not consumed.
func (p *playState) flameHitPlayers(b bullet) {
	if b.projectile == nil {
		return
	}
	for _, target := range p.livingPlayers() {
		reach := zombieBodyScale*entityGridPlayerBody + zombieShotPlayerScale*b.projectile.Width
		if zombieBlastEllipse(target.x-b.x, target.y-b.y, reach) {
			p.damagePlayer(target.index, flameFrameTime)
		}
	}
}

// stepGibFlames runs once per frame for a zombie in the gib hit phase.
func (p *playState) stepGibFlames(z *zombieState) {
	if !z.dying || z.deathState != zombieDeathGib || z.gibbed || z.deathAge >= p.zombieDeathDelayFor(*z) {
		return
	}
	p.gibFlameCountdown--
	if p.gibFlameCountdown >= 0 {
		return
	}
	p.gibFlameCountdown = weapons.GibFlameCountMin + int(zombieBounded(p.rng, weapons.GibFlameCountVar))
	heading := uint16(zombieBounded(p.rng, weapons.GibFlameBound))
	body := z.native.sizeZ
	if body <= 0 {
		body = z.size.Y
	}
	lift := float64(gibFlameHeightScale * float32(body))
	projectile, ok := weapons.NewGibFlame(z.x, z.y, heading, lift, p.rng)
	if !ok {
		return
	}
	p.bullets = append(p.bullets, bullet{x: z.x, y: z.y, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, hostile: true})
}
