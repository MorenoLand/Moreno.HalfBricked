package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// The projectiles the robot boss fires (Research/native/robot-boss-2026-10-14.md, decompiled for this port from
// libmortargame.so v7 and cross-checked in 1.2.5):
//
//   - CURVER, bullet kind 0x19 FLYER (factory FUN_000a2ae0 case 0x19, vtable 0x005bb488; init FUN_000a4868, init
//     hook FUN_000a46bc, shape FUN_000a73e0, hit FUN_000a4a0c, draw FUN_000a78c4; 1.2.5 FUN_00104ce0 / FUN_001071cc /
//     FUN_00104ee4): a rocket-textured flyer that is placed 1.1..1.2 * speed * life * 0.25 px from the player on a
//     random free line, fades in while it turns toward the player, then accelerates (speed * age / life) along its
//     heading; one hit deals 0.333 and starts the fade.
//   - BAZOOKA, kind 0x13 (vtable 0x005bb128): the player's rocket class with hit-player set; it hurts the player
//     for 1.5 * dt on contact and detonates (sound 8 + the shared blast, which then hurts at 1.5/s).
//   - MINIGUN, kind 0x11 THROUGH (vtable 0x005bb070, hit FUN_000a521c): the post-fire hook FUN_000baf68 re-initialises
//     each bullet as a 'beam' sprite from the boss towards the locked aim, speed 500, life 1, height 100 falling at the
//     sweep rate; only the two bullets of a burst with rounds mod 14 above 11.8 hurt (0.1 per overlapping frame, the
//     bullet is not consumed).
//   - The anger shockwaves are kind 0x1a (rex_shockwave.go); the death debris is the flame class (flame_flight.go).
const (
	robotFlyerBase     = 10.0   // DAT_000a4978
	robotFlyerScale    = 2.5    // DAT_000a4848
	robotFlyerSpeedMin = .5     // DAT_000a484c
	robotFlyerRingMin  = 1.1    // DAT_000a4850
	robotFlyerRingVar  = .1     // DAT_000a4860
	robotFlyerRingMul  = .25    // DAT_000a4854
	robotFlyerHeading  = 0xff3a // DAT_000a485c
	robotFlyerTrim     = 0x3ffc
	robotFlyerFadeIn   = .6 // DAT_000a763c
	robotFlyerAlphaMul = 255.0
	robotFlyerAlphaMax = 254.0
	robotFlyerAlphaMin = .1
	robotFlyerFall     = 160.0 // DAT_000a764c
	robotFlyerTurn     = .025  // DAT_000a7650
	robotFlyerFadeOut  = .5    // DAT_000a7654
	robotFlyerDamage   = .333  // DAT_000a4af0 0x3eaa7efa
	robotFlyerDrawW    = 1.2   // DAT_000a7a58
	robotFlyerDrawH    = 1.8   // DAT_000a7a5c
	robotFlyerTexture  = "Common0/Textures/rocket_SD"
	robotFlyerTries    = 512 // bound on the native "repeat until the line is free" loop

	robotRocketWidth  = 14.0 // DAT_000a5988
	robotRocketHeight = 28.0 // DAT_000a598c
	robotRocketAge    = .5   // DAT_000a5984: the rocket starts at half its life
	robotRocketRate   = 1.5  // DAT_000a520c: health per second on contact (scene dt * 1.5)
	robotRocketPlayer = 19.2 // 0.3 * 64

	robotBeamSpeed    = 500.0 // DAT_000bb0d4
	robotBeamLift     = 100.0 // DAT_000bb0d8
	robotBeamLife     = 1.0
	robotBeamBase     = 10.0 // DAT_000bb0c0
	robotBeamAlpha    = 50.0 // DAT_000bb0d0
	robotBeamAlphaVar = 200.0
	robotBeamHitRem   = 11.8 // DAT_000bb0b8: rounds mod 14 above this hurts the player
	robotBeamDamage   = .1   // DAT_000a5330
	robotBeamLaser    = "SFX_LASER_2"
	robotBeamTexture  = "Common0/Textures/beam"
	robotBazookaTimer = -.7 // DAT_000bb0dc: the gun timer after every rocket
	robotFlyerSound   = ""

	robotRocketSound = "SFX_MINE_EXPLODE" // FUN_000a3ca8 plays sound id 8
	robotBlastRate   = 1.5                // the rocket handler FUN_000a50f4 (the mine handler's 3.0 is the default)
)

