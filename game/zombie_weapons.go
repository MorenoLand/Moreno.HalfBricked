package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Zombie guns (armed_zombie, and any zombie type with a weapon attribute).
//
// Evidence (libmortargame.so v7; Research/native/zombie-types-2026-10-09.md):
//   - The weapon id comes from the type's "weapon" string (FUN_000b13a8: PISTOL 0,
//     SHOTGUN 1, UZI 2, MINIGUN 3, SNIPER 4, BAZOOKA 5, GRENADE 6, MINE 7,
//     FLAMER 8, THROWN 9, VOMIT 10, CURVER 11, DYNOMITE 12, NONE/unknown 13).
//     FUN_000b1438 fills the zombie gun records (0x2c bytes, from +0xd4) from the
//     ZombieWeapons XML but skips id 13, so NONE is an all-zero record.
//   - FUN_000a0b80 copies the 11-word record to +0x2e4: id, recoil seconds (the
//     fire timer +0x2e8 starts here), rate of fire seconds (+0x2ec, interval),
//     spread units, rounds (+0x2f4, doubled on copy), pellets per shot, life,
//     bullet kind (FUN_000b13f0: NORMAL 0x10 .. COWPAT 0x1e), speed. Type 7
//     (armed_zombie) then starts the timer at -0.75 and multiplies the interval by 2.25.
//   - FUN_0009f54c (slot +0x48) decides whether the zombie may shoot: rounds >= 1,
//     squared distance <= the sight radius (110..160 px, x1.5 for types 6 and 7,
//     growing from 0 over 4 s) and a clear line. FUN_0009fb10 then moves
//     state 1 -> 2 (brake 4/s) -> 3 (wait for timer >= interval, FUN_0009f7e4 fires
//     and resets the timer) -> 4 (accelerate back to walking) -> 1.
//   - FUN_0009f7e4 (fire): rounds -1; per pellet a projectile of the gun's kind at
//     the zombie position, heading = facing +- 2*spread, width = 0.46875 * body
//     height (0.3 for venom), hit-player 1, hit-entities 0, owner = the zombie.
//     For kind 0x10 the projectile pass FUN_000a5f74 tests the player ellipse
//     (0.3 * 64 + 0.5 * width) and FUN_000a521c deals 0.1 health and removes the
//     bullet. A bullet also ends on life, or on a collision value 1 tile.
//
// UNRESOLVED (not implemented, the zombie then behaves as unarmed): VENOM
// (VOMIT, kind 0x17: as decompiled, Life 0 makes FUN_000a4288 remove it on the
// first update), SPINNER (CURVER, 0x18: orbit set up by FUN_000a6640), DYNOMITE
// (0x1d: bouncing bomb FUN_000a700c / blast FUN_000a4dc4). Their flight and
// landing rules were not recovered completely enough to port without guessing.
const (
	zombieGunInterval     = 2.25 // DAT_000a0da0: armed_zombie interval multiplier
	zombieGunArmedTimer   = -.75 // 0xbf400000
	zombieShotWidth       = .46875
	zombieShotPlayerHit   = .1  // DAT_000a5330
	zombieShotPlayerScale = .5  // DAT_000a63d8
	zombieBrakeRate       = 4.0 // DAT_0009f51c
	zombieSightRampCap    = 1.85
	zombieSightResetTimer = -.5
)

// zombieGun is the weapon state of one zombie (record words +0x2e4..+0x30c).
type zombieGun struct {
	rounds   int     // +0x2f4
	interval float64 // +0x2ec
	timer    float64 // +0x2e8
	spread   float64 // +0x2f0 (units)
	pellets  int     // +0x2f8
	life     float64
	speed    float64
	kind     uint8
}

// zombieShot is a projectile fired by a zombie (kind 0x10 only).
type zombieShot struct {
	projectile weapons.NativeWeaponProjectile
	age        float64
}

func zombieGunName(id int) string {
	for name, value := range zombieGunIDs {
		if value == id && name != "NONE" {
			return name
		}
	}
	return ""
}

