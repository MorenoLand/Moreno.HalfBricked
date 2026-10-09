package game

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
)

// Zombie spawn record, health and damage model.
//
// Evidence (libmortargame.so v7, Ghidra addresses with image base 0x10000; the
// ReAgent exports under Research/.codex/ghidra-exports hold the decompilations,
// details in Research/native/zombie-types-2026-10-09.md):
//
//   - The wave type parser FUN_000c3534 reads, per <type>: name, chance,
//     strength (default string "0"), texture, weapon (default "NONE"), speed,
//     turnSpeed, size, alertRadius, deviateCycleSpeed and deviateAmount. The six
//     numeric attributes are "a" or "a,b" integer ranges (FUN_000c2f3c/FUN_000c2fa0)
//     whose defaults are speed -1,-1, turnSpeed -1,-1, size 15,16, alertRadius
//     200,300, deviateCycleSpeed 40,50, deviateAmount 15,20. The earlier note that
//     the parser ignores "size" was wrong: the string "size" is read at 0x000b3874.
//   - The spawn pass FUN_000bec48 rolls each range (FUN_000bec10: min + rnd(max-min),
//     nothing is drawn when min == max) into the 12-word spawn record, in the order
//     size, speed, turnSpeed, deviateAmount, alertRadius, deviateCycleSpeed.
//     Record words: [0] x, [1] y, [2] z, [3] speed, [4] size, [5] strength,
//     [6] turnSpeed, [7] deviateCycleSpeed, [8] deviateAmount, [9] alertRadius,
//     [10] weapon id, [11..] texture.
//   - The shared initialiser FUN_000a0b80 (vtable slot +0x8 of every zombie class)
//     turns the record into the zombie: health +0x2a8 = max health +0x2d0 =
//     strength (kept 100 when the record strength is negative), half size [4]
//     (24 when absent) so that +0x28/+0x2c/+0x30 = 2*half, speed +0x27c = [3]
//     (30 + rnd(30) when negative), turn factor +0x2a4 = [6]/120 (0.01 when negative).
const (
	zombieKindPlain     = 2 // zombie
	zombieKindShield    = 3 // shield_zombie
	zombieKindExploding = 4
	zombieKindSpeedy    = 5
	zombieKindCharging  = 6
	zombieKindArmed     = 7
	zombieKindSmart     = 8
	zombieKindProspect  = 9

	// FUN_000a521c: every bullet hit (kinds 0x10 NORMAL and 0x11 THROUGH) calls the
	// zombie damage slot +0x5c with 0x65.
	zombieBulletDamage = 101
	// FUN_000a4bbc: flame particles deal 1 per hit.
	zombieFlameDamage = 1

	// FUN_000a0304 stores 0.1 (DAT_000a058c) in the hit timer +0x2ac on every
	// damaging hit; a shielded hit stores 0.05 (DAT_000a05a4).
	zombieDamageFlash  = .1
	zombieShieldFlash  = .05
	zombieFlashMinByte = 0x32 // FUN_0009fb10 clamps the flash alpha to 0x32
	// FUN_000a0304: a shield zombie (FUN_0009f158: type byte == 3) hit by a type
	// 0x10 bullet whose heading is within 0x1c6f (DAT_000a057c) of its own
	// heading + 0x3ffc takes damage/10 (__divsi3(damage, -10)) instead.
	zombieShieldArc          = 0x1c6f
	zombieShieldBulletOffset = 0x3ffc
	zombieShieldDivisor      = 10

	// FUN_000a0304: a surviving type 0x10 hit plays sound 0x24 + rnd(2).
	zombieHurtSoundCount = 2

	// FUN_000a5f74: the projectile pass tests dx^2 + (dy/0.666)^2 < reach^2 with
	// reach = projectile +0x28 + 0.3 * zombie +0x28 (DAT_000a63d4).
	zombieBodyScale = .3

	// FUN_000a0b80 / FUN_0009fb10: after the spawn rise the body is
	// +0x28 = 0.796875 * +0x30 wide and +0x2c = +0x30 tall.
	zombieBodyWidthFactor = .796875
)

// zombieGunIDs is the zombie weapon table behind FUN_000b13a8 (hash table filled
// by the initialiser at 0x000b2900: indices 0..13).
var zombieGunIDs = map[string]int{"PISTOL": 0, "SHOTGUN": 1, "UZI": 2, "MINIGUN": 3, "SNIPER": 4, "BAZOOKA": 5, "GRENADE": 6, "MINE": 7, "FLAMER": 8, "THROWN": 9, "VOMIT": 10, "CURVER": 11, "DYNOMITE": 12, "NONE": 13}

