package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// The future boss (boss_robot, native zombie type 14, texture "bigboss", level 5.3 = World4Level2). Recovered from
// libmortargame.so v7 and cross-checked in 1.2.5 (Research/native/robot-boss-2026-10-14.md plus the decompilation
// done for this port): class factory FUN_000a2ae0 case 0xe (1.2.5 FUN_00102d70), post-init FUN_000bb0e0 (1.2.5
// FUN_0011dd70), per-frame AI FUN_000bb610 (1.2.5 FUN_0011e308), weapon pick FUN_000bb3f4 (FUN_0011e0e8), aim slot
// +0x48 FUN_000bab94 (FUN_0011d860), post-fire hook +0x64 FUN_000baf68 (FUN_0011dbf4), death start +0x50 FUN_000baa28,
// damage +0x5c FUN_000bae44, grid slot +0x3c FUN_000bad88, per-frame slot +0x18 FUN_000ba8f8, draw FUN_000ba898.
// The class reuses the shared zombie step FUN_0009fb10 (stepRobot mirrors it) and the shared state machine
// FUN_0009f32c (zombieStateMachine). v7 and 1.2.5 share every number used here (checked against the literal pools).
//
// The robot never walks (speed +0x27c = 0). It stands in the arena, aims at the player and cycles through weapons:
// CURVER (kind 0x19 homing flyers), MINIGUN (kind 0x11 beams), BAZOOKA (kind 0x13 rockets) and a teleport to the
// farthest of the level's marker tiles. Each time its health crosses 75%, 50% and 25% it gets angry: three orbs charge
// and release 1, 2 or 3 bursts of three shockwaves (kind 0x1a).
//
// UNRESOLVED / not ported: the particle spawner FUN_000b346c (the muzzle flash of the minigun sweep and the hit sparks
// of FUN_000bae44, not ported for any class either) and the boss health meter that FUN_000b48c0 eases (no boss HUD
// meter exists in the port).
const (
	robotSizeZ  = 130.0 // FUN_000bb0e0 +0x30 = DAT_000bb318 0x43020000 (the spawn record's 70 is a half size, 140, then replaced)
	robotHealth = 40000 // DAT_000bb31c 0x9c40

	robotHeadingInit = 0x3ffc // DAT_000bb314
	robotHeightScale = 2.0    // FUN_000ba8f8: +0x2c = 2 * +0x28 every frame while alive
	robotLiftIdle    = .75    // FUN_000bb0e0 +0x2d4 = 0x3f400000
	robotMaxSpots    = 10     // FUN_000bb0e0: at most ten tiles of value 0xb
	robotSpotValue   = 0xb    // collision value of a teleport marker tile
	robotSpotOffset  = 16.0   // DAT_000bb304: tile centre
	robotSpotTile    = 32.0   // DAT_000bb300

	robotWeaponMinigun  = 3
	robotWeaponBazooka  = 5
	robotWeaponCurver   = 0xb
	robotWeaponTeleport = 0xd

	robotPickRange        = 9  // rnd(9)
	robotPickVolleyLimit  = 6  // +0x3b8 < 6 picks CURVER, else the teleport
	robotPickAngerRatio   = .2 // DAT_000bb5e4
	robotCurverRounds     = 3
	robotCurverSpread     = 65338.0 // DAT_000bb5f0
	robotCurverLife       = .95     // DAT_000bb5f4
	robotCurverSpeed      = 230.0   // DAT_000bb5f8
	robotCurverInterval   = .66     // DAT_000bb5fc
	robotCurverPellets    = 2
	robotMinigunRounds    = 0x46
	robotBazookaRounds    = 4
	robotBazookaInterval  = .1    // DAT_000bb608
	robotPickTimer        = -1.0  // DAT_000bb60c
	robotTeleportStart    = .01   // DAT_000bb604
	robotTeleportEnd      = 1.0   // DAT_000bc4ac
	robotTeleportAlphaMul = 512.0 // DAT_000bc4b0
	robotTeleportLiftMul  = 2.0

	robotAngerFull      = 1.0
	robotAngerRestart   = .5
	robotAngerDecay     = .4    // DAT_000bc4b4
	robotAngerTrigger75 = .75   // DAT_000bbb0c
	robotAngerTrigger50 = .5    // DAT_000bbbbc
	robotAngerTrigger25 = .25   // DAT_000bbb10
	robotShockwaveLife  = .7    // DAT_000bbb80
	robotShockwaveSize  = 350.0 // DAT_000bbb84 (+0x30 after the init hook)
	robotShakeDuration  = 2.5   // DAT_000bbb88
	robotShakeAmpX      = 1.5   // 0x3fc00000
	robotShakeAmpY      = 1.0
	robotShakeAngleMul  = 1.0

	robotOrbs         = 3
	robotOrbRadiusIni = .42      // DAT_000bb308
	robotOrbRadiusTgt = .35      // DAT_000bbbb8
	robotOrbRadiusLag = .05      // DAT_000bbbb0
	robotOrbHeightLag = .1       // DAT_000bbbb4
	robotOrbIdleBase  = .5       // DAT_000bbbbc
	robotOrbIdleSwing = .05      // DAT_000bbbb0
	robotOrbSpinFast  = 131040.0 // DAT_000bbb04
	robotOrbSpinSlow  = 32760.0  // DAT_000bbb08
	robotOrbSpinMix   = .5       // DAT_000bbbbc
	robotOrbSquash    = .667     // DAT_000bbbc0
	robotOrbPulseRate = 6354.0   // DAT_000bbb14
	robotOrbPulseIdle = 4.0      // DAT_000bbb18
	robotOrbPulseAng  = 8.0      // DAT_000bbb28
	robotOrbGrow      = 15.0     // DAT_000bbba0
	robotOrbDrop      = .075     // DAT_000bbba4
	robotOrbLowAnger  = .2       // DAT_000bbb2c
	robotOrbMidAnger  = .65      // DAT_000bbbac
	robotOrbSwing     = .075     // DAT_000bbba4 (cos(pulse) amplitude)
	robotOrbSize      = 32.0     // FUN_000bb610 first frame: +0x3e4 = +0x3e8 = 0x42000000
	robotOrbStepMax   = 16       // colour channel step per frame
	robotOrbTexture   = "Common0/Textures/beam"

	robotZoomIdle  = .65 // DAT_000bbbac: camera zoom target while no script runs
	robotZoomAnger = 1.0
	robotZoomLag   = .05 // DAT_000bbbb0

	robotFlipMin  = 0xaaa0 // DAT_000bbb34 (compared against heading - 0x2aa8)
	robotFlipMax  = 0x554e // DAT_000bbb3c (compared against heading - 0x5551)
	robotFlipBase = 0x2aa8
	robotFlipAlt  = 0x5551

	robotDeathShakeX      = -80.0 // DAT_000bbb94
	robotDeathShakeY      = 100.0 // DAT_000bbb98
	robotDeathExplodeOdds = 4     // rnd(4) == 0 at frame 2
	robotDeathBlastScale  = .6    // DAT_000bc498 (frame 2) / .9 DAT_000bc4a4 (flames)
	robotDeathBlastHalf   = .3    // DAT_000bc49c / .45 DAT_000bc4a8
	robotDeathFlameScale  = .9
	robotDeathFlameHalf   = .45
	robotDeathYJitter     = 25   // FUN_00079634(rng, 0x19)
	robotDeathFlameSpeed  = .1   // DAT_000bb3e4: flame speed = +0x304 * 0.1
	robotDeathFlameLift   = 10.0 // DAT_000bb3f0
	robotDeathWaveEnd     = 1500
	robotDeathDamage      = 1000
	robotDeathClipFPS     = 2.0 // bigbossFalling: 4 frames at 2 fps (Common0_Sprites)
	robotAnimIdle         = "Idle"
	robotAnimRage         = "Rage"
	robotAnimDeath        = "Death"
	robotSightRange       = 1e9 // slot +0x48 always answers true while no script runs
	robotFlashSecs        = .1  // DAT_000baf64
	robotKillGun          = "EXPZOMBIE"
)

