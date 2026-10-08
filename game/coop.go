package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Local co-op. Native evidence (1.2.5): co-op is offered with two or more attached
// gamepads, or with two-point touch and no gamepad (FUN_000c3b8c). It requests
// (players=gamepads, shared screen), or (2, split screen) for touch. Players are
// indexed 0..3, each with two input channels (move and aim) and its own colour.
//
// Player 0 is the existing playState player. Players 1..3 are coopPlayers whose
// per-body state is swapped into the playState while they act, so the existing
// weapon, secondary-slot, pickup and drawing code works for them unchanged.
const maxCoopPlayers = 4

// coopAvailable follows the native availability test q>1 || (t>=2 && q==0).
// desktopPad is a port extension: on desktop one gamepad pairs with the
// keyboard and mouse for player 1.
func coopAvailable(gamepads, touchPoints int, desktopPad bool) bool {
	return gamepads > 1 || (touchPoints >= 2 && gamepads == 0) || (desktopPad && gamepads == 1)
}

// coopRequest is the (player count, split screen) the Co-op button asks for.
func coopRequest(gamepads, touchPoints int, desktopPad bool) (players int, split bool) {
	switch {
	case gamepads > 1:
		return min(gamepads, maxCoopPlayers), false
	case gamepads == 1 && desktopPad:
		return 2, false
	}
	return 2, true
}

// playerInput is one player's controls for a frame, regardless of device.
type playerInput struct {
	moveX, moveY float64
	aimX, aimY   float64
	secondary    bool
	secondaryHit bool // pressed this frame
}

func (in playerInput) aiming() bool { return math.Hypot(in.aimX, in.aimY) > .5 }

// bodyState is everything that belongs to one player's character.
type bodyState struct {
	x, y, spawnX, spawnY   float64
	angle                  int
	flipX                  bool
	health, maxHealth      float64
	weapon                 formats.Weapon
	shootCooldown          float64
	secondaryShootCooldown float64
	flash                  float64
	deathTimer             float64
	grenades               int
	secondaryType          string
	secondaryWeapon        string
	moving                 bool
	hurt                   float64 // seconds left on the red damage flash
	gun                    gunState
}

func (p *playState) body() bodyState {
	return bodyState{x: p.x, y: p.y, spawnX: p.spawnX, spawnY: p.spawnY, angle: p.angle, flipX: p.flipX, health: p.health, maxHealth: p.maxHealth, weapon: p.weapon, shootCooldown: p.shootCooldown, secondaryShootCooldown: p.secondaryShootCooldown, flash: p.flash, deathTimer: p.deathTimer, grenades: p.grenades, secondaryType: p.secondaryType, secondaryWeapon: p.secondaryWeapon, moving: p.moving, hurt: p.hurt, gun: p.gun}
}

func (p *playState) setBody(b bodyState) {
	p.x, p.y, p.spawnX, p.spawnY, p.angle, p.flipX = b.x, b.y, b.spawnX, b.spawnY, b.angle, b.flipX
	p.health, p.maxHealth, p.weapon = b.health, b.maxHealth, b.weapon
	p.shootCooldown, p.secondaryShootCooldown, p.flash, p.deathTimer = b.shootCooldown, b.secondaryShootCooldown, b.flash, b.deathTimer
	p.grenades, p.secondaryType, p.secondaryWeapon, p.moving, p.hurt = b.grenades, b.secondaryType, b.secondaryWeapon, b.moving, b.hurt
	p.gun = b.gun
}

type coopPlayer struct {
	index   int
	body    bodyState
	joined  bool
	respawn float64 // seconds until a dead player returns
	dead    bool
	// canReturn is set at death when a shared life paid for a respawn.
	canReturn bool
	// spawnAge is the seconds since the player stepped out of their portal.
	spawnAge float64
}

type coopState struct {
	players []*coopPlayer // players 1..3
	split   bool
}

func (p *playState) coopActive() bool { return p.coop != nil && len(p.coop.players) > 0 }

// startCoop adds players 1..count-1. They wait (hidden) until the entry script
// has finished, then appear beside player 0.
func (p *playState) startCoop(count int, split bool) {
	count = max(2, min(count, maxCoopPlayers))
	state := &coopState{split: split}
	for index := 1; index < count; index++ {
		state.players = append(state.players, &coopPlayer{index: index, body: bodyState{maxHealth: p.maxHealth, health: p.maxHealth, weapon: p.pistolWeapon(), angle: p.angle}})
	}
	p.coop = state
}

