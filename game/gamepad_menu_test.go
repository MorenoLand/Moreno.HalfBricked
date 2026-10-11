package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/hajimehoshi/ebiten/v2"
)

// fakePads drives padNav without a window.
type fakePads struct {
	tick    int64
	ids     []ebiten.GamepadID
	buttons map[ebiten.StandardGamepadButton]bool
	axes    map[ebiten.StandardGamepadAxis]float64
}

func newFakePads() *fakePads {
	return &fakePads{ids: []ebiten.GamepadID{0}, buttons: map[ebiten.StandardGamepadButton]bool{}, axes: map[ebiten.StandardGamepadAxis]float64{}}
}

func (f *fakePads) Connected() []ebiten.GamepadID { return f.ids }
func (f *fakePads) Button(_ ebiten.GamepadID, b ebiten.StandardGamepadButton) bool {
	return f.buttons[b]
}
func (f *fakePads) Axis(_ ebiten.GamepadID, a ebiten.StandardGamepadAxis) float64 { return f.axes[a] }
func (f *fakePads) Tick() int64                                                   { return f.tick }

// useFakePads installs the fake backend and a fresh edge detector for one test.
func useFakePads(t *testing.T) *fakePads {
	t.Helper()
	fake := newFakePads()
	oldBackend, oldNav, oldNotice, oldFocus := padBackend, padNav, padNotice, pausePadFocus
	padBackend, padNav, padNotice, pausePadFocus = fake, padNavState{}, controllerNoticeState{}, padMenuFocus{}
	t.Cleanup(func() { padBackend, padNav, padNotice, pausePadFocus = oldBackend, oldNav, oldNotice, oldFocus })
	return fake
}

func TestPadDpadIsAnEdgeNotAHold(t *testing.T) {
	pads := useFakePads(t)
	if uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("idle pad reported a press")
	}
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonLeftBottom] = true
	if !uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("d-pad down was not reported as KeyDown")
	}
	if !uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("the press must stay visible for every query of the same tick")
	}
	pads.tick++
	if uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("a held d-pad repeated")
	}
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonLeftBottom] = false
	uiKeyJustPressed(ebiten.KeyDown)
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonLeftBottom] = true
	if !uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("a second press was not reported")
	}
}

func TestPadStickPicksTheDominantDirection(t *testing.T) {
	pads := useFakePads(t)
	pads.tick++
	pads.axes[ebiten.StandardGamepadAxisLeftStickHorizontal] = .9
	pads.axes[ebiten.StandardGamepadAxisLeftStickVertical] = .6
	if !uiKeyJustPressed(ebiten.KeyRight) || uiKeyJustPressed(ebiten.KeyDown) {
		t.Fatal("a diagonal push must press only the dominant axis")
	}
	pads.tick++
	pads.axes[ebiten.StandardGamepadAxisLeftStickHorizontal] = .2
	pads.axes[ebiten.StandardGamepadAxisLeftStickVertical] = -.4
	if uiKeyJustPressed(ebiten.KeyUp) || uiKeyJustPressed(ebiten.KeyRight) {
		t.Fatal("a push below the threshold pressed a direction")
	}
	pads.tick++
	pads.axes[ebiten.StandardGamepadAxisLeftStickVertical] = -.8
	if !uiKeyJustPressed(ebiten.KeyUp) {
		t.Fatal("stick up past the threshold was not KeyUp")
	}
}

func TestPadFaceButtonsMapToConfirmBackAndPause(t *testing.T) {
	pads := useFakePads(t)
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonRightBottom] = true
	if !uiKeyJustPressed(ebiten.KeyEnter) || !uiKeyJustPressed(ebiten.KeyKPEnter) {
		t.Fatal("A is not Enter")
	}
	if uiKeyJustPressed(ebiten.KeyEscape) {
		t.Fatal("A pressed back")
	}
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonRightBottom] = false
	pads.buttons[ebiten.StandardGamepadButtonRightRight] = true
	if !uiKeyJustPressed(ebiten.KeyEscape) {
		t.Fatal("B is not back")
	}
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonRightRight] = false
	pads.buttons[ebiten.StandardGamepadButtonCenterRight] = true
	if !uiKeyJustPressed(ebiten.KeyEscape) {
		t.Fatal("Start does not open and close the pause menu")
	}
	if uiKeyJustPressed(ebiten.KeyP) {
		t.Fatal("an unmapped key was reported")
	}
}

