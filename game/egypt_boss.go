package game

import (
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// The Egyptian boss (boss_egyptian, native zombie type 12, the mummy of World2Level2). Recovered from libmortargame.so
// v7 and cross-checked against 1.2.5 (the two builds have the same logic and literals; the differences are listed
// below). v7: factory FUN_000a2ae0 case 0xc -> FUN_000b5824 (+0x328 = 0), vtable 0x005bbe68; post-init FUN_000b5198
// (vtable +0x78, run by +0x44 FUN_000b4864 at spawn), attack cycle FUN_000b5930 (+0x7c, called from the shared boss
// step FUN_000b48c0), may-attack FUN_000b5738 (+0x48), fire hook FUN_000b5558 (+0x64), damage FUN_000b5410 (+0x5c),
// death FUN_000b5070 (+0x50), flame burst FUN_000b5854, glow FUN_000b6198 (+0x40), alerted FUN_000b6348 (+0x74 = 1),
// cannot-fire FUN_000b5064 (+0x70 = 0), contact FUN_000b5060 (nop), move FUN_000b5138 (+0x3c: only lists the body in
// the entity grid). 1.2.5: AI FUN_00117868, post-init 0x001170b4, may-attack 0x00117690, hook 0x001174b0, damage
// 0x00117378, death 0x00116f80, boss step 0x00116340, glow 0x00118124, vtable 0x008c8340.
//
// Spawn (post-init):
//   - looks the level's prop up by its texture name "Sarcophagusclosed" (FUN_000bf978) and puts the boss on it;
//   - swaps that prop's texture for "sarcophagus" (sarcophagus_HD.png, the open coffin, same 256x128 size) and lifts
//     the prop 32 px and the boss 10 px (literals 32.0 / 10.0);
//   - facing 0x3ffc (down), speed 0, gun id 0 and no rounds, attack timer 1.0 and variant 2 (so the first attack after
//     the intro is the slam), state 1 (no rise), draw anchor 0.5 * size, width 0.796875 * size.
//
// Behaviour (FUN_000b5930, called every frame from the boss step, also while dead):
//   - facing flags: right when (facing - 0x2aa8) & 0xffff > 0xaaa0 (+0x33c), front when (facing - 0x5551) & 0xffff >
//     0x554e (+0x33d); both clear is the left side. Sprite animations come from the pharoh sprite's eight clips.
//   - The attack timer (+0x32c) is the lifetime of one close attack. At 0 and with no rounds left the boss rolls the
//     variant rnd(10) (<= 2: 0, <= 6: 1, else 2), plays SFX_MUMMY_ROAR_(1 + rnd(2)) (0x34 + rnd(2)), loads the FLAMER
//     record with no rounds and sets timer 1.0 / previous 1.1. While it runs (0.4 per second, 2.5 s):
//     variant 0: FireAttack, three flame bursts per frame at (+250u, 0), (0, +250u), (-250u, 0), u = 1 - timer;
//     variant 1: FireAttack, bursts at (0, +250u), (+176u, +176u), (-176u, +176u);
//     variant 2: Rage, the camera shakes as the timer falls through 0.65 and a kind 0x1a shockwave (life 1.6, 600
//     px, owner the boss) is released as it falls through 0.6 (with SFX_MUMMY_THUMP) and again through 0.5.
//   - When the timer runs out the boss picks its ranged attack with rnd(10): <= 2 MINIGUN (THROUGH bullets, 108
//     rounds in groups of 18, interval 0, wind-up timer -1, the beams of the fire hook), else CURVER (9 sickles, 13
//     below a quarter of its health, interval halved, or 0.05 when the roll was <= 5). The standard zombie state
//     machine fires them; the animations are AttackFront / AttackSide (CURVER) and RocketAttackFront / Side (hook).
//   - Dead: Death clip, flames harmless (hit flags 0) two per frame for 4 s (timer falls at 0.25 per second).
//
// 1.2.5 differences: the slam shakes every camera (FUN_00095d38, 2.0, 1.85, 1.85, 8.0 per camera instead of
// FUN_000be1fc 2.0, 1.85, 1.0, 1.0), and the damage slot refuses damage only while NO player is alive
// (FUN_000c3464) instead of while the one player is dead.
//
// PORT ADDITIONS: none in the behaviour; the glow overlay of the minigun wind-up is drawn with the exploding zombie's
// glow_overlay texture (the resource FUN_000b6198 draws is the "glow_overlay" resource of FUN_000a1748).
const (
	zombieKindEgypt   = 12
	egyptCoffinClosed = "Sarcophagusclosed"
	egyptCoffinOpen   = "sarcophagus"
	egyptCoffinLift   = 32.0
	egyptBossLift     = 10.0

	egyptFacing       = 0x3ffc // DAT_000b5300
	egyptRenderAnchor = .5     // +0x2d4 = 0x3f000000

	egyptRightBase  = 0x2aa8 // facing - 0x2aa8 above 0xaaa0: the right side
	egyptRightSpan  = 0xaaa0
	egyptFrontBase  = 0x5551 // facing - 0x5551 above 0x554e: front (up or down)
	egyptFrontSpan  = 0x554e
	egyptAttackRate = .4  // DAT_000b5db4
	egyptDeathRate  = .25 // DAT_000b5db8
	egyptArmPrev    = 1.1 // DAT_000b5da8

	egyptArmRoll    = 10
	egyptArmLowMax  = 2 // roll <= 2: variant 0
	egyptArmMidMax  = 6 // roll <= 6: variant 1
	egyptRangedRoll = 10
	egyptMinigunMax = 2 // roll <= 2: MINIGUN, else CURVER

	egyptGunMinigun = 3
	egyptGunFlamer  = 8
	egyptGunCurver  = 11

	egyptMinigunRounds   = 0x6c
	egyptMinigunKind     = 0x11
	egyptMinigunTimer    = -1.0 // DAT_000b5d94
	egyptCurverRounds    = 9
	egyptCurverRaged     = 13
	egyptCurverLowHealth = .25 // DAT_000b5db8 (0x3e800000)
	egyptCurverHalf      = .5  // DAT_000b5dbc
	egyptCurverFast      = .05 // DAT_000b5d98
	egyptCurverFastRoll  = 5

	egyptGroup       = 18 // rounds per beam group (idivmod 0x12)
	egyptBeamRoar    = 15
	egyptBeamHitRem  = 15.3 // the double at 0x000b5710
	egyptBeamSpeed   = 600.0
	egyptBeamLift    = 50.0 // DAT_000b5718
	egyptBeamBase    = 10.0
	egyptBeamAlpha   = 50.0
	egyptBeamAlphaVr = 200.0
	egyptAimFall     = 50.0  // DAT_000b5814
	egyptAimSpeed    = 600.0 // DAT_000b5810

	egyptArm0       = 250.0 // DAT_000b5d5c
	egyptArm1       = 176.0 // DAT_000b5d60
	egyptFlameSpeed = .1    // DAT_000b5920
	egyptFlameLift  = 10.0  // DAT_000b5924

	egyptSlamShake   = .65
	egyptSlamBlastA  = .6
	egyptSlamBlastB  = .5
	egyptBlastLife   = 1.6 // DAT_000b5d8c
	egyptBlastSize   = 600.0
	egyptShakeSecs   = 2.0  // 0x40000000
	egyptShakeAmp    = 1.85 // DAT_000b5d88
	egyptShakeAmpY   = 1.0
	egyptShakeAngle  = 1.0
	egyptShakeAngleH = 8.0 // 1.2.5 FUN_00095d38 angle multiplier

	egyptGlowBase  = 10.0  // DAT_000b6324
	egyptGlowRate  = 120.0 // DAT_000b6320
	egyptGlowScale = 1.5   // DAT_000b6328

	egyptSoundLaser  = "SFX_LASER_1"            // 0x16
	egyptSoundSickle = "SFX_MUMMY_SICKLE_THROW" // 0x37
	egyptSoundDeath  = "SFX_MUMMY_DEATH"        // 0x36
	egyptSoundThump  = "SFX_MUMMY_THUMP"        // 0x38
	egyptBladeSprite = "Common0/Textures/eblade_SD"
	egyptBeamTexture = "Common0/Textures/beam"
	egyptBlastSound  = rexShockwaveSound
)

// The pharoh sprite animations the class selects by name (the id table is filled by _INIT_56 at 0x000b6394: slot +0
// RocketAttackFront, +4 RocketAttackSide, +8 Death, +0xc FireAttack, +0x10 Rage, +0x14 AttackFront, +0x18 AttackSide,
// +0x80 Idle).
const (
	egyptAnimIdle              = "Idle"
	egyptAnimDeath             = "Death"
	egyptAnimFire              = "FireAttack"
	egyptAnimRage              = "Rage"
	egyptAnimAttackFront       = "AttackFront"
	egyptAnimAttackSide        = "AttackSide"
	egyptAnimRocketAttackFront = "RocketAttackFront"
	egyptAnimRocketAttackSide  = "RocketAttackSide"
)

// egyptBoss is the per-zombie state of the class (field names give the native offsets).
type egyptBoss struct {
	active       bool
	gunID        int     // +0x2e4 the loaded gun record id (0 none)
	timer        float64 // +0x32c attack / death countdown
	prev         float64 // +0x330 the timer of the previous frame
	fall         float64 // +0x334 beam height fall rate
	variant      int     // +0x338 close attack 0..2
	dead         bool    // +0x314
	deathDone    bool    // the one-shot death hook ran
	scored       bool    // the kill was credited by the death hook (the body is never removed in story mode)
	right, front bool    // +0x33c, +0x33d
}

func isEgyptName(name string) bool { return name == "boss_egyptian" }

// egyptBossCoffin opens the level's sarcophagus and returns where the boss stands.
func (p *playState) egyptBossCoffin(point formats.Vec2) (formats.Vec2, bool) {
	if p.world == nil {
		return point, false
	}
	for i := range p.world.Level.Props {
		prop := &p.world.Level.Props[i]
		if strings.EqualFold(prop.Texture, egyptCoffinClosed) {
			at := formats.Vec2{X: prop.X, Y: prop.Y - egyptBossLift}
			prop.Texture = egyptCoffinOpen
			prop.Y -= egyptCoffinLift
			return at, true
		}
	}
	return point, false
}

// initEgypt is the rest of the post-init FUN_000b5198 (the coffin is egyptBossCoffin): facing 0x3ffc, no gun, no
// speed, timer 1.0 and variant 2, state 1 and a full-size body.
func (p *playState) initEgypt(z *zombieState) {
	ai := &z.native.ai
	z.native.egypt = egyptBoss{active: true, timer: 1, variant: 2}
	z.native.facing = egyptFacing
	ai.gun.rounds = 0
	ai.state = 1
	z.speed = 0
	z.alpha = 1
	z.size = formats.Vec2{X: zombieBodyWidth(z.native.sizeZ), Y: z.native.sizeZ}
	z.animation = egyptAnimIdle
	z.fps = p.spriteFPS(z.texture, egyptAnimIdle)
	z.frame = 0
}

// egyptSetAnim is FUN_000f1d48(sprite, id, 0) plus the sprite's flip byte (+0x21): nothing but the flip changes when the
// clip is already current, otherwise the clip is switched and restarted.
func (p *playState) egyptSetAnim(z *zombieState, name string, flip bool) {
	z.flipX = flip
	if z.animation == name {
		return
	}
	z.animation = name
	z.frame = 0
	z.fps = p.spriteFPS(z.texture, name)
}

// egyptClampFrame holds a non-looping clip (Rage) on its last frame; FUN_000f1ca4 stops advancing there.
func (p *playState) egyptClampFrame(z *zombieState) {
	definition, ok := p.sprites.Find(z.texture)
	if !ok {
		return
	}
	animation, ok := definition.Animation(z.animation)
	if !ok || animation.Loop || animation.Frames <= 0 {
		return
	}
	if limit := float64(animation.Frames) - 1; z.frame > limit {
		z.frame = limit
	}
}

// egyptLoadGun is FUN_000b19b8(FUN_000b10dc(), id): the raw record of gun id copied to +0x2e4..+0x30c (rounds are not
// doubled here; every caller overwrites them).
func (p *playState) egyptLoadGun(z *zombieState, id int) {
	gun := zombieGun{}
	name := zombieGunName(id)
	for _, record := range p.zombieWeapons {
		if record.GunType != name {
			continue
		}
		bullet, ok := zombieBulletKinds[record.BulletType]
		if !ok {
			bullet = 0x10
		}
		gun = zombieGun{timer: record.RecoilSeconds, interval: record.RateOfFire, spread: record.SpreadUnits, rounds: record.Ammo, pellets: record.AmmoPerShot, life: record.Life, speed: record.Speed, kind: bullet}
		break
	}
	z.native.ai.gun = gun
	z.native.egypt.gunID = id
}

// egyptMayAttack is vtable slot +0x48 (FUN_000b5738), always true. With the MINIGUN loaded and the rounds at a multiple
// of 18 a new group starts: wind-up timer -1, one round spent, the aim latched at the player and the beam fall rate
// 50 / (distance / 600); the aim is not updated inside a group. Any other gun aims at the player every call.
func (p *playState) egyptMayAttack(z *zombieState, prey playerTarget, dx, dy float64) {
	e := &z.native.egypt
	ai := &z.native.ai
	if e.gunID != egyptGunMinigun {
		ai.desired = weapons.NativeWeaponDirection(dx, dy)
		return
	}
	if ai.gun.rounds%egyptGroup != 0 {
		return
	}
	ai.gun.timer = egyptMinigunTimer
	ai.gun.rounds--
	ai.desired = weapons.NativeWeaponDirection(dx, dy)
	distance := float32(math.Sqrt(float64(float32(z.x-prey.x)*float32(z.x-prey.x) + float32(z.y-prey.y)*float32(z.y-prey.y))))
	e.fall = float64(float32(egyptAimFall) / (distance / float32(egyptAimSpeed)))
}

// egyptStateMachine is FUN_0009f32c for states 2..4 of this class (type byte 12: the charging-zombie cases do not
// apply); the cannot-fire slot (+0x70) is the constant 0.
func (p *playState) egyptStateMachine(z *zombieState, dt float64) {
	ai := &z.native.ai
	gun := &ai.gun
	switch ai.state {
	case 2:
		ai.speedFactor -= dt * zombieBrakeRate
		if ai.speedFactor < 0 {
			ai.speedFactor, ai.state = 0, 3
		}
	case 3:
		if gun.interval <= gun.timer {
			p.egyptFire(z)
			return
		}
		gun.timer = math.Min(gun.timer+dt, gun.interval)
	case 4:
		ai.speedFactor += dt * zombieBrakeRate
		if ai.speedFactor > 1 {
			ai.speedFactor, ai.state = 1, 1
		}
	}
}

// stepEgypt is FUN_0009fb10 for this class: the always-alerted branch (slot +0x74 is the constant 1), the aim, the state
// selection, the state machine, the boss step (the attack cycle) and the heading step. The boss never moves (speed 0
// and the move slot only lists it in the entity grid).
func (p *playState) stepEgypt(z *zombieState, prey playerTarget, dx, dy, dt float64) {
	ai := &z.native.ai
	e := &z.native.egypt
	if e.dead {
		return
	}
	if ai.wander < zombieWanderMin {
		ai.wander = float64(zombieRandom(p.rng, .3)) + zombieWanderMin
	}
	p.egyptMayAttack(z, prey, dx, dy)
	if ai.state == 1 {
		ai.state = 2
	}
	if ai.state < 2 {
		if ai.speedFactor < 1 {
			ai.speedFactor = math.Min(1, ai.speedFactor+dt*zombieSpeedRamp)
		}
	} else {
		p.egyptStateMachine(z, dt)
	}
	p.egyptAI(z, dt)
	ai.phase += uint16(dt * float64(ai.cycle))
	if ai.state != 5 {
		z.native.facing = zombieHeadingStep(ai, z.native.facing, false)
	}
}

// egyptFlame is FUN_000b5854: one kind 0x16 flame at (x, y) with the height z + 10, a random heading, speed 0.1 * the
// loaded gun's speed and the owner the boss. Both hit flags are set while the boss lives and clear once it is dead
// (the death flames are harmless).
func (p *playState) egyptFlame(z *zombieState, x, y, height float64) {
	heading := uint16(zombieBounded(p.rng, weapons.GibFlameBound))
	speed := z.native.ai.gun.speed * egyptFlameSpeed
	projectile, ok := weapons.NewFlameDebris(x, y, heading, height+egyptFlameLift, speed, p.rng)
	if !ok {
		return
	}
	live := !z.native.egypt.dead
	p.bullets = append(p.bullets, bullet{x: x, y: y, vx: projectile.VX, vy: projectile.VY, life: projectile.Life, projectile: &projectile, hostile: live, noZombies: !live, ownerID: z.scriptID})
}

// egyptAI is FUN_000b5930 (1.2.5 FUN_00117868).
func (p *playState) egyptAI(z *zombieState, dt float64) {
	e := &z.native.egypt
	ai := &z.native.ai
	e.right, e.front = false, false
	facing := z.native.facing
	if uint16(facing-egyptRightBase) > egyptRightSpan {
		e.right = true
	} else if uint16(facing-egyptFrontBase) > egyptFrontSpan {
		e.front = true
	}
	running := p.samuraiScriptRunning()
	if !(e.timer > 0) {
		// No attack running: the ranged guns fire through the shared state machine until they are empty.
		if ai.state == 0 || e.dead || running {
			return
		}
		if e.gunID == egyptGunCurver {
			if e.front {
				p.egyptSetAnim(z, egyptAnimAttackFront, false)
			} else {
				p.egyptSetAnim(z, egyptAnimAttackSide, e.right)
			}
		}
		if ai.gun.rounds > 0 {
			return
		}
		roll := int(zombieBounded(p.rng, egyptArmRoll))
		switch {
		case roll <= egyptArmLowMax:
			e.variant = 0
		case roll <= egyptArmMidMax:
			e.variant = 1
		default:
			e.variant = 2
		}
		p.sfxQueue = append(p.sfxQueue, "SFX_MUMMY_ROAR_"+string(rune('1'+zombieBounded(p.rng, 2))))
		p.egyptLoadGun(z, egyptGunFlamer)
		ai.gun.rounds = 0
		e.timer, e.prev = 1, egyptArmPrev
		return
	}
	if e.dead {
		// The dying body: the Death clip and two harmless flames a frame above it while the 4 s timer runs.
		p.egyptSetAnim(z, egyptAnimDeath, false)
		e.timer = float64(float32(e.timer) - float32(dt)*float32(egyptDeathRate))
		for burst := 0; burst < 2; burst++ {
			x := z.x + float64(zombieRandom(p.rng, 50)-25)
			y := z.y + float64(zombieBounded(p.rng, 25))
			height := float64((float32(1)-float32(e.timer))*zombieRandom(p.rng, 50)) + float64(zombieRandom(p.rng, 25))
			p.egyptFlame(z, x, y, height)
		}
		return
	}
	if running {
		return
	}
	u := float32(1) - float32(e.timer)
	switch e.variant {
	case 0:
		p.egyptSetAnim(z, egyptAnimFire, false)
		p.egyptFlame(z, z.x+float64(u*egyptArm0), z.y, 0)
		p.egyptFlame(z, z.x, z.y+float64(u*egyptArm0), 0)
		p.egyptFlame(z, z.x-float64(u*egyptArm0), z.y, 0)
	case 1:
		p.egyptSetAnim(z, egyptAnimFire, false)
		p.egyptFlame(z, z.x, z.y+float64(u*egyptArm0), 0)
		p.egyptFlame(z, z.x+float64(u*egyptArm1), z.y+float64(u*egyptArm1), 0)
		p.egyptFlame(z, z.x-float64(u*egyptArm1), z.y+float64(u*egyptArm1), 0)
	default:
		p.egyptSetAnim(z, egyptAnimRage, false)
		timer, prev := float32(e.timer), float32(e.prev)
		if timer < egyptSlamShake && prev >= egyptSlamShake {
			p.egyptShake(z)
		}
		if (timer < egyptSlamBlastA && prev >= egyptSlamBlastA) || (timer < egyptSlamBlastB && prev >= egyptSlamBlastB) {
			p.rex.waves = append(p.rex.waves, rexShockwave{x: z.x, y: z.y, shot: p.newShot(), owner: z.scriptID, max: egyptBlastSize, life: egyptBlastLife, noCredit: true})
			p.sfxQueue = append(p.sfxQueue, egyptBlastSound)
			if prev >= egyptSlamBlastA {
				p.sfxQueue = append(p.sfxQueue, egyptSoundThump)
			}
		}
	}
	e.prev = e.timer
	e.timer = float64(float32(e.timer) - float32(dt)*float32(egyptAttackRate))
	if e.timer > 0 {
		return
	}
	// The close attack is over: pick the ranged gun.
	roll := int(zombieBounded(p.rng, egyptRangedRoll))
	if roll <= egyptMinigunMax {
		p.egyptLoadGun(z, egyptGunMinigun)
		ai.gun.interval = 0
		ai.gun.timer = egyptMinigunTimer
		ai.gun.rounds = egyptMinigunRounds
		ai.gun.kind = egyptMinigunKind
		return
	}
	p.egyptLoadGun(z, egyptGunCurver)
	ai.gun.rounds = egyptCurverRounds
	ratio := float32(0)
	if z.native.maxHealth != 0 {
		ratio = float32(z.health) / float32(z.native.maxHealth)
	}
	if ratio < egyptCurverLowHealth {
		ai.gun.rounds = egyptCurverRaged
		ai.gun.interval = float64(float32(ai.gun.interval) * float32(egyptCurverHalf))
		if roll <= egyptCurverFastRoll {
			ai.gun.interval = egyptCurverFast
		}
	}
}

// egyptShake is the slam's camera shake: FUN_000be1fc(camera, pos, 2.0, 1.85, 1.0, 1.0) in v7, FUN_00095d38(camera,
// pos, 2.0, 1.85, 1.85, 8.0) for every camera in 1.2.5 (one shared camera here).
func (p *playState) egyptShake(z *zombieState) {
	ampY, angleMul := egyptShakeAmpY, egyptShakeAngle
	if !cameraIsV7(p.waveBuild()) {
		ampY, angleMul = egyptShakeAmp, egyptShakeAngleH
	}
	p.startCameraShake(z.x, z.y, egyptShakeSecs, egyptShakeAmp, ampY, angleMul)
}

// egyptFire is FUN_0009f7e4 for this class followed by the post-fire hook FUN_000b5558 (1.2.5 0x001174b0) for every
// pellet: the timer resets, one round is spent and a CURVER sickle is thrown at the player (the hook stops and restarts
// SFX_MUMMY_SICKLE_THROW); with the MINIGUN loaded the hook turns the bullet into a 'beam' sprite from the boss along
// the latched aim: speed 600, height 50 falling at +0x334, life 1.0, width and height 10 + 10 * rem / 18, alpha byte
// 50 + 200 * rem / 18, laser sound at rem 15, and only the shots with rem above 15.3 hurt (each frame they overlap).
func (p *playState) egyptFire(z *zombieState) {
	ai := &z.native.ai
	e := &z.native.egypt
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
		if gun.kind == samuraiBladeKind {
			jitter = p.fireSpinner(z, heading, jitter, lift, fall, gun)
			if count := len(p.spinners); count > 0 {
				p.spinners[count-1].texture = egyptBladeSprite // +0x30c: the "eblade" resource of FUN_000a54d4(10)
			}
		}
		if e.gunID == egyptGunMinigun {
			rem := gun.rounds % egyptGroup
			if rem == egyptBeamRoar {
				p.sfxQueue = append(p.sfxQueue, egyptSoundLaser)
			}
			aim := ai.desired
			beam := robotBeam{x: z.x, y: z.y, heading: aim, life: robotBeamLife, speed: egyptBeamSpeed, lift: egyptBeamLift, fall: e.fall, hits: float64(rem) > egyptBeamHitRem}
			beam.dirX, beam.dirY = cosU16(aim), sinU16(aim)
			share := float64(float32(rem) / float32(egyptGroup))
			beam.size = share*egyptBeamBase + egyptBeamBase
			beam.alpha = uint8(int(egyptBeamAlpha + share*egyptBeamAlphaVr))
			if beam.speed > 0 && beam.fall != 0 {
				beam.heading = weapons.NativeWeaponDirection(beam.dirX, beam.fall/beam.speed+beam.dirY) // FUN_000a57e4 tilt
			}
			p.robotShots.beams = append(p.robotShots.beams, beam)
		} else if gun.kind == samuraiBladeKind {
			p.egyptSickleSound()
		}
	}
}

