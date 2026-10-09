package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// T-Rex boss AI: engagement, venom volleys, lunge, contact damage, drop-in.
//
// Evidence: Research/native/trex-ai-2026-10-09.md. The rex is entity kind 10
// (vtable 0x005bc000). Its AI is the shared zombie update FUN_0009fb10 with the
// rex overrides AI = FUN_000b9df0 (+0x48), contact = FUN_000b94e4 (+0x58),
// gate = FUN_000b94c4 (+0x70), always-sees-player = FUN_000ba288 (+0x74),
// boss pre-update FUN_000b48c0 (+0x34) -> rage/leap FUN_000b9624 (+0x7c) and
// reset FUN_000ba174 (+0x78, called from FUN_000b4864 at spawn). Movement is the
// shared FUN_000a18e4 (pos += heading * speed * dt * speedFactor), i.e. the rex
// walks through the same movement code as every other zombie.
//
// The rex carries zombie weapon record 10 (VOMIT: Common0_ZombieWeapons.xml, the
// index of "VOMIT" in the table filled at 0x000b2b4c). FUN_000ba174 loads it into
// the entity (+0x2e4..) and sets ammo +0x2f4 = 3. State 3 (shooting) fires the
// record through FUN_0009f7e4 while ammo > 0 (ammo is decremented there - that
// is the only writer, together with the reloads), which is what finally lets the
// gate FUN_000b94c4 (ammo <= 0 and not leaping) open the melee lunge and the
// health-fraction rage.
const (
	// FUN_000ba174 multiplies the sight range (squared) +0x274 by 3.0 (0x000ba250).
	// The base sight is (110 + rnd(50))^2 (FUN_000a0b80, no ramp for types >= 10).
	rexSightBase   = 110.0
	rexSightSpread = 50.0
	rexSightFactor = 3.0

	// FUN_000b9df0: the path/waypoint list is refreshed when the per-call counter
	// +0x334 reaches 0 (then 100); AI engages only while the counter is >= 31
	// (0x1f) with an empty list.
	rexAIPathPeriod   = 100
	rexAIEngageCutoff = 31

	// Speed factor +0x48 ramps (FUN_0009fb10 0x0009fee4 = 4.0; FUN_0009f32c
	// 0x0009f51c = 4.0).
	rexFactorRate = 4.0
	// Lunge: state 3 -> 5 sets the factor to 4.0 (0x0009f52c); state 5 decays it
	// at 4.0/s until 1.0 (0x0009f520).
	rexLungeFactor = 4.0
	// Melee gate wait: +0x2ec = 0.95 (0x0009f524) and the facing must be within
	// 0xe37 units (0x0009f528) of the desired heading.
	rexMeleeWait      = 0.95
	rexMeleeAngleUnit = 3639

	// Zombie weapon record 10 (VOMIT), Common0_ZombieWeapons.xml: Recoil 800 and
	// Rate_Of_Fire 800 (/1000 -> +0x2e8 / +0x2ec), Spread 17 (*182 units),
	// Ammo_Per_Shot 4 (projectiles per volley, +0x2f8), Speed 250, Life 0,
	// Bullet_Type VENOM (entity kind 0x17).
	rexFireInterval    = 0.8
	rexVolleyCount     = 4
	rexVolleySpread    = 17 * 182 // heading units (65536 = 360 degrees)
	rexVenomSpeed      = 250.0
	rexVenomMuzzle     = 16.0 // DAT_0009faf8, spawn offset along the facing for kind 0x17
	rexVenomHeightFrac = 0.3  // DAT_0009fae8: initial height = render size * 0.3
	rexAmmoReload      = 3    // FUN_000ba174 / landing
	rexAmmoContact     = 5    // FUN_000b94e4 when touching the player in state 5
	rexRoarThrottle    = 1.73 // seconds, length of T_rex_roar_1/2.ogg (native skips while 0x2a/0x2b play)
	rexRoarFactor      = 3.75 // FUN_000b9624: roar while the speed factor exceeds 3.75 (lunge start)
	rexContactDamage   = 0.05 // DAT_000b9614 per call, three calls per tick, times the frame delta
	rexContactCalls    = 3    // FUN_000b94e4 calls FUN_00094c6c three times
	rexContactReach    = 0.3  // FUN_000a1b08: 0.3 * (width + player width 64)
	rexPlayerWidth     = 64.0 // player +0x28
	rexDropDistanceMin = 80.0 // FUN_000ba094 -> FUN_000bfdc8(.., 80, 100, ..)
	rexDropDistanceMax = 100.0
	rexDropHeight      = 10.0 // DAT_000ba168, in render-size units
	rexDropGravity     = 6.0  // DAT_000ba16c

	// Venom projectile (kind 0x17, vtable 0x005bb370; update FUN_000a5f74 +
	// FUN_000a69c8, collide FUN_000a4288, init hook FUN_000a4210, draw FUN_000a7a64).
	rexVenomSize        = 10.0   // DAT_000a4978
	rexVenomGravity     = 500.0  // DAT_000a6a54
	rexVenomDamage      = 0.0005 // DAT_000a44c0 per FUN_00094c6c call
	rexVenomPuddleScale = 5.5    // DAT_000a44a0
	rexVenomPuddleLife  = 1.4    // DAT_000a427c (+ rnd(0.5))
	rexVenomVzSpan      = 5.0    // DAT_000a4284: vz = rnd(5) - 2
	rexVenomVzBase      = 2.0    // DAT_000a4278
	rexVenomChildren    = 1      // +0x80 bounce count, loop runs count + 3 droplets
	rexVenomTexture     = "Common0/Textures/vomit_splat_SD"
)

