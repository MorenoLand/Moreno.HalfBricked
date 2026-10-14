package game

import (
	"fmt"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// The Don Zombie's car (boss_gangster, native zombie type 11). Recovered from libmortargame.so v7
// (Research/native/gangster-car-2026-10-11.md): the class does not chase. Its vtable overrides
//
//   - FUN_000b732c (vtable +0x78, called from the post-init FUN_000b4864): x = -192, direction +0x32c = +1,
//     height +0x2c = width +0x28 * 0.65 (0.6 again every frame by the draw registration FUN_000b696c), sight
//     +0x274 = 73000 (squared px), then FUN_000b70b4 rolls a gun.
//   - FUN_000b6b90 (movement): x += dir * speed * dt * (0.33 + 0.66 * speedFactor); y never moves.
//   - FUN_000b7264 (bounds, vtable +0x7c): x < -192 -> x = -192, dir = +1; x > (levelWidth + 6) * 32 -> that x,
//     dir = -1; each turn rolls a new gun (FUN_000b70b4).
//   - FUN_000b696c (draw registration): contact when distance^2 < (0.3 * width + 0.3 * 64)^2: the player takes
//     3 * dt * 0.05 damage and the car's gun is emptied (+0x2f4 = 0).
//   - FUN_000b6e00 (may-shoot): the squared distance is within 73000, 64 <= x <= (levelWidth - 2) * 32 and the
//     line to the player is clear (or the state is already 2/4).
//
// The vertical knock-back the contact adds to the player (+0x20 += +-(0.2 + 0.5 * speedFactor)) is a native no-op:
// the player update FUN_00096818 rewrites +0x1c/+0x20/+0x24 from the controls (lines 276-291) before the movement
// reads them (lines 414-416), so it is deliberately not applied (G20). SFX_CAR_GUN (0x30) has no caller in the
// binary (sound-trigger-audit 2.x), so the car plays no gun sound.
//
// UNRESOLVED (not implemented): the tile collision FUN_00091918 of the movement, the boss meter hook FUN_000b48c0 beyond the
// health fraction the HUD already reads, and the bazooka / grenade shots the gun roll can pick (kinds 0x13 / 0x14
// are not ported, so those rolls leave the car without a gun).
const (
	zombieKindGangster  = 11
	zombieKindRobot     = 14  // boss_robot
	robotRenderAnchor   = .75 // FUN_000bb0e0: the robot boss sprite is lifted 0.75 * its size (plain zombies .35)
	carStartX           = -192.0
	carSightSq          = 73000.0
	carSpeedBase        = .33
	carSpeedSpan        = .66
	carHeightFactor     = .6
	carContactFactor    = .3
	carContactDamage    = .05
	carContactCalls     = 3
	carRightMarginTiles = 6
	carShootMinX        = 64.0
	carShootMarginTiles = 2
	carRateBase         = 1.0
	carRateHealthSpan   = .75
	carRateTimerLead    = .5
	carPistolRounds     = 20
	carTileSize         = 32.0
	carGunRollRange     = 14
	carRenderAnchor     = .2 // FUN_000b732c: +0x2d4
)

// gangsterCar is the per-zombie state of the car.
type gangsterCar struct {
	active  bool
	dir     float64
	wrecked bool // the wreck sprite has replaced the car (leaveCarWreck)
}

// leaveCarWreck replaces the dying car with its "Death" sprite (gangstercar_death: one 256x128 frame, the wrecked
// car), which stays on the street: world1_level2_exit pans to the boss's position and waits four seconds on it
// before the level-complete text. The car used to vanish the moment it died.
func (p *playState) leaveCarWreck(z *zombieState) {
	z.native.car.wrecked = true
	// FUN_000b6c58 (the car's death): a mine-class blast at the car (hit-player 0, hit-entities 1), a camera shake,
	// the wreck animation and SFX_CAR_DEATH (0x33). The shake arguments (1.5, 1.0, 1.0) are read as duration and
	// amplitudes; that mapping is inferred.
	p.sfxQueue = append(p.sfxQueue, "SFX_CAR_DEATH")
	p.spawnBlast(z.x, z.y, zombieBlastSound, killOrigin{}, true)
	// v7 FUN_000b6c58 calls FUN_000be1fc(pos, DAT_000b6dec = 3.0, 1.5, 1.0, 1.0): with FUN_000be1fc's signature (pos,
	// duration, ampX, ampY, angleMul) that is a 3.0 s shake, ampX 1.5, ampY 1.0, angle multiplier 1.0 (the earlier
	// "1.5 / 1.0 / 1.0" reading dropped the first literal). The 1.2.5 car death was not decoded: it keeps the old values.
	if p.waveBuild() == content.WaveBuildV7 {
		p.startCameraShake(z.x, z.y, 3.0, 1.5, 1, cameraShakeAngleMul(p.waveBuild()))
	} else {
		p.startCameraShake(z.x, z.y, 1.5, 1, 1, cameraShakeAngleMul(p.waveBuild()))
	}
	definition, ok := p.sprites.Find(z.texture)
	if !ok {
		return
	}
	index := -1
	for i, name := range definition.AnimationOrder {
		if name == "death" {
			index = i
		}
	}
	if index < 0 {
		return
	}
	if p.scriptEntities == nil {
		p.scriptEntities = map[int]*scriptEntity{}
	}
	if p.scriptNextEntity <= 0 {
		p.scriptNextEntity = 1
	}
	id := p.scriptNextEntity
	p.scriptNextEntity++
	// The wreck art is twice as wide as one car frame at the same pixel scale.
	scale := z.size.X / 128
	p.scriptEntities[id] = &scriptEntity{id: id, kind: "sprite", entityType: "car_wreck", texture: z.texture, animation: index,
		x: z.x, y: z.y, scaleX: 256 * scale, scaleY: 128 * scale, sized: true, alpha: 1, flipX: z.flipX}
}

func isGangsterCarName(name string) bool { return name == "boss_gangster" }

// newGangsterCar builds the car for a boss_gangster spawn entry.
func (p *playState) newGangsterCar(entry formats.SpawnType, point formats.Vec2, speed float64, texture string, scriptID int) zombieState {
	record := rollZombieSpawnRecord(entry, p.rng)
	half := float64(record.HalfSize)
	if record.HalfSize <= 0 {
		half = 24
	}
	sizeZ := half * 2
	health, rawPoints := 100.0, 0
	if entry.Strength >= 0 {
		health, rawPoints = entry.Strength, int(entry.Strength)
	}
	width := zombieBodyWidth(sizeZ)
	z := zombieState{
		x: carStartX, y: point.Y, speed: speed, health: health, rawPoints: rawPoints,
		size:    formats.Vec2{X: width, Y: width * carHeightFactor},
		texture: texture, scriptID: scriptID, alpha: 1,
	}
	z.collision = zombieBodyScale * width
	ai := zombieAI{riseMS: -1500, turn: .01, alertRadius: float64(record.AlertRadius), sightBase: carSightSq, sightRamp: 1}
	if record.TurnSpeed >= 0 {
		ai.turn = float64(float32(record.TurnSpeed) / 120)
	}
	z.native = zombieNative{kind: zombieKindGangster, maxHealth: health, sizeZ: sizeZ, brightness: 1, ai: ai}
	z.native.car = gangsterCar{active: true, dir: 1}
	p.rollCarGun(&z)
	return z
}

// rollCarGun is FUN_000b70b4: a random gun id, its record with half the rounds and a rate that follows the car's
// health fraction.
func (p *playState) rollCarGun(z *zombieState) {
	roll := zombieBounded(p.rng, carGunRollRange)
	var id int
	switch {
	case roll < 2:
		id = 2
	case roll < 5:
		id = 3
	case roll < 7:
		id = 0
	case roll == 7:
		id = 5
	case roll < 10:
		id = 6
	default:
		id = 13
	}
	gun := newZombieGun(zombieKindGangster, id, p.zombieWeapons)
	gun.rounds /= 4 // newZombieGun doubles the record's ammo; the car keeps half of it
	if id == 0 && gun.kind == 0x10 {
		gun.rounds = carPistolRounds
	}
	frac := 1.0
	if z.native.maxHealth > 0 {
		frac = math.Max(0, z.health) / z.native.maxHealth
	}
	gun.interval *= carRateBase + frac*carRateHealthSpan
	gun.timer = gun.interval - carRateTimerLead
	z.native.ai.gun = gun
	// FUN_000b70b4 ends with FUN_000ca410(0x31 + rnd(2)): SFX_CAR_TIRE_SCREECH_1 / _2 at the spawn and at every turn.
	p.sfxQueue = append(p.sfxQueue, fmt.Sprintf("SFX_CAR_TIRE_SCREECH_%d", 1+zombieBounded(p.rng, 2)))
}

// carMayShoot is FUN_000b6e00.
func (p *playState) carMayShoot(z *zombieState, distSq, targetX, targetY float64) bool {
	ai := &z.native.ai
	if distSq <= 1 || ai.state == 2 || ai.state == 4 {
		return true
	}
	levelW := float64(p.world.Level.Width)
	if ai.gun.rounds < 1 || distSq > carSightSq || z.x < carShootMinX || z.x > (levelW-carShootMarginTiles)*carTileSize {
		return false
	}
	return p.lineClear(z.x, z.y, targetX, targetY, 1)
}

// stepGangsterCar runs one frame of the car: the shared zombie state machine (braking, gun timer) and the lane
// drive with its turn-arounds. It does not chase the player.
func (p *playState) stepGangsterCar(z *zombieState, dx, dy, dt float64, drive bool) {
	ai := &z.native.ai
	p.stepZombieAI(z, dx, dy, dx, dy, dt)
	if ai.state != 0 {
		z.size.X = zombieBodyWidth(z.native.sizeZ)
	}
	z.size.Y = z.size.X * carHeightFactor
	if !drive || ai.state == 0 {
		return
	}
	car := &z.native.car
	z.x += car.dir * z.speed * dt * (carSpeedBase + ai.speedFactor*carSpeedSpan)
	right := (float64(p.world.Level.Width) + carRightMarginTiles) * carTileSize
	switch {
	case z.x < carStartX:
		z.x, car.dir = carStartX, 1
		p.rollCarGun(z)
	case z.x > right:
		z.x, car.dir = right, -1
		p.rollCarGun(z)
	}
}

// carContact is the contact test of FUN_000b696c.
func (p *playState) carContact(z *zombieState, prey playerTarget, dt float64) {
	if p.health <= 0 || z.native.ai.state == 0 {
		return
	}
	reach := carContactFactor*z.size.X + carContactFactor*entityGridPlayerBody
	dx, dy := prey.x-z.x, prey.y-z.y
	if dx*dx+dy*dy < reach*reach {
		p.damagePlayerAt(prey.index, carContactCalls*dt*carContactDamage, z.x, z.y)
		z.native.ai.gun.rounds = 0
	}
}