// robotOrb is one of the three orbiting orbs (native sub-entities at +0x3bc + 0x70 * i).
type robotOrb struct {
	x, y, z float64 // +0x3cc, +0x3d0, +0x3d4 (z is the height above the ground)
}

// robotBoss is the per-zombie state of the class (field names give the v7 offsets).
type robotBoss struct {
	active      bool
	time        float64 // +0x328 seconds without a running script
	anger       float64 // +0x534
	teleportT   float64 // +0x3b0 teleport timer
	teleportOut bool    // +0x3b4: fading out (rising) rather than fading in
	volley      int     // +0x3b8 picks since the last teleport
	prevRatio   float64 // +0x324 health ratio of the previous frame
	weapon      int     // +0x2e4 id of the loaded weapon
	lift        float64 // +0x2d4 sprite lift factor (0.75, rises while teleporting)
	spots       [robotMaxSpots][2]float64
	spotCount   int      // +0x3a8
	spot        int      // +0x3ac index of the spot the boss stands on
	orbRadius   float64  // +0x514
	orbHeight   float64  // +0x520
	orbRate     float64  // +0x50c spin rate
	orbAngle    uint16   // +0x510
	pulse       uint16   // +0x524
	colour      [4]uint8 // +0x518..0x51b current orb colour (R, G, B, A)
	target      [4]uint8 // +0x51c..0x51f target colour
	orbs        [robotOrbs]robotOrb
	shakeX      float64 // +0x528 draw offset of the falling body
	shakeY      float64 // +0x52c
	flag        bool    // global at 0x005db1a4: the facing hysteresis (sprite flip = !flag)
	burst       int     // global at 0x005db1b0: shockwave bursts still to release
	sweepPrev   float64 // global at 0x005db1a0: gun timer of the previous aim call
	sweepRate   float64 // +0x538 beam fall rate chosen when the sweep locks on
	dead        bool    // +0x314 == 1
	deathDone   bool    // the one-shot death start (FUN_000baa28) ran
}

