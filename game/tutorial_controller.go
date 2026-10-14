package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Tutorial_Controller panel (1.2.5 only). World0Level0's entry script shows it through the BrickUI_* callbacks
// when HasControllerAttached() is true:
//
//	BrickUI_DisplayScreen / BrickUI_PlayAnimation("AnimateIn", "AnimateIn_BackBox") / BrickUI_SetupTutorialScreen /
//	Controller_OnEnterScreen, then Idle() while BrickUI_GetPropertyBool("IsDisplayed") is true, then
//	Controller_OnLeaveScreen / BrickUI_RemoveScreen.
//
// Native evidence (1.2.5 libmortargame.so): SetupTutorialScreen is FUN_000e1c40. It enables exactly one of the nine
// Controller_* visuals (index FUN_000df950(0) of the connected controller type) and writes the matching title
// string into ControllerTitle (table 0x008e70dc: Moga ACE Power, Moga Pro Controller, Moga HERO Power, Moga
// Controller, Basic Controller, Extended Controller, Game Controller, Samsung Controller, ...). AnimateIn's first
// event sets IsDisplayed true, AnimateOut's last event sets it false, and BackButton's OnButtonPressed plays the
// AnimateOut pair (Tutorial_Controller.txt: BackButton IsDefault IsBack).
//
// PORT CHOICES (labelled, not native): the desktop port has no MOGA/Apple/Kindle detection, so the panel shows
// the Android_Generic visual ("Game Controller", index 6); the slide animations are collapsed to an instant
// show/hide (the keyframe times are not decoded); the Backing nine patch is a plain translucent rectangle.
type tutorialControllerPanel struct {
	shown       bool // BrickUI_DisplayScreen without BrickUI_RemoveScreen
	displayed   bool // the IsDisplayed property
	backBoxIn   bool // AnimateIn_BackBox played
	setup       bool // BrickUI_SetupTutorialScreen ran
	entered     bool // Controller_OnEnterScreen ran (the Back button owns the focus)
	controller  int  // index of the Controller_* visual enabled by the setup
	loadedFor   any
	screen      *formats.UIScreen
	screenTried bool
}

const (
	tutorialScreenName       = "Tutorial_Controller"
	tutorialAndroidGeneric   = 6
	tutorialBackBoxX         = 400.0
	tutorialBackBoxY         = 269.0
	tutorialContainerCenterY = 160.0
)

var tutorialControllerVisuals = []string{"Controller_Moga_ACE_Power", "Controller_Moga_PRO_Power", "Controller_Moga_HERO_Power", "Controller_Moga_Pocket", "Controller_Apple_Generic_Basic", "Controller_Apple_Generic_Advanced", "Controller_Android_Generic", "Controller_Android_Samsung_Generic", "Controller_Android_Kindle"}

var tutorialControllerTitles = []string{"Moga ACE Power", "Moga Pro Controller", "Moga HERO Power", "Moga Controller", "Basic Controller", "Extended Controller", "Game Controller", "Samsung Controller"}

func tutorialCheck(screen string) error {
	if screen != tutorialScreenName {
		return fmt.Errorf("BrickUI screen %q is not shipped by this port (only %s)", screen, tutorialScreenName)
	}
	return nil
}

func (t *tutorialControllerPanel) DisplayScreen(screen string) error {
	if err := tutorialCheck(screen); err != nil {
		return err
	}
	t.shown = true
	return nil
}

func (t *tutorialControllerPanel) PlayAnimation(screen, animation string) error {
	if err := tutorialCheck(screen); err != nil {
		return err
	}
	switch animation {
	case "AnimateIn":
		t.displayed = true
	case "AnimateOut":
		t.displayed = false
	case "AnimateIn_BackBox":
		t.backBoxIn = true
	case "AnimateOut_BackBox":
		t.backBoxIn = false
	default:
		return fmt.Errorf("%s has no animation %q", screen, animation)
	}
	return nil
}

func (t *tutorialControllerPanel) BoolProperty(screen, property string) (bool, bool, error) {
	if err := tutorialCheck(screen); err != nil {
		return false, false, err
	}
	if property == "IsDisplayed" {
		return t.displayed, true, nil
	}
	return false, false, nil
}

func (t *tutorialControllerPanel) RemoveScreen(screen string) error {
	if err := tutorialCheck(screen); err != nil {
		return err
	}
	*t = tutorialControllerPanel{}
	return nil
}

func (t *tutorialControllerPanel) SetupTutorialScreen() error {
	t.setup = true
	t.controller = tutorialAndroidGeneric
	return nil
}

func (t *tutorialControllerPanel) ControllerEnterScreen(screen string) error {
	if err := tutorialCheck(screen); err != nil {
		return err
	}
	t.entered = true
	return nil
}

