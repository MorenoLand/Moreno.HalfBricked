package game

import (
	"math"
	"strconv"
)

// The combo count is a pop-up, not a badge. The native live counter (comboTracker, state comboActive) sits behind
// the player and turns with him for as long as the combo is alive, which read as a permanent label stuck to Barry.
// What the player sees instead (the trackers underneath still run unchanged, so scoring and the tier words are the
// same):
//
//   - every hit or kill that raises the count pops "x<N>" at the spot where it happened, with a quick size pulse;
//   - while the count is catching up to a jump it counts up, a few numbers per frame;
//   - it holds still in the world (it does not follow Barry or his facing), drifts up a little, then fades away.
const (
	killPopHold     = 0.85 // seconds the number stays after the last increase
	killPopFade     = 0.4  // seconds it takes to fade out
	killPopRise     = 16.0 // world pixels it drifts upwards over its life
	killPopLift     = 52.0 // world pixels above Barry's position it appears at
	killPopCountSec = 60.0 // numbers per second while counting up
	killPopBaseSize = 22.0
	killPopPulseAdd = 0.55 // extra size right after an increase, decaying
)

// killPopState lives in comboSystem.
type killPopState struct {
	last   int     // the shared count seen last frame
	shown  float64 // the number being displayed (counts up to the shared count)
	age    float64 // seconds since the count last rose
	wx, wy float64 // world anchor
	pulse  float64
	active bool
}

// updateKillPop advances the pop-up from the shared combo count. The host only.
func (p *playState) updateKillPop(dt float64) {
	pop := &p.combo.kill
	shared := p.combo.sharedHits()
	if shared > pop.last {
		if !pop.active || pop.age > killPopHold {
			pop.shown = math.Max(0, float64(pop.last))
		}
		pop.active, pop.age, pop.pulse = true, 0, 1
		pop.wx, pop.wy = p.x, p.y-killPopLift
	}
	pop.last = shared
	if !pop.active {
		return
	}
	pop.age += dt
	pop.pulse *= math.Pow(.84, dt*60)
	if target := float64(shared); pop.shown < target {
		pop.shown = math.Min(target, pop.shown+killPopCountSec*dt)
		pop.age = math.Min(pop.age, killPopHold*.5) // keep it up while it is still counting
	}
	if pop.age > killPopHold+killPopFade {
		pop.active, pop.shown = false, 0
	}
}

// killPopPopup is the drawable pop-up in screen coordinates.
func (p *playState) killPopPopup() (comboPopup, bool) {
	pop := p.combo.kill
	if !pop.active || p.world == nil || int(pop.shown) < 1 {
		return comboPopup{}, false
	}
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	life := math.Min(1, pop.age/(killPopHold+killPopFade))
	rise := killPopRise * life * life
	x := (pop.wx-p.world.CameraX)*zoom + p.world.ViewportX
	y := (pop.wy-rise-p.world.CameraY)*zoom + p.world.ViewportY
	alpha := 1.0
	if pop.age > killPopHold {
		alpha = math.Max(0, 1-(pop.age-killPopHold)/killPopFade)
	}
	size := killPopBaseSize * (1 + killPopPulseAdd*pop.pulse)
	// Colour: the live tracker's tier colour when there is one, white otherwise.
	r, g, b := 255.0, 255.0, 255.0
	if t := p.combo.liveCounter(); t != nil && t.alpha > 0 {
		r, g, b = t.color[0], t.color[1], t.color[2]
	}
	return comboPopup{Text: "x" + strconv.Itoa(int(pop.shown)), X: x, Y: y, Size: size, R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(alpha * 255)}, true
}