func isRobotName(name string) bool { return name == "boss_robot" }

// initRobot is the class constructor tail and post-init FUN_000bb0e0 (1.2.5 FUN_0011dd70): heading 0x3ffc, speed 0,
// speed factor 1, teleport timer 0, volley 0, no rounds, health 40000, +0x30 = 130 (the sprite lift and the body size
// derive from it), lift 0.75, the teleport spots (tiles of value 0xb) and the spot the boss spawned on.
func (p *playState) initRobot(z *zombieState) {
	ai := &z.native.ai
	z.native.robot = robotBoss{active: true, prevRatio: 1, lift: robotLiftIdle, orbRadius: 0}
	r := &z.native.robot
	z.native.sizeZ = robotSizeZ
	z.native.maxHealth = robotHealth
	z.native.facing = robotHeadingInit
	z.health = robotHealth
	z.speed = 0
	ai.speedFactor = 1
	ai.gun = zombieGun{}
	z.size = formats.Vec2{X: zombieBodyWidth(robotSizeZ), Y: robotSizeZ}
	z.collision = zombieBodyScale * z.size.X
	r.orbRadius = z.size.X * robotOrbRadiusIni
	r.colour = [4]uint8{0xff, 0xff, 0xff, 0}
	r.target = [4]uint8{0xff, 0xff, 0xff, 0xff}
	p.robotLoadSpots(z)
}

// robotLoadSpots walks the level grid column by column (x outer, y inner) and stores the centre of every tile whose
// collision value is 0xb, at most ten; the boss starts on the spot containing its position.
func (p *playState) robotLoadSpots(z *zombieState) {
	r := &z.native.robot
	if p.world == nil {
		return
	}
	width, height := p.world.Level.Width, p.world.Level.Height
	for tx := 0; tx < width && r.spotCount < robotMaxSpots; tx++ {
		for ty := 0; ty < height && r.spotCount < robotMaxSpots; ty++ {
			if p.collisionValue(tx, ty) != robotSpotValue {
				continue
			}
			r.spots[r.spotCount] = [2]float64{robotSpotOffset + float64(tx)*robotSpotTile, robotSpotOffset + float64(ty)*robotSpotTile}
			if int(float32(z.y)*(1.0/32)) == ty && int(float32(z.x)*(1.0/32)) == tx {
				r.spot = r.spotCount
			}
			r.spotCount++
		}
	}
}

// robotGunFromRecord loads the zombie weapon record of one weapon id like FUN_000bb3f4: FUN_000b19b8 fills the
// gun words from the ZombieWeapons catalog and the rounds are divided by 3 (every record is then overridden
// per weapon below, so the divided value only survives for ids without an override).
func (p *playState) robotGunFromRecord(id int) zombieGun {
	name := zombieGunName(id)
	for _, record := range p.zombieWeapons {
		if record.GunType != name {
			continue
		}
		bullet, ok := zombieBulletKinds[record.BulletType]
		if !ok {
			bullet = 0x10
		}
		return zombieGun{timer: record.RecoilSeconds, interval: record.RateOfFire, spread: record.SpreadUnits, rounds: record.Ammo / 3, pellets: record.AmmoPerShot, life: record.Life, speed: record.Speed, kind: bullet}
	}
	return zombieGun{}
}

