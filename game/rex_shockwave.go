package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// T-Rex boss: rage, leap and shockwave.
//
// Evidence: Research/native/trex-shockwave-2026-10-08.md. The rex class is the
// v7 entity-kind 10 (FUN_000ba050, vtable 0x005bc000). Its update slot
// FUN_000b9624 runs a rage state machine:
//
//   - rage starts when the script calls MakeRexRage or the health fraction
//     falls through 0.66 / 0.33 (FUN_000b9624 0x000b9978..0x000b9a54);
//   - rage holds the rex still for 1000 ms (+0x33c), then it leaps
//     (+0x344): vertical speed 3.5, gravity 6 (growing by 15/s while
//     falling), drawn lifted by height * render size (FUN_000a06e8);
//   - on landing it spawns projectile kind 0x1a (the shockwave) at its own
//     position with lifetime 1.0 and shakes the camera.
//
// Projectile kind 0x1a (vtable 0x005bb3d0): the quad grows from 0 to 600
// over its 1 s life (FUN_000a3bc4 / FUN_000a3c94), is drawn with the
// "blast_radius" texture (FUN_000a77d8) and plays sound 12 on init. While it
// overlaps a zombie it deals 1 damage per tick (FUN_000a6ed4); while it
// overlaps the player it deals 0.025 health per tick and pushes the player
// (FUN_000a6ed4 -> FUN_00094c6c). Overlap is the ellipse test in FUN_000a5f74.
const (
	rexShockwaveLifetime = 1.0
	rexShockwaveMaxSize  = 600.0
	// 0x3f2a7efa = 0.66699994 (FUN_000a3bc4 draw height factor and the y divisor of
	// the overlap tests, DAT_000a63d0).
	rexShockwaveAspect = float64(float32(0.66699994))
	// FUN_000a5f74: a zombie/boss target is hit when
	// dx^2 + (dy/aspect)^2 < (wave.size + target.size*0.3)^2 (the wave size field
	// +0x28 is used whole, not halved); the player is hit when
	// dx^2 + (dy/aspect)^2 < (64*0.3 + wave.size*0.5)^2. Candidates come from the
	// entity grid box wave.pos +- wave.size*0.5 (see entity_grid.go).
	rexShockwaveTargetFactor = 0.3 // DAT_000a63d4
	rexShockwaveHalf         = float32(0.5)
	rexShockwaveZombieDamage = 1.0
	rexShockwavePlayerDamage = 0.025 // 0x3ccccccd, FUN_00094c6c(player, 0.025, wave)
	rexShockwaveTexture      = "Common0/Textures/blast_radius_SD"
	rexShockwaveSound        = "SFX_GRENADE_EXPLODE" // native sound id 12
	rexDeathSound            = "SFX_T_REX_DEATH"     // native sound id 0x2c

	rexRageMillis    = 1000.0
	rexLeapVelocity  = 3.5
	rexLeapGravity   = 6.0
	rexLeapFallAccel = 15.0
	// FUN_000b9624 0x000b9bc0: with the scene in play state the wave's +0x30 (600)
	// is multiplied by rex width / 200 (DAT_000b98c0), i.e. 3 x width.
	rexShockwaveScale = 600.0 / 200.0
	rexRageThreshold  = 0.66
	rexRageSecond     = 0.33

	// Rage gate (vtable +0x70, FUN_000b94c4): field +0x2f4 <= 0 and not leaping.
	// +0x2f4 is the ammo of the rex's weapon record (VOMIT, see rex_ai.go): it starts
	// at 3 (FUN_000ba174), FUN_0009f7e4 decrements it for every venom volley and the
	// landing reloads it to 3, so the health rage opens after the volleys are spent.

	// FUN_000be1fc(scene, pos, 1.5, 1.65, 1.0, 1.0) on landing (0x000b9c54..64):
	// duration 1.5 s, horizontal amplitude 1.65, vertical amplitude 1, angle
	// multiplier 1.
	rexShakeDuration = 1.5
	rexShakeAmpX     = 1.65
	rexShakeAmpY     = 1.0
	rexShakeAngleMul = 1.0
)

