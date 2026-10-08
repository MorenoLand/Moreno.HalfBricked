package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// Thrown grenades and dynamite.
//
// Everything here is read from libmortargame.so(v7) (image base 0x10000); the
// evidence table is Research/native/secondary-weapons-2026-10-08.md.
//
//   - The grenade child fires through FUN_000adbe8, the dynamite child (id 12,
//     vtable 0x005bbb60) through FUN_000ad5f0. Both create the projectile with
//     FUN_000a4868(pos, angle, lift = 20.0, life = Life, type, speed = Speed,
//     hitPlayer = 0, hitEntities = 1, owner, fall speed, tracker): lift 20 is the
//     literal 0x41a00000 (0x000add84 / 0x000ad794) and the initial vertical speed
//     is the child field +0x4c written by the set-definition slot (0x000ad3b0 /
//     0x000ad4d4) from the literal 0xc28c0000 = -70.0.
//   - Init (FUN_000a5ae8): state 2, the bounce flag +0x80 set, life, width, height
//     and depth all doubled (10 -> 20), so the fuse is 2 * Life = 2 s.
//   - Frame (FUN_000a5f74 then FUN_000a7218): age += dt; lift -= dt*fall; then in
//     state 2 fall += dt*500 (0x000a73c0), the object moves dir*speed*dt, a wall
//     hit pushes it out, flips the heading, multiplies the speed by .75
//     (0x000a73c4) and plays sound 0x13; it detonates when age >= life or the
//     speed drops below 52.500004 (0x000a73c8). Landing (FUN_000a4f50, lift < 0)
//     clamps lift to 0, halves the speed (0x000a50d4), plays 0x13 when the fall
//     speed exceeds 70 (0x000a50d8) and bounces with fall *= -.5 (0x000a50dc).
//     Passing over a zombie (the hit-entities pass) multiplies the speed by .95
//     (0x000a50e0) per listing when the bounce flag is set; it never explodes on
//     contact.
//   - Detonation (vtable +0x54): grenade FUN_000a3ca8 (sound 8 "mine_explode",
//     state 1 = blast.go). Dynamite FUN_000a44c4 first spawns 3 child bombs
//     (fire setters FUN_000a656c/6574/657c: count 3, generations 1) and then
//     runs FUN_000a3ca8.
const (
	thrownLift      = 20.0
	thrownFallStart = -70.0
	thrownGravity   = 500.0
	thrownWallSpeed = .75
	thrownMinSpeed  = float32(52.500004)
	thrownGroundHit = .5
	thrownBounceSnd = 70.0
	thrownBounceFal = -.5
	thrownZombieDrg = float32(.95)

	grenadeSize  = 20.0 // 10 doubled by FUN_000a5ae8
	dynamiteSize = 32.0 // FUN_000a4984 overwrites width/height after init

	// FUN_000a44c4 child spawn: the first angle is rand(360), then each child adds
	// (360/count) * (rnd(.4) + .8); life = (rnd(1.5) + .5) * 2 * parentLife and
	// speed = (rnd(1) + .5) * parentSpeed (the init doubles the life again);
	// children start at lift .5 with fall speed -21 and no bounce flag.
	dynamiteChildren      = 3
	dynamiteChildLift     = .5
	dynamiteChildFall     = -21.0
	dynamiteRndDenom      = 524287.0 // FUN_000a24dc: rand(0x7ffff) / 524287.0 * x
	dynamiteAngleUnitsPer = 182.0    // 0x000a46a8
)

const thrownBounceSound = "SFX_MULTIPLIER_TICK_DOWN" // sound id 0x13 = "multiplier_tick_down"

// thrownBomb is a native grenade (entity 0x14) or dynamite (0x1e) in flight.
type thrownBomb struct {
	x, y, lift, fall float64
	dirX, dirY       float64
	speed            float64
	age, life        float64
	size             float64
	dynamite         bool
	bounce           bool // +0x80: slows when it overlaps a zombie
	child            bool // +0x82
	parentLife       float64
	parentSpeed      float64
	origin           killOrigin
}

// throwGrenade launches a grenade or dynamite from (x, y) along (dirX, dirY).
func (p *playState) throwBomb(x, y, dirX, dirY, speed, life float64, dynamite bool, origin killOrigin) {
	size := grenadeSize
	if dynamite {
		size = dynamiteSize
	}
	p.thrown = append(p.thrown, thrownBomb{x: x, y: y, lift: thrownLift, fall: thrownFallStart, dirX: dirX, dirY: dirY,
		speed: speed, life: life * 2, size: size, dynamite: dynamite, bounce: true,
		parentLife: life * 2, parentSpeed: speed, origin: origin})
}

