package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// PORT ADDITION (not in the original binary): zombie unstick.
//
// The faithful model (zombie_ai.go, zombie_tiles.go) has no stuck timer, sidestep
// or path search for ordinary zombies, so one that walks head-on into a wall can
// stay wedged there and be shot forever (Ghidra: FUN_000a18e4 + FUN_000a17cc only
// move and push out). The user asked for a small, clearly separated improvement:
//
//   - a zombie that is alerted, is trying to move, and for zombieUnstickWindow
//     seconds made almost no progress toward its target while the tile push
//     held it against geometry is "blocked";
//   - for zombieUnstickHold seconds it then follows the navigation field
//     (zombie_navigation.go) or, without a route, slides along the wall tangent,
//     with its heading helped toward that direction;
//   - afterwards the native chase resumes unchanged.
//
// Native collision, separation (separateZombie) and the tile push are untouched,
// so a zombie still can never enter a wall, and open-field play is bit-identical
// because the detector requires tile pressure. Set zombieSmartExtras to false to
// get the purist original behaviour. (A var rather than a const only so tests can
// compare both modes; nothing changes it at run time.)
var zombieSmartExtras = true

const (
	zombieUnstickWindow    = .4  // seconds of pressed, stalled walking before it triggers
	zombieUnstickProgress  = .25 // progress below this fraction of the intended walk counts as stalled
	zombieUnstickHold      = 1.0 // seconds of forced route
	zombieUnstickExtend    = .5  // re-check period while the line is still blocked
	zombieUnstickMaxExtend = 6.0 // upper bound of one forced route
	zombieUnstickMinDist   = 40.0
	zombieUnstickPushEps   = .05
	zombieUnstickTurn      = .08 // extra heading turn per frame while forced (the native turn can be .01)
)

// zombieUnstick is the per-zombie detector state.
type zombieUnstick struct {
	window          float64
	frames, pressed int
	startDist       float64
	intent          float64
	normX, normY    float64
	hold            float64
	extended        float64
	tanX, tanY      float64
}

// zombieUnstickObserve runs after a zombie's move and tile push. intent is the
// length of the walk it attempted this frame (0 when it did not try to move),
// distance its distance to the target before the move and push the push-out.
func (p *playState) zombieUnstickObserve(z *zombieState, distance, intent, pushX, pushY, dt float64, needAlert bool) {
	if !zombieSmartExtras {
		return
	}
	u := &z.native.unstick
	if u.hold > 0 {
		u.hold -= dt
		if u.hold <= 0 {
			// Still walled off from the player: keep following the route in short
			// extensions (bounded) until the straight line is clear, then the native
			// chase resumes.
			prey := p.nearestPlayer(z.x, z.y)
			if u.extended < zombieUnstickMaxExtend && !p.lineClear(z.x, z.y, prey.x, prey.y, zombieCollisionRadius(*z)*.8) {
				u.hold, u.extended = zombieUnstickExtend, u.extended+zombieUnstickExtend
			} else {
				u.hold, u.extended = 0, 0
			}
		}
		return
	}
	if intent <= 0 {
		u.window, u.frames, u.pressed, u.intent, u.normX, u.normY = 0, 0, 0, 0, 0, 0
		return
	}
	if u.frames == 0 {
		u.startDist = distance
	}
	u.window += dt
	u.frames++
	u.intent += intent
	if math.Hypot(pushX, pushY) > zombieUnstickPushEps {
		u.pressed++
		u.normX += pushX
		u.normY += pushY
	}
	if u.window < zombieUnstickWindow {
		return
	}
	stalled := u.pressed*2 >= u.frames && u.startDist-distance < zombieUnstickProgress*u.intent && distance > zombieUnstickMinDist
	if stalled && needAlert {
		reach := float64(float32(p.alertNoise) * float32(z.native.ai.alertRadius))
		stalled = distance < reach
	}
	if stalled {
		u.hold = zombieUnstickHold
		u.tanX, u.tanY = p.zombieUnstickTangent(z, u.normX, u.normY, distance)
	}
	u.window, u.frames, u.pressed, u.intent, u.normX, u.normY = 0, 0, 0, 0, 0, 0
}

// zombieUnstickTangent picks the slide direction along the wall that was pressed
// against: the side that leads more toward the target, or a clear one when the
// approach was head-on.
func (p *playState) zombieUnstickTangent(z *zombieState, nx, ny, distance float64) (float64, float64) {
	length := math.Hypot(nx, ny)
	if length < 1e-6 {
		return 0, 0
	}
	nx, ny = nx/length, ny/length
	tx, ty := -ny, nx
	prey := p.nearestPlayer(z.x, z.y)
	dot := (prey.x-z.x)*tx + (prey.y-z.y)*ty
	if math.Abs(dot) < .2*math.Max(distance, 1) {
		radius := zombieCollisionRadius(*z)
		okA := p.lineClear(z.x, z.y, z.x+tx*48, z.y+ty*48, radius)
		okB := p.lineClear(z.x, z.y, z.x-tx*48, z.y-ty*48, radius)
		switch {
		case okA && !okB:
			dot = 1
		case okB && !okA:
			dot = -1
		case dot == 0:
			if (int(z.x)+int(z.y))&1 == 0 {
				dot = 1
			} else {
				dot = -1
			}
		}
	}
	if dot < 0 {
		tx, ty = -tx, -ty
	}
	// A little outward component keeps the body from grinding along the face.
	dx, dy := tx+.25*nx, ty+.25*ny
	l := math.Hypot(dx, dy)
	return dx / l, dy / l
}

// zombieUnstickDirection is the forced heading while a hold runs: the route of
// the navigation field when there is one, else the stored wall tangent.
func (p *playState) zombieUnstickDirection(z *zombieState, player int) (float64, float64, bool) {
	if !zombieSmartExtras || z.native.unstick.hold <= 0 {
		return 0, 0, false
	}
	if x, y, ok := p.navDirection(z.x, z.y, player); ok {
		return x, y, true
	}
	u := &z.native.unstick
	if u.tanX == 0 && u.tanY == 0 {
		return 0, 0, false
	}
	return u.tanX, u.tanY, true
}

// zombieUnstickAssist turns a wave zombie's heading faster toward the forced
// direction (applied after the native heading step of that frame).
func zombieUnstickAssist(z *zombieState, dirX, dirY float64) {
	want := weapons.NativeWeaponDirection(dirX, dirY)
	z.native.facing += uint16(int(float64(angleDifference(want, z.native.facing)) * zombieUnstickTurn))
}
