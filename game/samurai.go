package game

import (
	"fmt"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// The Japan boss (boss_samurai, native zombie type 13). Recovered from libmortargame.so v7 and cross-checked in
// 1.2.5 (Research/native/samurai-boss-2026-10-11.md): class factory FUN_000a2ae0 case 0xd, AI FUN_000bcfa0
// (1.2.5 FUN_0011fffc), path / may-attack FUN_000bcbc0, gun load FUN_000bceb4, post-init FUN_000bcf28, contact
// FUN_000bd528, death FUN_000bcaec, draw FUN_000bce68. The class reuses the shared zombie update FUN_0009fb10
// (stepSamurai mirrors it for this class) and the shared state machine FUN_0009f32c.
const (
	zombieKindSamurai = 13

	samuraiSightFactor    = 5.0    // DAT_000bcf8c: range^2 = sight^2 * 5 (post-init FUN_000bcf28)
	samuraiGunRounds      = 3      // FUN_000bceb4: +0x2f4 = 3 (not doubled)
	samuraiGunSpeed       = 250.0  // DAT_000bcf20 0x437a0000
	samuraiGunInterval    = .2     // DAT_000bcf24 0x3e4ccccd
	samuraiReloadSeconds  = 5.0    // DAT_000bd398: +0x338 once the rounds are gone
	samuraiPathReload     = 100    // +0x334 reload
	samuraiPathListReady  = 30     // list empty and counter > 30
	samuraiPathDropDistSq = 250000 // DAT_000bcdfc: 500 px
	samuraiWaypointSq     = .16    // DAT_000bce04: waypoint reached within 0.4 * width

	samuraiLungeInterval = .95  // DAT_0009f524
	samuraiLungeArc      = 3639 // DAT_0009f528 (heading units)
	samuraiLungeSpeed    = 4.0  // DAT_0009f52c and DAT_000bd3b0
	samuraiLungeBrake    = 4.0  // DAT_0009f51c

	samuraiTeleportRate    = .75      // DAT_000bd380: special timer per second
	samuraiTeleportSpeed   = 5.0      // DAT_000bd398 written to +0x48
	samuraiTeleportMinDist = 200.0    // DAT_000bd38c
	samuraiTeleportSpan    = 100.0    // DAT_000bd388
	samuraiTeleportDivisor = 524287.0 // DAT_000bd384
	samuraiTeleportBound   = 0x7ffff  // DAT_000bd3a4
	samuraiTeleportTries   = 512      // bound on the native "repeat until the line is free" loop (never reached on open ground)
	samuraiTeleportAlpha   = 254.0    // DAT_000bd390

	samuraiSpinRate     = .333    // DAT_000bd368 0x3eaa7efa: special timer per second
	samuraiSpinSpeedAdd = 1.1     // DAT_000bd36c
	samuraiSpinHeading  = 65338.0 // DAT_000bd374
	samuraiSpinSpinMul  = 4.0     // DAT_000bd378
	samuraiSpinWeave    = 3.0     // DAT_000bd3c8
	samuraiSpinTurn     = .666    // DAT_000bd37c 0x3f2a7efa
	samuraiSpinSpeed    = 2.0     // initial +0x48 of the spin (0x40000000)

	samuraiRollRange = 10

	samuraiAttackAnimSpeed  = 3.0     // DAT_000bd3c8: with a faster dash and
	samuraiAttackAnimDistSq = 10000.0 // DAT_000bd3cc: farther than 100 px the sheet is kept

	samuraiContactDamage = .15 // DAT_000bd5cc

	samuraiDeadFadeV7    = 1.0 / 12        // DAT_000b49b8 0x3daaaaab: survival fade per second (v7, x86)
	samuraiDeadFade125   = 1.0 / 6         // DAT_0011646c 0x3e2aaaab (1.2.5)
	samuraiShurikenSecs  = 24729.0 / 44100 // Shuriken_throw.ogg (OGG header: 24729 samples at 44100 Hz)
	samuraiDeathWaveEnd  = 1500            // FUN_000c0f18(scene+0x35d0c, 1500): the single replacement wave's timer (ms)
	samuraiDeathDamage   = 1000
	samuraiPhaseBase     = 5.0  // DAT_000bd3c4
	samuraiPhaseHealth   = .95  // DAT_000bd3c0 (the note's 3.0 was a misreading: see the research note)
	samuraiLungeKindMax  = 3    // phase > 3: no gun, dash again
	samuraiDeathClipRate = 2.0  // the Death clip is advanced by FUN_000a1b08 and by the AI in the same frame
	samuraiBladeKind     = 0x18 // SPINNER
)

const (
	samuraiAnimIdle   = "Idle"   // _INIT_61: ids [3]
	samuraiAnimAttack = "Attack" // ids [2]
	samuraiAnimSpin   = "Spin"   // ids [1]
	samuraiAnimDeath  = "Death"  // ids [0]
)

// samuraiBoss is the per-zombie state of the class (field names give the native offsets).
type samuraiBoss struct {
	active     bool
	time       float64 // +0x328 seconds without a running script
	reload     float64 // +0x338 gun reload countdown
	distSq     float64 // +0x33c squared distance to the player at the last may-attack call
	phase      int     // +0x340
	prevState  int     // +0x344
	special    float64 // +0x348 teleport / spin timer
	spin       uint16  // +0x34c spin heading
	prevRounds int     // +0x354
	pathTimer  int     // +0x334
	pathActive bool    // +0x32c list is not empty (a route around a wall)
	rangeSq    float64 // +0x274 = sight^2 * 5
	dead       bool    // +0x314
	deathDone  bool    // the one-shot death effects of FUN_000bcaec ran
}

func isSamuraiName(name string) bool { return name == "boss_samurai" }

// initSamurai is the class constructor tail and post-init FUN_000bcf28 (1.2.5 FUN_0011ff0c): the range is the
// sight radius squared times 5, the previous rounds are 0 and the gun is loaded (FUN_000bceb4). Speed, size and
// turn come from the spawn record through newWaveZombie (v7 reads the XML; 1.2.5 hard-codes the same numbers for
// world3_level2).
func (p *playState) initSamurai(z *zombieState) {
	ai := &z.native.ai
	s := &z.native.samurai
	*s = samuraiBoss{active: true, phase: 2, pathTimer: samuraiPathReload}
	s.rangeSq = ai.sightBase * samuraiSightFactor
	p.samuraiLoadGun(z)
}

// samuraiLoadGun is FUN_000bceb4: the CURVER record, then rounds 3, bullet speed 250, interval 0.2, spread 0 and
// the reload timer cleared.
func (p *playState) samuraiLoadGun(z *zombieState) {
	gun := zombieGun{}
	for _, record := range p.zombieWeapons {
		if record.GunType != zombieGunName(11) {
			continue
		}
		bullet, ok := zombieBulletKinds[record.BulletType]
		if !ok {
			bullet = 0x10
		}
		gun = zombieGun{timer: record.RecoilSeconds, pellets: record.AmmoPerShot, life: record.Life, kind: bullet}
		break
	}
	gun.rounds = samuraiGunRounds
	gun.speed = samuraiGunSpeed
	gun.spread = 0
	gun.interval = samuraiGunInterval
	z.native.ai.gun = gun
	z.native.samurai.reload = 0
}

// samuraiScriptRunning is FUN_000d9ab4.
func (p *playState) samuraiScriptRunning() bool {
	return p.scriptRuntime != nil && !p.scriptRuntime.Done()
}

// samuraiCannotFire is vtable slot +0x70 (FUN_000bce14): false while a script runs, true with no rounds,
// otherwise true while the phase is above 3 (the dash chain).
func (p *playState) samuraiCannotFire(z *zombieState) bool {
	if p.samuraiScriptRunning() {
		return false
	}
	if z.native.ai.gun.rounds < 1 {
		return true
	}
	return z.native.samurai.phase > samuraiLungeKindMax
}

// samuraiMayAttack is vtable slot +0x48 (FUN_000bcbc0, a copy of the smart zombie routine). The counter drops by one
// per call and reloads to 100; at <= 0 the route is dropped when the player is farther than 500 px, otherwise a tile
// trace to the player decides: clear drops the route, blocked builds one. The heading (+0x298) is the bearing to the
// first waypoint (the port's navigation step stands for the waypoint list FUN_0009bbb8 builds) or to the player when
// there is no route. Returns true (state 1 -> 2) with no route when the counter is above 30 and the player is within
// the range, or with a route when the gun has rounds and the line to the player is free.
func (p *playState) samuraiMayAttack(z *zombieState, distSq, targetX, targetY float64, prey int) bool {
	ai := &z.native.ai
	s := &z.native.samurai
	s.pathTimer--
	s.distSq = distSq
	body := zombieCollisionRadius(*z) * .8
	if s.pathTimer < 1 {
		if distSq > samuraiPathDropDistSq {
			s.pathActive = false
		} else {
			s.pathActive = !p.lineClear(z.x, z.y, targetX, targetY, body)
		}
		s.pathTimer = samuraiPathReload
	}
	vx, vy := targetX-z.x, targetY-z.y
	aim := distSq > z.size.X*z.size.X*samuraiWaypointSq
	result := false
	if !s.pathActive {
		result = s.pathTimer > samuraiPathListReady
	} else if ai.gun.rounds > 0 && p.lineClear(z.x, z.y, targetX, targetY, 1) {
		result = true
	} else if navX, navY, ok := p.navDirection(z.x, z.y, prey); ok {
		// The waypoint is a tile away: it is never inside 0.4 * width, so the heading always follows it.
		vx, vy, aim = navX, navY, true
	}
	if aim {
		ai.desired = weapons.NativeWeaponDirection(vx, vy)
	}
	return result && distSq <= s.rangeSq
}

// samuraiStateMachine is FUN_0009f32c for states 2..5 of this class (type byte 13: the charging-zombie special
// cases of states 2 and 3 do not apply).
func (p *playState) samuraiStateMachine(z *zombieState, dt float64) {
	ai := &z.native.ai
	gun := &ai.gun
	switch ai.state {
	case 2:
		ai.speedFactor -= dt * zombieBrakeRate
		if ai.speedFactor < 0 {
			ai.speedFactor, ai.state = 0, 3
		}
	case 3:
		if !p.samuraiCannotFire(z) {
			if gun.interval <= gun.timer {
				p.zombieFire(z)
				return
			}
			gun.timer = math.Min(gun.timer+dt, gun.interval)
			return
		}
		gun.interval = samuraiLungeInterval
		diff := angleDifference(ai.desired, z.native.facing)
		if gun.timer < gun.interval {
			gun.timer += dt
		} else if absInt(diff) <= samuraiLungeArc {
			ai.state, ai.speedFactor = 5, samuraiLungeSpeed
			return
		}
		next := float64(z.native.facing) + float64(diff)*ai.turn
		if next > 0 {
			z.native.facing = uint16(int(next))
		} else {
			z.native.facing = 0
		}
	case 4:
		ai.speedFactor += dt * zombieBrakeRate
		if ai.speedFactor > 1 {
			ai.speedFactor, ai.state = 1, 1
		}
	case 5:
		if ai.speedFactor > 1 {
			ai.speedFactor -= dt * samuraiLungeBrake
			return
		}
		gun.timer = 0
		ai.speedFactor, ai.state = 1, 1
	}
}

// samuraiAlphaByte reads the draw alpha as the native byte (+0x2b7).
func samuraiAlphaByte(alpha float64) int {
	return int(math.Max(0, math.Min(255, math.Round(alpha*255))))
}

// samuraiAI is vtable slot +0x7c (FUN_000bcfa0), called from the boss step after the state selection and before the
// rise, heading and move of the shared update.
func (p *playState) samuraiAI(z *zombieState, prey playerTarget, dt float64) {
	ai := &z.native.ai
	s := &z.native.samurai
	if p.samuraiScriptRunning() {
		// While a script runs (the intro) every timer is cleared and the gun is emptied; the previous-rounds copy is
		// not refreshed, so after the intro the first blades come from a contact in state 5.
		s.special, ai.speedFactor, s.reload, ai.gun.rounds = 0, 0, 0, 0
		if ai.state != 0 {
			ai.state = 1
		}
		return
	}
	s.time += dt
	if ai.gun.rounds < 1 && s.prevRounds > 0 {
		s.reload = samuraiReloadSeconds
	}
	if s.reload > 0 {
		s.reload -= dt
		if s.reload <= 0 {
			p.samuraiLoadGun(z)
		}
	}
	if s.special <= 0 {
		// The alpha byte climbs back to 0xff in steps of 0x10 (the last step closes the gap).
		b := samuraiAlphaByte(z.alpha)
		switch {
		case b == 0xff:
		case b > 0xef:
			b += 0xff - b
		default:
			b += 0x10
		}
		z.alpha = float64(b) / 255
	} else {
		ai.state = 5
		switch s.phase {
		case 2:
			s.special -= dt * samuraiSpinRate
			ai.speedFactor = s.special + samuraiSpinSpeedAdd
			spin := float64(s.spin) + (s.special+1)*samuraiSpinHeading*dt*samuraiSpinSpinMul
			if spin > 0 {
				s.spin = uint16(int(spin))
			} else {
				s.spin = 0
			}
			weave := int(cosU16(ai.phase) * ai.amplitude * samuraiSpinWeave)
			diff := angleDifference(ai.desired+uint16(weave), z.native.facing)
			z.native.facing += uint16(int(float64(diff) * ai.turn * samuraiSpinTurn))
			p.samuraiSpinSound()
			samuraiSetAnim(z, samuraiAnimSpin)
		case 1:
			ai.speedFactor = 0
			s.special -= dt * samuraiTeleportRate
			if s.special <= 0 {
				s.special = 0
				ai.speedFactor = samuraiTeleportSpeed
				p.samuraiRoarSound()
				p.samuraiTeleport(z, prey)
			}
			b := int(math.Max(0, s.special*samuraiTeleportAlpha))
			z.alpha = float64(b&0xff) / 255
		}
	}
	if ai.state == 5 {
		if s.prevState != 5 {
			p.samuraiRoll(z)
		}
		if z.animation != samuraiAnimAttack && (ai.speedFactor < samuraiAttackAnimSpeed || s.distSq < samuraiAttackAnimDistSq) {
			samuraiSetAnim(z, samuraiAnimAttack)
		}
	} else if s.prevState == 5 {
		if s.phase > samuraiLungeKindMax {
			// The dash chain: aim at the player, rush again at 4.0 and spend one phase step.
			ai.state, ai.speedFactor = 5, samuraiLungeSpeed
			z.native.facing = weapons.NativeWeaponDirection(prey.x-z.x, prey.y-z.y)
			s.phase--
			p.samuraiRoarSound()
		}
	} else {
		samuraiSetAnim(z, samuraiAnimIdle)
	}
	s.prevRounds = ai.gun.rounds
	s.prevState = ai.state
}

// samuraiRoll is the special attack roll on entering state 5: rnd(10), 0..1 plain lunge, 2..4 teleport, 5..7 spin,
// 8..9 the health based dash chain.
func (p *playState) samuraiRoll(z *zombieState) {
	ai := &z.native.ai
	s := &z.native.samurai
	roll := int(zombieBounded(p.rng, samuraiRollRange))
	switch {
	case roll < 2:
		s.phase = 0
	case roll < 5:
		s.special, s.phase = 1, 1
	case roll < 8:
		s.special, s.phase, ai.speedFactor = 1, 2, samuraiSpinSpeed
		s.spin = z.native.facing
		ai.phase = 0
	default:
		frac := 0.0
		if z.native.maxHealth != 0 {
			frac = z.health / z.native.maxHealth
		}
		s.phase = int(float32(samuraiPhaseBase) - float32(frac)*float32(samuraiPhaseHealth))
		p.samuraiRoarSound()
	}
}

// samuraiTeleport re-places the boss 200..300 px from the player along a random heading, repeating until the tile
// line to the player is free (the native loop has no bound; the cap only protects a closed-in level, where the
// boss then stays where it was).
func (p *playState) samuraiTeleport(z *zombieState, prey playerTarget) {
	body := zombieCollisionRadius(*z) * .8
	for try := 0; try < samuraiTeleportTries; try++ {
		heading := uint16(zombieBounded(p.rng, 0xff3a))
		z.native.facing = heading
		dist := samuraiTeleportMinDist + float64(float32(zombieBounded(p.rng, samuraiTeleportBound))/float32(samuraiTeleportDivisor))*samuraiTeleportSpan
		x := prey.x - cosU16(heading)*dist
		y := prey.y - sinU16(heading)*dist
		z.x, z.y = x, y
		if p.lineClear(x, y, prey.x, prey.y, body) {
			return
		}
	}
}

// stepSamurai is FUN_0009fb10 for this class: the always-alerted branch (slot +0x74 is the constant 1), the
// may-attack / state selection, the boss step (AI), the rise, the weave heading (not while in state 5) and the
// move. It returns the displacement before the tile push-out.
func (p *playState) stepSamurai(z *zombieState, prey playerTarget, dx, dy, dt float64) (moveX, moveY float64) {
	ai := &z.native.ai
	s := &z.native.samurai
	if s.dead {
		return 0, 0
	}
	distSq := dx*dx + dy*dy
	if ai.wander < zombieWanderMin {
		ai.wander = float64(zombieRandom(p.rng, .3)) + zombieWanderMin
	}
	may := p.samuraiMayAttack(z, distSq, z.x+dx, z.y+dy, prey.index)
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
		p.samuraiStateMachine(z, dt)
	}
	p.samuraiAI(z, prey, dt)
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
	if ai.state == 0 {
		return 0, 0
	}
	step := z.speed * dt * ai.speedFactor
	return cosU16(z.native.facing) * step, sinU16(z.native.facing) * step
}