// zombieBulletKinds is FUN_000b13f0: the index of Bullet_Type in the table filled
// at 0x000b2b4c plus 0x10 (NORMAL 0x10 .. COWPAT 0x1e); an unknown name is 0x10.
var zombieBulletKinds = map[string]uint8{"NORMAL": 0x10, "THROUGH": 0x11, "MULTI": 0x12, "BAZOOKA": 0x13, "GRENADE": 0x14, "MINE": 0x15, "FLAME": 0x16, "VENOM": 0x17, "SPINNER": 0x18, "FLYER": 0x19, "BLAST": 0x1a, "SAW_BLADE": 0x1b, "SENTRY_SPAWN": 0x1c, "DYNOMITE": 0x1d, "COWPAT": 0x1e}

// zombieSpawnRecord is the rolled spawn record of one zombie.
type zombieSpawnRecord struct {
	Kind        int
	HalfSize    int // [4]
	Speed       int // [3]; negative means "not specified"
	Strength    int // [5]
	TurnSpeed   int // [6]; negative means "not specified"
	CycleSpeed  int // [7]
	Amount      int // [8]
	AlertRadius int // [9]
	Gun         int // [10]
}

// zombieRange converts a parsed attribute into the native integer range and
// normalises it like FUN_000c2fa0 (min first). A zero pair stands for an absent
// attribute and yields the parser default.
func zombieRange(value formats.Vec2, fallbackLo, fallbackHi int) (int, int) {
	lo, hi := int(value.X), int(value.Y)
	if value.X == 0 && value.Y == 0 {
		return fallbackLo, fallbackHi
	}
	if hi < lo {
		lo, hi = hi, lo
	}
	return lo, hi
}

// zombieRoll is FUN_000bec10: min + rnd(max-min), no draw when the range is a point.
func zombieRoll(lo, hi int, rng *weapons.NativeRNG) int {
	if lo == hi || rng == nil {
		return lo
	}
	return lo + int(rng.Bounded(uint32(hi-lo)))
}

// rollZombieSpawnRecord builds the spawn record of one wave type the way
// FUN_000bec48 does (including the order in which the random numbers are drawn).
func rollZombieSpawnRecord(entry formats.SpawnType, rng *weapons.NativeRNG) zombieSpawnRecord {
	record := zombieSpawnRecord{Kind: nativeZombieTypes[entry.Name], Gun: 13, Strength: -1}
	if gun, ok := zombieGunIDs[entry.Weapon]; ok {
		record.Gun = gun
	}
	if entry.Strength >= 0 {
		record.Strength = int(entry.Strength)
	}
	lo, hi := zombieRange(entry.Size, 15, 16)
	record.HalfSize = zombieRoll(lo, hi, rng)
	lo, hi = zombieRange(entry.Speed, -1, -1)
	record.Speed = zombieRoll(lo, hi, rng)
	record.TurnSpeed = -1
	if entry.TurnSpeed != 0 {
		record.TurnSpeed = int(entry.TurnSpeed)
	}
	record.Amount = int(entry.DeviateAmount)
	if entry.DeviateAmount == 0 {
		lo, hi = 15, 20
		record.Amount = zombieRoll(lo, hi, rng)
	}
	record.AlertRadius = int(entry.AlertRadius)
	if entry.AlertRadius == 0 {
		record.AlertRadius = zombieRoll(200, 300, rng)
	}
	record.CycleSpeed = int(entry.DeviateCycleSpeed)
	if entry.DeviateCycleSpeed == 0 {
		record.CycleSpeed = zombieRoll(40, 50, rng)
	}
	return record
}

// zombieNative is the per-zombie state that comes from the native class. A zero
// kind marks a zombie that was not built from a spawn record (staged zombies in
// tests and capture scenes), which keeps the simple chase of the earlier port.
type zombieNative struct {
	kind       int
	maxHealth  float64
	sizeZ      float64 // +0x30 = 2 * half size
	brightness float64 // +0x2b8: 0.8 + rnd(0.199)
	facing     uint16  // +0x36
	ai         zombieAI
	trail      zombieTrail
}

// zombieBodyWidth is +0x28 once the zombie has risen.
func zombieBodyWidth(sizeZ float64) float64 {
	return float64(float32(sizeZ) * float32(zombieBodyWidthFactor))
}

// zombieHit describes one damaging contact for FUN_000a0304.
type zombieHit struct {
	damage int
	// kind is the projectile's entity byte (+0x35); 0 for non-projectiles.
	kind uint8
	// direction is the projectile's heading (+0x36).
	direction uint16
}