type rexShockwave struct {
	x, y, age float64
	shot      int
	owner     int
	max       float64 // +0x30; 600 scaled by width/200 (FUN_000b9624 0x000b9bc0), 0 means 600
}

func (w rexShockwave) size() float64 {
	max := w.max
	if max <= 0 {
		max = rexShockwaveMaxSize
	}
	return w.age / rexShockwaveLifetime * max
}

type rexBossState struct {
	maxHealth, lastRatio      float64
	height, velocity, gravity float64
	raged, leaping            bool
	ai                        rexAI
}

type rexBossField struct {
	bosses map[int]*rexBossState
	waves  []rexShockwave
	venom  []rexVenom
	dropIn map[int]bool // boss_rex spawned and still to be dropped in (FUN_000ba094)
}

// rexShockwaveSize is the quad width, which is also twice the overlap radius.
func rexShockwaveSize(age float64) float64 {
	return age / rexShockwaveLifetime * rexShockwaveMaxSize
}

// rexShockwaveAlpha is (1 - age/life) * 255 * 5 clamped to a byte (FUN_000a77d8).
func rexShockwaveAlpha(age float64) float64 {
	value := int((1 - age/rexShockwaveLifetime) * 255 * 5)
	if value < 0 {
		value = 0
	}
	if value > 255 {
		value = 255
	}
	return float64(value) / 255
}

// rexShockwaveHitsZombie is the zombie/boss ellipse test of FUN_000a5f74.
func rexShockwaveHitsZombie(waveX, waveY, size, x, y, targetSize float64) bool {
	dx := x - waveX
	dy := (y - waveY) / rexShockwaveAspect
	reach := size + targetSize*rexShockwaveTargetFactor
	return dx*dx+dy*dy < reach*reach
}

// rexShockwaveHitsPlayer is the player ellipse test of FUN_000a5f74.
func rexShockwaveHitsPlayer(waveX, waveY, size, x, y float64) bool {
	dx := x - waveX
	dy := (y - waveY) / rexShockwaveAspect
	reach := entityGridPlayerBody*rexShockwaveTargetFactor + size*float64(rexShockwaveHalf)
	return dx*dx+dy*dy < reach*reach
}

func (p *playState) rexBoss(scriptID int) *rexBossState {
	if p.rex.bosses == nil {
		return nil
	}
	return p.rex.bosses[scriptID]
}

// rexHeld reports whether the rex stands still: rage windup or mid-leap.
func (p *playState) rexHeld(zombie *zombieState) bool {
	if zombie.rexRageTimer > 0 {
		return true
	}
	state := p.rexBoss(zombie.scriptID)
	return state != nil && (state.raged || state.leaping)
}

// rexLeaping is the native +0x344 flag; MakeRexRage ignores a leaping rex.
func (p *playState) rexLeaping(scriptID int) bool {
	state := p.rexBoss(scriptID)
	return state != nil && state.leaping
}

// rexLift is how far the leap raises the sprite: height * render size.
func (p *playState) rexLift(zombie zombieState) float64 {
	if zombie.lift > 0 {
		return zombie.lift
	}
	state := p.rexBoss(zombie.scriptID)
	if state == nil || !state.leaping {
		return 0
	}
	return math.Max(0, state.height) * zombie.size.Y
}

func (p *playState) isRexBoss(zombie *zombieState) bool {
	entity := p.scriptEntities[zombie.scriptID]
	return entity != nil && entity.entityType == "boss_rex"
}