// samuraiDrawHeading is FUN_000bce68: while the spin runs the sprite is drawn with the spin heading (+0x34c).
func (z *zombieState) samuraiDrawHeading() uint16 {
	s := &z.native.samurai
	if s.active && s.special > 0 && s.phase == 2 {
		return s.spin
	}
	return z.native.facing
}

// samuraiContact is FUN_000bd528 (slot +0x58). The candidate test is the shared one (reach 0.3 * (width + 64));
// inside it the player takes dt * 0.15 every frame with no depth threshold, and a contact in state 5 reloads the
// gun. The velocity the native adds to the player (+0x1c..+0x24) is rewritten before use and has no visible effect.
func (p *playState) samuraiContact(z *zombieState, prey playerTarget, dt float64) {
	if z.native.ai.state == 0 || z.native.samurai.dead {
		return
	}
	reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*entityGridPlayerBody
	if math.Hypot(prey.x-z.x, prey.y-z.y) >= reach {
		return
	}
	p.damagePlayer(prey.index, dt*samuraiContactDamage)
	if z.native.ai.state == 5 {
		p.samuraiLoadGun(z)
	}
}

// samuraiDeadFadeRate is the survival-mode fade of the dead body per second (FUN_000b48c0 +0x320 decay).
func (p *playState) samuraiDeadFadeRate() float64 {
	if p.waveBuild() == content.WaveBuild125 {
		return samuraiDeadFade125
	}
	return samuraiDeadFadeV7
}