// rexAI is the per-rex slice of the zombie behaviour fields (names give offsets).
type rexAI struct {
	state    int     // +0x4c: 1 walking, 2 windup, 3 shooting / melee wait, 4 recover, 5 lunge
	factor   float64 // +0x48 speed factor
	ammo     int     // +0x2f4
	timer    float64 // +0x2e8
	interval float64 // +0x2ec
	counter  int     // +0x334
	rangeSq  float64 // +0x274
	lungeX   float64
	lungeY   float64
	roarLeft float64
	dropped  bool
}

func (p *playState) newRexAI() rexAI {
	sight := rexSightBase + float64(zombieRandom(p.rng, rexSightSpread))
	return rexAI{state: 1, factor: 1, ammo: rexAmmoReload, timer: rexFireInterval, interval: rexFireInterval, rangeSq: rexSightFactor * sight * sight}
}

// rexReload is the weapon record reload of FUN_000ba174 / the landing / the
// contact in state 5: timers come from the record, ammo is overridden.
func (a *rexAI) reload(ammo int) {
	a.ammo, a.timer, a.interval = ammo, rexFireInterval, rexFireInterval
}

// rexGateOpen is FUN_000b94c4: +0x2f4 <= 0 and not leaping.
func (s *rexBossState) gateOpen() bool { return s.ai.ammo <= 0 && !s.leaping }

// rexSpeedFactor multiplies the zombie speed in the shared movement step.
func (p *playState) rexSpeedFactor(zombie *zombieState) float64 {
	if !p.isRexBoss(zombie) {
		return 1
	}
	if p.rexHeld(zombie) {
		return 0
	}
	if state := p.rexBoss(zombie.scriptID); state != nil {
		return state.ai.factor
	}
	return 1
}

// rexAnimationFactor scales the sprite playback (FUN_000a1b08 advances the
// sprite by dt * +0x48; FUN_000b9cfc forces 1.0 while the rage timer runs).
func (p *playState) rexAnimationFactor(zombie *zombieState) float64 {
	if !p.isRexBoss(zombie) {
		return 1
	}
	if zombie.rexRageTimer > 0 {
		return 1
	}
	if p.rexHeld(zombie) {
		return 0
	}
	if state := p.rexBoss(zombie.scriptID); state != nil {
		return state.ai.factor
	}
	return 1
}

// rexLungeDirection is the locked heading of a lunge: state 5 skips the turn
// code of FUN_0009fb10 (0x000a0210), so the rex runs straight.
func (p *playState) rexLungeDirection(zombie *zombieState) (float64, float64, bool) {
	if !p.isRexBoss(zombie) {
		return 0, 0, false
	}
	state := p.rexBoss(zombie.scriptID)
	if state == nil || state.ai.state != 5 || (state.ai.lungeX == 0 && state.ai.lungeY == 0) {
		return 0, 0, false
	}
	return state.ai.lungeX, state.ai.lungeY, true
}

// scenePlaying is the native scene state == 1 test that FUN_00094c6c applies to
// every player hit (0x00094c98); the port maps it to "no cutscene script running"
// like the wave spawner and the train.
func (p *playState) scenePlaying() bool {
	return p.scriptRuntime == nil || p.scriptRuntime.Done()
}

