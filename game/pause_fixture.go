package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The native PauseScreen (1.2.5) carries a second fixture next to the Quit one:
// OptionsBox, a 161x57 HUD_BoxNinePatch window at (242, 45) with two HUD_ButtonNinePatch
// buttons -- ToggleMusic at (-28, -.5) and ToggleSound at (25.6, -.5), 42x50 and 43x50 --
// each holding a 24x24 Sound.tex icon, plus wires, two connectors and the light/dial decor.
// It slides in from y = -80 in 250 ms (AnimateIn). PauseScreen.txt wires it into the focus
// graph: Pause Up:Sound, Sound Left:Music, Music Right:Sound, both Down:Pause.
// Only this fixture is drawn here; the port keeps its own pause rows (pause_menu.go).
// The buttons act on the same flags as the options page (app.options.sound / .music), so the
// two screens stay in sync and both persist through the profile.
const (
	pauseToggleMusic = 1
	pauseToggleSound = 2
)

// pauseFixture returns the OptionsBox root of the shipped pause screen, nil without it (SD cache).
func (a *app) pauseFixture() *formats.UIComponent {
	screen := a.nativeScreen("PauseScreen")
	if screen == nil {
		return nil
	}
	root := screen.Find("OptionsBox")
	if root == nil || screen.FindIn("OptionsBox", "ToggleMusic") == nil || screen.FindIn("OptionsBox", "ToggleSound") == nil {
		return nil
	}
	return root
}

func (a *app) pauseToggleComponent(which int) *formats.UIComponent {
	screen := a.nativeScreen("PauseScreen")
	if screen == nil {
		return nil
	}
	if which == pauseToggleMusic {
		return screen.FindIn("OptionsBox", "ToggleMusic")
	}
	return screen.FindIn("OptionsBox", "ToggleSound")
}

// pauseToggleAt names the toggle button under a logical point (the fixture at rest), 0 for none.
func (a *app) pauseToggleAt(x, y float64) int {
	root := a.pauseFixture()
	if root == nil {
		return 0
	}
	for _, which := range []int{pauseToggleMusic, pauseToggleSound} {
		x0, y0, x1, y1 := nativeRect(a.pauseToggleComponent(which), root)
		if x >= x0 && x < x1 && y >= y0 && y < y1 {
			return which
		}
	}
	return 0
}

// pauseToggleState reports whether the effect (sound) or the music is on.
func (a *app) pauseToggleState(which int) bool {
	if which == pauseToggleMusic {
		return a.options.music
	}
	return a.options.sound
}

// togglePauseAudio flips one of the two flags exactly as the options page does and saves.
func (a *app) togglePauseAudio(which int) error {
	var musicEnabled func(bool)
	if a.sound != nil {
		musicEnabled = func(enabled bool) { a.sound.MusicEnabled(enabled && !a.silent) }
	}
	if which == pauseToggleMusic {
		a.options.activate(optionsMusic, musicEnabled)
	} else {
		a.options.activate(optionsSound, musicEnabled)
	}
	return a.savePlayerProfile()
}

// drawPauseFixture draws the fixture; hovered and focused are the toggle ids to lift.
func (a *app) drawPauseFixture(screen *ebiten.Image, hovered, focused int) {
	root := a.pauseFixture()
	if root == nil {
		return
	}
	offset := nativeSlideOffset(root, a.nativeSlide("pause", nativeAnimateInDuration(root)))
	icons := map[*formats.UIComponent]int{}
	for _, which := range []int{pauseToggleMusic, pauseToggleSound} {
		button := a.pauseToggleComponent(which)
		icons[button] = which
		for _, child := range button.Children {
			icons[child] = which
		}
	}
	a.drawNativeTree(screen, root, offset, a.menuTime, func(c *formats.UIComponent) nativeOverride {
		which, ok := icons[c]
		if !ok {
			return nativeOverride{}
		}
		ov := nativeOverride{}
		if which == hovered || which == focused {
			ov.brighten = .25
		}
		if c.Name == "Icon" {
			// Sound.tex is a 2x2 sheet: the left column is the icon, the right column the crossed-out one.
			x, y, _, _ := c.TexCoords()
			if !a.pauseToggleState(which) {
				x = .5
			}
			ov.texPos = &[2]float64{x, y}
		}
		return ov
	})
}

// updatePauseToggles handles the fixture's input: a click on a button, and the pad/keyboard
// focus of PauseScreen.txt (Up from the first row reaches Sound, Left/Right swap between the
// buttons, Down returns to the rows, Enter presses the focused button). It reports whether it
// consumed the tick.
func (a *app) updatePauseToggles() (bool, error) {
	if a.pauseFixture() == nil {
		return false, nil
	}
	f := &pausePadFocus
	if f.toggle != 0 {
		switch {
		case uiKeyJustPressed(ebiten.KeyLeft):
			if f.toggle == pauseToggleSound {
				f.toggle = pauseToggleMusic
				a.playControllerMove()
			}
			return true, nil
		case uiKeyJustPressed(ebiten.KeyRight):
			if f.toggle == pauseToggleMusic {
				f.toggle = pauseToggleSound
				a.playControllerMove()
			}
			return true, nil
		case uiKeyJustPressed(ebiten.KeyDown):
			f.toggle, f.row, f.active = 0, 0, true
			a.playControllerMove()
			return true, nil
		case uiKeyJustPressed(ebiten.KeyUp):
			return true, nil
		case uiKeyJustPressed(ebiten.KeyEnter):
			return true, a.togglePauseAudio(f.toggle)
		}
		return false, nil
	}
	if f.active && f.row == 0 && uiKeyJustPressed(ebiten.KeyUp) {
		f.toggle = pauseToggleSound
		a.playControllerMove()
		return true, nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		if which := a.pauseToggleAt(float64(x), float64(y)); which != 0 {
			f.reset()
			return true, a.togglePauseAudio(which)
		}
	}
	return false, nil
}
