package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
)

// The look-ahead literals are the binary's own bit patterns (Research/native/camera-build-2026-10-09.md).
func TestCameraV7LookaheadConstantsMatchTheBinary(t *testing.T) {
	for _, c := range []struct {
		name  string
		value float64
		bits  uint32
	}{
		{"x scale DAT_00096c08", v7LookaheadScaleX, 0x3FD9999A},
		{"y scale DAT_00096c0c", v7LookaheadScaleY, 0x3FCCCCCD},
		{"gain DAT_00096c10", v7LookaheadGain, 0x41C80000},
		{"break DAT_00096b70", v7LookaheadBreak, 0x3F800000},
		{"hold DAT_000974b4", v7LookaheadHold, 0x3E19999A},
		{"rate DAT_000974a4", v7LookaheadRate, 0x3CCCCCCD},
		{"rate at zero DAT_000974a0", v7LookaheadRateZero, 0x3CA3D70A},
	} {
		if bits := math.Float32bits(float32(c.value)); bits != c.bits {
			t.Fatalf("%s: bits %#x, want %#x", c.name, bits, c.bits)
		}
	}
}

// A heading that holds still for the hold time sets the target to heading * (1.7, 1.6) * 25 on the x and y axes,
// and the ease factor 0.025 applies to both axes once the target is set.
func TestCameraV7LookaheadHoldThenTargetOnBothAxes(t *testing.T) {
	const dt = 1.0 / 60
	var l v7Lookahead
	l.step(0.6, 0.8, dt)
	if l.targetX != 0 || l.targetY != 0 || math.Abs(l.hold-v7LookaheadHold) > 1e-12 {
		t.Fatalf("first call with a new heading: target %v,%v hold %v, want target 0,0 and hold 0.15", l.targetX, l.targetY, l.hold)
	}
	for i := 0; i < 5; i++ {
		l.step(0.6, 0.8, dt)
	}
	if l.targetX != 0 || l.targetY != 0 {
		t.Fatalf("target %v,%v set before the hold ran out", l.targetX, l.targetY)
	}
	for i := 0; i < 15; i++ {
		l.step(0.6, 0.8, dt)
	}
	wantX, wantY := 0.6*1.7*25, 0.8*1.6*25
	if math.Abs(l.targetX-wantX) > 1e-9 || math.Abs(l.targetY-wantY) > 1e-9 {
		t.Fatalf("target %v,%v, want %v,%v (x: 1.7*25, y: 1.6*25)", l.targetX, l.targetY, wantX, wantY)
	}
	// The ease moves the offset by 0.025 of the gap each call, so the offset is still short of the target here.
	if l.x <= 0 || l.x >= wantX || l.y <= 0 || l.y >= wantY {
		t.Fatalf("offset %v,%v after the target was set, want a partial step toward %v,%v", l.x, l.y, wantX, wantY)
	}
}

// The L1 change of the direction target decides the hold: 0.9 does not restart it, 1.0 does.
func TestCameraV7LookaheadBreakThresholdIsOneOnTheTarget(t *testing.T) {
	const dt = 1.0 / 60
	below := v7Lookahead{}
	below.step(0.999/42.5, 0, dt)
	if below.hold != 0 || math.Abs(below.targetX-0.999) > 1e-9 {
		t.Fatalf("change 0.999 (below 1): hold %v target %v, want hold 0 and target 0.999", below.hold, below.targetX)
	}
	at := v7Lookahead{}
	at.step(1.001/42.5, 0, dt)
	if math.Abs(at.hold-v7LookaheadHold) > 1e-12 || at.targetX != 0 {
		t.Fatalf("change 1.001 (above 1): hold %v target %v, want hold 0.15 and target 0", at.hold, at.targetX)
	}
}

// The ease factor is 0.02 only when the target is exactly (0,0); any other target uses 0.025 on both axes.
func TestCameraV7LookaheadRateFollowsTheTargetZero(t *testing.T) {
	const dt = 1.0 / 60
	zero := v7Lookahead{x: 10}
	zero.step(0, 0, dt)
	if want := 10 + (0-10)*v7LookaheadRateZero; math.Abs(zero.x-want) > 1e-12 {
		t.Fatalf("zero target: x %v, want %v (rate 0.02)", zero.x, want)
	}
	moving := v7Lookahead{lastX: 42.5}
	moving.step(1, 0, dt)
	if want := 42.5 * v7LookaheadRate; math.Abs(moving.x-want) > 1e-12 || moving.y != 0 {
		t.Fatalf("moving target: x %v y %v, want x %v (rate 0.025) and y 0", moving.x, moving.y, want)
	}
}

// Under a steady heading the offset converges to the scaled heading.
func TestCameraV7LookaheadConvergesToTheScaledHeading(t *testing.T) {
	const dt = 1.0 / 60
	var l v7Lookahead
	for i := 0; i < 600; i++ {
		l.step(0.6, 0.8, dt)
	}
	if math.Abs(l.x-0.6*42.5) > 1e-3 || math.Abs(l.y-0.8*40) > 1e-3 {
		t.Fatalf("offset %.4f,%.4f after 600 ticks, want %.4f,%.4f", l.x, l.y, 0.6*42.5, 0.8*40)
	}
}

// On the SD build an aiming player leads the camera by the eased look-ahead; the 1.2.5 build does not.
func TestUpdateCameraLookaheadOnSDOnly(t *testing.T) {
	for _, c := range []struct {
		build string
		wantX float64
	}{
		{content.WaveBuildV7, 1000 + 42.5 - 240},
		{content.WaveBuild125, 1000 - 240},
	} {
		p := cameraBuildRig(c.build)
		p.x, p.y = 1000, 800
		p.input = playerInput{aimX: 1}
		for i := 0; i < 600; i++ {
			p.updateCamera()
		}
		if math.Abs(p.world.CameraX-c.wantX) > 0.01 || math.Abs(p.world.CameraY-640) > 1e-9 {
			t.Fatalf("%s: camera %.4f,%.4f, want %.4f,640", c.build, p.world.CameraX, p.world.CameraY, c.wantX)
		}
	}
}

// A followed entity (SetCameraFollow) snaps to its position and takes no look-ahead.
func TestUpdateCameraLookaheadSkippedWhileFollowing(t *testing.T) {
	p := cameraBuildRig(content.WaveBuildV7)
	p.x, p.y = 1000, 800
	p.input = playerInput{aimX: 1}
	p.scriptCameraFollow = true
	for i := 0; i < 60; i++ {
		p.updateCamera()
	}
	if math.Abs(p.world.CameraX-760) > 1e-9 || math.Abs(p.world.CameraY-640) > 1e-9 {
		t.Fatalf("followed SD camera %.6f,%.6f, want 760,640 with no look-ahead", p.world.CameraX, p.world.CameraY)
	}
}

// The look-ahead state belongs to one play session.
func TestCameraV7LookaheadStateIsPerPlaySession(t *testing.T) {
	a := cameraBuildRig(content.WaveBuildV7)
	b := cameraBuildRig(content.WaveBuildV7)
	a.input = playerInput{aimX: 1}
	for i := 0; i < 30; i++ {
		a.updateCamera()
		b.updateCamera()
	}
	if a.v7LookaheadState() == b.v7LookaheadState() {
		t.Fatal("two play sessions share one look-ahead state")
	}
	if a.v7LookaheadState().x == 0 || b.v7LookaheadState().x != 0 {
		t.Fatalf("look-ahead offsets a=%v b=%v, want only the aiming session to move", a.v7LookaheadState().x, b.v7LookaheadState().x)
	}
}