// egyptSicklePlayer is the one channel of sound id 0x37: the hook stops it when it is still playing and starts it
// again, so a new throw cuts the previous one off (flushSfxQueue hands the queue entry to playEgyptSickle).
var egyptSicklePlayer *audio.Player

func (p *playState) egyptSickleSound() {
	p.sfxQueue = append(p.sfxQueue, egyptSoundSickle)
}

// playEgyptSickle is FUN_000ca828(0x37) / FUN_000ca7c8(0x37) / FUN_000ca410(0x37).
func (a *app) playEgyptSickle() {
	if egyptSicklePlayer != nil {
		_ = egyptSicklePlayer.Close()
		egyptSicklePlayer = nil
	}
	if a == nil || a.silent || !a.options.sound || a.sound == nil {
		return
	}
	if player, err := a.sound.PlayTracked(a.scriptSoundPath(egyptSoundSickle), .8, -1); err == nil {
		egyptSicklePlayer = player
	}
}

// egyptCanBeHurt is the front of the damage slot (v7 FUN_000b5410, 1.2.5 0x00117378): no damage once the player is
// dead (1.2.5: once every player is dead).
func (p *playState) egyptCanBeHurt() bool {
	if p.waveBuild() == content.WaveBuild125 {
		return len(p.livingPlayers()) > 0
	}
	return p.health > 0
}

