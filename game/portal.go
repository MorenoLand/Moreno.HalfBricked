package game

import "math"

// Portal presentation (v7 FUN_000a... FUN_0009d078 update, FUN_0009d45c draw; 1.2.5
// FUN_000fc3e8, identical literals at 0x000fc638..).
//
//   - Rotation: the angle +0x36 is a ushort stored from vcvt.u32.f32(angle - dt * speed *
//     65338). The conversion saturates negative values at 0 and the start value is 0, so
//     the drawn angle is always 0: the native portal does not turn (the second counter
//     +0x4c only counts up and is never drawn). The earlier port wrapped the angle.
//   - Rotation speed target: 1.2 while more than 1000 ms remain, 1.0 afterwards, 0 once
//     remaining < -600 ms (literals 0x0009d308 / 0x0009d304 / 0x0009d300). Speed lerp 0.05.
//   - Alpha byte +0x5f starts at 0 (colour 0x00ffffff): while open it gains dt * 1000 per
//     update up to 255; while closing the update subtracts dt * 1000, and when the step
//     would go below zero it adds the alpha to itself instead (assembly 0x0009d1bc..
//     0x0009d1d0: vmovmi s16,s15 then vadd), a native quirk that is reproduced.
const (
	portalLateRotationSpeed = 1.0
	portalAlphaStepPerFrame = 1000.0 / 60.0
)

// portalAngleStep is the ushort angle update; it never leaves 0 from a start of 0.
func portalAngleStep(angle, speed float64) float64 {
	next := float64(float32(angle) - float32(1.0/60.0)*float32(speed)*float32(portalRotationUnitsPerSecond))
	if next <= 0 {
		return 0
	}
	return math.Trunc(next)
}

// portalAlphaStep is the alpha byte update of FUN_0009d078.
func portalAlphaStep(alpha float64, closing bool) float64 {
	step := float32(portalAlphaStepPerFrame)
	a := float32(alpha)
	var next float32
	if closing {
		if a == 0 {
			step = 0
		}
		delta := step
		if a != 0 {
			if a-step >= 0 {
				delta = -step
			} else {
				delta = a
			}
		}
		next = a + delta
	} else {
		delta := float32(0)
		if a != 255 {
			delta = step
			if a+step > 255 {
				delta = 255 - a
			}
		}
		next = a + delta
	}
	if next <= 0 {
		return 0
	}
	if next > 255 {
		next = 255
	}
	return math.Trunc(float64(next))
}
