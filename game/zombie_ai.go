package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// zombieAI is the per-zombie behaviour state of the native update FUN_0009fb10
// (field names give the native offsets).
type zombieAI struct {
	state       int     // +0x4c: 0 rising, 1 walking, 2..5 shooting / charging, 6 dead
	speedFactor float64 // +0x48
	riseMS      float64 // +0x2c0 while rising: starts at -1500 (DAT_000a0ec0), spawn done at 500
	desired     uint16  // +0x298 desired heading
	phase       uint16  // +0x29c weave phase
	cycle       uint16  // +0x29e weave rate (deviateCycleSpeed * 182 units per second)
	amplitude   float64 // +0x2a0 weave amplitude in heading units (signed)
	turn        float64 // +0x2a4 fraction of the heading error turned per frame
	wander      float64 // +0x278 retarget timer
	sightBase   float64 // +0x26c squared sight radius
	sightRamp   float64 // +0x270 0..1
	alertRadius float64 // +0x2d8
	gun         zombieGun
}

// zombieRandom is FUN_000a24dc(rng, extent).
func zombieRandom(rng *weapons.NativeRNG, extent float32) float32 {
	if rng == nil {
		return 0
	}
	return weapons.NativeWeaponRandomFloat(rng, extent)
}

func zombieBounded(rng *weapons.NativeRNG, bound uint32) uint32 {
	if rng == nil {
		return 0
	}
	return rng.Bounded(bound)
}

// newWaveZombie is FUN_000a0b80 applied to a rolled spawn record, including the
// random numbers the initialiser draws (weave sign, weave phase, sight radius,
// retarget timer and brightness, in that order).
func newWaveZombie(record zombieSpawnRecord, rng *weapons.NativeRNG, catalog formats.ZombieWeaponCatalog, point formats.Vec2, speed float64, texture string, scriptID int) zombieState {
	health, rawPoints := 100.0, 0
	if record.Strength >= 0 {
		health, rawPoints = float64(record.Strength), record.Strength
	}
	half := float64(record.HalfSize)
	if record.HalfSize <= 0 {
		half = 24
	}
	sizeZ := half * 2
	ai := zombieAI{
		riseMS:    -1500,
		turn:      .01,
		amplitude: float64(float32(uint16(record.Amount * 182))),
		cycle:     uint16(record.CycleSpeed * 182),
	}
	if record.TurnSpeed >= 0 {
		ai.turn = float64(float32(record.TurnSpeed) / 120)
	}
	if zombieBounded(rng, 2) == 0 {
		ai.amplitude = -ai.amplitude
	}
	ai.phase = uint16(zombieBounded(rng, 0xff3a))
	sight := float64(zombieRandom(rng, 50)) + 110
	if record.Kind == zombieKindCharging || record.Kind == zombieKindArmed {
		sight *= 1.5
	}
	ai.sightBase = float64(float32(sight) * float32(sight))
	ai.wander = float64(zombieRandom(rng, .3)) + .2
	brightness := float64(zombieRandom(rng, .199)) + .8
	ai.alertRadius = float64(record.AlertRadius)
	ai.gun = newZombieGun(record.Kind, record.Gun, catalog)
	z := zombieState{
		x: point.X, y: point.Y, speed: speed, health: health, rawPoints: rawPoints,
		size:    formats.Vec2{X: zombieBodyWidth(sizeZ), Y: sizeZ},
		texture: texture, scriptID: scriptID, alpha: 1,
	}
	z.collision = zombieBodyScale * z.size.X
	z.native = zombieNative{kind: record.Kind, maxHealth: health, sizeZ: sizeZ, brightness: brightness, ai: ai}
	return z
}