// samuraiDeathSeconds is how long the dying body stays in the zombie list. In story mode the native body is never
// faded or removed (the pop sprite's playing flag that FUN_000a10fc waits for is never cleared, so the exit script
// can pan to it), so it stays for good; in survival FUN_000b48c0 removes it once its fade value (+0x320, 1.0 falling
// at 1/12 per second, 1/6 in 1.2.5) reaches 0.
func (p *playState) samuraiDeathSeconds(z zombieState) float64 {
	if !p.isSurvival() {
		return math.Inf(1)
	}
	return 1 / p.samuraiDeadFadeRate()
}

// samuraiSetAnim is FUN_000f1d48(sprite, id, 0): nothing when that animation is already current, otherwise the
// record is switched and its frame and time are reset.
func samuraiSetAnim(z *zombieState, name string) {
	if z.animation == name {
		return
	}
	z.animation = name
	z.frame = 0
}

// samuraiDeathPose is the draw state of the dying body: the Death clip (no loop, advanced at twice the clock: the
// shared update and the AI both tick the same clip), the last frame stays. The alpha byte is not touched after the
// death frame.
func (p *playState) samuraiDeathPose(z *zombieState) {
	samuraiSetAnim(z, samuraiAnimDeath)
	fps, frames := 3.0, 4.0
	if definition, ok := p.sprites.Find(z.texture); ok {
		if animation, ok := definition.Animation(samuraiAnimDeath); ok && animation.FPS > 0 && animation.Frames > 0 {
			fps, frames = animation.FPS, float64(animation.Frames)
		}
	}
	z.frame = math.Min(z.deathAge*fps*samuraiDeathClipRate, frames-1)
}

