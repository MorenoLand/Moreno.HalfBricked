package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Native particles. The class is entity kind 0x3c in v7 (1.2.5: 0x41): spawner FUN_000b346c (1.2.5 FUN_00114a7c), the
// init FUN_000b3298 (FUN_001148a4), the per-frame step FUN_000b3144 (FUN_00114734) and the draw FUN_000b35e0
// (FUN_00114be4). The six type records are filled by one loader (v7 0x000b3d94, 1.2.5 FUN_001155d0) with the same
// literals in both builds. Callers decompiled for this port:
//
//	type 0 (bloodsplatter_02, 2 per call, 0x1554 spread, speed 15, lift 10): a bullet of entity kind 0x10 hits a
//	  zombie, the Don Zombie car, the egyptian, the western boss or the robot (v7 FUN_000a0304, FUN_000b5410,
//	  FUN_000b6f24, FUN_000b848c, FUN_000bae44; 1.2.5 FUN_00116544, FUN_00117378, FUN_00119098, FUN_0011aa64,
//	  FUN_0011daf8); the rex and the samurai reach FUN_000a0304 through the shared boss damage slot FUN_000b4a98.
//	type 1 (same texture, bigger, 1 per call, full circle, speed 30.5, lift 10): the death spurt of a zombie in death
//	  state 1 while its hit timer is above 0.06 (v7 FUN_000a10fc, 1.2.5 FUN_001002bc).
//	type 5 (charge, 1 per call, stationary, 8 frames over 0.75 s): the muzzle flash that opens the robot's minigun
//	  sweep (v7 FUN_000bab94, 1.2.5 FUN_0011d860).
//
// Types 1 and 2 are also spawned by the player's damage and death (FUN_00094c6c, FUN_000962f4: player_hit_feedback.go)
// and by the rex's death (FUN_000b9624 with type 2: rex_death.go); type 4 (blue) is never used by a call site found.
const (
	particleKindHit    = 0
	particleKindSpurt  = 1
	particleKindFlash  = 5
	particleHitSpread  = 0x1554
	particleHitCount   = 2
	particleHitSpeed   = 15.0 // 0x41780000
	particleHitLift    = 10.0 // 0x41200000
	particleSpurtSpeed = 30.5 // 0x41f40000
	particleSpurtLift  = 10.0
	particleSpurtFull  = 0xff3a // spread of the death spurt: almost the whole circle
	particleSpurtAbove = 0.06   // the hit timer must be above this (0x3d75c28f)
	particleFlashRaise = 5.0    // y offset of the flash (DAT_000bad5c / 1.2.5 DAT_0011da18)
	particleFlashLift  = 100.0  // its height (DAT_000bad74 / DAT_0011da30)
	particleHalf       = 0.5    // DAT_000b35d4
	particleTick       = 1.0 / 60.0
)

// particleSpec is one 0x4c-byte type record.
type particleSpec struct {
	speedStart, speedEnd [2]float32 // +0x00, +0x04 / +0x08, +0x0c: random ranges of the start and end speed
	sizeStart, sizeEnd   [2]float32 // +0x10, +0x14 / +0x18, +0x1c
	gravity              float32    // +0x20
	colourStart          [4]uint8   // +0x24 RGBA at age 0
	colourEnd            [4]uint8   // +0x28 RGBA at age 1
	life                 [2]float32 // +0x30, +0x34 seconds
	bounce, friction     float32    // +0x38 (negative: no bounce), +0x3c
	frames               int        // +0x40 horizontal frames of the sheet
	texture              string     // +0x48
}

var particleSpecs = [6]particleSpec{
	{speedStart: [2]float32{70, 75}, speedEnd: [2]float32{60, 65}, sizeStart: [2]float32{16, 24}, sizeEnd: [2]float32{20, 26}, gravity: 100,
		colourStart: [4]uint8{255, 255, 255, 255}, colourEnd: [4]uint8{255, 255, 255, 255}, life: [2]float32{.25, .5},
		bounce: .35, friction: .95, frames: 4, texture: "bloodsplatter_02"},
	{speedStart: [2]float32{70, 75}, speedEnd: [2]float32{60, 65}, sizeStart: [2]float32{25, 30}, sizeEnd: [2]float32{25, 35}, gravity: 100,
		colourStart: [4]uint8{255, 255, 255, 255}, colourEnd: [4]uint8{255, 255, 255, 255}, life: [2]float32{.55, .6},
		bounce: .35, friction: .95, frames: 4, texture: "bloodsplatter_02"},
	// Type 2 is a copy of type 0 with larger sizes.
	{speedStart: [2]float32{70, 75}, speedEnd: [2]float32{60, 65}, sizeStart: [2]float32{50, 65}, sizeEnd: [2]float32{55, 70}, gravity: 100,
		colourStart: [4]uint8{255, 255, 255, 255}, colourEnd: [4]uint8{255, 255, 255, 255}, life: [2]float32{.25, .5},
		bounce: .35, friction: .95, frames: 4, texture: "bloodsplatter_02"},
	// Type 3 is never filled in: all zero, one frame, no texture.
	{frames: 1},
	{speedStart: [2]float32{90, 165}, speedEnd: [2]float32{80, 160}, sizeStart: [2]float32{19, 25}, sizeEnd: [2]float32{9, 12}, gravity: 50,
		colourStart: [4]uint8{0, 0x32, 0xf0, 0xff}, colourEnd: [4]uint8{0, 0xf0, 0xff, 0}, life: [2]float32{.25, .5},
		bounce: -1, friction: 1, frames: 1, texture: "particle"},
	{sizeStart: [2]float32{50, 55}, sizeEnd: [2]float32{50, 55}, gravity: 10,
		colourStart: [4]uint8{255, 255, 255, 255}, colourEnd: [4]uint8{255, 255, 255, 255}, life: [2]float32{.75, .75},
		bounce: .35, friction: .95, frames: 8, texture: "charge"},
}

