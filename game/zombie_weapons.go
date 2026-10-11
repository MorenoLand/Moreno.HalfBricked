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
//     the zombie position, heading = facing +- 2*spread, visual lift (+0x78 of
//     the bullet, the "height" argument of the init FUN_000a4868) = 0.46875 * body
//     height (0.3 for venom), fall rate (+0x7c) = (lift - 23.6) / (distance to the
//     player / speed), hit-player 1, hit-entities 0, owner = the zombie. The bullet
//     keeps its own 10 x -20 size (FUN_000a57e4 doubles the base 5 x -10). FUN_000a5f74
//     lowers the lift by dt * fall and ends the bullet when it goes below 0.
//     Nothing in the fire path plays a sound: the hook the zombie calls after each
//     bullet (slot +0x64 v7 / +0x70 1.2.5) is a bare return for the plain zombie
//     class. The XML Sound_Cue (only the THROWN record has one) is a resource handle.
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
	zombieShotLift        = .46875 // DAT_0009fae4: bullet lift per unit of body height
	zombieShotLiftFloor   = 23.6   // DAT_0009faec 0x41bccccd
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
	sound    string // PORT ADDITION: fire sound, see portZombieGunSound
}

// zombieShot is a projectile fired by a zombie (kind 0x10 only).
type zombieShot struct {
	projectile weapons.NativeWeaponProjectile
	age        float64
	fall       float64 // +0x7c: lift lost per second
}

// PORT ADDITION (not in the original binary): zombie guns fire with the matching weapon sound. The native fire path
// plays none (see the header), so the original zombies shoot silently. Switch it off with -zombie-gun-sound=false.
var portZombieGunSound = true

var zombieGunSounds = map[string]string{"PISTOL": "SFX_HANDGUN", "SHOTGUN": "SFX_SHOTGUN_1", "UZI": "SFX_UZI", "MINIGUN": "SFX_MINIGUN", "SNIPER": "SFX_RIFLE_1"}

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
		gun := zombieGun{sound: zombieGunSounds[name], rounds: record.Ammo * 2, interval: record.RateOfFire, timer: record.RecoilSeconds, spread: record.SpreadUnits, pellets: record.AmmoPerShot, life: record.Life, speed: record.Speed, kind: bullet}
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
	if portZombieGunSound && gun.sound != "" {
		p.sfxQueue = append(p.sfxQueue, gun.sound)
	}
	lift := z.native.sizeZ * zombieShotLift
	fall := zombieShotFall(lift, gun.speed, p.zombieShotDistance(z))
	jitter := int(gun.spread + gun.spread)
	for pellet := 0; pellet < gun.pellets; pellet++ {
		heading := z.native.facing
		if jitter > 0 {
			heading = heading - uint16(jitter) + uint16(zombieBounded(p.rng, uint32(jitter<<1)))
		}
		shot := zombieShot{fall: fall, projectile: weapons.NativeWeaponProjectile{X: z.x, Y: z.y, Direction: heading, Life: gun.life, Lift: lift, Width: 10, Height: -20, EntityType: gun.kind, Texture: "Common0/Textures/Bullet_SD"}}
		shot.projectile.VX = cosU16(heading) * gun.speed
		shot.projectile.VY = sinU16(heading) * gun.speed
		p.zombieShots = append(p.zombieShots, shot)
	}
}

// zombieShotDistance is the distance from the zombie to the player the native fire code reads (the position the
// last player update stored; the nearest living player here). Both z values are 0 on the ground.
func (p *playState) zombieShotDistance(z *zombieState) float64 {
	best := math.Inf(1)
	for _, target := range p.livingPlayers() {
		best = math.Min(best, math.Hypot(target.x-z.x, target.y-z.y))
	}
	if math.IsInf(best, 1) {
		return 0
	}
	return best
}

// zombieShotFall is the +0x7c argument of the bullet init: (lift - 23.6) / (distance / speed), float32 as native.
func zombieShotFall(lift, speed, distance float64) float64 {
	flight := float32(distance) / float32(speed)
	return float64((float32(lift) - float32(zombieShotLiftFloor)) / flight)
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
