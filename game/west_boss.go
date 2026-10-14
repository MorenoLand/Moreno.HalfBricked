package game

import (
	"image"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/MorenoLand/Moreno.HalfBricked/game/nativemath"
	"github.com/hajimehoshi/ebiten/v2"
)

// Mad Dog (boss_west, native zombie type 15, the DLC1 western boss). Recovered from libmortargame.so v7 and
// cross-checked in 1.2.5 (completion-gap-audit-2026-10-14.md G03): AddWesternBossZombie FUN_000dc83c builds the
// spawn record and the class FUN_000b7f08 (size 0x37c, vtable 0x005bbf78) takes it through FUN_000a0b80. Slots: post-init
// FUN_000b82d4 (+0x78), AI FUN_000b8720 (+0x7c, from the boss step FUN_000b7fc4 / FUN_000b48c0), may-attack FUN_000b7b24
// (+0x48), fire hook FUN_000b88a8 (+0x64), damage FUN_000b848c (+0x5c), death FUN_000b7e1c (+0x50), update FUN_000b7a5c
// (+0x18), draw FUN_000b8d08 (+0x40) with the muzzle flash FUN_000b8b18, helpers FUN_000b83dc (mode change),
// FUN_000b8014 (gun per mode), FUN_000b8624 (the rage fade), FUN_000b825c (spin-up), FUN_000b83a4 (mode timer),
// FUN_000b82ac (next health threshold). 1.2.5: post-init 0x0011a850, AI 0x0011acd8, hook 0x0011ae60, damage 0x0011aa64,
// death 0x00119ec8, may-attack 0x0011a0e0, mode change 0x0011a9b4, gun 0x0011a580, fade 0x0011abdc, spin 0x0011a7dc.
//
// Class behaviour:
//   - Spawn record (FUN_000dc83c): position, speed 0, half size 60, strength 40000, turn 100, cycle 250, weave 0, alert
//     250, no gun. v7 keeps those; the 1.2.5 post-init hard-codes health 40000, speed 92.5, size 140 x 140 and turn 0.1
//     (the numbers of the level XML's boss_west). The post-init also registers the boss as the scene's one boss in story
//     mode: a second one is removed at once.
//   - Always alerted (slot +0x74), no contact damage (+0x58 is a bare return), never any rise (the boss only exists
//     after the intro). The move is the plain zombie move; the may-attack slot returns true only inside the range
//     (+0x274 = 90000, set by the gun loader) when the body is on screen (empty route) or a route has rounds and a clear
//     trace; modes 2 and 3 always return true.
//   - Modes (+0x35c): 0 PISTOL (6 rounds, 0.5 s, speed 500), 1 DYNOMITE (3 grenades, 0.4 s, speed 350), 2 the rage
//     (NONE record, no rounds), 3 the grenade spin (45 rounds, 0.2 s, speed 350, the aim spins by +0x378 per frame once
//     the spin has ramped to 3500 at 1500 per second), 4 UZI (100 rounds, speed 400). When the rounds run out the mode
//     changes (0 <-> 1, 3 and 4 back to 0); mode 2 hands over to 3 or 4 by the rage counter (+0x360). The health
//     thresholds start at health - 22% and fall by 22% of the start health: a hit below one in mode 0 starts the rage
//     (mode 2: invulnerable, sound 0x3e, the body backs away at speed factor -1 while its alpha falls 0x10 a frame, then it
//     jumps to (650, 570), stays hidden 3 s, fades in 0x10 a frame (sound 0x3f), and after 0.5 s the next mode starts).
//     Once the thresholds are used up a 15 s timer changes the mode as well.
//   - Fire hook: a kind 0x10 bullet starts at the arm's muzzle (tables A / B by the arm flag and the arms sprite's
//     compass direction), aims at the player (mode 3: its own heading), height 10, life and speed of the gun, hits the
//     player AND other zombies, texture "maddog_bullet" 10 x 20; sound 0x3b (0x3c in mode 4); the muzzle flash timer
//     starts at 160 ms. A kind 0x1d bullet is a grenade from the boss position: height 25, fall -150, speed boost -10,
//     two bounces, size 40, life doubled, sound 0x3d (the init runs twice), and on detonation a kind 0x1a shockwave
//     (200 px, 0.4 s, owner the boss).
//   - Damage slot: none while the invulnerable flag is set or the player is dead (1.2.5: while every player is dead),
//     none from an exploding zombie's blast, a kind 0x10 bullet sprays two hit particles; reaching a health threshold in
//     mode 0 starts the rage; death at 0 health.
//   - Death hook FUN_000b7e1c: state 6, Death clip (4 frames at 5 fps), then FinalWords; the wave list is replaced by one
//     1500 ms wave, zombies of types 2..9 take 1000 damage and the portals close (story mode), the boss time is stored as
//     the best "bonus" time (stats +0xf8, 1.2.5's "Best Western Boss Time") and the boss counter goes up. SetWesternBossDead
//     (FUN_000b7fe4) switches the body to its Dead clip.
const (
	zombieKindWest = 15

	westScriptHalfSize = 60    // FUN_000dc83c record half size
	westScriptTurn     = 100   // record turn speed
	westScriptCycle    = 0xfa  // record cycle speed
	westScriptAlert    = 0xfa  // record alert radius
	westHealth         = 40000 // record strength (v7), hard-coded by the 1.2.5 post-init
	westSpeed125       = 92.5  // DAT_0011a970
	westSize125        = 140.0 // DAT_0011a974
	westTurn125        = 0.1   // DAT_0011a978
	westThresholdStep  = .22   // DAT_000b82d0
	westRangeSq        = 90000.0
	westModeSeconds    = 15.0 // DAT_000b8488
	westSpinCap        = 3500 // DAT_000b82a4
	westSpinRate       = 1500 // DAT_000b82a8
	westRageHide       = 3.0  // DAT_000b8718
	westRageWake       = .5   // 0x3f000000
	westRageX          = 650.0
	westRageY          = 570.0
	westAlphaStep      = 0x10
	westPathReload     = 100
	westPathDropSq     = 250000.0 // DAT_000b7e04
	westWaypointSq     = .16      // DAT_000b7e0c
	westTraceStep      = 32.0     // DAT_000bfcac
	westTraceRadius    = .25      // DAT_000b7dfc: the trace radius is 0.25 * width

	westPistolSpeed = 500.0 // DAT_000b823c
	westPistolGap   = .5
	westPistolShots = 6
	westBombSpeed   = 350.0 // DAT_000b8244
	westBombGap     = .4    // DAT_000b8248
	westBombShots   = 3
	westSpinGap     = .2 // DAT_000b8250
	westSpinShots   = 45
	westUziSpeed    = 400.0 // DAT_000b8258
	westUziShots    = 100

	westBulletLift   = 10.0  // DAT_000b8b00
	westBulletHeight = -20.0 // DAT_000b8b04
	westBulletWidth  = 10.0
	westBombLift     = 25.0   // DAT_000b8b10
	westBombFall     = -150.0 // DAT_000b8b0c
	westBombBoost    = -10.0  // DAT_000b8b14
	westBombBounces  = 2
	westFlashMillis  = 160.0 // DAT_000b8b08
	westFlashSplit   = 80.0
	westFlashWidth   = 45.0
	westFlashHeight  = 30.0
	westFlashDX      = 15.0
	westFlashDY      = 10.0
	westBlastLife    = .4  // DAT_000a410c
	westBlastSize    = 200.0
	westUnitsPerDeg  = 182.0

	westModePistol = 0
	westModeBomb   = 1
	westModeRage   = 2
	westModeSpin   = 3
	westModeUzi    = 4

	westGunPistol   = 0
	westGunUzi      = 2
	westGunDynomite = 12
	westGunNone     = 13

	westSoundFire    = "SFX_MD_FIRE"         // 0x3b
	westSoundSpecial = "SFX_MD_FIRE_SPECIAL" // 0x3c
	westSoundHaha    = "SFX_MADDOG_HAHA"     // 0x3e
	westSoundDraw    = "SFX_MADDOG_DRAW"     // 0x3f
	westBulletSprite = "DLC1/Textures/maddog_bullet_SD"
	westFlashSprite  = "Common0/Textures/muzzleflash_01_SD"
	westBodySprite   = "Maddog"
	westArmsSprite   = "MaddogArms"

	westAnimIdle       = "Idle"
	westAnimDeath      = "Death"
	westAnimDead       = "Dead"
	westAnimFinalWords = "FinalWords"
	westAnimShoot      = "Shoot"
)