// updateRexBoss runs once per tick after the zombies moved.
func (p *playState) updateRexBoss() {
	const dt = 1.0 / 60.0
	p.updateRexShockwaves()
	p.updateRexVenom()
	if p.cheats.freezeZombies {
		return
	}
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if !p.isRexBoss(zombie) {
			continue
		}
		if zombie.dying || zombie.spawnAway || zombie.health <= 0 {
			if p.rex.bosses[zombie.scriptID] != nil && zombie.dying && !p.isPresidentBoss(zombie) {
				// FUN_000b9d2c (vtable +0x50, the death hook): sound id 0x2c, T_rex_death.ogg.
				// (The president's clip for this id is UNRESOLVED.)
				p.sfxQueue = append(p.sfxQueue, rexDeathSound)
			}
			delete(p.rex.bosses, zombie.scriptID)
			continue
		}
		if p.rex.bosses == nil {
			p.rex.bosses = map[int]*rexBossState{}
		}
		state := p.rex.bosses[zombie.scriptID]
		if state == nil {
			// +0x338 starts at 0 (FUN_000ba174), so no threshold can fire before the
			// first frame in state 3 with the gate open stores the health fraction.
			state = &rexBossState{maxHealth: zombie.health, gravity: rexLeapGravity, ai: p.newRexAI()}
			p.rex.bosses[zombie.scriptID] = state
			if p.rex.dropIn[zombie.scriptID] {
				delete(p.rex.dropIn, zombie.scriptID)
				p.rexDropIn(zombie, state)
			}
		}
		if state.maxHealth <= 0 {
			state.maxHealth = zombie.health
		}
		// FUN_0009fb10: AI and state machine, then FUN_000b9624 (rage / leap) through
		// the boss pre-update, then the shared contact pass.
		p.updateRexAI(zombie, state, dt)
		p.updateRexRage(zombie, state, dt)
		p.rexLungeRoar(zombie, state)
		p.rexContact(zombie, state, dt)
		p.rexPresentation(zombie, state)
	}
}

// updateRexRage is the rage/leap part of FUN_000b9624.
func (p *playState) updateRexRage(zombie *zombieState, state *rexBossState, dt float64) {
	ratio := zombie.health / state.maxHealth
	if !state.leaping && zombie.rexRageTimer <= 0 && state.ai.state == 3 && state.gateOpen() {
		prev := state.lastRatio
		if (prev > rexRageThreshold && ratio <= rexRageThreshold) || (prev > rexRageSecond && ratio <= rexRageSecond) {
			// 0x000b9a1c: +0x338 = health / max, roar, timer = 1000.
			state.lastRatio = ratio
			zombie.rexRageTimer = rexRageMillis
			p.sfxQueue = append(p.sfxQueue, p.rexRoar(zombie))
		}
	}
	if zombie.rexRageTimer > 0 {
		state.raged = true
		return // the timer branch returns before the leap integration
	}
	if state.raged && !state.leaping {
		state.raged, state.leaping = false, true
		state.height, state.velocity, state.gravity = 0, rexLeapVelocity, rexLeapGravity
	}
	if state.leaping {
		if state.velocity < 0 {
			state.gravity += dt * rexLeapFallAccel
		}
		state.velocity -= dt * state.gravity
		state.height += state.velocity * dt
		if state.height < 0 {
			state.leaping, state.height, state.velocity = false, 0, 0
			// Landing: weapon record reload (ammo 3), shockwave, camera shake.
			state.ai.reload(rexAmmoReload)
			p.spawnRexShockwave(zombie.x, zombie.y, zombie.scriptID, rexShockwaveScale*zombie.size.X)
		}
	}
	if state.ai.state == 3 && state.gateOpen() {
		state.lastRatio = ratio
	}
}

// rexPresentation selects the sprite clip: Charge during the lunge
// (FUN_000b9624 0x000b9654), otherwise the default walk clip.
func (p *playState) rexPresentation(zombie *zombieState, state *rexBossState) {
	want := ""
	if state.ai.state == 5 {
		want = "Charge"
	}
	if zombie.animation != want {
		zombie.animation = want
		zombie.fps = p.spriteFPS(zombie.texture, want)
	}
}