// particle is one live particle (v7 offsets: position +0x10 / +0x14 / +0x18, direction +0x1c / +0x20, height speed
// +0x24, size +0x28 between +0x2c and +0x30, speed +0x40 / +0x44, age +0x48 / +0x4c, type +0x3c).
type particle struct {
	x, y, z            float32
	dirX, dirY         float32
	vz                 float32
	size, size0, size1 float32
	speed, speedRate   float32
	age, ageRate       float32
	kind               int
}

// spawnParticles is FUN_000b346c: count particles of one type at (x, y, z), flying around heading +- spread / 2 at
// speed with a height speed of speed + rnd(lift) - lift / 2. Per particle the shared generator is drawn six times:
// the angle, the height jitter, then the init's life, start speed, end speed and size.
func (p *playState) spawnParticles(kind int, x, y, z float64, heading uint16, spread uint32, count int, speed, lift float32) {
	if kind < 0 || kind >= len(particleSpecs) {
		return
	}
	for i := 0; i < count; i++ {
		angle := uint16(int(heading)-int(int32(spread)>>1)) + uint16(zombieBounded(p.rng, spread))
		vx := float32(cosU16(angle)) * speed
		vy := float32(sinU16(angle)) * speed
		vz := (speed + zombieRandom(p.rng, lift)) - lift*particleHalf
		p.particles = append(p.particles, p.newParticle(kind, float32(x), float32(y), float32(z), vx, vy, vz))
	}
}

// newParticle is the init FUN_000b3298. The velocity is normalised as a 3D vector for its direction (FUN_00098338);
// the height speed keeps its raw value.
func (p *playState) newParticle(kind int, x, y, z, vx, vy, vz float32) particle {
	spec := &particleSpecs[kind]
	pt := particle{x: x, y: y, z: z, kind: kind}
	life := zombieRandom(p.rng, spec.life[1]-spec.life[0]) + spec.life[0]
	pt.ageRate = 1 / life
	speed := zombieRandom(p.rng, spec.speedStart[1]-spec.speedStart[0]) + spec.speedStart[0]
	end := zombieRandom(p.rng, spec.speedEnd[1]-spec.speedEnd[0]) + spec.speedEnd[0]
	pt.speed = speed
	if end != speed {
		pt.speedRate = 1 / (end - speed)
	}
	if vx != 0 || vy != 0 || vz != 0 {
		length := float32(math.Sqrt(float64(vx*vx + vy*vy + vz*vz)))
		if length != 0 {
			vx, vy = vx/length, vy/length
		}
	}
	pt.dirX, pt.dirY, pt.vz = vx, vy, vz
	scale := zombieRandom(p.rng, 1)
	pt.size0 = spec.sizeStart[0] + (spec.sizeStart[1]-spec.sizeStart[0])*scale
	pt.size1 = spec.sizeEnd[0] + (spec.sizeEnd[1]-spec.sizeEnd[0])*scale
	return pt
}

// step is FUN_000b3144 (1.2.5 FUN_00114734) with the end of life tests before and after the age moves.
func (pt *particle) step(dt float32) bool {
	spec := &particleSpecs[pt.kind]
	if pt.age >= 1 {
		return false
	}
	pt.age += dt * pt.ageRate
	if pt.age >= 1 {
		return false
	}
	pt.speed += dt * pt.speedRate
	pt.vz -= dt * spec.gravity
	pt.x += pt.dirX * pt.speed * dt
	pt.y += pt.dirY * pt.speed * dt
	pt.z += pt.vz * dt
	pt.size = pt.size0 + (pt.size1-pt.size0)*pt.age
	if pt.z < 0 && spec.bounce >= 0 {
		pt.z = 0
		pt.vz = -(spec.bounce * pt.vz)
		pt.speed *= spec.friction
	}
	return true
}

