package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// Exploding zombie (spawn type "exploding_zombie", native entity type 4).
//
// Evidence is itemised in Research/native/exploding-zombie-2026-10-08.md and
// hazards-followup-2026-10-08.md. Everything marked CONFIRMED below was read from
// libmortargame.so(v7). Not ported (UNRESOLVED): the gib death state, whose
// particle spawner and animation tables were not decoded, and the burning child.

const (
	explodingZombieType = "exploding_zombie"

	// CONFIRMED: when the zombie's death transition runs (FUN_000a10fc, hit timer
	// <= 0) its slot 20 (FUN_0009f210) creates a type 0x15 projectile (the MINE
	// class) owned by the zombie and detonates it at the zombie's position. That
	// projectile is created with both hit flags set, so the blast hurts zombies
	// and the player.
	//
	// CONFIRMED: the type-4 vtable's slot 13 (FUN_0009f1c0) subtracts a second
	// frame time from the hit timer while the zombie is dying, so the shared
	// 0.1 s death delay (DAT_000a058c) runs at double speed.
	explodingDeathDelay = zombieDeathDelay / 2

	// CONFIRMED (FUN_000a5bb4 state 1, constants at 0x000a5e34): the blast radius
	// field is 160*(1 - age/.75*.5) while age < .15 s and 0 afterwards; the blast
	// state ends once age exceeds .75 s.
	zombieBlastRadius       = 160.0
	zombieBlastRadiusWindow = .15
	zombieBlastDuration     = .75

	// CONFIRMED (FUN_000a4cac): each tick a zombie overlaps the blast it takes 5
	// damage; the player takes frame-time * 3.0 health (DAT_000a4db4 = 3.0f).
	zombieBlastDamage       = 5
	zombieBlastPlayerPerSec = 3.0

	// CONFIRMED (FUN_000a5f74, constants at 0x000a63cc): overlap is an ellipse
	// test dx^2 + (dy/(2/3))^2 < r^2. Zombies: r = radius + .3*body width. The
	// player: r = .3*body width + .5*radius. The zombie broad phase queries the
	// entity grid around +-(.5*radius).
	zombieBlastYSquash     = 0x3f2a7efa // float32 bits of 0.6667
	zombieBlastBodyScale   = .3
	zombieBlastPlayerScale = .5
	zombieBlastQueryScale  = float32(.5)

	// CONFIRMED (see entity_grid.go): the entity grid has 16 px cells, every
	// entity is registered over +-(0.3 * 64) around its position (64 = the
	// player's +0x28 body size, player ctor FUN_00094a94), and the player-side
	// overlap test uses the same 64 px body.
	zombieBlastPlayerBody = entityGridPlayerBody
	zombieBlastBodyHalf   = zombieBlastBodyScale * zombieBlastPlayerBody

	// CONFIRMED: detonation plays sound id 8, which is "mine_explode" (FUN_000a3ca8
	// calls FUN_000ca410(8); entry 8 of the sound table at 0x005b6310 is the
	// string at 0x00563483). The visual is the shared explosion2 sheet drawn by
	// the same draw routine as the grenade (vtable slot 5 = 0x000a7d44).
	zombieBlastSound = "SFX_MINE_EXPLODE"

	// CONFIRMED: the aura sprite. FUN_000a225c draws, only while the zombie is
	// alive (+0x314 == 0) and its spawn timer +0x2c0 is >= 0, a sprite at the body
	// position (x, y - anchor*size) of 1.5x the body size, white, with alpha
	// byte = uint(70 + 50*g) (constants 70/50/500/1.5 at 0x000a23a8). The texture
	// is field +0x20 of the zombie-class texture block written by FUN_000a1748,
	// which loads "markerzombie" into +0x4 and "glow_overlay" (string 0x005623af)
	// into +0x20. g is the scene float at +0x514f8, rewritten every frame by the
	// scene update FUN_000867e4: phase16 += uint(dt * 36400.0) (literal
	// 0x00086a84 = 0x470e3000), g = sin(phase16) from the 4096-entry table
	// (FUN_001ba49c), so alpha swings 20..120 with a period of about 1.8 s.
	// The port has no spawn-rise timer (+0x2c0 ramps over 500 ms and scales the
	// alpha while the zombie emerges), so the aura is drawn at full strength.
	explodingGlowTexture = "Common0/Textures/glow_overlay"
	explodingGlowScale   = 1.5
	explodingGlowBase    = 70.0
	explodingGlowSwing   = 50.0
	explodingGlowRate    = 36400.0 // phase units per second, 65536 = one turn
)

