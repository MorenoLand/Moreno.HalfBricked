package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// The SPINNER bullet (kind 0x18, zombie weapon CURVER: the ninjas and the Japan boss). Recovered from
// libmortargame.so v7 and 1.2.5 (Research/native/samurai-spinner-2026-10-11.md): setup FUN_000a6640 (1.2.5
// FUN_00107550), move FUN_000a3e08 (FUN_00104428), update FUN_000a5f74 (FUN_00106460), hit FUN_000a4af4
// (FUN_00104fbc), draw FUN_000a7f34 (FUN_0010831c), fire tail FUN_0009f7e4 (FUN_001010a8).
//
// The blade does not fly straight: it moves on a fixed circle whose centre lies on the perpendicular bisector of
// the segment shooter -> aim point, at tangential speed = the gun speed, for 0.55 of a revolution. It hits the
// player only (one hit, then it is removed) and ends silently on its life, on a value 1 wall tile at its centre or
// when its height budget runs out.
const (
	spinnerBaseW        = float32(10)
	spinnerSizeScale    = float32(2.65)  // 0x4029999a
	spinnerDrawScale    = float32(1.2)   // draw square = 26.5 * 1.2
	spinnerLifeFraction = float32(.55)   // 0x3f0ccccd
	spinnerDamage       = float32(.333)  // 0x3eaa7efa
	spinnerReachScale   = float32(.3)    // DAT_000a63d0, times the player width 64
	spinnerWidthReach   = float32(.5)    // times the blade width
	spinnerAspect       = float32(.666)  // 0x3f2a7efa
	spinnerLead         = float32(60)    // player velocity multiplier
	spinnerPerp         = float32(.1)    // centre offset p = (rnd(1) + 0.1) * len
	spinnerSpinBase     = float32(32768) // DAT_000a3ecc
	spinnerSpinRate     = float32(2)     // +-2.0
	spinnerFullTurn     = float32(65536)
	spinnerHeadingUnits = float64(182) // draw rotation degrees = heading / 182
	spinnerShadowAlpha  = 128.0 / 255  // black pass colour (0, 0, 0, 0x80)
	spinnerTexture      = "Common0/Textures/shuriken_SD"
	spinnerPlayerWidth  = float32(entityGridPlayerBody)
)

// spinnerBlade is one flying blade (field names give the native offsets).
type spinnerBlade struct {
	x, y             float32
	w                float32 // +0x28, 26.5 after the setup
	heading          uint16  // +0x36 sprite heading
	age, life, speed float32 // +0x68, +0x5c, +0x60
	height, fall     float32 // +0x78, +0x7c
	cx, cy, radius   float32 // +0x80, +0x84, +0x8c
	orbit            uint16  // +0x90
	omega            float32 // +0x94 units per second, signed
}

func rndF(p *playState, extent float32) float32 { return zombieRandom(p.rng, extent) }

// newSpinnerBlade is init FUN_000a4868 plus the finisher FUN_000a57e4 (the heading is tilted by the fall) and the
// setup FUN_000a6640 for the aim point (ax, ay), including its two random draws. z is 0 for ground shooters and
// for the aim point, so the native 3D vectors are the 2D ones.
func (p *playState) newSpinnerBlade(x, y float32, angle uint16, lift, fall, speed float32, ax, ay float32) spinnerBlade {
	b := spinnerBlade{x: x, y: y, w: spinnerBaseW, heading: angle, speed: speed, height: lift, fall: fall}
	if speed > 0 && fall != 0 {
		dirX, dirY := float32(cosU16(angle)), float32(sinU16(angle))
		b.heading = weapons.NativeWeaponDirection(float64(dirX), float64(fall/speed+dirY))
	}
	dx, dy := ax-b.x, ay-b.y
	l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	var nx, ny float32
	if l != 0 {
		nx, ny = dx/l, dy/l
	}
	mx, my := b.x+nx*(l*.5), b.y+ny*(l*.5)
	perp := (rndF(p, 1) + spinnerPerp) * l // draw 4
	if zombieBounded(p.rng, 2) == 0 {      // draw 5
		perp = -perp
	}
	half := l * .5
	b.radius = float32(math.Sqrt(float64(half*half + perp*perp)))
	b.cx = mx - ny*perp
	b.cy = my + nx*perp
	b.orbit = weapons.NativeWeaponDirection(float64(b.x-b.cx), float64(b.y-b.cy))
	period := (b.radius*float32(math.Pi) + b.radius*float32(math.Pi)) / b.speed
	b.life = period * spinnerLifeFraction
	b.omega = spinnerFullTurn / period
	if perp < 0 {
		b.omega = -b.omega
	}
	b.w *= spinnerSizeScale
	return b
}