// robotFlyer is a live kind-0x19 projectile (field names give the v7 offsets).
type robotFlyer struct {
	x, y       float64
	dirX, dirY float64 // +0x1c, +0x20
	heading    uint16  // +0x36
	age, life  float64 // +0x68, +0x5c
	speed      float64 // +0x60
	lift, fall float64 // +0x78, +0x7c
	size       float64 // +0x28
	state      int     // +0x6c: 2 fading in and turning, 0 flying, 1 fading out
	alpha      int     // +0x80
}

// robotRocket is a live kind-0x13 projectile in flight.
type robotRocket struct {
	x, y       float64
	dirX, dirY float64
	heading    uint16
	age, life  float64
	speed      float64
	lift, fall float64
}

// robotBeam is a live kind-0x11 projectile re-initialised by the minigun's post-fire hook.
type robotBeam struct {
	x, y       float64
	dirX, dirY float64
	heading    uint16
	age, life  float64
	speed      float64
	lift, fall float64
	size       float64 // +0x28 = 10 + 10 * (rounds mod 14) / 14
	alpha      uint8   // +0x47
	hits       bool    // the hit-player flag
}

// robotShots holds the live projectiles of the robot class.
type robotShots struct {
	flyers  []robotFlyer
	rockets []robotRocket
	beams   []robotBeam
}

// robotFire is vtable slot +0x60 FUN_0009f7e4 for this class followed by the post-fire hook +0x64 (FUN_000baf68):
// the timer resets, one round is spent, and every pellet is created at the boss position (heading = facing with the
// gun's spread) with the height 0.46875 * +0x30 falling at (lift - 23.6) / (distance / speed).
func (p *playState) robotFire(z *zombieState) {
	ai := &z.native.ai
	r := &z.native.robot
	gun := &ai.gun
	gun.timer = 0
	if gun.rounds < 1 {
		return
	}
	gun.rounds--
	lift := z.native.sizeZ * zombieShotLift
	fall := zombieShotFall(lift, gun.speed, p.zombieShotDistance(z))
	jitter := int(gun.spread + gun.spread)
	for pellet := 0; pellet < gun.pellets; pellet++ {
		heading := z.native.facing
		if jitter > 0 {
			heading = heading - uint16(jitter) + uint16(zombieBounded(p.rng, uint32(jitter<<1)))
		}
		switch gun.kind {
		case 0x19:
			p.spawnRobotFlyer(z, heading, lift, fall, gun)
		case 0x13:
			dirX, dirY := cosU16(heading), sinU16(heading)
			rocket := robotRocket{x: z.x, y: z.y, dirX: dirX, dirY: dirY, heading: heading, life: gun.life, speed: gun.speed, lift: lift, fall: fall}
			rocket.age = rocket.life * robotRocketAge
			p.robotShots.rockets = append(p.robotShots.rockets, rocket)
		case 0x11:
			// The hook below re-initialises the bullet that FUN_000a4868 just made, so only the beam survives.
		}
		// Post-fire hook FUN_000baf68.
		switch r.weapon {
		case robotWeaponMinigun:
			rem := gun.rounds % 14
			if rem == 11 {
				p.sfxQueue = append(p.sfxQueue, robotBeamLaser) // FUN_000ca410(0x17): laser_2
			}
			aim := ai.desired
			beam := robotBeam{x: z.x, y: z.y, heading: aim, life: robotBeamLife, speed: robotBeamSpeed, lift: robotBeamLift, fall: r.sweepRate, hits: float64(rem) > robotBeamHitRem}
			beam.dirX, beam.dirY = cosU16(aim), sinU16(aim)
			share := float64(float32(rem) / float32(14))
			beam.size = share*robotBeamBase + robotBeamBase
			beam.alpha = uint8(int(robotBeamAlpha + share*robotBeamAlphaVar))
			if beam.speed > 0 && beam.fall != 0 {
				beam.heading = weapons.NativeWeaponDirection(beam.dirX, beam.fall/beam.speed+beam.dirY) // FUN_000a57e4 tilt
			}
			p.robotShots.beams = append(p.robotShots.beams, beam)
		case robotWeaponBazooka:
			gun.timer = robotBazookaTimer
		}
	}
}

