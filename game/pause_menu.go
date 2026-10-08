package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
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
		a.play.paused = false
	case 1:
		a.page = 3
	case 2:
		a.statsScreen, a.page = newStatsMenu(&a.statistics), 4
	case 3:
		a.askConfirm("QUIT TO MENU?", a.quitToMenu)
	case 4:
		a.askConfirm("QUIT TO DESKTOP?", func() error { return ebiten.Termination })
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
	return a.savePlayerProfile()
}
func (a *app) updatePauseMenu() error {
	if a.pauseOnline {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyP) {
			a.pauseOnline = false
		} else if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.pointer()
			a.activatePauseOnline(pauseRowHit(x, y, len(pauseOnlineLabels)))
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		return a.activatePauseMenu(0)
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		return a.activatePauseMenu(pauseMenuHit(x, y))
	}
	return nil
}
func (a *app) drawPauseMenu(screen *ebiten.Image) {
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
	a.textCentered(screen, title, 78, 1)
	if label := a.pauseLevelLabel(); label != "" && !a.pauseOnline {
		a.textCentered(screen, label, 56, .5)
	}
	pointerX, pointerY := a.pointer()
	hoveredRow := pauseRowHit(pointerX, pointerY, len(labels))
	for index, label := range labels {
		amount := a.hoverAmount("pause"+label, index == hoveredRow && a.confirm == nil)
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
	return a.play.levelInfo.DisplayName
}
