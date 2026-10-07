package main

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
	if x != 56 {
		t.Fatalf("native fixed origin X %g, want 56", x)
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
