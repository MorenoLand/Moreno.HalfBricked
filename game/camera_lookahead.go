package game

import "math"

// SD (v7) player look-ahead. The native player update FUN_00096818 ends with a block that runs only when
// FUN_000d9ab4() == 0 and FUN_000962f4() == 0. The block eases the look-ahead offset (player +0xa4/+0xa8) toward a
// target (+0xac/+0xb0) and then calls FUN_000bf23c(scene, player.x + look.x, player.y + look.y). FUN_000bf23c writes
// the clamped centre at once (camera_build.go: cameraTrackEaseV7 = 1). Addresses and bit patterns are listed in
// Research/native/camera-build-2026-10-09.md, section "v7 look-ahead".
const (
	v7LookaheadScaleX   = 1.7   // DAT_00096c08, 0x3FD9999A: x factor of the heading
	v7LookaheadScaleY   = 1.6   // DAT_00096c0c, 0x3FCCCCCD: y factor of the heading
	v7LookaheadGain     = 25.0  // DAT_00096c10, 0x41C80000: applied to both axes
	v7LookaheadBreak    = 1.0   // DAT_00096b70, 0x3F800000: L1 change of the target that restarts the hold
	v7LookaheadHold     = 0.15  // DAT_000974b4, 0x3E19999A: seconds the target is held after a change
	v7LookaheadRate     = 0.025 // DAT_000974a4, 0x3CCCCCCD: ease factor, applied to both axes
	v7LookaheadRateZero = 0.02  // DAT_000974a0, 0x3CA3D70A: ease factor when the target is (0,0)
)

// v7Lookahead holds the native look-ahead fields of one player: x/y are +0xa4/+0xa8, targetX/targetY are
// +0xac/+0xb0, lastX/lastY are +0xb4/+0xb8 (the direction target of the previous call) and hold is +0xbc.
type v7Lookahead struct {
	x, y             float64
	targetX, targetY float64
	lastX, lastY     float64
	hold             float64
}

// step runs one call of the native block. heading is the unit aim vector (native local_68/local_64 after the
// FUN_00098338 normalisation); dt is the call's time in seconds (native param_2).
//
// Native order: the direction target is (heading.x * 1.7 * 25, heading.y * 1.6 * 25), or the default globals when
// the heading is zero (UNRESOLVED: taken as (0,0) here). If the L1 change from the previous call is at least 1.0 the
// hold restarts at 0.15 s and the target stays. Otherwise a running hold counts down by dt, and with no hold left the
// target takes the direction. The ease rate is 0.02 when the target is (0,0) and 0.025 otherwise, the same factor on
// both axes. The previous direction is stored after the decision, then the offset eases toward the target.
func (l *v7Lookahead) step(headingX, headingY, dt float64) {
	var dirX, dirY float64
	if headingX != 0 || headingY != 0 {
		dirX = headingX * v7LookaheadScaleX * v7LookaheadGain
		dirY = headingY * v7LookaheadScaleY * v7LookaheadGain
	}
	switch {
	case math.Abs(l.lastX-dirX)+math.Abs(l.lastY-dirY) >= v7LookaheadBreak:
		l.hold = v7LookaheadHold
	case l.hold > 0:
		l.hold -= dt
	default:
		l.targetX, l.targetY = dirX, dirY
	}
	rate := v7LookaheadRate
	if l.targetX == 0 && l.targetY == 0 {
		rate = v7LookaheadRateZero
	}
	l.lastX, l.lastY = dirX, dirY
	l.x += (l.targetX - l.x) * rate
	l.y += (l.targetY - l.y) * rate
}

// unitHeading normalises a two-component input (native FUN_00098338 with the third component zero). The zero
// vector stays zero, as the native normaliser returns without changing it.
func unitHeading(x, y float64) (float64, float64) {
	n := math.Hypot(x, y)
	if n == 0 {
		return 0, 0
	}
	return x / n, y / n
}

// cameraHeading is the aim vector fed to the look-ahead. The native block reads one of two input pairs of the
// aim object (O +0x44/+0x48 or O +0x4c/+0x50), chosen by the player flag at +0x88. That mapping is UNRESOLVED, so the
// port takes the aim input when it is set (keyboard/gamepad right stick, touch right deflect) and falls back to the
// move input. The mouse aim is computed inside the aim code of the update and is not visible here.
func (p *playState) cameraHeading() (float64, float64) {
	if x, y := p.input.aimX, p.input.aimY; x != 0 || y != 0 {
		return unitHeading(x, y)
	}
	if x, y := p.rightDeflectX, p.rightDeflectY; x != 0 || y != 0 {
		return unitHeading(x, y)
	}
	return unitHeading(p.input.moveX, p.input.moveY)
}

// v7LookaheadState returns the look-ahead state of this play session.
func (p *playState) v7LookaheadState() *v7Lookahead {
	return &p.lookahead
}

// cameraLookaheadTick runs the look-ahead once for a logic tick and returns the offset to add to the focus. It is
// only called on the SD build, outside co-op and outside a followed entity (see updateCamera).
func (p *playState) cameraLookaheadTick(dt float64) (float64, float64) {
	state := p.v7LookaheadState()
	headingX, headingY := p.cameraHeading()
	state.step(headingX, headingY, dt)
	return state.x, state.y
}