// samuraiDrawAlpha is the alpha the body is drawn with. Survival only (FUN_000a06e8 for types 10..15): v7 forces the
// alpha from the fade value even while alive (a = fade < 0.5 ? 2 * fade : 1); 1.2.5 only once the boss is dying.
func (p *playState) samuraiDrawAlpha(z zombieState) (float64, bool) {
	if !(z.native.samurai.active || z.native.robot.active) || !p.isSurvival() {
		return 0, false
	}
	if !(z.native.samurai.dead || z.native.robot.dead) {
		if p.waveBuild() == content.WaveBuild125 {
			return 0, false
		}
		return 1, true
	}
	fade := math.Max(0, 1-z.deathAge*p.samuraiDeadFadeRate())
	a := 1.0
	if fade < .5 {
		a = 2 * fade
	}
	return float64(int(a*255)) / 255, true
}

// zombiesBlockExit reports whether a zombie other than the dead samurai's persistent body is still in the list.
func (p *playState) zombiesBlockExit() bool {
	for _, z := range p.zombies {
		if !(z.native.samurai.active && z.native.samurai.dead) && !(z.native.robot.active && z.native.robot.dead) {
			return true
		}
	}
	return false
}

// samuraiDeath is FUN_000bcaec, run once when the boss starts dying: the sprite switches to Death, and (story mode)
// the wave list is replaced by one 1500 ms wave, every living zombie of types 2..9 takes 1000 damage through its
// normal death, the spawn portals close, the boss time is stored as the best Japan boss time and the boss counter
// goes up. Survival (mode flag != 0) skips everything but the counter.
func (p *playState) samuraiDeath(z *zombieState) {
	s := &z.native.samurai
	if s.deathDone {
		return
	}
	s.deathDone = true
	s.dead = true
	z.native.ai.state = 6
	samuraiSetAnim(z, samuraiAnimDeath)
	z.native.ai.speedFactor = 0
	if p.isSurvival() || p.bossDefeated {
		p.noteBossKilled()
		return
	}
	p.bossDefeated = true
	// FUN_000c0f18: the wave list becomes one wave with a 1500 ms end timer and no spawners; the level's waves end
	// when it runs out (updateSamuraiWaveEnd).
	if p.world != nil {
		p.waveIndex = len(p.world.Level.Waves)
	}
	p.samuraiWaveEndMS, p.samuraiWaveEnding = samuraiDeathWaveEnd, true
	// FUN_0006ee40: entity types 2..9 that are alive take 1000 damage (slot +0x5c), so they die and pop normally.
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
	p.recordJapanBossTime(s.time)
	p.noteBossKilled()
}

