package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// The 1.2.5 OptionsScreen (HD cache only), drawn from OptionsScreen.uiscreen. Four animated roots
// (each AnimateIn 500 ms): ControlSetup (the Controls window: heading, Normal/Invert and
// Visible/Hidden, the two stick icons and the Fixed/Floating pairs), BottomRightBox (Default and
// Back), TopRightBox (sound and music buttons) and SizeControl (the pad-size preview, centred at
// (390, 150) at rest). The Overscan window is parked below the screen (y 380) for TV builds and
// is not shown. A label of the current choice is written in (100,228,100); the other of each
// pair in (127,127,127) -- the screen itself styles Btn_Overscan_On/Off that way.
//
// Port additions kept: the EXTRA page link (moved to the free strip under the window) and the
// pad-radius drag with the existing clamps (options_control.go).
type optionsLayout struct {
	roots                     [4]*formats.UIComponent // ControlSetup, BottomRightBox, TopRightBox, SizeControl
	actions                   map[*formats.UIComponent]optionsAction
	rootOf                    map[*formats.UIComponent]*formats.UIComponent
	leftStick, rightStick     *formats.UIComponent
	toggleSound, toggleMusic  *formats.UIComponent
	soundIcon, musicIcon      *formats.UIComponent
	defaultButton, backButton *formats.UIComponent
	size                      *formats.UIComponent
	overscan                  *formats.UIComponent
}

var optionsLabelNames = []struct {
	name   string
	action optionsAction
}{
	{"Normal", optionsNormal}, {"Invert", optionsInvert}, {"Visible", optionsVisible}, {"Hidden", optionsHidden},
	{"Left_Fixed", optionsLeftFixed}, {"Left_Floating", optionsLeftFloating}, {"Right_Fixed", optionsRightFixed}, {"Right_Floating", optionsRightFloating},
}

func (a *app) optionsLayout() *optionsLayout {
	ui := a.nativeScreen("OptionsScreen")
	if ui == nil {
		return nil
	}
	l := &optionsLayout{actions: map[*formats.UIComponent]optionsAction{}, rootOf: map[*formats.UIComponent]*formats.UIComponent{}}
	for i, name := range []string{"ControlSetup", "BottomRightBox", "TopRightBox", "SizeControl"} {
		if l.roots[i] = ui.Find(name); l.roots[i] == nil {
			return nil
		}
	}
	find := func(root int, name string) *formats.UIComponent {
		c := ui.FindIn(l.roots[root].Name, name)
		if c != nil {
			l.rootOf[c] = l.roots[root]
		}
		return c
	}
	l.leftStick, l.rightStick = find(0, "LeftStick"), find(0, "RightStick")
	l.overscan = find(0, "OverscanWindow")
	for _, label := range optionsLabelNames {
		c := find(0, label.name)
		if c == nil {
			return nil
		}
		l.actions[c] = label.action
	}
	l.defaultButton, l.backButton = find(1, "DefaultButton"), find(1, "BackButton")
	l.toggleSound, l.toggleMusic = find(2, "ToggleSound"), find(2, "ToggleMusic")
	l.size = l.roots[3]
	if l.leftStick == nil || l.rightStick == nil || l.defaultButton == nil || l.backButton == nil || l.toggleSound == nil || l.toggleMusic == nil {
		return nil
	}
	l.actions[l.defaultButton], l.actions[l.backButton] = optionsDefault, optionsBack
	l.actions[l.toggleSound], l.actions[l.toggleMusic] = optionsSound, optionsMusic
	for _, toggle := range []*formats.UIComponent{l.toggleSound, l.toggleMusic} {
		for _, child := range toggle.Children {
			if child.Name == "Icon" {
				if toggle == l.toggleSound {
					l.soundIcon = child
				} else {
					l.musicIcon = child
				}
			}
		}
	}
	return l
}

// optionsPadCenter is the centre of the pad-size preview at rest.
func (l *optionsLayout) padCenter() formats.Vec2 {
	x, y := nativeRestAnchor(l.size, l.size)
	return formats.Vec2{X: x, Y: y}
}

// optionsHitNative names the control under a logical point.
func (a *app) optionsHitNative(l *optionsLayout, x, y float64) optionsAction {
	for c, action := range l.actions {
		x0, y0, x1, y1 := nativeRect(c, l.rootOf[c])
		if x >= x0 && x < x1 && y >= y0 && y < y1 {
			return action
		}
	}
	center := l.padCenter()
	half := 80.0 // OPTIONS_SIZE_CENTER_SIZE_VAR 160x160: the largest pad
	if size, ok := a.variables.Vec2Value("OPTIONS_SIZE_CENTER_SIZE_VAR"); ok && size.X > 0 {
		half = size.X / 2
	}
	if math.Abs(x-center.X) <= half && math.Abs(y-center.Y) <= half {
		return optionsPad
	}
	return optionsNone
}