// Native zombie behaviour (FUN_0009fb10, movement FUN_000a18e4, push-out FUN_000a17cc
// -> FUN_000be3c0, separation FUN_000a1b08). Constants are the literal-pool values
// read from the v7 binary; every formula is per 60 Hz frame like the original.
const (
	zombieSpeedRamp       = 4.0   // DAT_0009fee4: +0x48 grows by 4 per second
	zombieSightRampRate   = .25   // DAT_0009ff00: +0x270 reaches 1 after 4 s
	zombieRiseStartMS     = -1500 // DAT_000a0ec0
	zombieRiseDoneMS      = 500   // 0x1f4
	zombieRiseSquash      = .25   // DAT_0009ff00 inside the rise
	zombieWanderMin       = .2    // DAT_0009fec8 / DAT_0009feb0
	zombieWanderReload    = .5    // DAT_0009fee0, + rnd(.2)
	zombieWanderMiss      = -1.7  // DAT_0009fed8: one retarget in three goes the wrong way
	zombieWanderBase      = .3    // DAT_0009fec0
	zombieAimMinDistSq    = 1.0   // DAT_0009f618
	zombieNoiseShot       = 70.0  // player +0x3e0 += 70 per shot (FUN_000947e0, DAT_00094a74)
	zombieNoiseMax        = 350.0 // DAT_00094a7c
	zombieNoiseRegen      = .5    // FUN_000962f4: +0x3e0 regrows to 1 at 0.5/s
	zombiePushScale       = 10.0  // DAT_000a2090
	zombieSeparationSize  = 1.25  // DAT_000a2074
	zombieContactDamage   = .075  // DAT_0009f79c: damage per second while deeply overlapping
	zombieContactDepth    = .075  // DAT_0009f79c * player body 64 = 4.8 px of overlap
	zombieBodyReachFactor = .3    // DAT_000a2070
	tileCollisionYSquash  = .666  // DAT_000be820
)

// addFireNoise is FUN_000947e0: every primary shot raises the player's noise
// factor (+0x3e0) by 70 up to 350. Zombies consider the player alerted while
// distance < noise * alertRadius, so one shot alerts the whole map.
func (p *playState) addFireNoise() {
	p.alertNoise = math.Min(zombieNoiseMax, p.alertNoise+zombieNoiseShot)
}

// updateAlertNoise is FUN_000962f4: a living player's noise regrows to 1; dying resets it.
func (p *playState) updateAlertNoise(dt float64) {
	if p.health <= 0 {
		p.alertNoise = 0
		return
	}
	if p.alertNoise < 1 {
		p.alertNoise = math.Min(1, p.alertNoise+dt*zombieNoiseRegen)
	}
}

// zombieAlerted is vtable slot +0x74 (FUN_0009f118): distSq < (noise * alertRadius)^2.
func zombieAlerted(distSq, noise, alertRadius float64) bool {
	reach := float64(float32(noise) * float32(alertRadius))
	return distSq < reach*reach
}

// zombieHeadingStep is the end of FUN_0009fb10: the heading turns toward the
// desired heading plus the weave offset cos(phase) * amplitude * speedFactor by
// turn * (angle error) per frame.
func zombieHeadingStep(ai *zombieAI, heading uint16, scripted bool) uint16 {
	weave := int(cosU16(ai.phase) * ai.amplitude * ai.speedFactor)
	if scripted {
		weave = int(cosU16(ai.phase) * ai.amplitude * ai.speedFactor * .1)
	}
	err := float64(angleDifference(ai.desired+uint16(weave), heading))
	turn := ai.turn
	if scripted {
		turn += .1
	}
	return heading + uint16(int(err*turn))
}

func cosU16(angle uint16) float64 { return math.Cos(2 * math.Pi * float64(angle>>4) / 4096) }
func sinU16(angle uint16) float64 { return math.Sin(2 * math.Pi * float64(angle>>4) / 4096) }

// zombieRetarget is the idle branch of FUN_0009fb10: when the timer runs out the
// desired heading moves 30..50% (one time in three -1.7 times that, i.e. away)
// toward the bearing of the target; the timer reloads to 0.5 + rnd(.2).
func zombieRetarget(ai *zombieAI, bearing uint16, rng *weapons.NativeRNG) {
	diff := float64(angleDifference(bearing, ai.desired))
	factor := 1.0
	if zombieBounded(rng, 3) == 0 {
		factor = zombieWanderMiss
	}
	r := float64(zombieRandom(rng, .2))
	ai.desired += uint16(int(diff * factor * (r + zombieWanderBase)))
	ai.wander = float64(zombieRandom(rng, .2)) + zombieWanderReload
}

// zombieRiseDims is the squash of the spawn rise: height (2-f)*sizeZ, width
// sizeZ*(0.25+0.75f)*0.796875 and alpha 1+253f with f = riseMS/500 clamped to 0..1.
func zombieRiseDims(sizeZ, riseMS float64) (w, h float64, alpha float64) {
	f := math.Max(0, math.Min(1, riseMS/zombieRiseDoneMS))
	h = sizeZ * (2 - f)
	w = sizeZ * (zombieRiseSquash + f*.75) * zombieBodyWidthFactor
	return w, h, (1 + f*253) / 255
}