// spawnRobotFlyer is FUN_000a4868 plus the init hook FUN_000a46bc: the size grows 2.5x, the speed is multiplied by
// rnd(1) + 0.5 and, with the hit-player flag, the body is put on the player's side: position = player - d * heading
// with d = (rnd(0.1) + 1.1) * speed * life * 0.25, a new random heading until the tile line to the player is free.
func (p *playState) spawnRobotFlyer(z *zombieState, heading uint16, lift, fall float64, gun *zombieGun) {
	f := robotFlyer{x: z.x, y: z.y, heading: heading, life: gun.life, speed: gun.speed, lift: lift, fall: fall, size: robotFlyerBase * robotFlyerScale}
	f.dirX, f.dirY = cosU16(heading), sinU16(heading)
	f.speed = float64(float32(f.speed) * (zombieRandom(p.rng, 1) + float32(robotFlyerSpeedMin)))
	prey := p.nearestPlayer(z.x, z.y)
	for try := 0; try < robotFlyerTries; try++ {
		d := float64(float32(zombieRandom(p.rng, robotFlyerRingVar))+float32(robotFlyerRingMin)) * f.speed * f.life * robotFlyerRingMul
		f.x, f.y = prey.x-d*f.dirX, prey.y-d*f.dirY
		if !p.robotRayBlocked(f.x, f.y, prey.x, prey.y) {
			break
		}
		f.heading = uint16(zombieBounded(p.rng, robotFlyerHeading))
		f.dirX, f.dirY = cosU16(f.heading-robotFlyerTrim), sinU16(f.heading-robotFlyerTrim)
	}
	f.state = 2
	p.robotShots.flyers = append(p.robotShots.flyers, f)
}

// robotRayBlocked is FUN_000bfcb4 with its last argument 0: the segment is walked in 4 px steps from its start and
// the first step on a collision value 1 tile blocks it.
func (p *playState) robotRayBlocked(x0, y0, x1, y1 float64) bool {
	if p.world == nil {
		return false
	}
	dx, dy := x1-x0, y1-y0
	length := math.Hypot(dx, dy)
	if length == 0 {
		return p.collisionValue(int(float32(x0)*(1.0/32)), int(float32(y0)*(1.0/32))) == 1
	}
	stepX, stepY := dx/length*4, dy/length*4
	x, y := x0, y0
	for i := 0; float64(i) < length*.25; i++ {
		if p.collisionValue(int(float32(x)*(1.0/32)), int(float32(y)*(1.0/32))) == 1 {
			return true
		}
		x += stepX
		y += stepY
	}
	return false
}

// robotPlayerReach is the player ellipse of FUN_000a5f74: dx^2 + (dy / 0.666)^2 < (0.3 * 64 + 0.5 * width)^2.
func robotPlayerHit(px, py, x, y, width float64) bool {
	reach := robotRocketPlayer + zombieShotPlayerScale*width
	return zombieBlastEllipse(px-x, py-y, reach)
}

// updateRobotShots advances every robot projectile one frame (FUN_000a5f74: age, height, shape, end tests, player
// ellipse test).
func (p *playState) updateRobotShots(dt float64) {
	s := &p.robotShots
	if len(s.flyers) == 0 && len(s.rockets) == 0 && len(s.beams) == 0 {
		return
	}
	liveFlyers := s.flyers[:0]
	for _, f := range s.flyers {
		if p.stepRobotFlyer(&f, dt) {
			liveFlyers = append(liveFlyers, f)
		}
	}
	s.flyers = liveFlyers
	liveRockets := s.rockets[:0]
	for _, rk := range s.rockets {
		if p.stepRobotRocket(&rk, dt) {
			liveRockets = append(liveRockets, rk)
		}
	}
	s.rockets = liveRockets
	liveBeams := s.beams[:0]
	for _, b := range s.beams {
		if p.stepRobotBeam(&b, dt) {
			liveBeams = append(liveBeams, b)
		}
	}
	s.beams = liveBeams
}

// robotAddSigned adds a signed float to a heading with the original's conversion (a non-positive sum becomes 0).
func robotAddSigned(angle uint16, add float64) uint16 {
	v := float64(float32(angle)) + add
	if !(v > 0) {
		return 0
	}
	return uint16(int(v))
}