// westMuzzle are the arm's muzzle offsets per compass direction (v7 0x005db00c / 0x005db06c, 1.2.5 0x008f52ec / 0x008f5364,
// written by _INIT_58 / _INIT_73): table A while the arm flag (+0x344) is set, B while it is clear.
var westMuzzleA = [8][3]float64{{-24, 25, 0}, {-2, -2, 0}, {33, -15, 0}, {38, -66, 0}, {float64(float32(26.7)), -61.5, 0}, {-40, -66, 0}, {-35, -15, 0}, {2, -6, 0}}
var westMuzzleB = [8][3]float64{{24, 22, 0}, {38, -12, 0}, {33, -27, 0}, {.5, float64(float32(-49.8)), 0}, {float64(float32(-21.6)), -61.5, 0}, {3.5, float64(float32(-49.8)), 0}, {-35, -27, 0}, {float64(float32(-42.8)), -14, 0}}

// westBoss is the per-zombie state of the class (field names give the native offsets).
type westBoss struct {
	active     bool
	registered bool    // this boss is the scene's registered boss (story mode)
	mode       int     // +0x35c
	phase      int     // +0x360 rage counter
	maxHealth  int     // +0x364
	threshold  int     // +0x368 health below which the next rage starts (<= 0: used up)
	modeTimer  float64 // +0x36c
	rage       int     // +0x370 rage sub-phase
	rageTimer  float64 // +0x374
	spin       int     // +0x378
	invuln     bool    // +0x2e1
	flash      float64 // +0x330 muzzle flash timer in ms
	flashX     float64 // +0x334
	flashY     float64 // +0x338
	flashAngle uint16  // +0x340
	armFlag    bool    // +0x343 (written by the hook, copied to +0x344 by the AI)
	armPrev    bool    // +0x344
	armDir     int     // the arms sprite's compass direction (+0x24 of the sprite)
	armsAnim   string  // the arms sprite's clip (Idle / Shoot)
	armsFrame  int
	rangeSq    float64 // +0x274
	pathTimer  int     // +0x354
	pathActive bool    // +0x34c list is not empty
	distSq     float64 // +0x358
	dead       bool    // +0x314
	deathDone  bool
	scored     bool
	animClock  float64 // seconds into the current body clip while dead
	lastHealth int     // the health the damage slot last saw (the threshold test runs when it changed)
}

