package game

import "math"

// PORT ADDITION (not in the original binary): slow motion on the kill that ends a wave.
//
// Under the all-dead wave rule a wave ends the moment its last zombie dies. That moment starts a slow-down: the
// simulation (fixed 1/60 s steps) runs on only some of the ticks, and what is drawn is interpolated between the last
// two simulated steps so the motion stays smooth instead of stuttering. The speed dives quickly to slowMoHoldScale,
// holds while the death plays out, and eases back to full speed. The kill that ends the final wave, and so the level,
// gets a longer one. Switch it off with -slow-motion=false.
var portSlowMo = true

const (
	slowMoHoldScale = 0.16 // simulated time per real time at the bottom of the effect
	slowMoWaveSecs  = 1.1  // real seconds of the effect for an ordinary wave
	slowMoFinalSecs = 2.6  // real seconds of the effect for the last wave of a level
	slowMoDiveShare = 0.12 // part of the effect spent diving into the hold
	slowMoHoldShare = 0.55 // part of the effect, from its start, after which it eases back
)

// slowSnap is the position state interpolated while the effect runs.
type slowSnap struct {
	x, y, camX, camY float64
	zombies          [][2]float64
	bullets          [][2]float64
	shots            [][2]float64
	gibs             [][2]float64
}

// startSlowMo begins the effect for the given real duration. A longer one in progress is kept.
func (p *playState) startSlowMo(seconds float64) {
	if !portSlowMo || seconds <= p.slowMoLeft {
		return
	}
	p.slowMoLeft, p.slowMoTotal = seconds, seconds
}

func smoothStep(t float64) float64 {
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

// slowMoScale is the fraction of simulated time per real time now (1 outside the effect).
func (p *playState) slowMoScale() float64 {
	if p.slowMoLeft <= 0 || p.slowMoTotal <= 0 {
		return 1
	}
	elapsed := (p.slowMoTotal - p.slowMoLeft) / p.slowMoTotal
	switch {
	case elapsed < slowMoDiveShare:
		return 1 - (1-slowMoHoldScale)*smoothStep(elapsed/slowMoDiveShare)
	case elapsed <= slowMoHoldShare:
		return slowMoHoldScale
	}
	ease := smoothStep((elapsed - slowMoHoldShare) / (1 - slowMoHoldShare))
	return slowMoHoldScale + (1-slowMoHoldScale)*ease
}

// slowMoSkipTick reports whether this 60 Hz tick is skipped. It advances the effect in real time. When it returns
// false during the effect, a simulation step runs this tick and the position state before it is kept for the draw.
func (p *playState) slowMoSkipTick() bool {
	if p.slowMoLeft <= 0 {
		p.slowInterp = false
		return false
	}
	p.slowMoAccum += p.slowMoScale()
	p.slowMoLeft -= 1.0 / 60.0
	if p.slowMoLeft <= 0 {
		p.slowMoLeft, p.slowMoAccum, p.slowInterp = 0, 0, false
		return false
	}
	if p.slowMoAccum >= 1 {
		p.slowMoAccum--
		p.slowPrev = p.captureSlowSnap()
		p.slowInterp = true
		return false
	}
	return true
}

func positions[T any](items []T, get func(*T) (float64, float64)) [][2]float64 {
	out := make([][2]float64, len(items))
	for i := range items {
		out[i][0], out[i][1] = get(&items[i])
	}
	return out
}

func (p *playState) captureSlowSnap() slowSnap {
	s := slowSnap{
		x: p.x, y: p.y,
		zombies: positions(p.zombies, func(z *zombieState) (float64, float64) { return z.x, z.y }),
		bullets: positions(p.bullets, func(b *bullet) (float64, float64) { return b.x, b.y }),
		shots:   positions(p.zombieShots, func(z *zombieShot) (float64, float64) { return z.projectile.X, z.projectile.Y }),
		gibs:    positions(p.gibBodies, func(z *zombieState) (float64, float64) { return z.x, z.y }),
	}
	if p.world != nil {
		s.camX, s.camY = p.world.CameraX, p.world.CameraY
	}
	return s
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

// applySlowInterp moves everything drawn to the point between the last two simulated steps and returns the function
// that puts the live state back. It does nothing outside the effect.
func (p *playState) applySlowInterp() func() {
	if !p.slowInterp || p.slowMoLeft <= 0 {
		return func() {}
	}
	t := p.slowMoAccum
	prev := p.slowPrev
	live := p.captureSlowSnap()
	p.x, p.y = lerp(prev.x, live.x, t), lerp(prev.y, live.y, t)
	if p.world != nil {
		p.world.CameraX, p.world.CameraY = lerp(prev.camX, live.camX, t), lerp(prev.camY, live.camY, t)
	}
	if len(prev.zombies) == len(p.zombies) {
		for i := range p.zombies {
			p.zombies[i].x, p.zombies[i].y = lerp(prev.zombies[i][0], live.zombies[i][0], t), lerp(prev.zombies[i][1], live.zombies[i][1], t)
		}
	}
	if len(prev.bullets) == len(p.bullets) {
		for i := range p.bullets {
			p.bullets[i].x, p.bullets[i].y = lerp(prev.bullets[i][0], live.bullets[i][0], t), lerp(prev.bullets[i][1], live.bullets[i][1], t)
		}
	}
	if len(prev.shots) == len(p.zombieShots) {
		for i := range p.zombieShots {
			p.zombieShots[i].projectile.X, p.zombieShots[i].projectile.Y = lerp(prev.shots[i][0], live.shots[i][0], t), lerp(prev.shots[i][1], live.shots[i][1], t)
		}
	}
	if len(prev.gibs) == len(p.gibBodies) {
		for i := range p.gibBodies {
			p.gibBodies[i].x, p.gibBodies[i].y = lerp(prev.gibs[i][0], live.gibs[i][0], t), lerp(prev.gibs[i][1], live.gibs[i][1], t)
		}
	}
	return func() {
		p.x, p.y = live.x, live.y
		if p.world != nil {
			p.world.CameraX, p.world.CameraY = live.camX, live.camY
		}
		restore := func(n int, set func(i int, x, y float64), from [][2]float64) {
			if len(from) != n {
				return
			}
			for i := range from {
				set(i, from[i][0], from[i][1])
			}
		}
		restore(len(p.zombies), func(i int, x, y float64) { p.zombies[i].x, p.zombies[i].y = x, y }, live.zombies)
		restore(len(p.bullets), func(i int, x, y float64) { p.bullets[i].x, p.bullets[i].y = x, y }, live.bullets)
		restore(len(p.zombieShots), func(i int, x, y float64) { p.zombieShots[i].projectile.X, p.zombieShots[i].projectile.Y = x, y }, live.shots)
		restore(len(p.gibBodies), func(i int, x, y float64) { p.gibBodies[i].x, p.gibBodies[i].y = x, y }, live.gibs)
	}
}
