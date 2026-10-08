package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func TestCameraShakeNativeWaveform(t *testing.T) {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng}
	// Camera 100 px right of the origin: angle 0, so target = (9*1.65, 0).
	p.shake.start(0, 0, 100, 0, 1.5, 1.65, 1, 1)
	if math.Abs(p.shake.targetX-9*1.65) > 1e-6 || math.Abs(p.shake.targetY) > 1e-9 {
		t.Fatalf("initial target = %.3f,%.3f", p.shake.targetX, p.shake.targetY)
	}
	x, _ := p.updateShake(1.0 / 60.0)
	if math.Abs(x-9*1.65*0.2) > 1e-6 {
		t.Fatalf("first step eases 20%% toward the target, got %.4f", x)
	}
	// Magnitude is bounded by 9 * remaining/total once the first target is reached.
	peak := 0.0
	for frame := 1; frame < 95; frame++ {
		ox, oy := p.updateShake(1.0 / 60.0)
		if frame > 30 {
			peak = math.Max(peak, math.Hypot(ox, oy))
		}
	}
	if peak <= 0 || peak > 9*(1-30.0/90.0)+4 {
		t.Fatalf("late peak %.3f outside the decaying 9*remaining/total envelope", peak)
	}
	if p.shake.active() {
		t.Fatal("the shake timer should have run out")
	}
	// After the timer ends the step is no longer applied: the offset freezes.
	fx, fy := p.updateShake(1.0 / 60.0)
	gx, gy := p.updateShake(1.0 / 60.0)
	if fx != gx || fy != gy || math.Hypot(fx, fy) > 1 {
		t.Fatalf("offset after expiry = %.3f,%.3f then %.3f,%.3f", fx, fy, gx, gy)
	}
}

func TestCameraShakeRotatesTargetBy0x6388Plus(t *testing.T) {
	rng := weapons.NewNativeRNG()
	s := cameraShake{remaining: 1, total: 1, angle: 0}
	s.step(0.1, &rng) // target == current == 0 -> new target on the first step
	if s.angle < 0x6388 || s.angle >= 0x6388+0x38e0 {
		t.Fatalf("angle = %#x, want 0x6388..0x6388+0x38e0", s.angle)
	}
	m := 0.9 * 9
	if math.Abs(math.Hypot(s.targetX, s.targetY)-m) > 0.02 {
		t.Fatalf("target magnitude %.3f, want %.3f", math.Hypot(s.targetX, s.targetY), m)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