// robotPick is FUN_000bb3f4. rnd(9): 0..2 CURVER (the teleport once six volleys passed), 3 CURVER, 4..6 MINIGUN,
// 7 BAZOOKA, 8 CURVER, or the rage when the health ratio is below 0.2. Every pick ends with the gun timer -1.
func (p *playState) robotPick(z *zombieState) {
	ai := &z.native.ai
	r := &z.native.robot
	if p.samuraiScriptRunning() {
		ai.gun.rounds, ai.gun.timer = 0, 0
		return
	}
	roll := int(zombieBounded(p.rng, robotPickRange))
	id := robotWeaponCurver
	switch {
	case roll < 3:
		if r.volley >= robotPickVolleyLimit {
			id = robotWeaponTeleport
		}
	case roll == 3:
	case roll < 7:
		id = robotWeaponMinigun
	case roll == 7:
		id = robotWeaponBazooka
	default:
		if r.prevRatio < robotPickAngerRatio {
			r.anger = robotAngerFull
			return
		}
	}
	r.volley++
	gun := p.robotGunFromRecord(id)
	switch id {
	case robotWeaponCurver:
		gun.rounds, gun.spread, gun.kind = robotCurverRounds, robotCurverSpread, 0x19
		gun.life, gun.speed, gun.pellets, gun.interval = robotCurverLife, robotCurverSpeed, robotCurverPellets, robotCurverInterval
	case robotWeaponMinigun:
		gun.interval, gun.rounds, gun.kind = 0, robotMinigunRounds, 0x11
	case robotWeaponTeleport:
		r.teleportOut, r.teleportT, r.volley = true, robotTeleportStart, 0
	case robotWeaponBazooka:
		gun.interval, gun.rounds = robotBazookaInterval, robotBazookaRounds
	}
	gun.timer = robotPickTimer
	r.weapon = id
	ai.gun = gun
}

// robotAim is vtable slot +0x48 (FUN_000bab94, 1.2.5 FUN_0011d860). dx, dy point from the boss to the player. While
// the boss is angry the heading is frozen. The minigun reloads its timer to -1 every 14 rounds (one round is spent
// on the pause), aims freely while the timer is below -0.7, and locks the heading when it climbs through -0.2; at
// that moment the beam fall rate is set from the distance. It returns !scriptRunning.
func (p *playState) robotAim(z *zombieState, dx, dy float64) bool {
	ai := &z.native.ai
	r := &z.native.robot
	if r.anger <= 0 {
		bearing := weapons.NativeWeaponDirection(dx, dy)
		if r.weapon == robotWeaponMinigun {
			if ai.gun.rounds%14 == 0 {
				ai.gun.timer = -1
				ai.gun.rounds--
			}
			timer := ai.gun.timer
			if timer < -.7 {
				ai.desired = bearing
			} else if timer >= -.2 && r.sweepPrev < -.2 {
				// (below -0.7 the muzzle flash FUN_000b346c(5, ...) fires when the sweep begins: particle spawner not ported)
				ai.desired = bearing
				r.sweepRate = 100 / (math.Hypot(dx, dy) / 500)
			}
			r.sweepPrev = timer
		} else {
			ai.desired = bearing
		}
	}
	return !p.samuraiScriptRunning()
}

// robotStepChannel is the colour easing of FUN_000bb610: up to 16 per frame, and the original's near-case
// arithmetic (delta = cur - target when the target is below and within 15) overshoots by design; byte wrap included.
func robotStepChannel(cur, target uint8) uint8 {
	c, t := int(cur), int(target)
	var delta int
	switch {
	case c == t:
		delta = 0
	case c < t:
		if c+15 < t {
			delta = 0x10
		} else {
			delta = t - c
		}
	default:
		if c-15 <= t {
			delta = (c - t) & 0xff
		} else {
			delta = 0xf0
		}
	}
	return uint8(c + delta)
}

