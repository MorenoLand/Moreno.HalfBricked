package game

import "math"

// PORT ADDITION (not in the original binary), switchable with -sentry-solid=false: a placed sentry is a solid body.
// Natively nothing collides with it (the player update only collides with the level, and the zombie separation pass
// only knows zombies and players), so Barry and the zombies walked straight through it. Here the player and the
// zombies are pushed back out of its footprint, the way a zombie is kept out of another zombie.
var portSentrySolid = true

// sentryBodyRadius is the sentry's footprint radius in world pixels (its stand is drawn 40 px wide).
const sentryBodyRadius = 14.0

// sentryCollisionDisplacement is the push that moves a body of the given radius at (x, y) out of every standing sentry.
func (p *playState) sentryCollisionDisplacement(x, y, radius float64) (float64, float64, bool) {
	if !portSentrySolid {
		return 0, 0, false
	}
	var pushX, pushY float64
	hit := false
	for index := range p.sentries {
		s := &p.sentries[index]
		if s.state == sentryStateDying {
			continue
		}
		dx, dy := x-s.x, y-s.y
		reach := radius + sentryBodyRadius
		dist := math.Hypot(dx, dy)
		if dist >= reach {
			continue
		}
		if dist < .001 {
			dx, dy, dist = 1, 0, 1
		}
		overlap := reach - dist
		pushX += dx / dist * overlap
		pushY += dy / dist * overlap
		hit = true
	}
	return pushX, pushY, hit
}