// newZombieGun builds the gun of a zombie from its weapon id and the catalog;
// unsupported or missing guns have no rounds.
func newZombieGun(kind, gunID int, catalog formats.ZombieWeaponCatalog) zombieGun {
	name := zombieGunName(gunID)
	for _, record := range catalog {
		if record.GunType != name {
			continue
		}
		bullet, ok := zombieBulletKinds[record.BulletType]
		if !ok {
			bullet = 0x10
		}
		gun := zombieGun{rounds: record.Ammo * 2, interval: record.RateOfFire, timer: record.RecoilSeconds, spread: record.SpreadUnits, pellets: record.AmmoPerShot, life: record.Life, speed: record.Speed, kind: bullet}
		if kind == zombieKindArmed {
			gun.timer = zombieGunArmedTimer
			gun.interval *= zombieGunInterval
		}
		if gun.kind != 0x10 {
			gun.rounds = 0 // UNRESOLVED kinds: see the header comment
		}
		return gun
	}
	return zombieGun{}
}

// zombieMayShoot is FUN_0009f54c: the unconditional true in states 2 and 4.
func (p *playState) zombieMayShoot(z *zombieState, distSq float64, targetX, targetY float64) bool {
	ai := &z.native.ai
	if distSq <= zombieAimMinDistSq || ai.state == 2 || ai.state == 4 {
		return true
	}
	if ai.gun.rounds < 1 || ai.sightBase*ai.sightRamp < distSq {
		return false
	}
	return p.lineClear(z.x, z.y, targetX, targetY, 1)
}

// zombieStateMachine is FUN_0009f32c for states 2..4 of a gun carrying zombie.
func (p *playState) zombieStateMachine(z *zombieState, dt float64) {
	ai := &z.native.ai
	switch ai.state {
	case 2:
		ai.speedFactor -= dt * zombieBrakeRate
		if ai.speedFactor < 0 {
			ai.speedFactor, ai.state = 0, 3
		}
	case 3:
		if ai.gun.interval <= ai.gun.timer {
			p.zombieFire(z)
			return
		}
		ai.gun.timer = math.Min(ai.gun.timer+dt, ai.gun.interval)
	case 4:
		ai.speedFactor += dt * zombieBrakeRate
		if ai.speedFactor > 1 {
			ai.speedFactor, ai.state = 1, 1
		}
	}
}

// zombieFire is FUN_0009f7e4.
func (p *playState) zombieFire(z *zombieState) {
	ai := &z.native.ai
	gun := &ai.gun
	gun.timer = 0
	if gun.rounds < 1 {
		return
	}
	gun.rounds--
	if z.native.kind < 10 && ai.sightRamp < 1 {
		ai.sightRamp += gun.interval
		if ai.sightRamp >= zombieSightRampCap {
			ai.sightRamp, gun.timer = 0, zombieSightResetTimer
		}
	}
	jitter := int(gun.spread + gun.spread)
	for pellet := 0; pellet < gun.pellets; pellet++ {
		heading := z.native.facing
		if jitter > 0 {
			heading = heading - uint16(jitter) + uint16(zombieBounded(p.rng, uint32(jitter<<1)))
		}
		width := z.native.sizeZ * zombieShotWidth
		shot := zombieShot{projectile: weapons.NativeWeaponProjectile{X: z.x, Y: z.y, Direction: heading, Life: gun.life, Lift: width, Width: width, Height: -width, EntityType: gun.kind, Texture: "Common0/Textures/Bullet_SD"}}
		shot.projectile.VX = cosU16(heading) * gun.speed
		shot.projectile.VY = sinU16(heading) * gun.speed
		p.zombieShots = append(p.zombieShots, shot)
	}
}

// updateZombieShots moves zombie bullets and applies FUN_000a521c to the players.
func (p *playState) updateZombieShots(dt float64) {
	live := p.zombieShots[:0]
	for _, shot := range p.zombieShots {
		pr := &shot.projectile
		shot.age += dt
		pr.Age = shot.age
		pr.X += pr.VX * dt
		pr.Y += pr.VY * dt
		if shot.age >= pr.Life || p.projectileBlockedAt(pr.X, pr.Y) {
			continue
		}
		hit := false
		for _, target := range p.livingPlayers() {
			reach := zombieBodyScale*entityGridPlayerBody + zombieShotPlayerScale*pr.Width
			if zombieBlastEllipse(target.x-pr.X, target.y-pr.Y, reach) {
				p.damagePlayer(target.index, zombieShotPlayerHit)
				hit = true
				break
			}
		}
		if !hit {
			live = append(live, shot)
		}
	}
	p.zombieShots = live
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