// ensureCoopPlayers grows the roster to count players (player 1 included).
func (p *playState) ensureCoopPlayers(count int) {
	count = min(count, maxCoopPlayers)
	if p.coop == nil {
		p.startCoop(count, false)
		return
	}
	for index := len(p.coop.players) + 1; index < count; index++ {
		p.coop.players = append(p.coop.players, &coopPlayer{index: index, body: bodyState{maxHealth: p.maxHealth, health: p.maxHealth, weapon: p.pistolWeapon(), angle: p.angle}})
	}
}

func (p *playState) pistolWeapon() formats.Weapon {
	if pistol, ok := p.weapons.Find("PISTOL"); ok {
		return pistol
	}
	return p.weapon
}

// withPlayer runs fn with the coop player's body swapped into the play state.
func (p *playState) withPlayer(c *coopPlayer, fn func()) {
	saved := p.body()
	p.setBody(c.body)
	fn()
	c.body = p.body()
	p.setBody(saved)
}

// playerTarget is a living player a zombie can chase or hurt.
type playerTarget struct {
	index int
	x, y  float64
}

func (p *playState) livingPlayers() []playerTarget {
	var list []playerTarget
	if p.health > 0 {
		list = append(list, playerTarget{index: 0, x: p.x, y: p.y})
	}
	if p.coopActive() {
		for _, c := range p.coop.players {
			if c.joined && !c.dead && c.body.health > 0 {
				list = append(list, playerTarget{index: c.index, x: c.body.x, y: c.body.y})
			}
		}
	}
	return list
}

// nearestPlayer returns the closest living player, falling back to player 0.
func (p *playState) nearestPlayer(x, y float64) playerTarget {
	best, bestDistance := playerTarget{index: 0, x: p.x, y: p.y}, math.Inf(1)
	for _, target := range p.livingPlayers() {
		if distance := math.Hypot(target.x-x, target.y-y); distance < bestDistance {
			best, bestDistance = target, distance
		}
	}
	return best
}

// The original game has no recovered damage indicator for the player, so the
// port flashes Barry red while he is being hurt (the same cue zombies get).
const hurtFlashDuration = .35

// hurtTint fades Barry from red back to his normal colour as the flash runs
// out; while a zombie keeps hitting him it stays red. A zero base means normal.
func hurtTint(hurt, clock float64, base [3]float32) [3]float32 {
	if hurt <= 0 {
		return base
	}
	t := float32(min(hurt/hurtFlashDuration, 1))
	normal := [3]float32{1, 1, 1}
	if base != ([3]float32{}) {
		normal = base
	}
	red := [3]float32{1.6, .35, .35}
	var out [3]float32
	for i := range out {
		out[i] = normal[i] + (red[i]-normal[i])*t
	}
	return out
}

// damagePlayer applies contact damage to the given player.
func (p *playState) damagePlayer(index int, amount float64) {
	if amount <= 0 || p.cheats.god || (index == 0 && p.shielded()) {
		return
	}
	if index == 0 {
		p.health = math.Max(0, p.health-amount)
		p.hurt = hurtFlashDuration
		return
	}
	for _, c := range p.coop.players {
		if c.index == index {
			c.body.health = math.Max(0, c.body.health-amount)
			c.body.hurt = hurtFlashDuration
		}
	}
}

// cameraFocus is the point the camera follows: the centre of the living players.
func (p *playState) cameraFocus() (float64, float64) {
	if !p.coopActive() {
		return p.x, p.y
	}
	living := p.livingPlayers()
	if len(living) == 0 {
		return p.x, p.y
	}
	var sumX, sumY float64
	for _, target := range living {
		sumX, sumY = sumX+target.x, sumY+target.y
	}
	return sumX / float64(len(living)), sumY / float64(len(living))
}

// allPlayersDown is true when every player is dead for good.
func (p *playState) allPlayersDown() bool {
	if p.health > 0 {
		return false
	}
	if p.coopActive() {
		for _, c := range p.coop.players {
			if c.joined && (!c.dead || c.canReturn) {
				return false
			}
		}
	}
	return true
}

// clampToView keeps co-op players on the shared screen.
func (p *playState) clampToView(x, y *float64, margin float64) {
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	*x = math.Max(p.world.CameraX+margin, math.Min(p.world.CameraX+float64(logicalWidth)/zoom-margin, *x))
	*y = math.Max(p.world.CameraY+margin, math.Min(p.world.CameraY+float64(logicalHeight)/zoom-margin, *y))
}

const coopRespawnDelay = 2.0