// rexSteer is the heading the rex walks. A lunge keeps its locked heading; when
// the line to the player is blocked the native AI (FUN_000b9df0) follows a
// waypoint list built by FUN_0009bbb8, which the port's navigation field replaces.
func (p *playState) rexSteer(zombie *zombieState, playerIndex int, dirX, dirY float64) (float64, float64) {
	if !p.isRexBoss(zombie) {
		return dirX, dirY
	}
	if lx, ly, ok := p.rexLungeDirection(zombie); ok {
		return lx, ly
	}
	if p.world != nil && p.tileSize > 0 {
		prey := p.nearestPlayer(zombie.x, zombie.y)
		if !p.lineClear(zombie.x, zombie.y, prey.x, prey.y, zombieCollisionRadius(*zombie)*.8) {
			if navX, navY, ok := p.navDirection(zombie.x, zombie.y, playerIndex); ok {
				return navX, navY
			}
		}
	}
	return dirX, dirY
}

// updateRexAI is the FUN_0009fb10 chase branch for the rex: AI engagement
// (FUN_000b9df0), the state transitions and the state machine FUN_0009f32c.
func (p *playState) updateRexAI(zombie *zombieState, state *rexBossState, dt float64) {
	ai := &state.ai
	ai.roarLeft = math.Max(0, ai.roarLeft-dt)
	if p.rexHeld(zombie) {
		// FUN_000b9624 forces the state to 1 and the factor to 0 while the rage
		// timer runs or the rex leaps, and FUN_000b9df0 returns 0 without touching
		// its counter while +0x344 is set.
		ai.state, ai.factor = 1, 0
		return
	}
	prey := p.nearestPlayer(zombie.x, zombie.y)
	targetX, targetY := prey.x, prey.y
	dx, dy := targetX-zombie.x, targetY-zombie.y
	distSq := dx*dx + dy*dy
	alive := len(p.livingPlayers()) > 0

	ai.counter--
	if ai.counter <= 0 {
		ai.counter = rexAIPathPeriod
	}
	engaged := alive && ai.counter >= rexAIEngageCutoff && distSq <= ai.rangeSq
	if engaged && p.world != nil && p.tileSize > 0 && !p.lineClear(zombie.x, zombie.y, targetX, targetY, 0) {
		// Native: a blocked line builds a waypoint list (FUN_0009bbb8) and AI
		// reports 0 while it follows it. The port's navigation field plays that role.
		engaged = false
	}
	if !engaged {
		if ai.state == 3 {
			ai.state = 4
		}
	} else if ai.state == 1 {
		ai.state = 2
	}
	if ai.state < 2 {
		if ai.factor < 1 {
			ai.factor = math.Min(1, ai.factor+dt*rexFactorRate)
		}
		return
	}
	switch ai.state {
	case 2:
		ai.factor -= dt * rexFactorRate
		if ai.factor < 0 {
			ai.factor, ai.state = 0, 3
		}
	case 4:
		ai.factor += dt * rexFactorRate
		if ai.factor > 1 {
			ai.factor, ai.state = 1, 1
		}
	case 3:
		if !state.gateOpen() {
			if ai.interval <= ai.timer {
				p.fireRexVenom(zombie, state, dx, dy)
				return
			}
			ai.timer = math.Min(ai.timer+dt, ai.interval)
			return
		}
		ai.interval = rexMeleeWait
		if ai.timer < ai.interval {
			ai.timer += dt
			return
		}
		// The port faces the target instantly, so the 0xe37 facing test is always met.
		if length := math.Hypot(dx, dy); length > 0 {
			ai.lungeX, ai.lungeY = dx/length, dy/length
		}
		ai.state, ai.factor = 5, rexLungeFactor
	case 5:
		if ai.factor > 1 {
			ai.factor -= dt * rexFactorRate
			return
		}
		ai.timer, ai.factor, ai.state = 0, 1, 1
	}
}

// rexLungeRoar is FUN_000b9624 0x000b9830: a roar while the speed factor is above
// 3.75 and neither 0x2a nor 0x2b plays.
func (p *playState) rexLungeRoar(zombie *zombieState, state *rexBossState) {
	if state.ai.factor > rexRoarFactor && state.ai.roarLeft <= 0 {
		p.sfxQueue = append(p.sfxQueue, p.rexRoar(zombie))
		state.ai.roarLeft = rexRoarThrottle
	}
}

// rexContact is FUN_000b94e4, called by FUN_000a1b08 for every overlap with the
// player: dist < 0.3 * (rex width + 64). Unless the rex leaps it calls
// FUN_00094c6c three times with dt * 0.05; touching the player in state 5
// reloads the weapon record with ammo 5.
func (p *playState) rexContact(zombie *zombieState, state *rexBossState, dt float64) {
	if state.leaping || !p.scenePlaying() {
		return
	}
	reach := rexContactReach * (zombie.size.X + rexPlayerWidth)
	for _, target := range p.livingPlayers() {
		dx, dy := target.x-zombie.x, target.y-zombie.y
		if dx*dx+dy*dy >= reach*reach {
			continue
		}
		for call := 0; call < rexContactCalls; call++ {
			p.damagePlayer(target.index, dt*rexContactDamage)
		}
		if state.ai.state == 5 {
			state.ai.reload(rexAmmoContact)
		}
	}
}