// stepRobotFlyer is FUN_000a5f74 for kind 0x19 with the shape update FUN_000a73e0 and the hit FUN_000a4a0c.
func (p *playState) stepRobotFlyer(f *robotFlyer, dt float64) bool {
	f.age += dt
	f.lift -= dt * f.fall
	switch f.state {
	case 2:
		span := f.life * robotFlyerFadeIn
		a := f.age / span * robotFlyerAlphaMul
		if a > robotFlyerAlphaMax {
			a = robotFlyerAlphaMax
		}
		if a < robotFlyerAlphaMin {
			a = robotFlyerAlphaMin
		}
		f.alpha = int(a)
		f.fall = (robotFlyerFadeIn - f.age/f.life) * robotFlyerFall
		if f.age > span {
			f.state, f.alpha, f.fall, f.age = 0, 255, 0, 0
		}
		prey := p.nearestPlayer(f.x, f.y)
		toward := weapons.NativeWeaponDirection(prey.x-f.x, prey.y-f.y)
		diff := angleDifference(toward+robotFlyerTrim, f.heading)
		f.heading = robotAddSigned(f.heading, float64(diff)*robotFlyerTurn)
		f.dirX, f.dirY = cosU16(f.heading-robotFlyerTrim), sinU16(f.heading-robotFlyerTrim)
	case 0:
		d := dt * f.speed * (f.age / f.life)
		f.x += f.dirX * d
		f.y += f.dirY * d
	default:
		f.x += f.dirX * f.speed * dt
		f.y += f.dirY * f.speed * dt
		limit := f.life * robotFlyerFadeIn * robotFlyerFadeOut
		a := f.age / limit * robotFlyerAlphaMul
		if a > robotFlyerAlphaMax {
			a = robotFlyerAlphaMax
		}
		if a < robotFlyerAlphaMin {
			a = robotFlyerAlphaMin
		}
		f.alpha = 0xff - int(a)
		if limit < f.age {
			return false
		}
	}
	if (f.state == 0 && (f.life <= f.age || p.projectileBlockedAt(f.x, f.y))) || f.lift < 0 {
		// FUN_000a4a0c(0, null): only a flying flyer reacts, it starts the fade at the speed reached so far.
		if f.state == 0 {
			f.speed *= f.age / f.life
			f.age, f.state = 0, 1
		}
		return true
	}
	for _, target := range p.livingPlayers() {
		if !robotPlayerHit(target.x, target.y, f.x, f.y, f.size) {
			continue
		}
		if f.state == 0 {
			f.speed *= f.age / f.life
			f.age, f.state = 0, 1
			p.damagePlayer(target.index, robotFlyerDamage)
		}
		break
	}
	return true
}

// stepRobotRocket is FUN_000a5f74 for kind 0x13 (shape FUN_000a6ddc, hit FUN_000a50f4): the rocket flies straight and
// detonates at the end of its life, on a wall tile, when its height runs out or on the player (1.5 * dt first).
func (p *playState) stepRobotRocket(rk *robotRocket, dt float64) bool {
	rk.age += dt
	rk.lift -= dt * rk.fall
	rk.x += rk.dirX * rk.speed * dt
	rk.y += rk.dirY * rk.speed * dt
	if rk.life <= rk.age || p.projectileBlockedAt(rk.x, rk.y) || rk.lift < 0 {
		p.spawnRobotBlast(rk.x, rk.y)
		return false
	}
	for _, target := range p.livingPlayers() {
		if robotPlayerHit(target.x, target.y, rk.x, rk.y, robotRocketWidth) {
			p.damagePlayer(target.index, dt*robotRocketRate)
			p.spawnRobotBlast(rk.x, rk.y)
			return false
		}
	}
	return true
}

// spawnRobotBlast is FUN_000a3ca8 on a robot rocket: sound 8, the explosion clip and the shared blast, which hurts
// the player at 1.5 per second (the rocket handler) and never zombies (the rocket has hit-entities 0).
func (p *playState) spawnRobotBlast(x, y float64) {
	p.sfxQueue = append(p.sfxQueue, robotRocketSound)
	p.explosions = append(p.explosions, explosionState{x: x, y: y})
	p.zombieBlasts = append(p.zombieBlasts, zombieBlast{x: x, y: y, id: p.newShot(), playerRate: robotBlastRate, noZombies: true})
}

// stepRobotBeam is FUN_000a5f74 for the re-initialised kind-0x11 bullet: it flies at 500 px/s and ends silently on
// its life, a wall tile or when its height (100, falling at the sweep rate) runs out; with the hit flag it deals 0.1
// every frame it overlaps the player and is not consumed (kind 0x11).
func (p *playState) stepRobotBeam(b *robotBeam, dt float64) bool {
	b.age += dt
	b.lift -= dt * b.fall
	b.x += b.dirX * b.speed * dt
	b.y += b.dirY * b.speed * dt
	if b.age >= b.life || p.projectileBlockedAt(b.x, b.y) || b.lift < 0 {
		return false
	}
	if b.hits {
		for _, target := range p.livingPlayers() {
			if robotPlayerHit(target.x, target.y, b.x, b.y, b.size) {
				p.damagePlayer(target.index, robotBeamDamage)
				break
			}
		}
	}
	return true
}