func isWestName(name string) bool { return name == "boss_west" }

// westRecord is the 11-word record of FUN_000dc83c.
func westRecord() zombieSpawnRecord {
	return zombieSpawnRecord{Kind: zombieKindWest, HalfSize: westScriptHalfSize, Speed: 0, Strength: westHealth, TurnSpeed: westScriptTurn,
		CycleSpeed: westScriptCycle, Amount: 0, AlertRadius: westScriptAlert, Gun: westGunNone}
}

// westBossRegistered reports whether a living registered boss (the post-init's scene slot +0x51464, also written by the
// mummy) exists.
func (p *playState) westBossRegistered(except *zombieState) bool {
	for index := range p.zombies {
		z := &p.zombies[index]
		if z == except || z.spawnAway || z.dying || z.health <= 0 {
			continue
		}
		if z.native.west.registered || z.native.egypt.active && !z.native.egypt.dead {
			return true
		}
	}
	return false
}

// initWest is the post-init FUN_000b82d4 (1.2.5 0x0011a850). It returns false when a boss is already registered: the new
// one is flagged for removal.
func (p *playState) initWest(z *zombieState) bool {
	ai := &z.native.ai
	w := &z.native.west
	*w = westBoss{active: true, phase: 1, armFlag: true, armPrev: true, armsAnim: westAnimIdle}
	if !p.isSurvival() && p.westBossRegistered(z) {
		z.spawnAway, z.health = true, 0
		return false
	}
	if p.waveBuild() != content.WaveBuildV7 {
		// 1.2.5 hard-codes the level XML's numbers: 40000 health, speed 92.5, a 140 x 140 body, turn 0.1.
		z.health, z.native.maxHealth, z.rawPoints = westHealth, westHealth, westHealth
		z.speed = westSpeed125
		z.native.sizeZ = westSize125
		z.size = formats.Vec2{X: westSize125, Y: westSize125}
		ai.turn = westTurn125
	}
	z.alpha = 1
	z.animation = westAnimIdle
	z.fps = p.spriteFPS(z.texture, westAnimIdle)
	w.maxHealth = int(z.health)
	w.threshold = w.maxHealth
	w.threshold = p.westNextThreshold(w)
	w.lastHealth = int(z.health)
	w.registered = !p.isSurvival()
	return true
}

// zombieShotHitsZombie is the entity pass of a bullet with the hit-entities flag (FUN_000a5f74 -> FUN_000a521c): the
// first zombie other than the owner inside the ellipse takes the bullet's 101 damage and the bullet ends.
func (p *playState) zombieShotHitsZombie(shot zombieShot) bool {
	pr := shot.projectile
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.health <= 0 || z.dying || z.spawnAway || (shot.ownerID != 0 && z.scriptID == shot.ownerID) {
			continue
		}
		if !zombieBlastEllipse(z.x-pr.X, z.y-pr.Y, pr.Width+zombieBodyScale*z.size.X) {
			continue
		}
		if p.zombieGridVisits(z, pr.X, pr.Y, float32(pr.Width)*zombieBlastQueryScale) == 0 {
			continue
		}
		if z.invulnerable {
			return true
		}
		hit := zombieHit{damage: zombieBulletDamage, kind: 0x10, direction: pr.Direction, located: true, x: pr.X, y: pr.Y, lift: pr.Lift}
		if p.hurtZombie(z, hit) {
			z.dying, z.deathAge = true, 0
			p.classifyZombieDeath(z, 0x10, pr.X, pr.Y)
		}
		return true
	}
	return false
}