// fireRexVenom is FUN_0009f7e4 for record 10: ammo is decremented, the timer
// restarts and four VENOM projectiles leave at the facing +-2*spread.
func (p *playState) fireRexVenom(zombie *zombieState, state *rexBossState, dx, dy float64) {
	ai := &state.ai
	ai.timer = 0
	if ai.ammo < 1 {
		return
	}
	ai.ammo--
	length := math.Hypot(dx, dy)
	if length == 0 {
		dx, dy, length = 0, 1, 1
	}
	facing := math.Atan2(dy, dx)
	spread := 2 * rexVolleySpread
	for shot := 0; shot < rexVolleyCount; shot++ {
		units := 0
		if spread > 0 {
			units = int(zombieBounded(p.rng, uint32(spread*2))) - spread
		}
		heading := facing + float64(units)*2*math.Pi/65536
		p.rex.venom = append(p.rex.venom, p.newRexVenom(zombie.x+math.Cos(facing)*rexVenomMuzzle, zombie.y+math.Sin(facing)*rexVenomMuzzle, heading, rexVenomHeight(zombie), rexVenomSpeed, false))
	}
}

func rexVenomHeight(zombie *zombieState) float64 { return zombie.size.Y * rexVenomHeightFrac }

// rexRoar picks the roar: ids 0x2a / 0x2b are T_REX_ROAR_1/2 in World0; the
// president boss uses its own roar clips (names used by president_boss.script).
func (p *playState) rexRoar(zombie *zombieState) string {
	var draw uint32
	if p.rng != nil {
		draw = p.rng.Bounded(0)
	}
	pick := 1 + int(draw>>31)
	if p.isPresidentBoss(zombie) {
		return fmt.Sprintf("PresidentRoar%d", pick)
	}
	return fmt.Sprintf("SFX_T_REX_ROAR_%d", pick)
}

func (p *playState) isPresidentBoss(zombie *zombieState) bool {
	return strings.Contains(strings.ToLower(zombie.texture), "georgewashington")
}

// rexVenom is the kind-0x17 projectile of the rex's weapon record.
type rexVenom struct {
	x, y       float64
	dirX, dirY float64
	heading    float64
	speed      float64
	height, vz float64 // +0x78, +0x7c
	age, life  float64 // +0x68, +0x5c
	size       float64 // +0x28
	state      int     // +0x6c: 0 flying, 2 puddle
	children   int     // +0x80
	removed    bool
}

func (p *playState) newRexVenom(x, y, heading, height, speed float64, child bool) rexVenom {
	v := rexVenom{x: x, y: y, dirX: math.Cos(heading), dirY: math.Sin(heading), heading: heading, speed: speed, height: height, size: rexVenomSize}
	// Init hook FUN_000a4210: bounce count 1, vz = rnd(5) - 2, life = 1.4 + rnd(0.5).
	v.vz = float64(zombieRandom(p.rng, rexVenomVzSpan)) - rexVenomVzBase
	v.children = rexVenomChildren
	v.life = rexVenomPuddleLife + float64(zombieRandom(p.rng, .5))
	return v
}

func (p *playState) venomWallBlocks(x, y float64) bool {
	return p.projectileBlockedAt(x, y)
}

// updateRexVenom is one FUN_000a5f74 tick for every venom projectile.
func (p *playState) updateRexVenom() {
	const dt = 1.0 / 60.0
	if len(p.rex.venom) == 0 {
		return
	}
	// Children spawned this tick join the list after the pass.
	var spawned []rexVenom
	live := p.rex.venom[:0]
	for i := range p.rex.venom {
		v := p.rex.venom[i]
		v.age += dt
		v.height -= dt * v.vz
		if v.state == 2 {
			v.height = 0
			if v.life <= v.age {
				continue
			}
		} else {
			v.vz += dt * rexVenomGravity
			v.x += v.dirX * v.speed * dt
			v.y += v.dirY * v.speed * dt
		}
		if (v.state == 0 && (v.life <= v.age || p.venomWallBlocks(v.x, v.y))) || v.height < 0 {
			if v.state == 0 && v.height >= 0 {
				continue // FUN_000a4288: removed in flight (lifetime or wall)
			}
			if v.state == 0 {
				spawned = append(spawned, p.splatRexVenom(&v)...)
			}
			live = append(live, v)
			continue
		}
		hit := false
		reach := rexPlayerWidth*rexShockwaveTargetFactor + v.size*0.5
		for _, target := range p.livingPlayers() {
			if !p.scenePlaying() {
				break
			}
			dx, dy := target.x-v.x, (target.y-v.y)/rexShockwaveAspect
			if dx*dx+dy*dy < reach*reach {
				p.damagePlayer(target.index, rexVenomDamage)
				hit = true
			}
		}
		if hit && v.state == 0 {
			continue // flying venom is consumed by the player
		}
		live = append(live, v)
	}
	p.rex.venom = append(live, spawned...)
}