func (t *tutorialControllerPanel) ControllerLeaveScreen(screen string) error {
	if err := tutorialCheck(screen); err != nil {
		return err
	}
	t.entered = false
	return nil
}

// pressBack is the BackButton's OnButtonPressed: it plays AnimateOut and AnimateOut_BackBox.
func (t *tutorialControllerPanel) pressBack() {
	t.displayed = false
	t.backBoxIn = false
}

func (t *tutorialControllerPanel) visible() bool { return t != nil && t.shown && t.displayed }

// tutorialBackRect is the BackButton's hit rectangle (BackBox 400,269 + BackButton -1,-13, 75x45).
func tutorialBackRect() image.Rectangle {
	cx, cy := tutorialBackBoxX-1, tutorialBackBoxY-13
	return image.Rect(int(cx-37.5), int(cy-22.5), int(cx+37.5), int(cy+22.5))
}

// updateTutorialPanel presses the Back button: Enter / Space / Escape, the pad confirm/back button or a click on
// the button (Back is the default and only button).
func (a *app) updateTutorialPanel() {
	if a.play == nil || !a.play.tutorial.visible() {
		return
	}
	pressed := uiKeyJustPressed(ebiten.KeyEnter) || uiKeyJustPressed(ebiten.KeyKPEnter) || uiKeyJustPressed(ebiten.KeySpace) || uiKeyJustPressed(ebiten.KeyEscape)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		pressed = pressed || image.Pt(x, y).In(tutorialBackRect())
	}
	if pressed {
		a.play.tutorial.pressBack()
	}
}

func (a *app) tutorialScreen() *formats.UIScreen {
	t := &a.play.tutorial
	if t.screenTried && t.loadedFor == any(a.pack) {
		return t.screen
	}
	t.screenTried, t.loadedFor, t.screen = true, any(a.pack), nil
	if a.pack == nil {
		return nil
	}
	path, ok := a.pack.SourcePath("Common0/UserInterface/screens/Tutorial_Controller.uiscreen")
	if !ok {
		return nil
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil
	}
	if screen, err := formats.ParseUIScreen(data); err == nil {
		t.screen = screen
	}
	return t.screen
}

func (a *app) drawTutorialPanel(screen *ebiten.Image) {
	if a.play == nil || !a.play.tutorial.visible() {
		return
	}
	t := &a.play.tutorial
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 150})
	ui := a.tutorialScreen()
	if ui == nil {
		return
	}
	container := ui.FindIn(tutorialScreenName, "Container")
	if container == nil {
		container = ui.Find("Container")
	}
	if container == nil {
		return
	}
	_, cy := container.Position()
	offset := formats.Vec2{X: 0, Y: tutorialContainerCenterY - cy}
	for _, child := range container.Children {
		switch child.Class {
		case "ComponentTexture":
			a.drawUITexture(screen, child, offset)
		case "ComponentWindow":
			x, y := uiCenter(child, offset, 0, 0)
			w, _ := child.Width()
			h, _ := child.Height()
			a.drawRect(screen, x-w/2, y-h/2, w, h, color.RGBA{10, 20, 12, 200})
		case "ComponentText":
			a.drawTutorialText(screen, child, offset, tutorialControllerTitles[t.controller])
		}
	}
	name := tutorialControllerVisuals[t.controller]
	for _, child := range container.Children {
		if child.Name != name || child.Class != "ComponentVisual" {
			continue
		}
		for _, part := range child.Children {
			switch part.Class {
			case "ComponentTexture":
				a.drawUITexture(screen, part, offset)
			case "ComponentText":
				a.drawTutorialText(screen, part, offset, "")
			}
		}
		break
	}
	if t.backBoxIn {
		rect := tutorialBackRect()
		a.drawRect(screen, float64(rect.Min.X), float64(rect.Min.Y), float64(rect.Dx()), float64(rect.Dy()), color.RGBA{30, 60, 30, 230})
		a.drawUIText(screen, "Okay", float64(rect.Min.X+rect.Dx()/2), float64(rect.Min.Y+rect.Dy()/2), float64(rect.Dx()), "", 16, formats.Vec2{})
	}
}

func (a *app) drawTutorialText(screen *ebiten.Image, component *formats.UIComponent, offset formats.Vec2, override string) {
	text := component.String("text")
	if override != "" {
		text = override
	}
	size, ok := component.FontSize()
	if !ok || size <= 0 {
		size = 12
	}
	x, y := uiCenter(component, offset, 0, 0)
	width, _ := component.Width()
	a.drawUIText(screen, text, x, y, width, component.String("alignment"), size, formats.Vec2{})
}
