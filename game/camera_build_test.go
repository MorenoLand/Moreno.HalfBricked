package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// cameraBuildRig is a 200x200-tile level viewed at zoom 1 (a 480x320 view) on the given cache build.
func cameraBuildRig(build string) *playState {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng, tileSize: 32, world: &viewer.Viewer{Level: formats.Level{Width: 200, Height: 200}, Zoom: 1}}
	setTestBuild(p, build)
	return p
}

// The constants are the binary's own values: 0.07f is 0x3D8F5C29 at 1.2.5 0x000961f0 (FUN_00096030),
// 8.0f is 0x41000000 in FUN_0013e0b8 and 1.0f is 0x3F800000 in v7 FUN_000db67c.
func TestCameraBuildConstantsMatchTheBinary(t *testing.T) {
	if bits := math.Float32bits(float32(cameraTrackEase125)); bits != 0x3D8F5C29 {
		t.Fatalf("1.2.5 track ease bits %#x, want the 0.07f literal 0x3D8F5C29", bits)
	}
	if bits := math.Float32bits(float32(cameraShakeAngleMul125)); bits != 0x41000000 {
		t.Fatalf("1.2.5 shake angle multiplier bits %#x, want 8.0f 0x41000000", bits)
	}
	if bits := math.Float32bits(float32(cameraShakeAngleMulV7)); bits != 0x3F800000 {
		t.Fatalf("v7 shake angle multiplier bits %#x, want 1.0f 0x3F800000", bits)
	}
	if cameraTrackEaseV7 != 1 {
		t.Fatalf("v7 track ease %v, want 1: v7 FUN_000bf23c stores the clamped centre at once", cameraTrackEaseV7)
	}
}

func TestPlayCameraEaseByBuildAndFollow(t *testing.T) {
	for _, c := range []struct {
		name   string
		build  string
		follow bool
		want   float64
	}{
		{"1.2.5 auto track", content.WaveBuild125, false, 0.07},
		{"v7 auto track", content.WaveBuildV7, false, 1},
		{"1.2.5 followed entity", content.WaveBuild125, true, 1},
		{"v7 followed entity", content.WaveBuildV7, true, 1},
	} {
		p := cameraBuildRig(c.build)
		p.scriptCameraFollow = c.follow
		if got := p.playCameraEase(); got != c.want {
			t.Fatalf("%s: ease %v, want %v", c.name, got, c.want)
		}
	}
}

// One auto-tracked logic tick moves the centre 7% of the way to the target on 1.2.5 and all the way on v7.
// The target is the view centred on the player: (1000-240, 800-160).
func TestUpdateCameraEasesOnlyOn125(t *testing.T) {
	for _, c := range []struct {
		build string
		ease  float64
	}{{content.WaveBuild125, 0.07}, {content.WaveBuildV7, 1}} {
		p := cameraBuildRig(c.build)
		p.x, p.y = 1000, 800
		p.updateCamera()
		wantX, wantY := (1000-240)*c.ease, (800-160)*c.ease
		if math.Abs(p.world.CameraX-wantX) > 1e-9 || math.Abs(p.world.CameraY-wantY) > 1e-9 {
			t.Fatalf("%s: camera %.6f,%.6f after one tick, want %.6f,%.6f", c.build, p.world.CameraX, p.world.CameraY, wantX, wantY)
		}
	}
}

// A followed entity (SetCameraFollow) is written at once on both builds: 1.2.5 FUN_00096030's follow branch
// and v7 FUN_000bf404 both store the clamped target without easing.
func TestFollowedCameraSnapsOnBothBuilds(t *testing.T) {
	for _, build := range []string{content.WaveBuild125, content.WaveBuildV7} {
		p := cameraBuildRig(build)
		p.x, p.y = 1000, 800
		p.scriptCameraFollow = true
		p.updateCamera()
		if math.Abs(p.world.CameraX-760) > 1e-9 || math.Abs(p.world.CameraY-640) > 1e-9 {
			t.Fatalf("%s: followed camera %.6f,%.6f, want 760,640 at once", build, p.world.CameraX, p.world.CameraY)
		}
	}
}

// GetCameraX/Y: 1.2.5 returns the centre plus the shake offset (FUN_00095aa8), v7 returns the clamped centre
// alone (FUN_000db61c, FUN_000db650).
func TestGetCameraXYPerBuild(t *testing.T) {
	for _, c := range []struct {
		build        string
		wantX, wantY float64
	}{
		{content.WaveBuild125, 1000 + 4, 800 - 3},
		{content.WaveBuildV7, 1000, 800},
	} {
		p := cameraBuildRig(c.build)
		p.setScriptCamera(1000, 800)
		p.shake.currentX, p.shake.currentY = 4, -3
		host := &playScriptHost{play: p}
		x, err := host.Call("GetCameraX", nil)
		if err != nil {
			t.Fatal(err)
		}
		y, err := host.Call("GetCameraY", nil)
		if err != nil {
			t.Fatal(err)
		}
		gotX, okX := x.Values[0].(float64)
		gotY, okY := y.Values[0].(float64)
		if !okX || !okY || math.Abs(gotX-c.wantX) > 1e-9 || math.Abs(gotY-c.wantY) > 1e-9 {
			t.Fatalf("%s: GetCameraX/Y = %v,%v, want %v,%v", c.build, x.Values, y.Values, c.wantX, c.wantY)
		}
	}
}

// The CameraShake callback passes angle multiplier 8 on 1.2.5 (FUN_0013e0b8) and 1 on v7 (FUN_000db67c), so the
// vertical target is sin(mul * angle).
func TestCameraShakeAngleMulByBuild(t *testing.T) {
	for _, c := range []struct {
		build string
		mul   float64
	}{{content.WaveBuild125, 8}, {content.WaveBuildV7, 1}} {
		p := cameraBuildRig(c.build)
		p.setScriptCamera(0, 0)
		cx, cy := p.scriptCameraCenterX(), p.scriptCameraCenterY()
		host := &playScriptHost{play: p}
		if _, err := host.Call("CameraShake", []scripting.Value{12, 24, 1.5, 1.5}); err != nil {
			t.Fatal(err)
		}
		angle := weapons.NativeWeaponDirection(cx-12, cy-24)
		wantY := shakeSin(uint16(int(float64(angle)*c.mul)&0xffff)) * 9
		if p.shake.angle != angle || math.Abs(p.shake.targetY-wantY) > 1e-9 {
			t.Fatalf("%s: CameraShake angle %#x target.y %.6f, want angle %#x target.y %.6f", c.build, p.shake.angle, p.shake.targetY, angle, wantY)
		}
	}
}
