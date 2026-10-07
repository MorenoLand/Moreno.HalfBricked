package main

import (
	"encoding/json"
	"math"
	"testing"
)

func TestOptionsNativeDefaults(t *testing.T) {
	for _, compact := range []bool{false, true} {
		c := nativeOptionsDefaults(compact)
		want := float32(56)
		if compact {
			want = 42
		}
		if !c.Normal || !c.Visible || !c.LeftFloating || !c.RightFloating || c.PadRadius != want {
			t.Fatalf("defaults %v: %+v", compact, c)
		}
	}
}
func TestDeviceVisibilityDefaultsAndSavedChoice(t *testing.T) {
	for _, mobile := range []bool{false, true} {
		menu := newOptionsMenu(true, true)
		menu.useDeviceDefaults(mobile)
		if menu.controls.Visible != mobile {
			t.Fatal("wrong device default")
		}
		settings := menu.Settings()
		settings.Visible = !mobile
		if err := menu.RestoreSettings(settings); err != nil || menu.controls.Visible != !mobile {
			t.Fatal("saved choice lost")
		}
		menu.activate(optionsDefault, nil)
		if menu.controls.Visible != mobile {
			t.Fatal("Default ignored device detection")
		}
	}
}
func TestOptionsControlActions(t *testing.T) {
	menu := newOptionsMenu(true, true)
	for _, action := range []optionsAction{optionsInvert, optionsHidden, optionsLeftFixed, optionsRightFixed, optionsNormal, optionsVisible, optionsLeftFloating, optionsRightFloating} {
		if menu.activate(action, nil) || !menu.controls.selected(action) {
			t.Fatalf("action %d: %+v", action, menu.controls)
		}
	}
}
func TestOptionsPadDragNativeClampAndRelease(t *testing.T) {
	menu := newOptionsMenu(true, true)
	menu.dragPad(428, 150, 372, 150, true, true)
	menu.dragPad(440, 150, 372, 150, false, true)
	if menu.controls.PadRadius != 68 {
		t.Fatalf("radius %v", menu.controls.PadRadius)
	}
	menu.dragPad(600, 150, 372, 150, false, true)
	if menu.controls.PadRadius != 80 {
		t.Fatal("upper clamp")
	}
	menu.dragPad(372, 150, 372, 150, false, true)
	if menu.controls.PadRadius != 32 {
		t.Fatal("lower clamp")
	}
	menu.dragPad(372, 150, 372, 150, false, false)
	menu.dragPad(600, 150, 372, 150, false, true)
	if menu.controls.PadRadius != 32 || menu.padDragging {
		t.Fatal("released drag changed radius")
	}
}
func TestOptionsDefaultRestoresControlsAndAudio(t *testing.T) {
	menu := newOptionsMenu(false, false)
	menu.compact = true
	menu.controls = optionsControls{PadRadius: 80}
	calls := 0
	menu.activate(optionsDefault, func(enabled bool) {
		if !enabled {
			t.Fatal("music disabled")
		}
		calls++
	})
	if !menu.sound || !menu.music || menu.controls != nativeOptionsDefaults(true) || calls != 1 {
		t.Fatalf("default %+v calls %d", menu, calls)
	}
}
func TestOptionsSettingsJSONRoundTrip(t *testing.T) {
	menu := newOptionsMenu(false, true)
	menu.activate(optionsInvert, nil)
	menu.activate(optionsLeftFixed, nil)
	menu.controls.PadRadius = 63.5
	data, err := json.Marshal(menu.Settings())
	if err != nil {
		t.Fatal(err)
	}
	var settings optionsSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	restored := newOptionsMenu(true, false)
	if err := restored.RestoreSettings(settings); err != nil {
		t.Fatal(err)
	}
	if restored.Settings() != menu.Settings() {
		t.Fatalf("round trip %s", data)
	}
	bindings := restored.StickBindings()
	if bindings[0].Movement || bindings[0].Floating || !bindings[1].Movement || !bindings[1].Floating || bindings[1].Radius != 63.5 {
		t.Fatalf("bindings %+v", bindings)
	}
}
func TestOptionsInvalidSettingsLeaveStateUnchanged(t *testing.T) {
	menu := newOptionsMenu(true, true)
	original := menu.Settings()
	for _, radius := range []float32{0, 31, 81, float32(math.NaN()), float32(math.Inf(1))} {
		settings := original
		settings.PadRadius = radius
		if menu.RestoreSettings(settings) == nil || menu.Settings() != original {
			t.Fatalf("accepted %v", radius)
		}
	}
}
func TestOptionsNativeFixedOrigin(t *testing.T) {
	for _, test := range []struct {
		radius float32
		right  bool
		x, y   float32
	}{{56, false, 100, 440}, {56, true, 860, 440}, {120, false, 120, 420}, {120, true, 840, 420}} {
		x, y := nativeOptionsFixedOrigin(960, 540, test.radius, test.right)
		if x != test.x || y != test.y {
			t.Fatalf("origin %v,%v want %v,%v", x, y, test.x, test.y)
		}
	}
}
func TestOptionsNativeInputNormalization(t *testing.T) {
	x, y, fx, fy := nativeOptionsStickInput(0, 7, 56, 1, true)
	if x != 0 || y != 0 || fx != 0 || fy != 0 {
		t.Fatal("dead zone")
	}
	x, y, fx, fy = nativeOptionsStickInput(0, 36.4, 56, 1, true)
	if math.Abs(float64(y)-.5) > 1e-6 || x != 0 || fx != 0 || fy != 0 {
		t.Fatalf("analog %v,%v follow %v,%v", x, y, fx, fy)
	}
	x, y, fx, fy = nativeOptionsStickInput(0, 100, 56, .5, true)
	if x != 0 || y != 1 || fx != 0 || fy != 30 {
		t.Fatal("floating follow")
	}
	_, _, fx, fy = nativeOptionsStickInput(0, 100, 56, .5, false)
	if fx != 0 || fy != 0 {
		t.Fatal("fixed origin followed")
	}
}
