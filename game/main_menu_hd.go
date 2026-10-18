package game

import (
	"image"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/menumotion"
	"github.com/hajimehoshi/ebiten/v2"
)

// Port addition (HD cache only), after the iPad build's main menu: the board where the Android 1.2.5 menu has
// More Games becomes LANGUAGES (the MoreGamesZombie group of MainScreen.uiscreen, Menu_Zombie_4), Options takes the
// bottom-left board the Quit zombie held, and Quit is a grey plate in the bottom-left corner. The SD cache keeps
// its four boards. The Android data has no Languages screen or label art: the label is typeset like the
// Achievements one, and the screen is languages.go.
const menuActionLanguages = 6

// languagesEnabled shows the LANGUAGES board. It is off: the build holds no translated game text (the BrickNet string
// tables are store and account dialogs the game never shows), so the board would open a list that changes nothing.
// The board, label and screen (languages.go) stay in place; set this to true when translations exist.
const languagesEnabled = false

var mainMenuButtonsHD = []menuButton{
	{zombie: 4, labelRow: -1, action: menuActionLanguages, cx: 76, cy: 105, width: 128, height: 64, angle: -0.10},
	{zombie: 2, labelRow: 0, action: 0, cx: 210, cy: 194, width: 128, height: 64, angle: 0.40},
	{zombie: 3, labelRow: 8, action: 2, cx: 80, cy: 244, width: 128, height: 64, angle: -0.16},
	{zombie: 2, labelRow: 14, action: 3, cx: 377, cy: 264, width: 128, height: 64, angle: 0.02},
	{zombie: menuAchievementsZombie, labelRow: -1, action: menuActionAchievements, cx: 186, cy: 75, width: 128, height: 64},
}

// mainMenuButtonsHDNoLanguages is the list while languagesEnabled is off: Achievements takes the slot the Languages
// board would hold (top-left), so the menu has no gap; turning languagesEnabled on brings the two-board layout back.
var mainMenuButtonsHDNoLanguages = []menuButton{
	{zombie: menuAchievementsZombie, labelRow: -1, action: menuActionAchievements, cx: 76, cy: 105, width: 128, height: 64, angle: -0.10},
	{zombie: 2, labelRow: 0, action: 0, cx: 210, cy: 194, width: 128, height: 64, angle: 0.40},
	{zombie: 3, labelRow: 8, action: 2, cx: 80, cy: 244, width: 128, height: 64, angle: -0.16},
	{zombie: 2, labelRow: 14, action: 3, cx: 377, cy: 264, width: 128, height: 64, angle: 0.02},
}

// menuButtons is the main menu's board list for the loaded pack.
func (a *app) menuButtons() []menuButton {
	if a != nil && a.pack != nil && a.titleNative() && a.mainMenuLayout(menuActionLanguages) != nil {
		if languagesEnabled {
			return mainMenuButtonsHD
		}
		return mainMenuButtonsHDNoLanguages
	}
	return mainMenuButtons
}

// quitCornerRect is the corner Quit plate (the iPad build's grey plate, about 51x29 units of its 480x320 layout).
func quitCornerRect() image.Rectangle { return image.Rect(0, 288, 58, 320) }

// quitCornerAvailable reports whether the main menu has the corner Quit plate instead of a Quit board.
func (a *app) quitCornerAvailable() bool {
	if a == nil || a.page != 0 || a.menuClick != nil || a.confirm != nil {
		return false
	}
	boards := a.menuButtons()
	return len(boards) > 0 && (&boards[0] == &mainMenuButtonsHD[0] || &boards[0] == &mainMenuButtonsHDNoLanguages[0])
}

func (a *app) drawQuitCorner(screen *ebiten.Image) {
	if !a.quitCornerAvailable() {
		return
	}
	px, py := a.pointer()
	rect := quitCornerRect()
	brighten := float32(0)
	if image.Pt(px, py).In(rect) {
		brighten = .2
	}
	// The plate is drawn larger than the corner and clipped by the screen edge, so only its top-right corner shows.
	a.drawNativeNinePatchRect(screen, "Textures/HUD_ButtonNinePatch.tex", -12, float64(rect.Min.Y), float64(rect.Dx()+12), float64(rect.Dy()+12), brighten)
	shade := 64.0
	a.drawNativeSans(screen, "QUIT", 24, float64(rect.Min.Y)+15, 44, "", 14, shade, shade, shade, nativeTextStyle{bold: true, em: 1.2})
}

// drawLanguagesLabel draws the "LANGUAGES" text of the MoreGames group's sign, like the Achievements label.
func (a *app) drawLanguagesLabel(screen *ebiten.Image, layout *menuGroupLayout, motion menumotion.NativeMenuZombieMotion) {
	const text = "LANGUAGES"
	const supersample = 4
	w, h := achievementsLabelFit, 23.0
	if a.menuLangLabel == nil {
		a.menuLangLabel = ebiten.NewImage(int(w)*supersample, int(h)*supersample)
		saveX, saveY := a.frontendScaleX, a.frontendScaleY
		a.frontendScaleX, a.frontendScaleY = supersample, supersample
		a.drawNativeSans(a.menuLangLabel, text, w/2, h/2, w, "", 14, 100, 228, 100, nativeTextStyle{bold: true, spacing: .08, em: 1})
		a.frontendScaleX, a.frontendScaleY = saveX, saveY
	}
	bounds := a.menuLangLabel.Bounds()
	label := menuGroupChild{x: layout.button.x, y: layout.button.y, w: w, h: h}
	a.drawImage(screen, a.menuLangLabel, menuGroupChildOptions(layout, motion, label, float64(bounds.Dx()), float64(bounds.Dy()), float64(bounds.Dx())/2, float64(bounds.Dy())/2, 1, 1))
}