// robotZoomStep eases the native camera zoom toward target by 5% per frame (the shared camera global at 0x005d222c,
// written through the port's world zoom).
func (p *playState) robotZoomStep(target float64) {
	if p.world == nil {
		return
	}
	z := nativeZoomFromPort(p.world.BaseZoom())
	next := float64(float32(z) + (float32(target)-float32(z))*float32(robotZoomLag))
	if next != z {
		p.world.SetZoom(portZoomFromNative(next))
	}
}

// robotAI is vtable slot +0x7c FUN_000bb610 (called by the boss step FUN_000b48c0 after the state selection and
// before the rise and the heading step of the shared update).
func (p *playState) robotAI(z *zombieState, prey playerTarget, dt float64) {
	ai := &z.native.ai
	r := &z.native.robot
	running := p.samuraiScriptRunning()
	if !running {
		r.time += dt
	}
	if ai.state != 6 {
		h := int(z.native.facing)
		if (h-robotFlipBase)&0xffff > robotFlipMin {
			r.flag = true
		} else if (h-robotFlipAlt)&0xffff <= robotFlipMax {
			r.flag = false
		}
		if (r.anger > 0 && r.teleportT <= 0) || z.bossRage {
			samuraiSetAnim(z, robotAnimRage)
		} else {
			samuraiSetAnim(z, robotAnimIdle)
		}
		z.flipX = !r.flag
	}
	// Orb colour target and spin target of the idle frame.
	spin := robotOrbSpinSlow
	if ai.gun.timer < 0 || r.teleportT > 0 {
		switch r.weapon {
		case robotWeaponMinigun:
			r.target = [4]uint8{0x80, 0x80, 0xff, 0xff}
		case robotWeaponBazooka:
			r.target = [4]uint8{0x80, 0xff, 0xff, 0xff}
		case robotWeaponCurver:
			r.target = [4]uint8{0xff, 0x80, 0x80, 0xff}
		default:
			r.target[3] = 0
		}
		spin = robotOrbSpinFast
	} else {
		r.target = [4]uint8{0xff, 0xff, 0xff, 0xff}
	}
	// Anger thresholds: the health ratio falls through 0.75, 0.5 and 0.25.
	ratio := 0.0
	if z.native.maxHealth != 0 {
		ratio = z.health / z.native.maxHealth
	}
	for _, threshold := range [3]float64{robotAngerTrigger75, robotAngerTrigger50, robotAngerTrigger25} {
		if ratio < threshold && !(r.prevRatio < threshold) {
			r.anger = robotAngerFull
			break
		}
	}
	step := dt * robotOrbPulseRate
	r.pulse = robotAddAngle(r.pulse, step*robotOrbPulseIdle)
	if r.anger <= 0 || r.teleportT > 0 {
		if !running {
			p.robotZoomStep(robotZoomIdle)
			r.orbHeight += (robotOrbIdleBase + cosU16(r.pulse)*robotOrbIdleSwing - r.orbHeight) * robotOrbHeightLag
			r.orbRadius += (z.size.X*robotOrbRadiusTgt - r.orbRadius) * robotOrbRadiusLag
		}
	} else {
		if ai.state != 6 {
			samuraiSetAnim(z, robotAnimRage)
		}
		ai.gun.timer = 0
		a := r.anger
		r.pulse = robotAddAngle(r.pulse, step*a*robotOrbPulseAng)
		hold := 1.0
		spin = 0
		blue := [4]uint8{0, 0, 0xff, 0xff}
		cyan := [4]uint8{0, 0xff, 0xff, 0xff}
		switch {
		case a < robotAngerTrigger25:
			r.target = blue
			if a < robotOrbLowAnger {
				r.orbHeight += (0 - r.orbHeight) * robotOrbHeightLag
				hold = 0
			}
		default:
			spin = (2 + a*3) * robotOrbSpinSlow
			switch {
			case a < robotAngerTrigger50:
				r.target = cyan
			case a < robotOrbMidAnger:
				r.target = blue
				hold = robotOrbIdleBase + cosU16(r.pulse)*robotOrbSwing
				switch {
				case r.prevRatio <= robotAngerTrigger25:
					r.burst = 3
				case r.prevRatio <= robotAngerTrigger50:
					r.burst = 2
				default:
					r.burst = 1
				}
			default:
				r.target = cyan
			}
		}
		r.orbHeight += (hold - r.orbHeight) * robotOrbHeightLag
		r.orbRadius += dt * robotOrbGrow
		if r.burst > 0 && r.orbHeight < robotOrbDrop {
			r.burst--
			if r.burst != 0 {
				r.anger = robotAngerRestart
			}
			p.robotShockwaves(z)
		}
		if ai.state != 6 {
			p.robotZoomStep(robotZoomAnger)
		}
	}
	if ai.state == 6 {
		r.target[3] = 0
		if !p.isSurvival() {
			p.robotKillSweep(z)
		}
	}
	r.orbRate += (spin - r.orbRate) * robotOrbSpinMix
	r.orbAngle = robotAddAngle(r.orbAngle, r.orbRate*dt)
	for c := 0; c < 4; c++ {
		r.colour[c] = robotStepChannel(r.colour[c], r.target[c])
	}
	// Orb positions (the orbs ride an ellipse around the boss, height = body height * +0x520).
	height := z.size.Y
	for i := 0; i < robotOrbs; i++ {
		a := uint16(int16((i<<16)/3)) + r.orbAngle
		r.orbs[i] = robotOrb{
			x: cosU16(a)*r.orbRadius + z.x,
			y: z.y + sinU16(a)*r.orbRadius*robotOrbSquash,
			z: height * r.orbHeight,
		}
	}
	switch {
	case r.dead:
		p.robotDyingFrame(z)
		z.alpha = 1
	case r.teleportT <= 0:
		if ai.gun.rounds > 0 || r.anger > 0 {
			if ai.state != 0 && r.anger > 0 {
				r.anger -= dt * robotAngerDecay
			}
		} else {
			p.robotPick(z)
		}
	default:
		if !r.teleportOut {
			r.teleportT -= dt
			if r.teleportT <= 0 {
				r.teleportT = 0
				r.teleportOut = true
				p.robotPick(z)
				r.teleportOut = false
			}
		} else {
			r.teleportT += dt
			if r.teleportT >= robotTeleportEnd {
				r.teleportT = robotTeleportEnd
				r.teleportOut = false
				p.robotTeleport(z, prey)
			}
		}
		r.lift = robotTeleportLiftMul*r.teleportT + robotLiftIdle
		alpha := int((robotTeleportEnd - r.teleportT) * robotTeleportAlphaMul)
		if alpha < 0 {
			alpha = 0
		}
		if alpha > 0xfe {
			alpha = 0xff
		}
		z.alpha = float64(alpha) / 255
	}
	r.prevRatio = ratio
}

