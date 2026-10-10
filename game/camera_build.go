package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/content"

// Play camera constants per build. The build is the cache the level came from (playState.waveBuild):
// content.WaveBuild125 is the 1.2.5 build (HD cache), content.WaveBuildV7 the 1.2.1 build (SD cache,
// libmortargame.so v7). Every value below is a binary literal or a native call-site argument; the
// addresses are in Research/native/camera-build-2026-10-09.md.
const (
	// 1.2.5 FUN_00096030 moves the centre toward its auto-tracked target by pos += 0.07 * (target - pos) once
	// per call. The factor is the float32 literal at 0x000961f0 (0x3D8F5C29), loaded into s18 at 0x00096058.
	cameraTrackEase125 = 0.07
	// v7 has no easing on the centre: FUN_00096818 hands the player position plus its lookahead to
	// FUN_000bf23c, which stores the clamped value directly (scene +0x1ac54 / +0x1ac58). The v7 rates 0.025
	// (0x000974a4) and 0.02 (0x000974a0) ease the lookahead offset, which the port does not model.
	cameraTrackEaseV7 = 1.0

	// 1.2.5 FUN_0013e0b8 calls FUN_00095d38 with angle multiplier 0x41000000 (8.0). v7 FUN_000db67c passes
	// 0x3f800000 (1.0) in the same argument.
	cameraShakeAngleMul125 = 8.0
	cameraShakeAngleMulV7  = 1.0
)

// cameraIsV7 reports whether the play camera follows the v7 (SD) rules.
func cameraIsV7(build string) bool { return build == content.WaveBuildV7 }

// cameraTrackEase is the per-logic-tick factor by which an auto-tracked camera centre moves toward its target.
func cameraTrackEase(build string) float64 {
	if cameraIsV7(build) {
		return cameraTrackEaseV7
	}
	return cameraTrackEase125
}

// cameraShakeAngleMul is the angle multiplier the CameraShake script callback passes to the shake start.
func cameraShakeAngleMul(build string) float64 {
	if cameraIsV7(build) {
		return cameraShakeAngleMulV7
	}
	return cameraShakeAngleMul125
}

// playCameraEase is the factor the play camera uses this logic tick. A followed entity (SetCameraFollow)
// snaps on both builds: 1.2.5 FUN_00096030's follow branch and v7 FUN_000bf404 both write the clamped
// target at once. Only auto tracking eases, and only on 1.2.5.
func (p *playState) playCameraEase() float64 {
	if p.scriptCameraFollow {
		return 1
	}
	return cameraTrackEase(p.waveBuild())
}

// scriptGetCameraX is the value of the GetCameraX script callback. 1.2.5 FUN_0013e030 -> FUN_00095aa8 returns
// the centre plus the shake offset (cam +0x24 + cam +0x18), so a lerp from GetCameraX feeds the shake back in.
// v7 FUN_000db61c returns the clamped centre alone (scene +0x1ac54), without the shake.
func (p *playState) scriptGetCameraX() float64 {
	x := p.scriptCameraCenterX()
	if !cameraIsV7(p.waveBuild()) {
		x += p.shake.currentX
	}
	return x
}

// scriptGetCameraY is the GetCameraY counterpart: 1.2.5 FUN_00095aa8 (cam +0x28 + cam +0x1c) and v7 FUN_000db650
// (scene +0x1ac58, centre only).
func (p *playState) scriptGetCameraY() float64 {
	y := p.scriptCameraCenterY()
	if !cameraIsV7(p.waveBuild()) {
		y += p.shake.currentY
	}
	return y
}
