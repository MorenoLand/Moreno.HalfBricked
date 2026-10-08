package game

import (
	"fmt"
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

	rexRageMillis    = 1000.0
	rexLeapVelocity  = 3.5
	rexLeapGravity   = 6.0
	rexLeapFallAccel = 15.0
	rexRageThreshold = 0.66
	rexRageSecond    = 0.33

	// The rex constructor/reset FUN_000ba174 sets field +0x2f4 to 3 and the health
	// threshold rage is only evaluated when FUN_000b94c4 (vtable +0x70) reports
	// +0x2f4 <= 0 and the rex is not leaping. No code was found that lowers the
	// field for the rex (it is the ammo/burst word of the copied zombie weapon
	// record; only the armed-zombie fire path touches it), so the health rage never
	// opens and only the script's MakeRexRage starts a leap.
	rexInitialGate = 3

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
}

type rexBossState struct {
	maxHealth, lastRatio      float64
	height, velocity, gravity float64
	raged, leaping            bool
	gate                      int // native +0x2f4
}

type rexBossField struct {
	bosses map[int]*rexBossState
	waves  []rexShockwave
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
	if p.cheats.freezeZombies {
		return
	}
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if !p.isRexBoss(zombie) {
			continue
		}
		if zombie.dying || zombie.spawnAway || zombie.health <= 0 {
			delete(p.rex.bosses, zombie.scriptID)
			continue
		}
		if p.rex.bosses == nil {
			p.rex.bosses = map[int]*rexBossState{}
		}
		state := p.rex.bosses[zombie.scriptID]
		if state == nil {
			state = &rexBossState{maxHealth: zombie.health, lastRatio: 1, gravity: rexLeapGravity, gate: rexInitialGate}
			p.rex.bosses[zombie.scriptID] = state
		}
		if state.maxHealth <= 0 {
			state.maxHealth = zombie.health
		}
		ratio := zombie.health / state.maxHealth
		if !state.leaping && zombie.rexRageTimer <= 0 && state.gate <= 0 {
			// Native gate (vtable +0x70, FUN_000b94c4): +0x2f4 <= 0 and not leaping.
			if (state.lastRatio > rexRageThreshold && ratio <= rexRageThreshold) || (state.lastRatio > rexRageSecond && ratio <= rexRageSecond) {
				zombie.rexRageTimer = rexRageMillis
				p.sfxQueue = append(p.sfxQueue, p.rexRoar())
			}
		}
		state.lastRatio = ratio
		if zombie.rexRageTimer > 0 {
			state.raged = true
		} else if state.raged && !state.leaping {
			state.raged, state.leaping = false, true
			state.height, state.velocity, state.gravity = 0, rexLeapVelocity, rexLeapGravity
		}
		if !state.leaping {
			continue
		}
		if state.velocity < 0 {
			state.gravity += dt * rexLeapFallAccel
		}
		state.velocity -= dt * state.gravity
		state.height += state.velocity * dt
		if state.height < 0 {
			state.leaping, state.height, state.velocity = false, 0, 0
			p.spawnRexShockwave(zombie.x, zombie.y, zombie.scriptID)
		}
	}
}

// rexRoar is the roar the rage start plays: sound id 0x2a plus the top bit of
// a 32-bit RNG draw (0x000b9a34.., ids 0x2a/0x2b = T_REX_ROAR_1/2).
func (p *playState) rexRoar() string {
	var draw uint32
	if p.rng != nil {
		draw = p.rng.Bounded(0)
	}
	return fmt.Sprintf("SFX_T_REX_ROAR_%d", 1+int(draw>>31))
}

func (p *playState) spawnRexShockwave(x, y float64, owner int) {
	p.rex.waves = append(p.rex.waves, rexShockwave{x: x, y: y, shot: p.newShot(), owner: owner})
	p.sfxQueue = append(p.sfxQueue, rexShockwaveSound)
	p.startCameraShake(x, y, rexShakeDuration, rexShakeAmpX, rexShakeAmpY, rexShakeAngleMul)
}

// rexShockwaveInWall is the removal test of FUN_000a5f74: the tile under the
// wave centre, (int)(x/32), (int)(y/32), equals 1.
func (p *playState) rexShockwaveInWall(x, y float64) bool {
	return p.collisionValue(int(float32(x)*(1.0/32)), int(float32(y)*(1.0/32))) == 1
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
	size := rexShockwaveSize(wave.age)
	origin := killOrigin{gun: "TREX", shot: wave.shot}
	for _, target := range p.livingPlayers() {
		if !rexShockwaveHitsPlayer(wave.x, wave.y, size, target.x, target.y) {
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
		zombie.hitFlash = zombieHitFlashDuration
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
	size := rexShockwaveSize(age)
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
		vertices, visible := rexShockwaveVertices(wave.age, x, y, zoom, frontendX, frontendY, texture.Bounds().Dx(), texture.Bounds().Dy())
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