// updateParticles steps every live particle one frame.
func (p *playState) updateParticles(dt float64) {
	if len(p.particles) == 0 {
		return
	}
	live := p.particles[:0]
	step := float32(dt)
	for _, pt := range p.particles {
		if pt.step(step) {
			live = append(live, pt)
		}
	}
	p.particles = live
}

// particleFrame is the frame of the sheet the particle shows: frames * age, truncated (the step stores the offset
// frame * width and, for a particle flying left, the right edge first so that the quad is mirrored).
func (pt particle) frame() int {
	frames := particleSpecs[pt.kind].frames
	frame := int(pt.age * float32(frames))
	if frame < 0 {
		frame = 0
	}
	if frame >= frames {
		frame = frames - 1
	}
	return frame
}

// colour is the vertex colour of FUN_000b35e0: each channel runs from the start to the end colour with the age and
// is truncated to a byte.
func (pt particle) colour() [4]uint8 {
	spec := &particleSpecs[pt.kind]
	var out [4]uint8
	for c := 0; c < 4; c++ {
		from, to := float32(spec.colourStart[c]), float32(spec.colourEnd[c])
		v := from + (to-from)*pt.age
		if v > 0 {
			out[c] = uint8(int(v))
		}
	}
	return out
}

// particleDraw is a particle ready for the depth-sorted draw.
type particleDraw struct {
	x, y, z, size float64
	frame         int
	flip          bool
	kind          int
	colour        [4]uint8
}

func (p *playState) particleList() []particleDraw {
	list := make([]particleDraw, 0, len(p.particles))
	for _, pt := range p.particles {
		list = append(list, particleDraw{x: float64(pt.x), y: float64(pt.y), z: float64(pt.z), size: float64(pt.size), frame: pt.frame(), flip: pt.dirX < 0, kind: pt.kind, colour: pt.colour()})
	}
	return list
}

// particleTexture finds the sheet in whichever cache is loaded (the HD cache keeps charge as charge_HD, the SD cache
// as charge_SD; bloodsplatter_02 and particle have no suffix in the SD cache).
func (a *app) particleTexture(name string) (*ebiten.Image, error) {
	if texture, err := a.Texture("Common0/Textures/" + name + "_SD"); err == nil {
		return texture, nil
	}
	return a.Texture("Common0/Textures/" + name)
}

// drawParticle draws one particle: a square of its size centred at (x, y - z), the sheet cut into frames across, the
// quad mirrored for a particle flying left, tinted by the colour gradient.
func (a *app) drawParticle(screen *ebiten.Image, pt particleDraw) {
	if a.play == nil || a.play.world == nil || pt.size <= 0 || pt.kind < 0 || pt.kind >= len(particleSpecs) {
		return
	}
	spec := &particleSpecs[pt.kind]
	texture, err := a.particleTexture(spec.texture)
	if err != nil || spec.frames <= 0 {
		return
	}
	bounds := texture.Bounds()
	cell := bounds.Dx() / spec.frames
	if cell <= 0 || bounds.Dy() <= 0 {
		return
	}
	source := texture.SubImage(image.Rect(bounds.Min.X+pt.frame*cell, bounds.Min.Y, bounds.Min.X+(pt.frame+1)*cell, bounds.Max.Y)).(*ebiten.Image)
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx, sy := a.renderScale()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(cell)/2, -float64(bounds.Dy())/2)
	scaleX := pt.size * zoom / float64(cell) * sx
	if pt.flip {
		scaleX = -scaleX
	}
	options.GeoM.Scale(scaleX, pt.size*zoom/float64(bounds.Dy())*sx)
	options.GeoM.Translate(((pt.x-world.CameraX)*zoom+world.ViewportX)*sx, ((pt.y-pt.z-world.CameraY)*zoom+world.ViewportY)*sy)
	options.ColorScale.Scale(float32(pt.colour[0])/255, float32(pt.colour[1])/255, float32(pt.colour[2])/255, 1)
	options.ColorScale.ScaleAlpha(float32(pt.colour[3]) / 255)
	a.drawImage(screen, source, options)
}

// zombieDeathSpurt is the death state 1 branch of the dying slot FUN_000a10fc (1.2.5 FUN_001002bc), reached while
// the AI state is not 6: one type 1 particle per frame at (x, y, lift * size) while the hit timer of the killing
// blow is above 0.06. The shared step lowers the timer first (FUN_0009fb10). A gib (death state 2) spawns flames
// instead, and the boss classes keep their own death sequences (not ported here).
func (p *playState) zombieDeathSpurt(z *zombieState) {
	if z.deathState != zombieDeathNormal || z.native.kind < zombieKindPlain || z.native.kind > zombieKindProspect {
		return
	}
	if z.native.ai.state == 6 || z.hitFlash <= particleSpurtAbove {
		return
	}
	p.spawnParticles(particleKindSpurt, z.x, z.y, z.native.sizeZ*zombieRenderAnchor, 0, particleSpurtFull, 1, particleSpurtSpeed, particleSpurtLift)
}