// egyptDeath is the death hook FUN_000b5070 (1.2.5 0x00116f80), run once: state 6, dead, timer 1.0, SFX_MUMMY_DEATH
// and, in story mode, the wave list replaced by one 1500 ms wave, every living zombie of types 2..9 takes 1000
// damage, the spawn portals close; the best time and the boss counter are written by noteBossDeathStats. The body
// stays for good in story mode (as the samurai's, see samuraiDeathSeconds) and its kill is credited here.
func (p *playState) egyptDeath(z *zombieState) {
	e := &z.native.egypt
	if e.deathDone {
		return
	}
	e.deathDone, e.dead = true, true
	e.timer = 1
	z.native.ai.state = 6
	p.sfxQueue = append(p.sfxQueue, egyptSoundDeath)
	if !e.scored {
		e.scored = true
		p.levelKills = int(int32(p.levelKills) + 1)
		if p.hudVisible {
			points := int32(z.rawPoints) / 10 * 10
			p.score = int(int32(p.score) + (points/20)*int32(p.multiplier))
		}
	}
	if p.isSurvival() || p.bossDefeated {
		return
	}
	p.bossDefeated = true
	if p.world != nil {
		p.waveIndex = len(p.world.Level.Waves)
	}
	p.samuraiWaveEndMS, p.samuraiWaveEnding = samuraiDeathWaveEnd, true
	for index := range p.zombies {
		other := &p.zombies[index]
		if other == z || other.dying || other.health <= 0 || other.spawnAway {
			continue
		}
		switch {
		case other.native.kind >= zombieKindPlain && other.native.kind <= zombieKindProspect:
			if p.hurtZombie(other, zombieHit{damage: samuraiDeathDamage}) {
				other.dying, other.deathAge = true, 0
				p.classifyZombieDeath(other, 0x10, other.x, other.y)
			}
		case other.native.kind == 0 && !p.zombieIsBoss(*other):
			// PORT: staged zombies without a native class (capture scenes) have no type to match; they are removed.
			other.health, other.spawnAway = 0, true
		}
	}
	p.closePortals()
}

