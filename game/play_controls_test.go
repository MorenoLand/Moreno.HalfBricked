package game

import "testing"

func TestOptionsSwapRolesWithoutAxisInversion(t *testing.T) {
	p := &playState{}
	c := nativeOptionsDefaults(false)
	c.Normal = false
	p.configureControls(c, 960, 540)
	p.startControlTouch(100, 200)
	if p.stick != 2 || p.rightBaseX != 100 || p.rightBaseY != 200 {
		t.Fatal("left control did not become floating fire")
	}
	p.startControlTouch(400, 240)
	if p.stick != 1 || p.leftBaseX != 400 || p.leftBaseY != 240 {
		t.Fatal("right control did not become floating movement")
	}
}
func TestFixedControlsPreserveConfiguredOrigin(t *testing.T) {
	p := &playState{}
	c := nativeOptionsDefaults(false)
	c.LeftFloating, c.RightFloating = false, false
	p.configureControls(c, 960, 540)
	x, y := p.leftBaseX, p.leftBaseY
	p.startControlTouch(100, 200)
	if p.stick != 1 || p.leftBaseX != x || p.leftBaseY != y {
		t.Fatal("fixed base followed touch")
	}
	if x != 50 {
		t.Fatalf("native fixed origin X %g, want 50", x)
	}
}
func TestFloatingControlsClampScreenPixelOrigin(t *testing.T) {
	p := &playState{controlWidth: 960, controlHeight: 540}
	c := nativeOptionsDefaults(false)
	c.PadRadius = 80
	p.configureControls(c, 960, 540)
	p.startControlTouch(0, 320)
	if p.leftBaseX != 40 || p.leftBaseY != 460.0*logicalHeight/540 {
		t.Fatalf("unclamped origin %g,%g", p.leftBaseX, p.leftBaseY)
	}
	p.updateControlTouch(p.leftBaseX+40, p.leftBaseY, 960, 540)
	if p.leftDeflectX < .76 || p.leftDeflectX > .78 {
		t.Fatalf("scaled input radius %g", p.leftDeflectX)
	}
}
func TestSecondaryControlFitsViewportAndMatchesHitbox(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		p := &playState{grenades: 1, controlWidth: 2560, controlHeight: 1440, rightBaseX: 479, rightBaseY: 319, mobileControls: mobile, controls: optionsControls{PadRadius: 80}}
		x, y, width, height := p.secondaryButtonGeometry()
		if x+width/2 > logicalWidth || x-width/2 < 0 || y+height/2 > logicalHeight || y-height/2 < 0 {
			t.Fatal("secondary control escaped viewport")
		}
		if !p.secondaryButtonContains(x, y) || p.secondaryButtonContains(x+width, y) {
			t.Fatal("secondary hitbox differs from drawn geometry")
		}
		if width != 64 || height != 32 {
			t.Fatalf("secondary control %gx%g logical units, want 64x32 so it scales with the window", width, height)
		}
	}
}
func TestHiddenControlsContinueAcceptingInput(t *testing.T) {
	p := &playState{}
	c := nativeOptionsDefaults(false)
	c.Visible = false
	p.configureControls(c, 960, 540)
	p.startControlTouch(100, 200)
	if p.controls.Visible || p.stick != 1 || p.leftBaseX != 100 || p.leftBaseY != 200 {
		t.Fatal("hidden control disabled input or lost its origin")
	}
}

// The grenade button sits a fixed logical gap (20 less half its height) above the
// drawn ring at every window size; it used to float further away as the window grew.
func TestControlButtonStaysAtRingTopWhenWindowGrows(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		for _, size := range [][2]int{{480, 320}, {960, 640}, {1920, 1080}} {
			p := &playState{grenades: 1, controlWidth: size[0], controlHeight: size[1], mobileControls: mobile}
			p.configureControls(nativeOptionsDefaults(false), size[0], size[1])
			_, y, _, height := p.secondaryButtonGeometry()
			_, baseY := p.rightStickAnchor()
			ringTop := baseY - p.padRingHalfHeight()
			if got := ringTop - (y + height/2); got < 3.99 || got > 4.01 {
				t.Fatalf("mobile=%v %v: button bottom is %g above the ring, want 4", mobile, size, got)
			}
		}
	}
}