// noteBossKilled is the global counter at the tail of FUN_000bcaec (+0x12c on the stats object).
func (p *playState) noteBossKilled() {
	if p.statistics != nil {
		p.statistics.BossesKilled++
		p.statistics.Available["Bosses Killed"] = true
	}
}

// recordJapanBossTime stores the boss time (seconds, truncated) as the best Japan boss time when it is unset (-1)
// or not larger than the stored one.
func (p *playState) recordJapanBossTime(seconds float64) {
	if p.statistics == nil {
		return
	}
	t := float32(seconds)
	best := p.statistics.BestJapanTime
	if best == -1 || !(float32(uint32(best)) < t) {
		p.statistics.BestJapanTime = int32(uint32(t))
		p.statistics.Available["Best Japan Boss Time"] = true
	}
}

// closePortals is FUN_0006f0c0: every open spawn portal's remaining time (+0x1c) is clamped to -600 ms, so each
// one starts closing.
func (p *playState) closePortals() {
	const portalLifetime, portalCloseRemaining = 2.0, -0.6
	for index := range p.portals {
		if portalLifetime-p.portals[index].age > portalCloseRemaining {
			p.portals[index].age = portalLifetime - portalCloseRemaining
		}
	}
}

// updateSamuraiWaveEnd runs the replacement wave's timer (FUN_000bf120 on the one-wave list): while the living
// count is below the build's gate it falls by (int)(t - dt * 1000) per frame and, at <= 0 with no spawners, the
// waves are over.
func (p *playState) updateSamuraiWaveEnd() {
	if !p.samuraiWaveEnding || p.wavesFinished {
		return
	}
	if p.livingZombies() >= p.waveGateLimit() {
		return
	}
	p.samuraiWaveEndMS = int(float32(p.samuraiWaveEndMS) - float32(1.0/60.0)*float32(waveFrameMS))
	if p.samuraiWaveEndMS <= 0 {
		p.wavesFinished, p.samuraiWaveEnding = true, false
		p.noteWave()
	}
}

// samuraiRoarSound is FUN_000ca410(0x34 + rnd(2)): SFX_MUMMY_ROAR_1 / _2. The draw comes before the teleport's
// heading and distance draws.
func (p *playState) samuraiRoarSound() {
	p.sfxQueue = append(p.sfxQueue, fmt.Sprintf("SFX_MUMMY_ROAR_%d", 1+zombieBounded(p.rng, 2)))
}

// samuraiPlayShuriken starts SFX_SHURIKEN (id 0x14, no loop, gain 1.0) and holds its channel for the clip length.
func (p *playState) samuraiPlayShuriken() {
	p.sfxQueue = append(p.sfxQueue, "SFX_SHURIKEN")
	p.samuraiShurikenUntil = p.time + samuraiShurikenSecs
}

// samuraiSpinSound is the spin's per-frame `if !FUN_000ca828(0x14) FUN_000ca410(0x14)`.
func (p *playState) samuraiSpinSound() {
	if p.time >= p.samuraiShurikenUntil {
		p.samuraiPlayShuriken()
	}
}