// drawRobotShots draws the robot's projectiles: flyers (the rocket texture, 1.2 x 1.8 times the 25 px body, rotated
// by heading / 182 degrees, first a black pass then a white one, each at half the fade alpha), rockets (14 x 28,
// rotated heading / 182 + 90) and beams (a square of the beam texture at the byte alpha).
func (a *app) drawRobotShots(screen *ebiten.Image) {
	if a.play == nil || a.play.world == nil {
		return
	}
	s := &a.play.robotShots
	if len(s.flyers) == 0 && len(s.rockets) == 0 && len(s.beams) == 0 {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx, sy := a.renderScale()
	place := func(textureName string, x, y, lift, width, height, degrees float64, tint [3]float32, alpha float64) {
		texture, err := a.Texture(textureName)
		if err != nil {
			return
		}
		tw, th := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
		if tw <= 0 || th <= 0 {
			return
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-tw/2, -th/2)
		options.GeoM.Scale(width*zoom/tw*sx, height*zoom/th*sx)
		options.GeoM.Rotate(degrees * math.Pi / 180)
		options.GeoM.Translate(((x-world.CameraX)*zoom+world.ViewportX)*sx, ((y-lift-world.CameraY)*zoom+world.ViewportY)*sy)
		options.ColorScale.Scale(tint[0], tint[1], tint[2], 1)
		options.ColorScale.ScaleAlpha(float32(alpha))
		a.drawImage(screen, texture, options)
	}
	for _, f := range s.flyers {
		half := float64(f.alpha>>1) / 255
		w, h := f.size*robotFlyerDrawW, f.size*robotFlyerDrawH
		degrees := float64(f.heading) / 182
		place(robotFlyerTexture, f.x, f.y, f.lift, w, h, degrees, [3]float32{0, 0, 0}, half)
		place(robotFlyerTexture, f.x, f.y, f.lift, w, h, degrees, [3]float32{1, 1, 1}, half)
	}
	for _, rk := range s.rockets {
		place(robotFlyerTexture, rk.x, rk.y, rk.lift, robotRocketWidth, robotRocketHeight, float64(rk.heading)/182+90, [3]float32{1, 1, 1}, 1)
	}
	for _, b := range s.beams {
		place(robotBeamTexture, b.x, b.y, b.lift, b.size, b.size, float64(b.heading)/182+90, [3]float32{1, 1, 1}, float64(b.alpha)/255)
	}
}

// robotOrbDraw is one orb sprite for the depth-sorted draw: a 32 x 32 quad of the beam texture tinted with the
// boss's current orb colour at (x, y - z).
type robotOrbDraw struct {
	x, y, z float64
	colour  [4]uint8
}

// robotOrbs lists the visible orbs of every living or dying robot.
func (p *playState) robotOrbList() []robotOrbDraw {
	var list []robotOrbDraw
	for index := range p.zombies {
		z := &p.zombies[index]
		r := &z.native.robot
		if !r.active || z.spawnAway || r.colour[3] == 0 {
			continue
		}
		for _, orb := range r.orbs {
			list = append(list, robotOrbDraw{x: orb.x, y: orb.y, z: orb.z, colour: r.colour})
		}
	}
	return list
}

// drawRobotOrb draws one orb.
func (a *app) drawRobotOrb(screen *ebiten.Image, orb robotOrbDraw) {
	if a.play == nil || a.play.world == nil {
		return
	}
	texture, err := a.Texture(robotOrbTexture)
	if err != nil {
		return
	}
	tw, th := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	if tw <= 0 || th <= 0 {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx, sy := a.renderScale()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-tw/2, -th/2)
	options.GeoM.Scale(robotOrbSize*zoom/tw*sx, robotOrbSize*zoom/th*sx)
	options.GeoM.Translate(((orb.x-world.CameraX)*zoom+world.ViewportX)*sx, ((orb.y-orb.z-world.CameraY)*zoom+world.ViewportY)*sy)
	options.ColorScale.Scale(float32(orb.colour[0])/255, float32(orb.colour[1])/255, float32(orb.colour[2])/255, 1)
	options.ColorScale.ScaleAlpha(float32(orb.colour[3]) / 255)
	a.drawImage(screen, texture, options)
}