// westNextThreshold is FUN_000b82ac: the threshold minus 22% of the start health (the float product truncated).
func (p *playState) westNextThreshold(w *westBoss) int {
	return w.threshold - int(float32(w.maxHealth)*float32(westThresholdStep))
}

// westGunRecord is FUN_000b19b8 on the catalog: the raw record of a gun id (an all-zero gun when the catalog has none,
// as for NONE and for DYNOMITE in a build without the DLC records).
func (p *playState) westGunRecord(id int) zombieGun {
	name := zombieGunName(id)
	for _, record := range p.zombieWeapons {
		if name == "" || record.GunType != name {
			continue
		}
		bullet, ok := zombieBulletKinds[record.BulletType]
		if !ok {
			bullet = 0x10
		}
		return zombieGun{timer: record.RecoilSeconds, interval: record.RateOfFire, spread: record.SpreadUnits, rounds: record.Ammo, pellets: record.AmmoPerShot, life: record.Life, speed: record.Speed, kind: bullet}
	}
	return zombieGun{}
}

// westLoadGun is FUN_000b8014: the gun of the current mode. With a script running the rounds are cleared only.
func (p *playState) westLoadGun(z *zombieState) {
	w := &z.native.west
	ai := &z.native.ai
	if p.samuraiScriptRunning() {
		ai.gun.rounds = 0
		ai.gun.timer = 0
		return
	}
	w.rangeSq = westRangeSq
	switch w.mode {
	case westModePistol:
		ai.gun = p.westGunRecord(westGunPistol)
		ai.gun.interval, ai.gun.rounds, ai.gun.speed = westPistolGap, westPistolShots, westPistolSpeed
	case westModeBomb:
		ai.gun = p.westGunRecord(westGunDynomite)
		ai.gun.speed, ai.gun.rounds, ai.gun.interval = westBombSpeed, westBombShots, westBombGap
		w.armsAnim = westAnimIdle
	case westModeRage:
		ai.gun = p.westGunRecord(westGunNone)
		w.armsAnim = westAnimIdle
	case westModeSpin:
		ai.gun = p.westGunRecord(westGunDynomite)
		ai.gun.speed, ai.gun.rounds, ai.gun.interval = westBombSpeed, westSpinShots, westSpinGap
		w.armsAnim = westAnimIdle
	case westModeUzi:
		ai.gun = p.westGunRecord(westGunUzi)
		ai.gun.rounds, ai.gun.speed = westUziShots, westUziSpeed
	}
	ai.gun.timer = 0
}

// westModeChange is FUN_000b83dc: rage starts the fade mode (invulnerable, SFX_MADDOG_HAHA); otherwise the mode steps
// (0 -> 1, 1 / 3 / 4 -> 0, 2 -> 3 or 4 by the rage counter). The new mode's gun is loaded and the 15 s timer restarts.
func (p *playState) westModeChange(z *zombieState, rage bool) {
	w := &z.native.west
	if rage {
		w.mode, w.rage = westModeRage, 0
		p.westSetInvuln(z, true)
		p.sfxQueue = append(p.sfxQueue, westSoundHaha)
	} else {
		switch w.mode {
		case westModePistol:
			w.mode = westModeBomb
		case westModeBomb, westModeSpin, westModeUzi:
			w.mode = westModePistol
		case westModeRage:
			counter := w.phase
			next := counter
			if counter-3 < 0 || counter-3 > 1 {
				if counter < 5 {
					next = counter + 2
				} else {
					next = counter - 2
				}
			}
			w.mode = next
			w.phase = counter + 1
		}
	}
	p.westLoadGun(z)
	w.modeTimer = westModeSeconds
}

// westSetInvuln sets the +0x2e1 flag; the port's invulnerable flag makes bullets stop at the body as they do at a
// scripted invulnerable zombie.
func (p *playState) westSetInvuln(z *zombieState, value bool) {
	z.native.west.invuln = value
	z.invulnerable = value
}