// robotAddAngle adds a (positive) float to a 16-bit angle with the original's saturating float to ushort conversion.
func robotAddAngle(angle uint16, add float64) uint16 {
	v := float64(float32(angle)) + add
	if !(v > 0) {
		return 0
	}
	return uint16(int(v))
}

// robotTeleport moves the boss to the spot with the largest squared distance to the player, never the current one
// (a tie keeps spot 0), and sets +0x3ac.
func (p *playState) robotTeleport(z *zombieState, prey playerTarget) {
	r := &z.native.robot
	best, index := 0.0, 0
	for k := 0; k < r.spotCount; k++ {
		dx, dy := r.spots[k][0]-prey.x, r.spots[k][1]-prey.y
		d := dy*dy + dx*dx
		if best < d && r.spot != k {
			index, best = k, d
		}
	}
	r.spot = index
	if r.spotCount > 0 {
		z.x, z.y = r.spots[index][0], r.spots[index][1]
	}
}

// robotShockwaves releases one kind-0x1a shockwave per orb at the orb's position (owner the boss, life 0.7, final
// size 350, the init hook plays sound 12 for each) and shakes the camera once.
func (p *playState) robotShockwaves(z *zombieState) {
	r := &z.native.robot
	for i := 0; i < robotOrbs; i++ {
		p.rex.waves = append(p.rex.waves, rexShockwave{x: r.orbs[i].x, y: r.orbs[i].y, shot: p.newShot(), owner: z.scriptID, max: robotShockwaveSize, life: robotShockwaveLife, noCredit: true})
		p.sfxQueue = append(p.sfxQueue, rexShockwaveSound)
	}
	p.startCameraShake(z.x, z.y, robotShakeDuration, robotShakeAmpX, robotShakeAmpY, robotShakeAngleMul)
}