// rexDropIn is FUN_000ba094, run from the spawn reset FUN_000ba174: the rex is
// moved to a random point 80..100 px from the player with a clear line and a free
// body (FUN_000bfdc8), and starts airborne at height 10 with the leap flag set,
// so every boss_rex lands with a shockwave next to the player.
func (p *playState) rexDropIn(zombie *zombieState, state *rexBossState) {
	prey := p.nearestPlayer(zombie.x, zombie.y)
	lo, hi := rexDropDistanceMin, rexDropDistanceMax
	for attempt := 0; attempt <= 200; attempt++ {
		distance := lo + float64(zombieRandom(p.rng, float32(hi-lo)))
		angle := float64(zombieBounded(p.rng, 0xFFF0)) * 2 * math.Pi / 65536
		x, y := prey.x-math.Cos(angle)*distance, prey.y-math.Sin(angle)*distance
		free := true
		if p.world != nil && p.tileSize > 0 {
			free = p.lineClear(x, y, prey.x, prey.y, 0)
			if free {
				_, _, hit := p.collisionDisplacement(x, y, zombieCollisionRadius(*zombie), p.tileSize)
				free = !hit
			}
		}
		if attempt > 100 {
			lo, hi = lo*0.9, hi*0.9 // DAT_000bff74 shrinks the ring after 100 misses
		}
		if free || attempt == 200 {
			zombie.x, zombie.y = x, y
			break
		}
	}
	if entity := p.scriptEntities[zombie.scriptID]; entity != nil {
		entity.x, entity.y = zombie.x, zombie.y
	}
	state.leaping, state.height, state.velocity, state.gravity = true, rexDropHeight, 0, rexDropGravity
}

// markRexSpawn queues the drop-in of a freshly spawned boss_rex.
func (p *playState) markRexSpawn(scriptID int, entityType string) {
	if entityType != "boss_rex" {
		return
	}
	if p.rex.dropIn == nil {
		p.rex.dropIn = map[int]bool{}
	}
	p.rex.dropIn[scriptID] = true
}

func (p *playState) spawnRexShockwave(x, y float64, owner int, max ...float64) {
	wave := rexShockwave{x: x, y: y, shot: p.newShot(), owner: owner}
	if len(max) > 0 {
		wave.max = max[0]
	}
	p.rex.waves = append(p.rex.waves, wave)
	p.sfxQueue = append(p.sfxQueue, rexShockwaveSound)
	p.startCameraShake(x, y, rexShakeDuration, rexShakeAmpX, rexShakeAmpY, rexShakeAngleMul)
}

// rexShockwaveInWall is the removal test of FUN_000a5f74: the tile under the
// wave centre, (int)(x/32), (int)(y/32), equals 1.
func (p *playState) rexShockwaveInWall(x, y float64) bool {
	return p.projectileBlockedAt(x, y)
}

// updateRexShockwaves ages each wave, then hits whatever it overlaps.
func (p *playState) updateRexShockwaves() {
	const dt = 1.0 / 60.0
	p.refreshEntityGrid()
	live := p.rex.waves[:0]
	for _, wave := range p.rex.waves {
		wave.age += dt
		if wave.age >= rexShockwaveLifetime || p.rexShockwaveInWall(wave.x, wave.y) {
			continue
		}
		p.rexShockwaveHit(wave)
		live = append(live, wave)
	}
	p.rex.waves = live
}

func (p *playState) rexShockwaveHit(wave rexShockwave) {
	size := wave.size()
	origin := killOrigin{gun: "TREX", shot: wave.shot}
	for _, target := range p.livingPlayers() {
		if !p.scenePlaying() || !rexShockwaveHitsPlayer(wave.x, wave.y, size, target.x, target.y) {
			continue
		}
		// FUN_000a6ed4: FUN_00094c6c(player, 0.025, wave) and a velocity add of
		// 0.5 * unit(player - wave) to player+0x1c. The velocity add has no effect:
		// the player update FUN_00096818 rewrites +0x1c from the controls every
		// frame (0x00096bfc) before it is read, and nothing else consumes it.
		p.damagePlayer(target.index, rexShockwavePlayerDamage)
	}
	query := float32(size) * rexShockwaveHalf
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if zombie.dying || zombie.spawnAway || zombie.health <= 0 || zombie.invulnerable || (wave.owner != 0 && zombie.scriptID == wave.owner) {
			continue
		}
		if !rexShockwaveHitsZombie(wave.x, wave.y, size, zombie.x, zombie.y, zombie.size.X) {
			continue
		}
		visits := p.zombieGridVisits(zombie, wave.x, wave.y, query)
		if visits == 0 {
			continue
		}
		zombie.health -= rexShockwaveZombieDamage * float64(visits)
		zombie.hitFlash = zombieDamageFlash
		if zombie.health <= 0 {
			p.creditKill(origin)
			zombie.dying = true
			zombie.deathAge = 0
		}
	}
}