// zombieBlast is a live "mine class" blast projectile (native entity 0x15, state
// 1): the death blast of an exploding zombie and, through spawnBlast, every
// player grenade, mine, rocket, dynamite and sentry-death explosion (blast.go).
type zombieBlast struct {
	x, y, age float64
	id        int
	// origin credits the kills; the zero value is an exploding zombie's blast.
	origin killOrigin
	// harmless means the blast was created with hit-player = 0 (FUN_000ad8e0 and
	// friends pass 0, 1: player-fired and sentry blasts never hurt the player).
	harmless bool
}

func (p *playState) isExplodingZombie(z zombieState) bool {
	if z.mirrorExploding {
		return true // set on a guest from the host's snapshot
	}
	if z.scriptID == 0 {
		return false
	}
	entity := p.scriptEntities[z.scriptID]
	return entity != nil && entity.entityType == explodingZombieType
}

// zombieDeathDelayFor is how long a killed zombie lingers before its death
// transition (and, for exploding zombies, the blast).
func (p *playState) zombieDeathDelayFor(z zombieState) float64 {
	delay := zombieDeathDelay
	if z.deathDelay > 0 {
		delay = z.deathDelay // a gib's hit timer (zombie_gib.go)
	}
	if p.isExplodingZombie(z) {
		return delay / 2 // the exploding zombie's hit timer runs at double speed (FUN_0009f1c0)
	}
	return delay
}

// explodeZombie runs the type-4 death hook: detonate a blast at the zombie. The
// blast hurts on later ticks (updateZombieBlasts), never inside the caller's
// zombie-list pass.
func (p *playState) explodeZombie(z zombieState) {
	if !z.dying || z.spawnAway || !p.isExplodingZombie(z) {
		return
	}
	p.spawnBlast(z.x, z.y, zombieBlastSound, killOrigin{gun: "EXPZOMBIE", shot: p.newShot()}, false)
}

// zombieBlastRadiusAt is FUN_000a5bb4 state 1 for a blast of the given age.
func zombieBlastRadiusAt(age float64) float64 {
	if age < zombieBlastRadiusWindow {
		return float64(float32(zombieBlastRadius) - float32(age/zombieBlastDuration)*float32(zombieBlastRadius)*.5)
	}
	return 0
}

func zombieBlastEllipse(dx, dy, reach float64) bool {
	squash := float64(math.Float32frombits(zombieBlastYSquash))
	dy /= squash
	return dx*dx+dy*dy < reach*reach
}

// zombieBlastCellHits counts how many times the native projectile pass lists
// the zombie: the pass walks every cell of the box pos +- radius*0.5 and every
// entity registered in each cell with no de-duplication and no early exit for
// this projectile class (FUN_000a5f74), so a zombie registered in several of
// the queried cells is hit once per cell (entity_grid.go).
func (p *playState) zombieBlastCellHits(b zombieBlast, radius float64, z *zombieState) int {
	return p.zombieGridVisits(z, b.x, b.y, float32(radius)*zombieBlastQueryScale)
}

// updateZombieBlasts advances every blast one frame and applies its damage.
func (p *playState) updateZombieBlasts() {
	const dt = 1.0 / 60.0
	p.refreshEntityGrid()
	active := p.zombieBlasts[:0]
	for _, blast := range p.zombieBlasts {
		blast.age += dt
		if blast.age > zombieBlastDuration {
			continue
		}
		p.applyZombieBlast(blast, dt)
		active = append(active, blast)
	}
	p.zombieBlasts = active
}