// updateCoopPlayers advances players 1..3 one frame from their inputs.
func (p *playState) updateCoopPlayers(inputs []playerInput) {
	if !p.coopActive() || p.world == nil {
		return
	}
	scriptActive := p.scriptRuntime != nil && !p.scriptRuntime.Done()
	for _, c := range p.coop.players {
		var input playerInput
		if c.index-1 < len(inputs) {
			input = inputs[c.index-1]
		}
		if !c.joined {
			if scriptActive || p.health <= 0 {
				continue
			}
			p.joinCoopPlayer(c)
		}
		p.updateCoopPlayer(c, input, scriptActive)
	}
}

// joinCoopPlayer places the player beside player 0 (or a living teammate).
func (p *playState) joinCoopPlayer(c *coopPlayer) {
	anchorX, anchorY := p.x, p.y
	c.body.x, c.body.y = anchorX+float64(34*c.index), anchorY
	c.body.spawnX, c.body.spawnY = c.body.x, c.body.y
	c.body.health, c.body.maxHealth = p.maxHealth, p.maxHealth
	c.body.weapon = p.pistolWeapon()
	c.joined, c.dead = true, false
	c.spawnAge = 0
	p.addPortal(c.body.x, c.body.y)
	p.resolveBody(&c.body.x, &c.body.y)
}

func (p *playState) resolveBody(x, y *float64) {
	radius := p.radius
	if radius <= 0 {
		radius = playerCollisionRadius
	}
	for resolve := 0; resolve < 4; resolve++ {
		pushX, pushY, hit := p.collisionDisplacement(*x, *y, radius, p.tileSize)
		if !hit {
			break
		}
		*x += pushX
		*y += pushY
	}
}

func (p *playState) updateCoopPlayer(c *coopPlayer, input playerInput, scriptActive bool) {
	const dt = 1.0 / 60.0
	// Teammate positions are read before this body is swapped in.
	teammates := p.livingPlayersExcluding(c.index)
	c.spawnAge = math.Min(c.spawnAge+dt, 10)
	p.withPlayer(c, func() {
		if p.health <= 0 {
			p.coopDeath(c, teammates)
			return
		}
		p.shootCooldown = math.Max(0, p.shootCooldown-dt)
		p.secondaryShootCooldown = math.Max(0, p.secondaryShootCooldown-dt)
		p.flash = math.Max(0, p.flash-dt)
		if scriptActive && p.secondaryPlayersWait {
			p.moving = false
			return
		}
		dx, dy := input.moveX, input.moveY
		if length := math.Hypot(dx, dy); length > 1 {
			dx, dy = dx/length, dy/length
		}
		if length := math.Hypot(dx, dy); length < .25 {
			dx, dy = 0, 0
		}
		p.stepBody(dx, dy, false, false, false)
		if input.aiming() {
			p.angle, p.flipX = barryDirection(input.aimX, input.aimY)
			if p.fireGateOpen() {
				p.fire(input.aimX, input.aimY)
			}
		} else if p.moving {
			p.angle, p.flipX = barryDirection(dx, dy)
		}
		p.tickGun(false)
		if input.secondaryHit && p.grenades > 0 && p.secondaryShootCooldown <= 0 {
			aimX, aimY := barryAimDirection(p.angle, p.flipX)
			p.fireSecondary(aimX, aimY)
		}
		p.clampToView(&p.x, &p.y, 12)
	})
}

// coopDeath runs while a coop player's health is zero: it spends a shared life
// and returns the player beside a living teammate, or leaves them down.
func (p *playState) coopDeath(c *coopPlayer, living []playerTarget) {
	const dt = 1.0 / 60.0
	if !c.dead {
		c.dead = true
		c.respawn = coopRespawnDelay
		c.canReturn = p.lives > 0
		if c.canReturn {
			p.lives--
		}
		p.bloodPops = append(p.bloodPops, bloodPop{x: p.x, y: p.y, variant: len(p.bloodPops) % 3})
		return
	}
	if !c.canReturn {
		return
	}
	if c.respawn > 0 {
		c.respawn -= dt
		return
	}
	if len(living) == 0 {
		return
	}
	p.x, p.y = living[0].x+20, living[0].y
	p.health = p.maxHealth
	p.weapon = p.pistolWeapon()
	p.grenades, p.secondaryType = 0, ""
	c.dead, c.canReturn = false, false
	c.spawnAge = 0
	p.addPortal(p.x, p.y)
	p.resolveBody(&p.x, &p.y)
}

func (p *playState) livingPlayersExcluding(index int) []playerTarget {
	var list []playerTarget
	for _, target := range p.livingPlayers() {
		if target.index != index {
			list = append(list, target)
		}
	}
	return list
}
