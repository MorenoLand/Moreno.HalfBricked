package game

import (
	"image"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
	"github.com/hajimehoshi/ebiten/v2"
)

// 1.2.5 MainScreen additions (G15). MainScreen.uiscreen has six zombie groups (menu-zombie-motion-1.2.5 note:
// Play, Achievements, MoreGames, Options, Stats, Quit) plus the News, Privacy and Credits buttons. The port adds the
// Achievements group and the Credits button; MoreGames (platform service), News and Privacy (web) are not added.
//
//   - AchievementsZombie: group (199,60) under ZombieContainer (41,6), 131x151, base scale 0.75, ZombieBack
//     Menu_Zombie_5, Screen (4,25) 110x71 rotation 0, hands 62x35 at (-55,18) rot 90 and 64x34 at (63,22) rot -93,
//     Button (3.1,23.3) 88x57 with the string "ACHIEVEMENTS" in colour (100,228,100). The button list anchor is the
//     Screen centre: group centre (240,66) + (4,25) * 0.75.
//   - The native label is typeset text (the Button_Text atlas has no such word); the port draws it with its bitmap
//     font in the same green, fitted into the 79 units of the other groups' Text component.
//   - CreditsButton (btn_Information, ButtonContainer pos (160,2) 39x40 inside BottomRightButton): the container's
//     docking is UNRESOLVED, so the icon sits flush with the bottom-right corner (PORT PLACEMENT). It opens the
//     credits state and returns to the main menu.
const (
	menuActionAchievements = 5
	menuAchievementsZombie = 5
	achievementsLabelFit   = 79.0
)

var achievementsLabelGreen = [3]float32{100.0 / 255, 228.0 / 255, 100.0 / 255}

// creditsIconRect is the Credits button's hit rectangle.
func creditsIconRect() image.Rectangle {
	return image.Rect(logicalWidth-39, logicalHeight-40, logicalWidth, logicalHeight)
}

// mainMenuButtonAvailable reports whether a main menu button exists in the loaded pack: the Achievements group needs
// MainScreen.uiscreen (the 1.2.5 menu); the older cache has none.
func (a *app) mainMenuButtonAvailable(index int) bool {
	if index < 0 || index >= len(a.menuButtons()) {
		return false
	}
	if action := a.menuButtons()[index].action; action == menuActionAchievements || action == menuActionLanguages {
		if action == menuActionLanguages && !languagesEnabled {
			return false
		}
		return a.mainMenuLayout(action) != nil
	}
	return true
}

// creditsButtonAvailable reports whether the main menu shows the Credits button.
func (a *app) creditsButtonAvailable() bool {
	return a != nil && a.pack != nil && a.page == 0 && a.menuClick == nil && a.mainMenuLayout(menuActionAchievements) != nil && a.creditsUIScreen() != nil
}

// openCreditsFromMenu starts the credits state from the main menu; leaving it returns to the main menu.
func (a *app) openCreditsFromMenu() error {
	focus := ""
	if controller := a.uiController("CreditsScreen"); controller != nil {
		focus = controller.Default
	}
	a.credits = newCreditsState(creditsExit{}, focus)
	a.credits.backPage = 0
	a.page = creditsPage
	a.playCreditsTrack()
	return nil
}

// drawCreditsButton draws the information icon (top half of btn_Information, the unpressed frame).
func (a *app) drawCreditsButton(screen *ebiten.Image) {
	if !a.creditsButtonAvailable() {
		return
	}
	texture, err := a.Texture("Frontend0/Textures/Btn_Information_SD")
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	frame := texture.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Min.Y+bounds.Dy()/2)).(*ebiten.Image)
	rect := creditsIconRect()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	if px, py := a.pointer(); image.Pt(px, py).In(rect) {
		options.ColorScale.Scale(1.3, 1.3, 1.3, 1) // hover
	}
	options.GeoM.Scale(float64(rect.Dx())/float64(frame.Bounds().Dx()), float64(rect.Dy())/float64(frame.Bounds().Dy()))
	options.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	a.drawImage(screen, frame, options)
}

// hideUnavailableMenuZombies hides the groups the pack does not have.
func (a *app) hideUnavailableMenuZombies() {
	for i := range a.menuZombies {
		if !a.mainMenuButtonAvailable(i) {
			a.menuZombies[i].SetState(menumotion.NativeMenuZombieHidden)
		}
	}
}

// drawAchievementsLabel draws the "ACHIEVEMENTS" Button text on the group's Screen: letter-spaced
// capitals in the Text component's green, fitted into its 79 x 23 units like the original's sign labels.
func (a *app) drawAchievementsLabel(screen *ebiten.Image, layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion) {
	const text = "ACHIEVEMENTS"
	const supersample = 4 // the label is typeset at four times its size so it survives the group's scaling
	w, h := achievementsLabelFit, 23.0
	if a.menuAchLabel == nil {
		a.menuAchLabel = ebiten.NewImage(int(w)*supersample, int(h)*supersample)
		saveX, saveY := a.frontendScaleX, a.frontendScaleY
		a.frontendScaleX, a.frontendScaleY = supersample, supersample
		a.drawNativeSans(a.menuAchLabel, text, w/2, h/2, w, "", 14, 100, 228, 100, nativeTextStyle{bold: true, spacing: .08, em: 1})
		a.frontendScaleX, a.frontendScaleY = saveX, saveY
	}
	bounds := a.menuAchLabel.Bounds()
	label := menuGroupChild{x: layout.button.x, y: layout.button.y, w: w, h: h}
	a.drawImage(screen, a.menuAchLabel, menuGroupChildOptions(layout, motion, label, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, 1, 1))
}