func (p *playState) applyZombieBlast(b zombieBlast, dt float64) {
	radius := zombieBlastRadiusAt(b.age)
	if !b.harmless {
		for _, target := range p.livingPlayers() {
			reach := zombieBlastBodyHalf + radius*zombieBlastPlayerScale
			if zombieBlastEllipse(target.x-b.x, target.y-b.y, reach) {
				p.damagePlayer(target.index, dt*zombieBlastPlayerPerSec)
			}
		}
	}
	origin := b.origin
	for index := range p.zombies {
		zombie := &p.zombies[index]
		if zombie.health <= 0 || zombie.dying || zombie.spawnAway || zombie.invulnerable {
			continue
		}
		reach := radius + zombieBlastBodyScale*zombie.size.X
		if !zombieBlastEllipse(zombie.x-b.x, zombie.y-b.y, reach) {
			continue
		}
		hits := p.zombieBlastCellHits(b, radius, zombie)
		// One takeDamage(5) per listing. FUN_000a4cac pays the weapon tracker
		// 0.05 for every hit that leaves the zombie alive.
		for visit := 0; visit < hits && zombie.health > 0; visit++ {
			zombie.health -= zombieBlastDamage
			zombie.hitFlash = zombieDamageFlash
			if zombie.health > 0 {
				p.comboCredit(origin, comboHitBlast)
			}
		}
		if zombie.health <= 0 {
			p.creditKill(origin)
			zombie.dying = true
			zombie.deathAge = 0
			p.classifyZombieDeath(zombie, zombieBlastKind(origin), b.x, b.y)
		}
	}
}

// explodingGlowPhase is the scene's 16-bit pulse phase after the given number
// of frames at 1/60 s: each frame adds uint(dt*36400) = 606 units.
func explodingGlowPhase(frames int) uint16 {
	frame := float32(1.0 / 60.0)
	step := int(frame * float32(explodingGlowRate))
	return uint16(frames * step)
}

// explodingGlowAlpha is the pulsing aura alpha (0..1) at play time `time`.
func explodingGlowAlpha(time float64) float64 {
	g := shakeSin(explodingGlowPhase(int(time*60 + .5)))
	return float64(uint8(uint32(explodingGlowBase+explodingGlowSwing*g))) / 255
}

// drawExplodingGlow draws the aura of a living exploding zombie over its body.
func (a *app) drawExplodingGlow(screen *ebiten.Image, zombie zombieState) {
	if a.play == nil || a.play.world == nil || zombie.dying || zombie.health <= 0 || !a.play.isExplodingZombie(zombie) {
		return
	}
	texture, err := a.Texture(explodingGlowTexture)
	if err != nil || texture.Bounds().Dx() <= 0 || texture.Bounds().Dy() <= 0 {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	frontendX, frontendY := a.renderScale()
	height := zombie.size.Y
	if height <= 0 {
		height = nativeZombieDefaultRenderSize
	}
	width := zombie.size.X
	if width <= 0 {
		width = nativeZombieDefaultRenderSize
	}
	x := (zombie.x-world.CameraX)*zoom + world.ViewportX
	y := (zombie.y-world.CameraY)*zoom + world.ViewportY - height*zombieRenderAnchor*zoom*frontendX/frontendY
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
	options.GeoM.Scale(width*explodingGlowScale*zoom*frontendX/float64(texture.Bounds().Dx()), height*explodingGlowScale*zoom*frontendX/float64(texture.Bounds().Dy()))
	options.GeoM.Translate(x, y)
	alpha := float32(explodingGlowAlpha(a.play.time))
	options.ColorScale.Scale(alpha, alpha, alpha, alpha)
	a.drawImage(screen, texture, options)
}

// captureExplodingScene stages an exploding zombie among ordinary zombies next
// to the player for -capture-state play-exploding (alive, glow visible) and
// play-exploding-blast (already dying, so the blast fires within the capture).
func (a *app) captureExplodingScene(blast bool) error {
	a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
	if err := a.openPlay(); err != nil {
		return err
	}
	p := a.play
	p.closeScript()
	p.dialogueIndex = len(p.dialogue)
	p.hudVisible, p.moveControl, p.shootControl = true, true, true
	entry := formats.SpawnType{Name: explodingZombieType, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, Texture: "cyborg"}
	p.spawnZombieAt(entry, formats.Vec2{X: p.x + 70, Y: p.y + 10})
	exploder := len(p.zombies) - 1
	for _, offset := range []formats.Vec2{{X: 55, Y: 40}, {X: 95, Y: 45}, {X: 80, Y: -25}, {X: 100, Y: 0}, {X: 60, Y: -10}} {
		p.spawnZombieAt(formats.SpawnType{Name: "zombie", Strength: 100, Size: formats.Vec2{X: 29, Y: 31}}, formats.Vec2{X: p.x + offset.X, Y: p.y + offset.Y})
	}
	for index := range p.zombies {
		p.zombies[index].speed = 0
	}
	p.portals = nil
	if blast {
		p.zombies[exploder].health, p.zombies[exploder].dying = 0, true
	}
	return nil
}