// fireSpinner is the kind 0x18 branch of the zombie fire tail: one blade at the shooter's position (no muzzle
// offset, no sound) aimed at the player's position plus a random jitter of up to spread/182 px plus the player's
// velocity times 60. half is the heading jitter range that the native loop variable carries (divided by 182 and
// reused for the aim jitter); the new value is returned.
func (p *playState) fireSpinner(z *zombieState, heading uint16, half int, lift, fall float64, gun *zombieGun) int {
	half /= 182
	d := float32(half)
	rx := rndF(p, d)       // draw 2
	ry := rndF(p, 2*d) - d // draw 3
	target := p.nearestPlayer(z.x, z.y)
	var vx, vy float32
	if target.index == 0 {
		// The player's velocity field is the input vector times 0.81 (FUN_00096818); the port keeps it as the
		// achievement motion (NativeAchievementMotion is the same expression).
		vx, vy = p.achievementMotionX, p.achievementMotionY
	}
	ax := float32(target.x) + rx + vx*spinnerLead
	ay := float32(target.y) + ry + vy*spinnerLead
	p.spinners = append(p.spinners, p.newSpinnerBlade(float32(z.x), float32(z.y), heading, float32(lift), float32(fall), float32(gun.speed), ax, ay))
	if z.native.samurai.active {
		// The class's post-fire hook (slot +0x64, FUN_000bce54) plays SFX_SHURIKEN once per blade; the plain class's is a bare return.
		p.samuraiPlayShuriken()
	}
	return half
}

// spinnerDamageFor is the amount a blade deals: 0.333. The 1.2.5 halving (FUN_000f2208) is applied for every hit
// in damagePlayerFrom.
func (p *playState) spinnerDamageFor() float64 { return float64(spinnerDamage) }

// updateSpinners advances every blade one frame (FUN_000a5f74): age and height, the orbit move, the end tests, then
// the player ellipse test.
func (p *playState) updateSpinners(dt float64) {
	if len(p.spinners) == 0 {
		return
	}
	step := float32(dt)
	live := p.spinners[:0]
	for _, b := range p.spinners {
		b.age += step
		b.height -= step * b.fall
		// FUN_000a3e08: ARM vcvt.u32.f32 saturates a negative angle to 0, so a clockwise blade sticks at orbit 0.
		a := float32(b.orbit) + step*b.omega
		if a > 0 {
			b.orbit = uint16(int32(a))
		} else {
			b.orbit = 0
		}
		b.x = b.cx + float32(cosU16(b.orbit))*b.radius
		b.y = b.cy + float32(sinU16(b.orbit))*b.radius
		spin := spinnerSpinRate
		if !(b.omega > 0) {
			spin = -spinnerSpinRate
		}
		abs := b.omega
		if abs < 0 {
			abs = -abs
		}
		spin *= 2*abs + spinnerSpinBase
		b.heading = uint16(int32(int16(b.heading)) + int32(step*spin))
		if b.age >= b.life || p.projectileBlockedAt(float64(b.x), float64(b.y)) || b.height < 0 {
			continue // silent end
		}
		hit := false
		for _, target := range p.livingPlayers() {
			dx := float32(target.x) - b.x
			dy := (float32(target.y) - b.y) / spinnerAspect
			reach := spinnerPlayerWidth*spinnerReachScale + b.w*spinnerWidthReach
			if dx*dx+dy*dy < reach*reach {
				p.damagePlayerAt(target.index, p.spinnerDamageFor(), float64(b.x), float64(b.y))
				hit = true
				break
			}
		}
		if !hit {
			live = append(live, b)
		}
	}
	p.spinners = live
}

// drawSpinners draws the blades: texture shuriken, a 31.8 px square centred at (x, y), rotated heading / 182
// degrees, mirrored when omega <= 0, first a black copy at alpha 0x80 and then the normal copy.
func (a *app) drawSpinners(screen *ebiten.Image) {
	if a.play == nil || len(a.play.spinners) == 0 {
		return
	}
	texture, err := a.Texture(spinnerTexture)
	if err != nil {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx, sy := a.renderScale()
	size := float64(spinnerBaseW * spinnerSizeScale * spinnerDrawScale)
	for _, b := range a.play.spinners {
		x := (float64(b.x)-world.CameraX)*zoom + world.ViewportX
		y := (float64(b.y)-world.CameraY)*zoom + world.ViewportY
		for pass := 0; pass < 2; pass++ {
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
			scaleX := size * zoom / float64(texture.Bounds().Dx())
			scaleY := size * zoom / float64(texture.Bounds().Dy())
			if !(b.omega > 0) {
				scaleX = -scaleX
			}
			options.GeoM.Scale(scaleX*sx, scaleY*sx)
			options.GeoM.Rotate(float64(b.heading) / spinnerHeadingUnits * math.Pi / 180)
			options.GeoM.Translate(x*sx, y*sy)
			if pass == 0 {
				options.ColorScale.Scale(0, 0, 0, spinnerShadowAlpha)
			}
			a.drawImage(screen, texture, options)
		}
	}
}