// stepRobot is FUN_0009fb10 for this class: the always-alerted branch (slot +0x74 is the constant 1), the aim /
// state selection, the state machine, the boss step (AI), the rise and the heading step. The robot never moves.
func (p *playState) stepRobot(z *zombieState, prey playerTarget, dx, dy, dt float64) {
	ai := &z.native.ai
	r := &z.native.robot
	if r.dead {
		return
	}
	if ai.wander < zombieWanderMin {
		ai.wander = float64(zombieRandom(p.rng, .3)) + zombieWanderMin
	}
	may := p.robotAim(z, dx, dy)
	if !may || ai.state == 0 {
		if ai.state == 3 {
			ai.state = 4
			if ai.gun.timer > -.25 {
				ai.gun.timer -= dt
			}
		}
	} else if ai.state == 1 {
		ai.state = 2
	}
	if ai.state < 2 {
		if ai.speedFactor < 1 {
			ai.speedFactor = math.Min(1, ai.speedFactor+dt*zombieSpeedRamp)
		}
	} else {
		p.zombieStateMachine(z, dt)
	}
	p.robotAI(z, prey, dt)
	if ai.state == 0 {
		ai.speedFactor = 0
		ai.riseMS += dt * 1000
		if ai.riseMS >= zombieRiseDoneMS {
			ai.state, ai.riseMS = 1, 0
		}
		z.size.X, z.size.Y, z.alpha = zombieRiseDims(z.native.sizeZ, ai.riseMS)
		if ai.state == 1 {
			z.size = formats.Vec2{X: zombieBodyWidth(z.native.sizeZ), Y: z.native.sizeZ}
			z.alpha = 1
		}
	}
	ai.phase += uint16(dt * float64(ai.cycle))
	if ai.state != 5 {
		z.native.facing = zombieHeadingStep(ai, z.native.facing, false)
	}
	// Slot +0x18 (FUN_000ba8f8): the drawn height is twice the width while the boss lives.
	z.size.Y = z.size.X * robotHeightScale
}

// robotGridExtent is slot +0x3c (FUN_000bad88): while the teleport timer is below 0.25 the boss is listed in the
// entity grid over +-0.3 * width; from 0.25 on it is unlisted (bullets pass through).
func (z *zombieState) robotGridExtent() (float32, bool) {
	r := &z.native.robot
	if r.teleportT >= .25 {
		return 0, false
	}
	return float32(z.size.X) * entityGridBodyScale, true
}

// robotDeath is vtable slot +0x50 (FUN_000baa28, 1.2.5 FUN_0011d6e8), run once when the boss starts dying: state 6,
// the Death clip, the width becomes the (last drawn) height, and a kind-0x15 blast goes off at the body (it hurts
// zombies, not the player). Story mode then replaces the wave list by one 1500 ms wave, closes the portals, stores
// the boss time as the best future boss time and counts the boss; survival only counts it.
func (p *playState) robotDeath(z *zombieState) {
	r := &z.native.robot
	if r.deathDone {
		return
	}
	r.deathDone = true
	r.dead = true
	z.hitFlash = 0 // FUN_000bae44 clears the hit timer on the lethal hit
	z.native.ai.state = 6
	z.native.ai.speedFactor = 0
	samuraiSetAnim(z, robotAnimDeath)
	z.size.X = z.size.Y
	p.spawnBlast(z.x, z.y, zombieBlastSound, killOrigin{gun: robotKillGun}, true)
	if p.isSurvival() || p.bossDefeated {
		p.noteBossKilled()
		return
	}
	p.bossDefeated = true
	if p.world != nil {
		p.waveIndex = len(p.world.Level.Waves)
	}
	p.samuraiWaveEndMS, p.samuraiWaveEnding = robotDeathWaveEnd, true
	p.robotKillSweep(z)
	p.closePortals()
	p.recordFutureBossTime(r.time)
	p.noteBossKilled()
}

// robotKillSweep is FUN_0006ee40: every living zombie of types 2..9 takes 1000 damage (slot +0x5c) through its
// normal death. The boss runs it once at its death and then every frame while it is in state 6 outside survival.
func (p *playState) robotKillSweep(self *zombieState) {
	for index := range p.zombies {
		other := &p.zombies[index]
		if other == self || other.dying || other.health <= 0 || other.spawnAway {
			continue
		}
		switch {
		case other.native.kind >= zombieKindPlain && other.native.kind <= zombieKindProspect:
			if p.hurtZombie(other, zombieHit{damage: robotDeathDamage}) {
				other.dying, other.deathAge = true, 0
				p.classifyZombieDeath(other, 0x10, other.x, other.y)
			}
		case other.native.kind == 0 && !p.zombieIsBoss(*other):
			// PORT: staged zombies without a native class (capture scenes) have no type to match; they are removed.
			other.health, other.spawnAway = 0, true
		}
	}
}

