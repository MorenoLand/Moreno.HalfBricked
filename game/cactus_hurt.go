package game

import (
	"math"
	"strings"
)

// PORT ADDITION (-cactus-hurt, on by default): the cacti of the western levels prick whoever walks into them. The
// original's cactuses are scenery with a collision tile and do no damage. A cactus prop (texture "cactuses", 64x128,
// its base on the prop's own y) hurts the player while he stands against its base, at cactusHurtRate health per
// second through the normal damage path (so the 1.2.5 halving, the regeneration delay and the hurt sound apply).
var portCactusHurt = true

const (
	cactusHurtRate   = .15 // health per second while touching (twice the plain zombie's 0.075)
	cactusReachX     = 34.0
	cactusReachY     = 40.0
	cactusBaseAnchor = 16.0 // the contact point sits this far above the prop's y (the middle of the base tile)
)

type cactusPoint struct{ x, y float64 }

// cactusPoints lists the cactus bases of the current level, once per level.
func (p *playState) cactusPoints() []cactusPoint {
	if p.world == nil {
		return nil
	}
	if p.cactusWorld == p.world {
		return p.cactusList
	}
	p.cactusWorld, p.cactusList = p.world, nil
	for _, prop := range p.world.Level.Props {
		if strings.Contains(strings.ToLower(prop.Texture), "cactus") {
			p.cactusList = append(p.cactusList, cactusPoint{x: float64(int(prop.X)), y: float64(int(prop.Y)) - cactusBaseAnchor})
		}
	}
	return p.cactusList
}

// cactusTouched reports whether the body at (x, y) stands against a cactus base.
func (p *playState) cactusTouched(x, y float64) bool {
	for _, c := range p.cactusPoints() {
		dx, dy := (x-c.x)/cactusReachX, (y-c.y)/cactusReachY
		if dx*dx+dy*dy < 1 {
			return true
		}
	}
	return false
}

// updateCactusHurt runs once per frame for the primary body.
func (p *playState) updateCactusHurt(dt float64) {
	if !portCactusHurt || p.health <= 0 || p.playerUnspawned || p.paused {
		return
	}
	if p.cactusTouched(p.x, p.y) {
		p.damagePlayer(0, math.Max(0, dt)*cactusHurtRate)
	}
}