// stepZombieAI runs one frame of FUN_0009fb10 for a wave zombie that is not driven
// by a script walk and returns the displacement before the push-out. bearing is
// the direction to the target (player), distSq its squared distance.
func (p *playState) stepZombieAI(z *zombieState, dx, dy, aimX, aimY, dt float64) (moveX, moveY float64) {
	ai := &z.native.ai
	distSq := dx*dx + dy*dy
	if ai.sightRamp < 1 && z.native.kind <= zombieKindProspect {
		ai.sightRamp = math.Min(1, ai.sightRamp+dt*zombieSightRampRate)
	}
	bearing := weapons.NativeWeaponDirection(aimX, aimY)
	alerted := zombieAlerted(distSq, p.alertNoise, ai.alertRadius)
	if alerted {
		// FUN_0009f54c: the aim follows the target exactly while alerted.
		if ai.wander < zombieWanderMin {
			ai.wander = float64(zombieRandom(p.rng, .3)) + zombieWanderMin
		}
		if distSq > zombieAimMinDistSq {
			ai.desired = bearing
		}
		if ai.state != 0 {
			if p.zombieMayShoot(z, distSq, z.x+dx, z.y+dy) {
				if ai.state == 1 {
					ai.state = 2
				}
			} else if ai.state == 3 {
				ai.state = 4
				if ai.gun.timer > -.25 {
					ai.gun.timer -= dt
				}
			}
		}
	} else if ai.wander > 0 {
		ai.wander -= dt
		if ai.wander <= 0 {
			zombieRetarget(ai, bearing, p.rng)
		}
	}
	if ai.state > 1 && alerted {
		p.zombieStateMachine(z, dt)
	} else if ai.state != 5 {
		ai.speedFactor = math.Min(1, ai.speedFactor+dt*zombieSpeedRamp)
		if ai.state > 1 {
			ai.state = 1
		}
	}
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
	z.native.facing = zombieHeadingStep(ai, z.native.facing, false)
	if ai.state == 0 {
		return 0, 0
	}
	step := z.speed * dt * ai.speedFactor
	return cosU16(z.native.facing) * step, sinU16(z.native.facing) * step
}

// separateZombie is the entity pass of FUN_000a1b08 for one zombie that is not
// rising: for every other zombie closer than 0.3 * (width + other width) that is
// not much smaller (self width < 1.25 * other width) the body is pushed away by
// overlap * dt * 10 along the line between them, so big zombies shove small ones.
func (p *playState) separateZombie(index int, dt float64) {
	z := &p.zombies[index]
	if z.native.kind != 0 && z.native.ai.state == 0 {
		return
	}
	var pushX, pushY float64
	for other := range p.zombies {
		o := &p.zombies[other]
		if other == index || o.spawnAway || (o.native.kind != 0 && o.native.ai.state == 0) {
			continue
		}
		reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*o.size.X
		dx, dy := o.x-z.x, o.y-z.y
		dist := math.Hypot(dx, dy)
		if dist >= reach || z.size.X >= o.size.X*zombieSeparationSize {
			continue
		}
		if dist < .001 {
			angle := uint16(zombieBounded(p.rng, 0xff3a))
			dx, dy, dist = cosU16(angle), sinU16(angle), 1
		}
		overlap := (reach - dist) * dt * zombiePushScale
		pushX -= dx / dist * overlap
		pushY -= dy / dist * overlap
	}
	if pushX != 0 || pushY != 0 {
		z.x += pushX
		z.y += pushY
		ox, oy := p.nativeTilePush(z.x, z.y, zombieBodyScale*z.size.X)
		z.x, z.y = z.x+ox, z.y+oy
	}
}

// zombieContact is FUN_0009f6e8: against the player the contact reach is
// 0.3 * (zombie width + player width 64); only an overlap deeper than 0.075 * 64
// hurts, at 0.075 health per second (dt * 0.075 per frame).
func (p *playState) zombieContact(z *zombieState, prey playerTarget, dt float64) {
	reach := zombieBodyReachFactor*z.size.X + zombieBodyReachFactor*entityGridPlayerBody
	overlap := reach - math.Hypot(prey.x-z.x, prey.y-z.y)
	if overlap > zombieContactDepth*entityGridPlayerBody {
		p.damagePlayer(prey.index, dt*zombieContactDamage)
	}
}