// westTrace is FUN_000bfb54: from the start every 32 px (while i < length / 32) a grid of points -r .. r in 32 px steps
// is tested; a collision value 1 (and 2 with the flag) blocks. It returns true when blocked.
func (p *playState) westTrace(x0, y0, x1, y1, radius float64, flag bool) bool {
	if p.world == nil {
		return false
	}
	dx, dy := x1-x0, y1-y0
	length := float32(math.Sqrt(float64(float32(dx*dx + dy*dy))))
	if length == 0 {
		return false
	}
	stepX, stepY := float64(float32(dx)/length*float32(westTraceStep)), float64(float32(dy)/length*float32(westTraceStep))
	count := length * float32(1.0/32)
	x, y := x0, y0
	for i := 0; float32(i) < count; i++ {
		for ox := -radius; ox < radius; ox += westTraceStep {
			for oy := -radius; oy < radius; oy += westTraceStep {
				value := p.collisionValue(int(float32(x+ox)*(1.0/32)), int(float32(y+oy)*(1.0/32)))
				if value == 1 || (flag && value == 2) {
					return true
				}
			}
		}
		x += stepX
		y += stepY
	}
	return false
}

// scriptHeading is the heading units of a script rotation in degrees (the callbacks multiply by 182; a non-positive
// product converts to 0 as in nativeSpriteDirection).
func scriptHeading(degrees float64) uint16 {
	units := float32(degrees) * 182
	if units > 0 {
		return uint16(int16(int(units)))
	}
	return 0
}

// westVisible is the draw flag the entity manager tests (+0x41, FUN_000a1968's first test: the body's box meets the
// 480 x 320 screen); the native may-attack slot reads its copy in the entity flags (bit 0x8).
func (p *playState) westVisible(z *zombieState) bool {
	if p.world == nil {
		return true
	}
	zoom := p.markerZoom()
	sx, sy := p.markerScreen(z.x, z.y-z.native.sizeZ*zombieRenderAnchor)
	halfW, halfH := z.size.X*zoom/2, z.size.Y*zoom/2
	return sx+halfW > 0 && sx-halfW < logicalWidth && sy+halfH > 0 && sy-halfH < logicalHeight
}

// westMayAttack is vtable slot +0x48 (FUN_000b7b24, 1.2.5 0x0011a0e0). vx, vy is the vector to the player, distSq its
// squared length.
func (p *playState) westMayAttack(z *zombieState, vx, vy, distSq, targetX, targetY float64, prey int) bool {
	w := &z.native.west
	ai := &z.native.ai
	if w.mode > westModeBomb {
		// modes 2 and 3 skip the route logic and always report in range
		if w.mode == westModeSpin {
			ai.desired += uint16(w.spin)
		} else {
			ai.desired = weapons.NativeWeaponDirection(vx, vy)
		}
		return true
	}
	radius := float64(float32(z.size.X) * float32(westTraceRadius))
	w.pathTimer--
	w.distSq = distSq
	if w.pathTimer < 1 {
		if distSq > westPathDropSq {
			w.pathActive = false
		} else {
			// FUN_000bfb54 blocked -> a route is built (FUN_0009bbb8), clear -> the list is dropped.
			w.pathActive = p.westTrace(z.x, z.y, targetX, targetY, radius, true)
		}
		w.pathTimer = westPathReload
	}
	// The route's heading follows the next waypoint (the port's navigation step stands for the waypoint list
	// FUN_0009bbb8 builds, as for the samurai): the node is a tile away, never inside 0.4 * width.
	aim := distSq > z.size.X*z.size.X*westWaypointSq
	result := false
	if !w.pathActive {
		result = p.westVisible(z)
	} else if ai.gun.rounds > 0 && !p.westTrace(z.x, z.y, targetX, targetY, radius, false) {
		result = true
	} else if navX, navY, ok := p.navDirection(z.x, z.y, prey); ok {
		vx, vy, aim = navX, navY, true
	}
	if aim {
		ai.desired = weapons.NativeWeaponDirection(vx, vy)
	}
	return result && distSq <= w.rangeSq
}