// egyptDeathPose is the draw state of the dying body: the Death clip (loop 1) from the death hook on.
func (p *playState) egyptDeathPose(z *zombieState) {
	p.egyptSetAnim(z, egyptAnimDeath, false)
	fps := z.fps
	if fps <= 0 {
		fps = 8
	}
	z.frame = z.deathAge * fps
}

// drawEgyptGlow is the glow FUN_000b6198 (1.2.5 0x00118124) draws while the MINIGUN is loaded and the boss lives: a
// white glow_overlay quad 1.5 times the body at its centre, alpha byte 10 + (timer + 1) * 120 (the wind-up timer runs
// -1 .. 0).
func (a *app) drawEgyptGlow(screen *ebiten.Image, z zombieState) {
	e := &z.native.egypt
	if !e.active || e.dead || z.dying || e.gunID != egyptGunMinigun || a.play == nil || a.play.world == nil {
		return
	}
	texture, err := a.Texture(explodingGlowTexture)
	if err != nil || texture.Bounds().Dx() <= 0 || texture.Bounds().Dy() <= 0 {
		return
	}
	alpha := float32(int(egyptGlowBase+(z.native.ai.gun.timer+1)*egyptGlowRate)&0xff) / 255
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	frontendX, frontendY := a.renderScale()
	x := (z.x-world.CameraX)*zoom + world.ViewportX
	y := (z.y-world.CameraY)*zoom + world.ViewportY - z.native.sizeZ*egyptRenderAnchor*zoom*frontendX/frontendY
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
	options.GeoM.Scale(z.size.X*egyptGlowScale*zoom/float64(texture.Bounds().Dx()), z.size.Y*egyptGlowScale*zoom*frontendX/frontendY/float64(texture.Bounds().Dy()))
	options.GeoM.Translate(x, y)
	options.ColorScale.Scale(alpha, alpha, alpha, alpha)
	options.GeoM.Scale(frontendX, frontendY)
	screen.DrawImage(texture, options)
}