// hurtZombie is the zombie damage slot FUN_000a0304 for the plain zombie classes
// (the boss classes subtract the damage without the shield rule). It returns
// true when the zombie is dead afterwards (the original returns 1 only on the
// killing hit). The caller handles the kill bookkeeping.
func (p *playState) hurtZombie(z *zombieState, hit zombieHit) bool {
	if z.invulnerable {
		return false
	}
	if z.dying || z.health <= 0 {
		return true
	}
	amount, flash := float64(hit.damage), zombieDamageFlash
	if z.native.kind == zombieKindShield && hit.kind == 0x10 {
		if absInt(angleDifference(z.native.facing, hit.direction+zombieShieldBulletOffset)) <= zombieShieldArc {
			amount, flash = float64(hit.damage/zombieShieldDivisor), zombieShieldFlash
		}
	}
	z.health -= amount
	z.hitFlash = flash
	if z.health > 0 {
		if hit.kind == 0x10 {
			p.queueZombieHurtSound()
		}
		return false
	}
	return true
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// queueZombieHurtSound is FUN_000ca410(0x24 + rnd(2)): zombie_hurt_1 / _2. The
// symbol is queued at most once per frame.
func (p *playState) queueZombieHurtSound() {
	name := "SFX_ZOMBIE_HURT_1"
	if p.rng != nil && p.rng.Bounded(zombieHurtSoundCount) == 1 {
		name = "SFX_ZOMBIE_HURT_2"
	}
	for _, queued := range p.sfxQueue {
		if queued == "SFX_ZOMBIE_HURT_1" || queued == "SFX_ZOMBIE_HURT_2" {
			return
		}
	}
	p.sfxQueue = append(p.sfxQueue, name)
}

// bulletReachesZombie is the hit test of the projectile pass FUN_000a5f74: the
// zombie must be registered in a grid cell of the projectile's query box
// (+-0.5 * width) and inside the ellipse
// dx^2 + (dy/0.666)^2 < (width + 0.3 * zombie body width)^2.
func (p *playState) bulletReachesZombie(b bullet, z *zombieState) bool {
	width := 10.0
	if b.projectile != nil && b.projectile.Width > 0 {
		width = b.projectile.Width
	}
	if !zombieBlastEllipse(z.x-b.x, z.y-b.y, width+zombieBodyScale*z.size.X) {
		return false
	}
	return p.zombieGridVisits(z, b.x, b.y, float32(width)*zombieBlastQueryScale) > 0
}

// damageFromBullet applies one bullet or flame particle hit to a living zombie
// and reports whether it died. The tracker credit is paid before the damage, as
// in FUN_000a521c.
func (p *playState) damageFromBullet(b bullet, z *zombieState) bool {
	kind := uint8(0x10)
	damage := zombieBulletDamage
	var direction uint16
	if b.projectile != nil {
		kind, direction = b.projectile.EntityType, b.projectile.Direction
		if kind == 0x16 {
			damage = zombieFlameDamage
		}
	}
	lethal := p.hurtZombie(z, zombieHit{damage: damage, kind: kind, direction: direction})
	p.comboBulletHit(b, !lethal)
	return lethal
}

// bulletStopsAtFirstTarget is the tail of FUN_000a5f74: after the first target
// the pass returns for entity kinds 0x10, 0x11 and 0x18; every other kind (flame
// particles, blasts) keeps listing the remaining entities.
func bulletStopsAtFirstTarget(b bullet) bool {
	if b.projectile == nil {
		return true
	}
	switch b.projectile.EntityType {
	case 0x10, 0x11, 0x18:
		return true
	}
	return false
}

// zombieTint is the vertex colour of FUN_0009fb10: the colour byte is
// alpha(+0x2b0) * brightness(+0x2b8) with +0x2b0 ramping down to 0x32 while the
// hit timer runs and back to 0xff afterwards, so a hit flashes the zombie dark
// (not red) for 0.1 s, and every zombie is 0.8..1.0 as bright as the art.
func zombieTint(z zombieState) float32 {
	brightness := z.native.brightness
	if brightness <= 0 {
		brightness = 1
	}
	if z.hitFlash > 0 {
		return float32(zombieFlashMinByte) / 255 * float32(brightness)
	}
	return float32(brightness)
}

// zombieDeathPop is the death presentation of FUN_000a10fc for a normal kill
// (state 1): rnd(6) < 3 (or a boss type) plays pop animation 0; 3 or 4 plays
// animation 1 when the heading is mostly sideways (|cos| > 0.5) with the body
// shifted by 0.5 body widths away from the facing side, else animation 2; 5
// plays animation 2. (The sprite flip and the doubled width of animation 1 are
// not drawn by the port.)
func (p *playState) zombieDeathPop(z zombieState) (variant int, x float64) {
	x = z.x
	roll := zombieBounded(p.rng, 6)
	if roll < 3 || z.native.kind > zombieKindProspect {
		return 0, x
	}
	if roll < 5 {
		cos := cosU16(z.native.facing)
		if math.Abs(cos) > .5 {
			if cos < 0 {
				return 1, x + z.size.X*.5
			}
			return 1, x - z.size.X*.5
		}
	}
	return 2, x
}

// queueZombieDeathSound is FUN_0009f7ac: zombie_death_01..04 (0x26 + rnd(4)).
func (p *playState) queueZombieDeathSound() {
	n := uint32(0)
	if p.rng != nil {
		n = p.rng.Bounded(4)
	}
	p.sfxQueue = append(p.sfxQueue, fmt.Sprintf("SFX_ZOMBIE_DEATH_%02d", n+1))
}
