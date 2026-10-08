package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// The buzzsaw (weapon class 6, update 0x000a9f0c) and its SAW_BLADE projectile
// (entity kind 0x1b, vtable 0x005bb4f0). Evidence: Research/native/
// buzzsaw-dualpistol-2026-10-08.md.
//
//   - The weapon update runs every frame (0x000947e0) and never consults the
//     trigger: once the 0.03 s rate has elapsed it spawns three blades fanned at
//     facing - spread, facing, facing + spread and spends one ammo, on every frame
//     until the ammo is gone (the +0x14 timer is never reset). See tickGun.
//   - The blade has no draw routine (slot 5 is the empty stub 0x000a3bf4). It
//     lives 0.1 s, moves 300 px/s, dies in solid tiles, ignores the player and
//     overlaps zombies within 42 + .3*width on an ellipse (0x000a5f74).
//   - Contact (0x000a5338) with a living target credits 1.01 combo, plays
//     chainsaw_rev_1/2, damages the target by 9999 and keeps flying when the
//     target died; a dead or invulnerable target destroys the blade.
const sawBladeKind = 0x1b

// sawHitShakeDuration is the camera shake length each valid contact starts.
const sawHitShakeDuration = float64(float32(0.2))

// maxSfxPerFrame is the native sound channel table size (FUN_000ca410 scans 8
// channels), the most sounds one frame can start.
const maxSfxPerFrame = 8

// spawnSawVolley is the spawn loop of 0x000a9f0c: three blades from the weapon
// position along the player's facing - spread, facing and facing + spread. The
// native code reads the 16-bit heading +0x36; the port has the nine drawn
// facing columns (barryAimDirection).
func (p *playState) spawnSawVolley() {
	dirX, dirY := barryAimDirection(p.angle, p.flipX)
	volley, err := weapons.NativePrimaryVolley(p.weapon, weapons.NativeWeaponDirection(dirX, dirY), p.rng)
	if err != nil || volley.AmmoConsumed == 0 {
		return
	}
	origin := p.primaryOrigin(p.newShot())
	for _, shot := range volley.Shots {
		projectile, ok := weapons.NewNativeWeaponProjectile(shot, p.weapon.BulletType, p.x, p.y, p.rng)
		if !ok {
			return
		}
		p.bullets = append(p.bullets, bullet{x: projectile.X, y: projectile.Y, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, origin: origin})
	}
	p.weapon.Ammo -= volley.AmmoConsumed
	p.achieve.nonPistol = true
}

// stepSawBlade advances one blade by a frame (0x000a5f74 with the saw's move
// 0x000a6970 and contact 0x000a5338). It reports whether the blade survives.
func (p *playState) stepSawBlade(b *bullet) bool {
	const dt = 1.0 / 60.0
	projectile := b.projectile
	age := float32(projectile.Age) + float32(dt)
	projectile.Age = float64(age)
	b.x += b.vx * dt
	b.y += b.vy * dt
	projectile.X, projectile.Y = b.x, b.y
	if float32(projectile.Life) <= age || p.isSolid(b.x, b.y) {
		return false
	}
	sounds := 0
	dead := false
	extent := float32(weapons.SawBladeRadius) * weapons.SawBladeQueryScale
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if zombie.spawnAway || !weapons.SawBladeOverlap(zombie.x-b.x, zombie.y-b.y, zombie.size.X) {
			continue
		}
		// The pass lists an entity once per grid cell it is registered in
		// (entity_grid.go), so a zombie killed by the first listing destroys the
		// blade on the next one (0x000a8534 rejects health <= 0).
		for visit := p.zombieGridVisits(zombie, b.x, b.y, extent); visit > 0; visit-- {
			if zombie.health <= 0 || zombie.dying {
				dead = true
				continue
			}
			p.comboCredit(b.origin, weapons.SawBladeHitCredit)
			if sounds < maxSfxPerFrame {
				sounds++
				p.sfxQueue = append(p.sfxQueue, weapons.SawBladeHitSound(p.sawHitRandom()))
			}
			// 0x000a5338 also calls FUN_000be1fc, the camera shake start (camera_shake.go),
			// at the target's position: duration .2 (literal 0x3e4ccccd at 0x000a5428),
			// amplitudes and angle multiplier 1.0.
			p.startCameraShake(zombie.x, zombie.y, sawHitShakeDuration, 1, 1, 1)
			if zombie.invulnerable {
				// takeDamage returns without damage and +0x314 stays 0: the blade dies.
				dead = true
				continue
			}
			zombie.health -= weapons.SawBladeHitDamage
			zombie.hitFlash = zombieHitFlashDuration
			if zombie.health <= 0 {
				p.creditKill(b.origin)
				zombie.dying = true
				zombie.deathAge = 0
			}
		}
	}
	return !dead
}

func (p *playState) sawHitRandom() uint32 {
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
	return p.rng.Bounded(0)
}
