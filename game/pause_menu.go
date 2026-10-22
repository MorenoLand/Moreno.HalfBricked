package game

import (
	"math"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
)

var pauseMenuLabels = [...]string{"RESUME", "OPTIONS", "STATS", "QUIT TO MENU", "QUIT TO DESKTOP", "ACHIEVEMENTS", "JOIN / HOST"}

// pauseMenuOrder is the top-to-bottom order of the rows as action indices: the
// quit entries sit at the bottom.
var pauseMenuOrder = [...]int{0, 1, 2, 5, 6, 3, 4}

var pauseOnlineLabels = [...]string{"HOST ONLINE", "JOIN ONLINE", "BACK"}

const (
	pauseMenuTop    = 112
	pauseMenuStride = 26
)

func pauseMenuHit(x, y int) int {
	row := pauseRowHit(x, y, len(pauseMenuOrder))
	if row < 0 {
		return -1
	}
	return pauseMenuOrder[row]
}

func pauseRowHit(x, y, rows int) int {
	if x < 140 || x > 340 {
		return -1
	}
	for index := 0; index < rows; index++ {
		if y >= pauseMenuTop+index*pauseMenuStride && y < pauseMenuTop+index*pauseMenuStride+22 {
			return index
		}
	}
	return -1
}

// activatePauseOnline runs the JOIN / HOST submenu.
func (a *app) activatePauseOnline(index int) {
	switch index {
	case 0:
		a.pauseOnline = false
		a.startNetHost()
	case 1:
		a.pauseOnline = false
		a.startNetJoin()
	case 2:
		a.pauseOnline = false
	}
}
func (a *app) activatePauseMenu(index int) error {
	switch index {
	case 0:
		return a.closeMenuAnimated("pause", func() error { a.play.paused = false; return nil })
	case 1:
		a.page = 3
	case 2:
		a.statsScreen, a.page = newStatsMenuBuild(&a.statistics, a.statsWestern()), 4
	case 3:
		a.askConfirm("QUIT TO MENU?", a.quitToMenu)
	case 4:
		a.askNativeQuit(platform.QuitGame)
	case 5:
		a.openAchievements()
	case 6:
		a.pauseOnline = true
	}
	return nil
}
// quitToMenu leaves the level for the main menu (after the player confirmed).
func (a *app) quitToMenu() error {
	a.stopWeaponPlayback()
	a.play.closeScript()
	a.play, a.page = nil, 0
	a.setMenuMusic()
	a.editorReturnFromTest() // a level started from the editor goes back to it
	return a.savePlayerProfile()
}
func (a *app) updatePauseMenu() error {
	pausePadFocus.touch(padBackend.Tick())
	if a.pauseOnline {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && a.pauseFixture() != nil {
			x, y := a.pointer()
			if which := a.pauseToggleAt(float64(x), float64(y)); which != 0 {
				return a.togglePauseAudio(which)
			}
		}
		if uiKeyJustPressed(ebiten.KeyEscape) || uiKeyJustPressed(ebiten.KeyP) {
			a.pauseOnline = false
			pausePadFocus.reset()
		} else if move := menuFocusMove(); move != 0 {
			if pausePadFocus.step(move, len(pauseOnlineLabels)) {
				a.playControllerMove()
			}
		} else if uiKeyJustPressed(ebiten.KeyEnter) && pausePadFocus.active {
			row := pausePadFocus.row
			pausePadFocus.reset()
			a.activatePauseOnline(row)
		} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			pausePadFocus.reset()
			x, y := a.pointer()
			a.activatePauseOnline(pauseRowHit(x, y, len(pauseOnlineLabels)))
		}
		return nil
	}
	if uiKeyJustPressed(ebiten.KeyP) {
		return a.activatePauseMenu(0)
	}
	if handled, err := a.updatePauseToggles(); handled || err != nil {
		return err
	}
	// Controller focus (PauseScreen.txt: the resume node is the default, Up/Down walk the rows).
	if move := menuFocusMove(); move != 0 {
		if pausePadFocus.step(move, len(pauseMenuOrder)) {
			a.playControllerMove()
		}
		return nil
	}
	if uiKeyJustPressed(ebiten.KeyEnter) && pausePadFocus.active {
		action := pauseMenuOrder[pausePadFocus.row]
		pausePadFocus.reset()
		return a.activatePauseMenu(action)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		pausePadFocus.reset()
		x, y := a.pointer()
		return a.activatePauseMenu(pauseMenuHit(x, y))
	}
	return nil
}
func (a *app) drawPauseMenu(screen *ebiten.Image) {
	if n := &a.native; n.closeKey == "pause" && n.slideDur["pause"] > 0 {
		// Closing: the fixture slides out (nativeSlide runs backwards) while the dim, title and rows fade with it.
		fade := math.Max(0, math.Min(1, 1-(a.menuTime-n.closeStart)/n.slideDur["pause"]))
		layer := ebiten.NewImage(screen.Bounds().Dx(), screen.Bounds().Dy())
		a.drawPauseMenuBody(layer)
		options := &ebiten.DrawImageOptions{}
		options.ColorScale.ScaleAlpha(float32(fade))
		screen.DrawImage(layer, options)
		layer.Deallocate()
		return
	}
	a.drawPauseMenuBody(screen)
}

func (a *app) drawPauseMenuBody(screen *ebiten.Image) {
	a.drawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 160})
	var labels []string
	title := "PAUSED"
	if a.pauseOnline {
		labels, title = pauseOnlineLabels[:], "JOIN / HOST"
	} else {
		for _, action := range pauseMenuOrder {
			labels = append(labels, pauseMenuLabels[action])
		}
	}
	pointerX, pointerY := a.pointer()
	hoveredToggle := 0
	if a.confirm == nil {
		hoveredToggle = a.pauseToggleAt(float64(pointerX), float64(pointerY))
	}
	a.drawPauseFixture(screen, hoveredToggle, pausePadFocus.toggle)
	a.textCentered(screen, title, 78, 1)
	if label := a.pauseLevelLabel(); label != "" && !a.pauseOnline {
		if a.pauseFixture() != nil {
			// The native sound/music fixture sits where the label used to be: the title wraps into the room left of it
			// (the fixture starts at x ~ 163) instead of running underneath.
			for i, line := range a.wrapToWidth(label, 148, .42) {
				a.text(screen, line, 8, 6+float64(i)*12, .42)
			}
		} else {
			a.textCentered(screen, label, 56, .5)
		}
	}
	hoveredRow := pauseRowHit(pointerX, pointerY, len(labels))
	for index, label := range labels {
		focused := pausePadFocus.active && index == pausePadFocus.row
		amount := a.hoverAmount("pause"+label, (index == hoveredRow || focused) && a.confirm == nil)
		a.drawHoverText(screen, label, logicalWidth/2, float64(pauseMenuTop+5+index*pauseMenuStride), .5, amount)
	}
}

// pauseLevelLabel names where the player is: the level's own title, or Survival.
func (a *app) pauseLevelLabel() string {
	if a.play == nil {
		return ""
	}
	if a.play.isSurvival() {
		return "Survival"
	}
	return levelTextFlat(a.play.levelInfo.DisplayName, " ")
}