// optionsExtraRect is the click region of the port's EXTRA link.
func (a *app) optionsExtraRect() (x0, y0, x1, y1 float64) {
	if a.optionsLayout() != nil {
		return 318, 305, 418, 320
	}
	return extraButtonRect()
}

func (a *app) optionsActionAt(x, y float64) optionsAction {
	if l := a.optionsLayout(); l != nil {
		return a.optionsHitNative(l, x, y)
	}
	return optionsHit(a.variables, x, y)
}

func (a *app) optionsPadCenter() (formats.Vec2, bool) {
	if l := a.optionsLayout(); l != nil {
		return l.padCenter(), true
	}
	return a.variables.Vec2Value("OPTIONS_SIZE_CENTER_POS_VAR")
}

func (a *app) drawOptionsNative(screen *ebiten.Image, menu *optionsMenu, l *optionsLayout) {
	progress := a.nativeSlide("options", nativeAnimateInDuration(l.roots[0]))
	px, py := a.pointer()
	hover := optionsNone
	if a.confirm == nil {
		hover = a.optionsHitNative(l, float64(px), float64(py))
	}
	moveStick, gunStick := "Textures/Analog_Nub_Move_HD.tex", "Textures/Analog_Nub_Gun_HD.tex"
	leftTexture, rightTexture := moveStick, gunStick
	if !menu.controls.Normal {
		leftTexture, rightTexture = gunStick, moveStick
	}
	override := func(c *formats.UIComponent) nativeOverride {
		switch c {
		case l.overscan:
			return nativeOverride{skip: true}
		case l.leftStick:
			return nativeOverride{texture: leftTexture}
		case l.rightStick:
			return nativeOverride{texture: rightTexture}
		case l.size:
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) { a.drawOptionsPad(target, menu, l, offset) }}
		case l.soundIcon, l.musicIcon:
			// Sound.tex: speaker row below, note row above; crossed-out copy in the right column.
			x, y := 0.0, 0.0
			enabled := menu.sound
			if c == l.soundIcon {
				y = .5
			} else {
				enabled = menu.music
			}
			if !enabled {
				x = .5
			}
			return nativeOverride{texPos: &[2]float64{x, y}}
		}
		action, ok := l.actions[c]
		if !ok {
			return nativeOverride{}
		}
		ov := nativeOverride{}
		if action == hover {
			ov.brighten = .25
		}
		if action >= optionsNormal && action <= optionsRightFloating && c.Texture() == "" {
			if menu.controls.selected(action) {
				ov.rgba = &[4]float64{100, 228, 100, 255}
			} else {
				ov.rgba = &[4]float64{127, 127, 127, 255}
			}
		}
		return ov
	}
	for _, root := range l.roots {
		a.drawNativeTree(screen, root, nativeSlideOffset(root, progress), a.menuTime, override)
	}
}

// drawOptionsPad draws the pad-size preview: the ring (twice the pad radius), the nub and an arrow
// on each side of the ring (arrow.tex, 32x16 from OPTIONS_THUMB_ARROW_SIZE_VAR, centred on the
// ring edge and pointing away from the centre).
func (a *app) drawOptionsPad(screen *ebiten.Image, menu *optionsMenu, l *optionsLayout, offset formats.Vec2) {
	x, y := l.size.Anchor()
	center := formats.Vec2{X: x + offset.X, Y: y + offset.Y}
	radius := float64(menu.controls.PadRadius)
	a.drawOptionsControlTexture(screen, "Common0/Textures/Analog_Back_SD", center, formats.Vec2{X: radius * 2, Y: radius * 2})
	nub := formats.Vec2{X: 48, Y: 48}
	if size, ok := a.variables.Vec2Value("OPTIONS_STICK_SIZE_VAR"); ok {
		nub = size
	}
	a.drawOptionsControlTexture(screen, "Common0/Textures/Analog_Nub_SD", center, nub)
	arrow, err := a.Texture("Common0/Textures/arrow")
	if err != nil {
		return
	}
	size := formats.Vec2{X: 32, Y: 16}
	if value, ok := a.variables.Vec2Value("OPTIONS_THUMB_ARROW_SIZE_VAR"); ok {
		size = value
	}
	bounds := arrow.Bounds()
	for _, side := range []struct{ dx, dy, degrees float64 }{{0, -1, 0}, {1, 0, 90}, {0, 1, 180}, {-1, 0, 270}} {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		options.GeoM.Scale(size.X/float64(bounds.Dx()), size.Y/float64(bounds.Dy()))
		options.GeoM.Rotate(side.degrees * math.Pi / 180)
		options.GeoM.Translate(center.X+side.dx*radius, center.Y+side.dy*radius)
		a.drawImage(screen, arrow, options)
	}
}

// actionComponent is the component that performs an action.
func (l *optionsLayout) actionComponent(action optionsAction) *formats.UIComponent {
	for c, a := range l.actions {
		if a == action {
			return c
		}
	}
	return nil
}