// updateThrown advances every thrown bomb one 1/60 s frame.
func (p *playState) updateThrown() {
	const dt = 1.0 / 60.0
	var spawned []thrownBomb
	kept := p.thrown[:0]
	for _, b := range p.thrown {
		b.age += dt
		b.lift -= dt * b.fall
		// FUN_000a7218, state 2.
		b.fall += dt * thrownGravity
		nx, ny := b.x+b.dirX*b.speed*dt, b.y+b.dirY*b.speed*dt
		if p.isSolid(nx, ny) {
			// UNRESOLVED: the native wall response subtracts FUN_000be3c0's push-out
			// vector from the heading and normalises it (FUN_00098338); the push-out
			// magnitude was not recovered, so the heading is reflected on the
			// blocked axis instead. Speed .75 and sound 0x13 are native.
			blockedX, blockedY := p.isSolid(nx, b.y), p.isSolid(b.x, ny)
			switch {
			case blockedX && !blockedY:
				b.y = ny
				b.dirX = -b.dirX
			case blockedY && !blockedX:
				b.x = nx
				b.dirY = -b.dirY
			default:
				b.dirX, b.dirY = -b.dirX, -b.dirY
			}
			b.speed = float64(float32(b.speed) * float32(thrownWallSpeed))
			p.sfxQueue = append(p.sfxQueue, thrownBounceSound)
		} else {
			b.x, b.y = nx, ny
		}
		if b.age >= b.life-1e-9 || float32(b.speed) < thrownMinSpeed {
			spawned = append(spawned, p.detonateThrown(b)...)
			continue
		}
		// FUN_000a4f50, landing.
		if b.lift < 0 {
			b.lift = 0
			b.speed *= thrownGroundHit
			if b.fall > thrownBounceSnd {
				p.sfxQueue = append(p.sfxQueue, thrownBounceSound)
			}
			b.fall *= thrownBounceFal
		}
		b.slowOverZombies(p)
		kept = append(kept, b)
	}
	p.thrown = append(kept, spawned...)
}

// slowOverZombies is the hit-entities pass of FUN_000a5f74 for a bomb in flight:
// every listing of an overlapped zombie runs FUN_000a4f50, which multiplies the
// speed by .95 while the bounce flag is set.
func (b *thrownBomb) slowOverZombies(p *playState) {
	if !b.bounce {
		return
	}
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if zombie.health <= 0 || zombie.dying || zombie.spawnAway {
			continue
		}
		if !zombieBlastEllipse(zombie.x-b.x, zombie.y-b.y, b.size+zombieBlastBodyScale*zombie.size.X) {
			continue
		}
		visits := p.zombieGridVisits(zombie, b.x, b.y, float32(b.size)*zombieBlastQueryScale)
		for visit := 0; visit < visits; visit++ {
			b.speed = float64(float32(b.speed) * thrownZombieDrg)
		}
	}
}

// detonateThrown is the vtable +0x54 slot: the blast, preceded for a parent
// dynamite by its three children (FUN_000a44c4).
func (p *playState) detonateThrown(b thrownBomb) []thrownBomb {
	var kids []thrownBomb
	if b.dynamite && !b.child {
		if p.rng == nil {
			rng := weapons.NewNativeRNG()
			p.rng = &rng
		}
		angle := float32(p.rng.Bounded(360))
		rnd := func(limit float32) float32 {
			return float32(p.rng.Bounded(524287)) / float32(dynamiteRndDenom) * limit
		}
		for i := 0; i < dynamiteChildren; i++ {
			angle += float32(360.0/float32(dynamiteChildren)) * (rnd(.4) + .8)
			life := (rnd(1.5) + .5) * float32(b.parentLife)
			speed := (rnd(1) + .5) * float32(b.parentSpeed)
			units := uint16(int(angle * float32(dynamiteAngleUnitsPer)))
			radians := float64(units) * 2 * math.Pi / 65536
			kids = append(kids, thrownBomb{x: b.x, y: b.y, lift: dynamiteChildLift, fall: dynamiteChildFall,
				dirX: math.Cos(radians), dirY: math.Sin(radians), speed: float64(speed), life: float64(life) * 2,
				size: dynamiteSize, dynamite: true, child: true, origin: b.origin})
		}
	}
	p.spawnBlast(b.x, b.y, zombieBlastSound, b.origin, true)
	return kids
}

// drawThrown draws the bombs: the item at (x, y - lift) turned to its heading
// (FUN_000a7d44, states 0/2/3), square at the bomb's width.
func (a *app) drawThrown(screen *ebiten.Image) {
	if a.play == nil || a.play.world == nil || len(a.play.thrown) == 0 {
		return
	}
	zoom := a.play.world.Zoom
	for _, b := range a.play.thrown {
		name := "Common0/Textures/grenade_SD"
		if b.dynamite {
			name = "DLC1/Textures/Dynomite_SD"
		}
		texture, err := a.Texture(name)
		if err != nil {
			continue
		}
		w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-w/2, -h/2)
		options.GeoM.Rotate(math.Atan2(b.dirY, b.dirX) + math.Pi/2)
		options.GeoM.Scale(b.size*zoom/w, b.size*zoom/h)
		options.GeoM.Translate((b.x-a.play.world.CameraX)*zoom+a.play.world.ViewportX, (b.y-b.lift-a.play.world.CameraY)*zoom+a.play.world.ViewportY)
		a.drawImage(screen, texture, options)
	}
}
