package game

import (
	"image"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Gib deaths.
//
// A lethal hit leaves a zombie in death state 2 (v7 +0x314, 1.2.5 +0x330) when the
// attacker is a flame (projectile kind 0x16) or a blast of kind 0x13..0x15 whose centre
// is more than 48 px from the zombie. Every other lethal hit is death state 1, the normal
// death (main.go's removal loop with zombieDeathPop in zombie_model.go), left unchanged.
//
// Natives: v7 FUN_000a0304 (takeDamage) and FUN_000a10fc (death update); 1.2.5
// FUN_00116544 (takeDamage, the name Ghidra gives 0x000ff16c) and FUN_001002bc (death
// update). Both builds read the same constants (v7 0x000a0570 = 48.0, 0x000a0574 = 1.0,
// 0x000a0578 = 1.6, 0x000a0598 = 0.2; 1.2.5 0x000ff3dc = 48.0, 0x000ff3e0 = 1.0,
// 0x000ff3e4 = 1.6, 0x000ff3fc = 0.2). Gib research: Research/native/gib-death-2026-10-09.md.

// Native death states.
const (
	zombieDeathNormal = 1 // pop animation, then fade (unchanged)
	zombieDeathGib    = 2 // Charred while the hit timer runs, then Disintegrate
)

// zombieGibConstants are the gib constants of one build.
type zombieGibConstants struct {
	reach     float64 // blast distance beyond which the victim gibs (48 px)
	hitBase   float64 // hit timer of a gib in seconds (1.6)
	hitSpread float64 // the draw that shortens it: hit = (1 - draw * spread) * base (0.2)
}

// zombieGibV7 is libmortargame.so (v7), the SD cache (1.2.1); zombieGib125 is 1.2.5, the HD cache.
var (
	zombieGibV7  = zombieGibConstants{reach: 48, hitBase: 1.6, hitSpread: 0.2}
	zombieGib125 = zombieGibConstants{reach: 48, hitBase: 1.6, hitSpread: 0.2}
)

// zombieGibConstantsFor picks the constants of the build the level was loaded from (waveBuild).
func zombieGibConstantsFor(build string) zombieGibConstants {
	if build == content.WaveBuild125 {
		return zombieGib125
	}
	return zombieGibV7
}

// The ZombieDeaths clips a gib plays on the zombie's own sprite. Index 1 is Charred
// (v7 FUN_000a10c0(z, 1) in the hit phase, 1.2.5 FUN_00100080(z, 1)) and index 2 is
// Disintegrate (the death transition, FUN_000a10c0(z, 2) with r1 = death state 2 in both
// builds). The names come from the native tables filled by _INIT_45 (v7) and _INIT_60
// (1.2.5); frames and fps come from the active cache's sprite catalog.
const (
	zombieGibHitClip   = "Charred"
	zombieGibDeathClip = "Disintegrate"
)

// zombieBlastKind is the native projectile kind of a blast class, from the blast's origin:
// bazooka rockets 0x13, grenades 0x14, and mines plus the exploding zombie's blast 0x15.
// Dynamite (COWPAT, 0x1e) and every other origin return 0, which never gibs.
func zombieBlastKind(origin killOrigin) uint8 {
	switch origin.gun {
	case "ROCKET":
		return 0x13
	case "GRENADE":
		return 0x14
	case "MINE", "EXPZOMBIE":
		return 0x15
	}
	return 0
}

// zombieDeathStateFor is the death-state choice of takeDamage for a lethal hit by a
// projectile of the given kind, distance being the attacker's distance from the zombie.
func zombieDeathStateFor(kind uint8, distance float64, c zombieGibConstants) int {
	if kind == 0x16 {
		return zombieDeathGib
	}
	if kind >= 0x13 && kind <= 0x15 && distance > c.reach {
		return zombieDeathGib
	}
	return zombieDeathNormal
}

// zombieGibHitTime is the hit timer of a gib: (1 - draw * spread) * base seconds, with
// draw = Bounded(0x7ffff) / 524287.5 (v7 FUN_000a24dc over FUN_00079634, 1.2.5
// FUN_000dd3ac over the same generator). It lies in (base*(1-spread), base].
func zombieGibHitTime(rng *weapons.NativeRNG, c zombieGibConstants) float64 {
	var bounded uint32
	if rng != nil {
		bounded = rng.Bounded(0x7ffff)
	}
	draw := float32(bounded) / float32(524287.5) * float32(c.hitSpread)
	return float64((float32(1) - draw) * float32(c.hitBase))
}

// bulletNativeKind is the native projectile kind of a bullet (0x10 for plain shots).
func bulletNativeKind(b bullet) uint8 {
	if b.projectile != nil {
		return b.projectile.EntityType
	}
	return 0x10
}

// zombieIsBoss reports whether the zombie is a boss, whose damage is not takeDamage.
func (p *playState) zombieIsBoss(z zombieState) bool {
	entity := p.scriptEntities[z.scriptID]
	return entity != nil && isBossType(entity.entityType)
}

// classifyZombieDeath sets the death state and hit timer of a zombie that a lethal hit
// has just marked dying. The hit came from a projectile of the given native kind at (x,
// y). Zombies that already have a state, and zombies that are not dying, are left alone.
// Only gib draws use the generator, so normal deaths keep the random sequence intact.
func (p *playState) classifyZombieDeath(z *zombieState, kind uint8, x, y float64) {
	if !z.dying || z.deathState != 0 {
		return
	}
	c := zombieGibConstantsFor(p.waveBuild())
	z.deathState = zombieDeathNormal
	z.deathDelay = zombieDeathDelay
	if p.zombieIsBoss(*z) {
		return
	}
	if zombieDeathStateFor(kind, math.Hypot(x-z.x, y-z.y), c) == zombieDeathGib {
		z.deathState = zombieDeathGib
		z.deathDelay = zombieGibHitTime(p.rng, c)
	}
}

// zombieDeathClip returns one clip of the ZombieDeaths sprite.
func zombieDeathClip(catalog formats.SpriteCatalog, name string) (formats.SpriteAnimation, bool) {
	definition, ok := catalog.Find("ZombieDeaths")
	if !ok {
		return formats.SpriteAnimation{}, false
	}
	return definition.Animation(name)
}

// zombieGibStripCell is the cell of one frame of a one-angle gib strip whose frames run
// across the texture (Disintegrate: 512x128 or 256x64 for five frames, cells 0.8 aspect).
func zombieGibStripCell(frame, frames int, bounds image.Rectangle) image.Rectangle {
	return barryCellRect(frame, 0, frames, 1, bounds.Dx(), bounds.Dy())
}

// zombieGibClipLength is the playing time of the one-shot Disintegrate clip (frames / fps).
func zombieGibClipLength(catalog formats.SpriteCatalog) float64 {
	animation, ok := zombieDeathClip(catalog, zombieGibDeathClip)
	if !ok || animation.Frames <= 0 || animation.FPS <= 0 {
		return 0
	}
	return float64(animation.Frames) / animation.FPS
}

// zombieGibClip is the clip a zombie draws in place of its walk or idle animation. While
// a gib's hit timer runs it is Charred, looping on deathAge. A gib body after its death
// transition is Disintegrate, one shot on presentAge. ok is false for every other zombie.
func (a *app) zombieGibClip(z zombieState) (formats.SpriteAnimation, int, bool) {
	switch {
	case z.gibbed:
		animation, found := zombieDeathClip(a.sprites, zombieGibDeathClip)
		if !found || animation.Frames <= 0 || animation.FPS <= 0 {
			return formats.SpriteAnimation{}, 0, false
		}
		frame := min(int(math.Floor(z.presentAge*animation.FPS)), animation.Frames-1)
		return animation, max(frame, 0), true
	case z.dying && z.deathState == zombieDeathGib:
		animation, found := zombieDeathClip(a.sprites, zombieGibHitClip)
		if !found || animation.Frames <= 0 || animation.FPS <= 0 {
			return formats.SpriteAnimation{}, 0, false
		}
		frame := int(math.Floor(z.deathAge*animation.FPS)) % animation.Frames
		return animation, max(frame, 0), true
	}
	return formats.SpriteAnimation{}, 0, false
}

// addZombieGibBody is the death transition of a gib (state 6 with death state 2). The
// zombie leaves the live list as any death does, but no blood pop is made. A copy of it
// stays where it fell and plays Disintegrate once.
func (p *playState) addZombieGibBody(z zombieState) {
	z.gibbed = true
	z.presentAge = 0
	p.gibBodies = append(p.gibBodies, z)
}

// updateZombieGibBodies advances the Disintegrate presentations and drops each body when
// its one-shot clip has finished (FUN_000f1e28 reads the clip's playing flag at +0x20 in
// v7 and +0x24 in 1.2.5; there is no fade and no blood).
func (p *playState) updateZombieGibBodies(dt float64) {
	if len(p.gibBodies) == 0 {
		return
	}
	length := zombieGibClipLength(p.sprites)
	active := p.gibBodies[:0]
	for _, body := range p.gibBodies {
		body.presentAge += dt
		if body.presentAge < length {
			active = append(active, body)
		}
	}
	p.gibBodies = active
}
