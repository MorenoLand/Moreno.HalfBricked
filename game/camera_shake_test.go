package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
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

// Native 1.2.5 FUN_00095d38 takes the angle from the rendered camera (cam+0x2c/+0x30 = view centre plus
// the shake offset), not from the top-left corner that world.CameraX holds.
func TestCameraShakeOriginIsRenderedCentreNotTopLeft(t *testing.T) {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng, tileSize: 32, world: &viewer.Viewer{Level: formats.Level{Width: 200, Height: 200}, Zoom: 1}}
	p.setScriptCamera(1000, 800)
	cx, cy := p.scriptCameraCenterX(), p.scriptCameraCenterY()
	p.startCameraShake(200, 300, 1.5, 1.5, 1, 1)
	if want := weapons.NativeWeaponDirection(cx-200, cy-300); p.shake.angle != want {
		t.Fatalf("angle %#x from the view centre, want %#x", p.shake.angle, want)
	}
	p.shake.currentX, p.shake.currentY = 4, -3
	p.startCameraShake(200, 300, 1.5, 1.5, 1, 1)
	if want := weapons.NativeWeaponDirection(cx+4-200, cy-3-300); p.shake.angle != want {
		t.Fatalf("angle %#x with a shake offset, want %#x", p.shake.angle, want)
	}
}

// Native 1.2.5 FUN_0013e0b8 calls FUN_00095d38(cam, origin, duration, ampX, 1.0, 8.0): the vertical
// target uses sin(8 * angle), not sin(angle).
func TestCameraShakeCallbackUsesNative125AngleMultiplierEight(t *testing.T) {
	rng := weapons.NewNativeRNG()
	play := &playState{rng: &rng}
	host := &playScriptHost{play: play}
	if _, err := host.Call("CameraShake", []scripting.Value{12, 24, 1.5, 1.5}); err != nil {
		t.Fatal(err)
	}
	angle := weapons.NativeWeaponDirection(-12, -24) // no world: the camera sits at 0,0
	wantY := shakeSin(uint16(int(float64(angle)*8)&0xffff)) * 9
	if play.shake.angle != angle || math.Abs(play.shake.targetY-wantY) > 1e-9 {
		t.Fatalf("CameraShake angle %#x target.y %.6f, want angle %#x target.y %.6f", play.shake.angle, play.shake.targetY, angle, wantY)
	}
}

// The shake steps from the logic tick (native scene update), so reading the draw offset must not
// advance it and a paused game must not step it.
func TestCameraShakeStepsOncePerLogicTick(t *testing.T) {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng}
	p.startCameraShake(0, 0, 1.5, 1, 1, 1)
	start := p.shake.remaining
	p.shakeOffset()
	if p.shake.remaining != start {
		t.Fatalf("reading the draw offset advanced the shake to %.6f", p.shake.remaining)
	}
	p.stepShakeTick()
	if math.Abs(p.shake.remaining-(start-1.0/60)) > 1e-12 {
		t.Fatalf("one logic tick left remaining %.6f, want %.6f", p.shake.remaining, start-1.0/60)
	}
	p.paused = true
	p.stepShakeTick()
	if math.Abs(p.shake.remaining-(start-1.0/60)) > 1e-12 {
		t.Fatalf("a paused tick advanced the shake to %.6f", p.shake.remaining)
	}
}