// westStep is FUN_0009fb10 for this class: the always-alerted branch, the aim / state selection, the shared state machine
// (the cannot-fire slot is the constant 0), the boss step (the AI), the heading and the move. It returns the
// displacement before the tile push-out.
func (p *playState) westStep(z *zombieState, prey playerTarget, dx, dy, dt float64) (moveX, moveY float64) {
	ai := &z.native.ai
	w := &z.native.west
	if w.dead {
		return 0, 0
	}
	distSq := dx*dx + dy*dy
	if ai.wander < zombieWanderMin {
		ai.wander = float64(zombieRandom(p.rng, .3)) + zombieWanderMin
	}
	if health := int(z.health); health != w.lastHealth {
		// the damage slot's threshold test runs on every hit (blasts and saws subtract the health directly)
		w.lastHealth = health
		p.westAfterHurt(z)
	}
	may := p.westMayAttack(z, dx, dy, distSq, z.x+dx, z.y+dy, prey.index)
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
	p.westAI(z, prey, dt)
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

// westAI is FUN_000b8720 for a living boss (1.2.5 0x0011acd8): the arm flag latch, the boss clock, the mode timer (only
// once the thresholds are used up), the gun reload at zero rounds, the rage fade or the spin-up and the arms clip.
func (p *playState) westAI(z *zombieState, prey playerTarget, dt float64) {
	w := &z.native.west
	ai := &z.native.ai
	w.armPrev = w.armFlag
	running := p.samuraiScriptRunning()
	if w.threshold <= 0 {
		w.modeTimer -= dt
		if w.modeTimer <= 0 {
			p.westModeChange(z, false)
		}
	}
	if ai.gun.rounds == 0 {
		p.westLoadGun(z)
	}
	switch w.mode {
	case westModeRage:
		p.westRage(z, dt)
	case westModeSpin:
		if float64(w.spin) < westSpinCap {
			ai.gun.timer = 0
			w.spin = int(int32(w.spin) + int32(uint16(int(float32(dt)*float32(westSpinRate)))))
		}
	}
	if !running {
		switch ai.state {
		case 3:
			if w.mode != westModeBomb && w.mode != westModeRage && w.armsAnim != westAnimShoot {
				w.armsAnim = westAnimShoot
			}
		case 4:
			if w.armsAnim != westAnimIdle {
				w.armsAnim = westAnimIdle
			}
		}
	}
}

// westRage is FUN_000b8624: fade out backing away, jump to (650, 570), hidden for 3 s, fade in, 0.5 s, next mode.
func (p *playState) westRage(z *zombieState, dt float64) {
	w := &z.native.west
	ai := &z.native.ai
	alpha := samuraiAlphaByte(z.alpha)
	switch w.rage {
	case 0:
		if alpha > westAlphaStep {
			alpha -= westAlphaStep
			z.alpha = float64(alpha) / 255
			ai.speedFactor = -1
			return
		}
		z.alpha = 0
		ai.speedFactor = 0
		z.x, z.y = westRageX, westRageY
		w.rage = 1
		w.rageTimer = westRageHide
	case 1:
		w.rageTimer -= dt
		if w.rageTimer > 0 {
			return
		}
		if alpha > 0xef {
			z.alpha = 1
			p.westSetInvuln(z, false)
			w.rage = 2
			w.rageTimer = westRageWake
			p.sfxQueue = append(p.sfxQueue, westSoundDraw)
			return
		}
		alpha += westAlphaStep
		z.alpha = float64(alpha) / 255
	case 2:
		w.rageTimer -= dt
		if w.rageTimer <= 0 {
			p.westModeChange(z, false)
		}
	}
}

// westDirIndex is the compass direction the arms sprite stores while it is drawn (FUN_000f21f4: 9 directions of 40
// degrees for 5 angles, 8 folded onto 0).
func westDirIndex(heading uint16) int {
	degrees := float32(heading) / float32(182.0417)
	index := int(math.Floor(math.Ceil(float64(float32(360)-degrees+float32(90)))/40)) % 9
	if index < 0 {
		index += 9
	}
	if index > 7 {
		index -= 8
	}
	return index
}

// westFire is FUN_0009f7e4 for this class with the post-fire hook FUN_000b88a8 (1.2.5 0x0011ae60) on every pellet.
func (p *playState) westFire(z *zombieState) {
	ai := &z.native.ai
	w := &z.native.west
	gun := &ai.gun
	gun.timer = 0
	if gun.rounds < 1 {
		return
	}
	gun.rounds--
	jitter := int(gun.spread + gun.spread)
	prey := p.nearestPlayer(z.x, z.y)
	for pellet := 0; pellet < gun.pellets; pellet++ {
		heading := z.native.facing
		if jitter > 0 {
			heading = heading - uint16(jitter) + uint16(zombieBounded(p.rng, uint32(jitter<<1)))
		}
		switch gun.kind {
		case 0x10:
			dir := w.armDir
			table := &westMuzzleB
			if w.armPrev {
				table = &westMuzzleA
			}
			mx, my := z.x+table[dir][0], z.y+table[dir][1]
			aim := heading
			if w.mode != westModeSpin {
				aim = weapons.NativeWeaponDirection(prey.x-mx, prey.y-my)
			}
			projectile := weapons.NativeWeaponProjectile{X: mx, Y: my, Direction: aim, Life: gun.life, Lift: westBulletLift, Width: westBulletWidth, Height: westBulletHeight, EntityType: 0x10, Texture: westBulletSprite}
			projectile.VX = cosU16(aim) * gun.speed
			projectile.VY = sinU16(aim) * gun.speed
			p.zombieShots = append(p.zombieShots, zombieShot{projectile: projectile, hitsZombies: true, ownerID: z.scriptID})
			if w.mode == westModeUzi {
				p.sfxQueue = append(p.sfxQueue, westSoundSpecial)
			} else {
				p.sfxQueue = append(p.sfxQueue, westSoundFire)
			}
			if dir != 4 && ((dir != 5 && dir != 3) || w.armPrev) {
				w.flash, w.flashX, w.flashY = westFlashMillis, mx, my
				w.flashAngle = uint16(0x4000 - (dir << 13))
			}
		case zombieBombKind:
			// The init runs twice (FUN_0009f7e4 and the hook), so the throw sound plays twice.
			p.sfxQueue = append(p.sfxQueue, zombieBombFireSound, zombieBombFireSound)
			p.thrown = append(p.thrown, thrownBomb{x: z.x, y: z.y, lift: westBombLift, fall: westBombFall, dirX: cosU16(heading), dirY: sinU16(heading),
				speed: gun.speed, life: gun.life * 2, size: zombieBombBaseSize * zombieBombScale, bounce: true,
				zombieBomb: true, bounces: westBombBounces, ownerID: z.scriptID, boost: westBombBoost, shockwave: true})
		}
	}
	w.armFlag = !w.armPrev
	if gun.rounds < 1 {
		p.westModeChange(z, false)
	}
}

// westCanBeHurt is the front of the damage slot (v7 FUN_000b848c, 1.2.5 0x0011aa64).
func (p *playState) westCanBeHurt(z *zombieState) bool {
	if z.native.west.invuln {
		return false
	}
	if p.waveBuild() == content.WaveBuild125 {
		return len(p.livingPlayers()) > 0
	}
	return p.health > 0
}

// westAfterHurt is the tail of the damage slot once the damage was subtracted: crossing the health threshold in mode 0
// starts the rage (the threshold moves down by 22% of the start health).
func (p *playState) westAfterHurt(z *zombieState) {
	w := &z.native.west
	if z.health <= 0 || w.dead {
		return
	}
	if int(z.health) < w.threshold && w.mode == westModePistol {
		w.threshold = p.westNextThreshold(w)
		p.westModeChange(z, true)
	}
}

// westDeath is the death hook FUN_000b7e1c (1.2.5 0x00119ec8), run once.
func (p *playState) westDeath(z *zombieState) {
	w := &z.native.west
	if w.deathDone {
		return
	}
	w.deathDone, w.dead = true, true
	z.native.ai.state = 6
	z.invulnerable = false
	w.invuln = false
	z.animation = westAnimDeath
	z.frame = 0
	z.fps = p.spriteFPS(z.texture, westAnimDeath)
	w.animClock = 0
	if !w.scored {
		w.scored = true
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

// westDeadStep is the dead branch of FUN_000b8720: the Death clip runs (4 frames, no loop) and then FinalWords loops;
// SetWesternBossDead (westSetDead) replaces it with the Dead clip.
func (p *playState) westDeadStep(z *zombieState, dt float64) {
	w := &z.native.west
	w.animClock += dt
	definition, ok := p.sprites.Find(z.texture)
	var animation formats.SpriteAnimation
	if ok {
		animation, ok = definition.Animation(z.animation)
	}
	fps, frames, loop := z.fps, 4.0, true
	if ok && animation.FPS > 0 && animation.Frames > 0 {
		fps, frames, loop = animation.FPS, float64(animation.Frames), animation.Loop
	}
	if fps <= 0 {
		fps = 5
	}
	total := w.animClock * fps
	if !loop && total >= frames {
		z.frame = frames - 1
		if z.animation == westAnimDeath {
			z.animation, z.frame, w.animClock = westAnimFinalWords, 0, 0
			z.fps = p.spriteFPS(z.texture, westAnimFinalWords)
		}
		return
	}
	z.frame = total
}

// westSetDead is FUN_000b7fe4 (the SetWesternBossDead callback): the body switches to the Dead clip.
func (p *playState) westSetDead(z *zombieState) {
	w := &z.native.west
	z.animation = westAnimDead
	z.frame = 0
	z.fps = p.spriteFPS(z.texture, westAnimDead)
	w.animClock = 0
	z.speed = 0
}

// westPostUpdate is the arms part of FUN_000b7a5c: the arms clip copies the body frame (or, shooting, follows the gun
// timer: half the sheet per arm) and the flash timer decays by 1000 per second.
func (p *playState) westPostUpdate(z *zombieState, dt float64) {
	w := &z.native.west
	if w.dead {
		return
	}
	w.armDir = westDirIndex(z.native.facing)
	rows := 4
	if definition, ok := p.sprites.Find(westArmsSprite); ok {
		if animation, ok := definition.Animation(w.armsAnim); ok && animation.Frames > 0 {
			rows = animation.Frames
		}
	}
	if w.armsAnim == westAnimShoot {
		base := 0.5
		if w.armPrev {
			base = 0
		}
		gun := z.native.ai.gun
		v := base
		if gun.interval != 0 {
			v += float64(float32(gun.timer) / float32(gun.interval) * .5)
		}
		frame := int(v * float64(rows))
		if frame < 0 {
			frame = 0
		}
		if frame > rows-1 {
			frame = rows - 1
		}
		w.armsFrame = frame
	} else {
		bodyRows := 4
		if definition, ok := p.sprites.Find(z.texture); ok {
			if animation, ok := definition.Animation(z.animation); ok && animation.Frames > 0 {
				bodyRows = animation.Frames
			}
		}
		frame := int(math.Floor(z.frame)) % bodyRows
		if frame < 0 {
			frame += bodyRows
		}
		if frame > rows-1 {
			frame = rows - 1
		}
		w.armsFrame = frame
	}
	w.flash = math.Max(0, w.flash-dt*1000)
}

// drawWestArms is FUN_000b8d08 followed by the muzzle flash FUN_000b8b18: the arms sprite overlays the body with the
// same box and heading, then the two-frame muzzleflash_01 quad sits 15 / 10 px along the flash angle.
func (a *app) drawWestArms(screen *ebiten.Image, z zombieState, flipX bool, screenX, screenY float64, alpha float64) {
	w := &z.native.west
	if !w.active || w.dead || a.play == nil || a.play.world == nil {
		return
	}
	scale := a.play.world.Zoom
	if scale <= 0 {
		scale = 1
	}
	frontendX, frontendY := a.renderScale()
	if animation, ok := a.spriteAnimation(westArmsSprite, w.armsAnim); ok {
		if texture, err := a.Texture(animation.Texture); err == nil {
			columns, rows := 5, 4
			if animation.Angles > 0 {
				columns = animation.Angles
			}
			if animation.Frames > 0 {
				rows = animation.Frames
			}
			angle := int(math.Round(float64(z.angle) * float64(columns-1) / 8))
			if entity := a.play.scriptEntities[z.scriptID]; entity != nil && entity.rotationSet {
				angle, flipX = nativeSpriteDirection(entity.rotation, columns)
			}
			frame := w.armsFrame % rows
			rect := atlasCellRect(animation.Atlas, angle, frame, columns, rows, texture.Bounds().Dx(), texture.Bounds().Dy())
			if rect.Dx() > 0 && rect.Dy() > 0 {
				source := texture.SubImage(rect).(*ebiten.Image)
				options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
				options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
				scaleX, scaleY := nativemath.ZombieSpriteScale(z.size.X, z.size.Y, rect, scale, frontendX, frontendY)
				scaleX, scaleY = scriptEntityRenderScale(scaleX, scaleY, flipX, z.flipY)
				options.GeoM.Scale(scaleX, scaleY)
				if alpha < 1 {
					options.ColorScale.ScaleAlpha(float32(math.Max(0, alpha)))
				}
				options.GeoM.Translate(screenX, screenY)
				a.drawImage(screen, source, options)
			}
		}
	}
	if w.flash > 0 && w.flash < westFlashMillis {
		texture, err := a.Texture(westFlashSprite)
		if err != nil || texture.Bounds().Dx() <= 0 || texture.Bounds().Dy() <= 0 {
			return
		}
		cx := float64(cosU16(w.flashAngle))*westFlashDX + w.flashX
		cy := w.flashY - sinU16(w.flashAngle)*westFlashDY
		half := texture.Bounds().Dx() / 2
		left := 0
		if w.flash <= westFlashSplit {
			left = half
		}
		source := texture.SubImage(image.Rect(left, 0, left+half, texture.Bounds().Dy())).(*ebiten.Image)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(half)/2, -float64(texture.Bounds().Dy())/2)
		options.GeoM.Scale(westFlashWidth*scale/float64(half)*frontendX, westFlashHeight*scale/float64(texture.Bounds().Dy())*frontendX)
		options.GeoM.Rotate(float64(w.flashAngle) / westUnitsPerDeg * math.Pi / 180)
		world := a.play.world
		options.GeoM.Translate(((cx-world.CameraX)*scale+world.ViewportX)*frontendX, ((cy-world.CameraY)*scale+world.ViewportY)*frontendY)
		a.drawImage(screen, source, options)
	}
}