// splatRexVenom is the landing branch of FUN_000a4288: droplets, then the
// projectile itself becomes a puddle.
func (p *playState) splatRexVenom(v *rexVenom) []rexVenom {
	var droplets []rexVenom
	if v.children > 0 {
		for i := 0; i <= v.children+2; i++ {
			units := float64(zombieBounded(p.rng, 0xFF3A))
			speedRoll := float64(zombieRandom(p.rng, .15))
			heading := v.heading + units*2*math.Pi/65536
			d := p.newRexVenom(v.x, v.y, heading, 0, (speedRoll-.45)*v.speed, true)
			d.children = v.children - 1
			d.size = v.size * (float64(zombieRandom(p.rng, .1)) + .55)
			d.vz = v.vz * -.4
			droplets = append(droplets, d)
		}
	}
	v.state, v.age, v.vz, v.height = 2, 0, 0, 0
	v.size *= rexVenomPuddleScale
	return droplets
}

// rexVenomAlpha is FUN_000a7a64: puddles fade as (2 - 2 * age / life) * 255.
func rexVenomAlpha(age, life float64) float64 {
	value := int((2 - (age+age)/life) * 255)
	if value < 0 {
		value = 0
	}
	if value > 255 {
		value = 255
	}
	return float64(value) / 255
}

func (a *app) drawRexVenom(screen *ebiten.Image) {
	if screen == nil || a.play == nil || a.play.world == nil || len(a.play.rex.venom) == 0 {
		return
	}
	texture, err := a.Texture(rexVenomTexture)
	if err != nil {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	frontendX, frontendY := a.renderScale()
	tw, th := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	if tw <= 0 || th <= 0 {
		return
	}
	for _, v := range a.play.rex.venom {
		x, y := (v.x-world.CameraX)*zoom+world.ViewportX, (v.y-world.CameraY)*zoom+world.ViewportY
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Translate(-tw/2, -th/2)
		switch v.state {
		case 0:
			// In flight: size x 2*size, rotated by heading + 90 degrees, grey, alpha 200.
			options.GeoM.Scale(v.size/tw, 2*v.size/th)
			options.GeoM.Rotate(v.heading + math.Pi/2)
			options.ColorScale.Scale(150.0/255, 150.0/255, 150.0/255, 200.0/255)
			y -= v.height * zoom
		case 2:
			options.GeoM.Scale(v.size/tw, v.size*rexShockwaveAspect/th)
			alpha := float32(rexVenomAlpha(v.age, v.life))
			options.ColorScale.Scale(alpha, alpha, alpha, alpha)
		default:
			continue
		}
		options.GeoM.Scale(zoom*frontendX, zoom*frontendX)
		options.GeoM.Translate(x*frontendX, y*frontendY)
		screen.DrawImage(texture, options)
	}
}

// captureRexVenom is the "play-rex-venom" capture state: World0Level2 with the
// intro cleared, the rex 180 px from Barry about to spit its first volleys.
func (a *app) captureRexVenom() error {
	if err := a.selectCaptureLevel("World0Level2"); err != nil {
		return err
	}
	if err := a.openPlay(); err != nil {
		return err
	}
	p := a.play
	p.closeScript()
	p.dialogueIndex = len(p.dialogue)
	p.hudVisible = true
	p.waveIndex = len(p.world.Level.Waves)
	p.cheats.god = true
	id := p.scriptNextEntity
	p.scriptNextEntity++
	texture := "rexwalk_1"
	p.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: "boss_rex", x: p.x + 180, y: p.y, scaleX: 1, scaleY: 1, alpha: 1, texture: texture, speed: 0}
	size := nativeSpawnRenderSize(formats.Vec2{X: 80, Y: 80})
	p.zombies = append(p.zombies, zombieState{x: p.x + 180, y: p.y, speed: 0, health: 25000, rawPoints: 25000, size: formats.Vec2{X: size, Y: size}, texture: texture, scriptID: id, alpha: 1, fps: p.spriteFPS(texture, "")})
	return nil
}