func TestMainMenuNavKeyFollowsThePad(t *testing.T) {
	pads := useFakePads(t)
	pads.tick++
	pads.buttons[ebiten.StandardGamepadButtonLeftRight] = true
	if got := mainMenuNavKey(); got != "Right" {
		t.Fatalf("pad right gave %q", got)
	}
}

func TestPauseMenuPadFocusWalksRowsWithoutWrapping(t *testing.T) {
	var focus padMenuFocus
	if !focus.step(1, 7) || !focus.active || focus.row != 0 {
		t.Fatalf("first press must focus the default node: %+v", focus)
	}
	for i := 0; i < 10; i++ {
		focus.step(1, 7)
	}
	if focus.row != 6 {
		t.Fatalf("row %d, want the last row (no wrap)", focus.row)
	}
	if focus.step(1, 7) {
		t.Fatal("moving past the last row reported a move")
	}
	for i := 0; i < 10; i++ {
		focus.step(-1, 7)
	}
	if focus.row != 0 {
		t.Fatalf("row %d, want the first row", focus.row)
	}
	focus.step(1, 7)
	focus.touch(10)
	focus.touch(40) // the menu was closed for several ticks
	if focus.active || focus.row != 0 {
		t.Fatalf("a reopened menu kept its focus: %+v", focus)
	}
}

func TestControllerNoticeFollowsShowKeyframes(t *testing.T) {
	for _, c := range []struct{ age, y float64 }{{0, -80}, {.125, -50}, {.25, -20}, {1, -20}, {1.5, -20}, {1.65, -50}} {
		y, visible := controllerNoticeY(c.age)
		if !visible || !near(y, c.y) {
			t.Fatalf("age %v: y %v visible %v, want %v", c.age, y, visible, c.y)
		}
	}
	if _, visible := controllerNoticeY(1.8); visible {
		t.Fatal("the banner is still shown after the animation ended")
	}
}

func TestControllerNoticeShowsOnConnect(t *testing.T) {
	pads := newFakePads()
	pads.ids = nil
	var notice controllerNoticeState
	notice.update(pads, 1.0/60)
	if notice.active {
		t.Fatal("notice shown with no controller")
	}
	pads.ids = []ebiten.GamepadID{3}
	notice.update(pads, 1.0/60)
	if !notice.active {
		t.Fatal("connecting a controller did not show the notice")
	}
	for frame := 0; frame < 60*2; frame++ {
		notice.update(pads, 1.0/60)
	}
	if notice.active {
		t.Fatal("the notice never ended")
	}
	pads.ids = nil
	notice.update(pads, 1.0/60)
	pads.ids = []ebiten.GamepadID{3}
	notice.update(pads, 1.0/60)
	if !notice.active {
		t.Fatal("re-attaching did not show the notice again")
	}
}

func TestFlipPadSticksSwapsMoveAndAim(t *testing.T) {
	in := flipPadSticks(playerInput{moveX: 1, moveY: .5, aimX: -1, aimY: -.25, secondary: true})
	if in.moveX != -1 || in.moveY != -.25 || in.aimX != 1 || in.aimY != .5 || !in.secondary {
		t.Fatalf("%+v", in)
	}
}

func TestShippedControllerGraphsAreAvailable(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		a := &app{pack: pack}
		pause := a.uiController("PauseScreen")
		if pause == nil {
			t.Logf("%s: no controller graphs in this cache", root)
			continue
		}
		if pause.def != "Pause" || pause.back != "Pause" {
			t.Fatalf("%s: PauseScreen default %q back %q", root, pause.def, pause.back)
		}
		if target, ok := pause.neighbour("Pause", "Down"); !ok || target != "Quit" {
			t.Fatalf("%s: Pause Down -> %q", root, target)
		}
		options := a.uiController("OptionsScreen_Controller")
		if options == nil || options.back != "Back" {
			t.Fatalf("%s: OptionsScreen_Controller %+v", root, options)
		}
	}
}

func TestAnalogMoveScaleFollowsNativeVectorRule(t *testing.T) {
	for _, c := range []struct{ length, want float64 }{{0, 0}, {.25, .5}, {.5, 1}, {1, 1}, {1.414, 1}} {
		if got := analogMoveScale(c.length); !near(got, c.want) {
			t.Fatalf("length %v: scale %v want %v", c.length, got, c.want)
		}
	}
}