// rexShockwaveVertices is the quad FUN_000a77d8 submits: centred on the wave,
// width = size, height = size * 0.664, faded by rexShockwaveAlpha.
func rexShockwaveVertices(age, x, y, zoom, frontendX, frontendY float64, textureWidth, textureHeight int) ([4]ebiten.Vertex, bool) {
	return rexShockwaveVerticesSized(rexShockwave{age: age}, x, y, zoom, frontendX, frontendY, textureWidth, textureHeight)
}

func rexShockwaveVerticesSized(wave rexShockwave, x, y, zoom, frontendX, frontendY float64, textureWidth, textureHeight int) ([4]ebiten.Vertex, bool) {
	age, size := wave.age, wave.size()
	if !(age >= 0 && age < rexShockwaveLifetime) || size <= 0 || textureWidth <= 0 || textureHeight <= 0 || zoom <= 0 || frontendX <= 0 || frontendY <= 0 {
		return [4]ebiten.Vertex{}, false
	}
	alpha := float32(rexShockwaveAlpha(age))
	centerX, centerY := x*frontendX, y*frontendY
	halfWidth := size / 2 * zoom * frontendX
	halfHeight := size * rexShockwaveAspect / 2 * zoom * frontendX
	x0, x1, y0, y1 := float32(centerX-halfWidth), float32(centerX+halfWidth), float32(centerY-halfHeight), float32(centerY+halfHeight)
	w, h := float32(textureWidth), float32(textureHeight)
	return [4]ebiten.Vertex{
		{DstX: x0, DstY: y0, SrcX: 0, SrcY: 0, ColorR: alpha, ColorG: alpha, ColorB: alpha, ColorA: alpha},
		{DstX: x1, DstY: y0, SrcX: w, SrcY: 0, ColorR: alpha, ColorG: alpha, ColorB: alpha, ColorA: alpha},
		{DstX: x0, DstY: y1, SrcX: 0, SrcY: h, ColorR: alpha, ColorG: alpha, ColorB: alpha, ColorA: alpha},
		{DstX: x1, DstY: y1, SrcX: w, SrcY: h, ColorR: alpha, ColorG: alpha, ColorB: alpha, ColorA: alpha},
	}, true
}

func (a *app) drawRexShockwaves(screen *ebiten.Image) {
	if screen == nil || a.play == nil || a.play.world == nil || len(a.play.rex.waves) == 0 {
		return
	}
	texture, err := a.Texture(rexShockwaveTexture)
	if err != nil {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	frontendX, frontendY := a.renderScale()
	for _, wave := range a.play.rex.waves {
		x, y := (wave.x-world.CameraX)*zoom+world.ViewportX, (wave.y-world.CameraY)*zoom+world.ViewportY
		vertices, visible := rexShockwaveVerticesSized(wave, x, y, zoom, frontendX, frontendY, texture.Bounds().Dx(), texture.Bounds().Dy())
		if !visible {
			continue
		}
		screen.DrawTriangles(vertices[:], []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterLinear, DisableMipmaps: true})
	}
}

// captureRexShockwave is the "play-rex-shockwave" capture state: World0Level2
// with the intro and waves cleared, a rex about to leap and a crowd beside it.
func (a *app) captureRexShockwave() error {
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
	add := func(kind, texture string, x, y, speed, health, size float64) {
		id := p.scriptNextEntity
		p.scriptNextEntity++
		p.scriptEntities[id] = &scriptEntity{id: id, kind: "zombie", entityType: kind, x: x, y: y, scaleX: 1, scaleY: 1, alpha: 1, texture: texture, speed: speed}
		p.zombies = append(p.zombies, zombieState{x: x, y: y, speed: speed, health: health, rawPoints: 100, size: formats.Vec2{X: size, Y: size}, texture: texture, scriptID: id, alpha: 1, fps: p.spriteFPS(texture, "")})
	}
	add("boss_rex", "rexwalk_1", p.x+170, p.y, 170, 25000, nativeSpawnRenderSize(formats.Vec2{X: 80, Y: 80}))
	p.zombies[0].rexRageTimer = rexRageMillis
	for i := 0; i < 8; i++ {
		add("zombie", "cavezombie", p.x+120+float64(i%4)*26, p.y-60+float64(i/4)*120, 0, 100, 48)
	}
	return nil
}