// recordFutureBossTime stores the boss time (seconds, truncated) as the best future boss time (stats +0xf4) when
// it is unset (-1) or not larger than the stored one.
func (p *playState) recordFutureBossTime(seconds float64) {
	if p.statistics == nil {
		return
	}
	t := float32(seconds)
	best := p.statistics.BestFutureTime
	if best == -1 || !(float32(uint32(best)) < t) {
		p.statistics.BestFutureTime = int32(uint32(t))
		p.statistics.Available["Best Future Boss Time"] = true
	}
}

// robotDeathClip is the Death clip state of the dying body: the frame index and the progress FUN_000f210c returns
// (frame / (n - 1) + fraction / n, clamped to 1). The clip runs at the real clock (slot +0x18 advances the sprite
// by dt once per frame) and the last frame stays.
func (p *playState) robotDeathClip(z *zombieState) (frame int, progress float64) {
	fps, frames := robotDeathClipFPS, 4.0
	if definition, ok := p.sprites.Find(z.texture); ok {
		if animation, ok := definition.Animation(robotAnimDeath); ok && animation.FPS > 0 && animation.Frames > 0 {
			fps, frames = animation.FPS, float64(animation.Frames)
		}
	}
	total := z.deathAge * fps
	last := frames - 1
	z.frame = math.Min(total, last)
	if int(total) >= int(last) {
		return int(last), 1
	}
	frame = int(total)
	progress = float64(frame)/last + (total-float64(frame))/frames
	return frame, math.Min(1, progress)
}

// robotDyingFrame is the +0x314 == 1 branch of FUN_000bb610: the body slides (-80, +100) * progress while the Death
// clip plays; at frame 2 one frame in four sets off a blast beside the body, from frame 3 on a flame spurts every
// frame; the colour alpha is forced to 0xff.
func (p *playState) robotDyingFrame(z *zombieState) {
	r := &z.native.robot
	frame, progress := p.robotDeathClip(z)
	r.shakeX = robotDeathShakeX * progress
	r.shakeY = robotDeathShakeY * progress
	z.flipX = false
	width := z.size.X
	if frame < 3 {
		if frame == 2 && zombieBounded(p.rng, robotDeathExplodeOdds) == 0 {
			ox := float64(zombieRandom(p.rng, float32(width*robotDeathBlastScale))) - width*robotDeathBlastHalf
			oy := float64(zombieBounded(p.rng, robotDeathYJitter))
			zombieRandom(p.rng, 100)
			zombieRandom(p.rng, 25)
			p.spawnBlast(z.x+ox, z.y+oy, zombieBlastSound, killOrigin{gun: robotKillGun}, true)
		}
		return
	}
	ox := float64(zombieRandom(p.rng, float32(width*robotDeathFlameScale))) - width*robotDeathFlameHalf
	oy := float64(zombieBounded(p.rng, robotDeathYJitter))
	zombieRandom(p.rng, 50)
	zombieRandom(p.rng, 25)
	heading := uint16(zombieBounded(p.rng, weapons.GibFlameBound))
	speed := z.native.ai.gun.speed * robotDeathFlameSpeed
	projectile, ok := weapons.NewFlameDebris(z.x+ox, z.y+oy, heading, robotDeathFlameLift, speed, p.rng)
	if !ok {
		return
	}
	p.bullets = append(p.bullets, bullet{x: z.x + ox, y: z.y + oy, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, hostile: true, noZombies: true})
}

// robotDeathSeconds is how long the dying body stays in the zombie list: for good in story mode (the pop sprite's
// playing flag is never cleared, so the exit script can pan to the body, as for the samurai), and the survival fade
// of FUN_000b48c0 otherwise.
func (p *playState) robotDeathSeconds(z zombieState) float64 {
	if !p.isSurvival() {
		return math.Inf(1)
	}
	if p.waveBuild() == content.WaveBuild125 {
		return 1 / samuraiDeadFade125
	}
	return 1 / samuraiDeadFadeV7
}

// robotBodyFlip is the sprite flip the draw uses for the robot (the facing hysteresis of FUN_000bb610).
func (z *zombieState) robotBodyFlip() bool { return !z.native.robot.flag }
