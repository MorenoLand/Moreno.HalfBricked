package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
)

var pauseMenuLabels = [...]string{"RESUME", "OPTIONS", "STATS", "QUIT TO MENU", "QUIT TO DESKTOP", "ACHIEVEMENTS"}

func pauseMenuHit(x, y int) int {
	if x < 140 || x > 340 {
		return -1
	}
	for index := range pauseMenuLabels {
		if y >= 120+index*30 && y < 145+index*30 {
			return index
		}
	}
	return -1
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
		a.stopWeaponPlayback()
		a.play.closeScript()
		a.play, a.page = nil, 0
		a.setMenuMusic()
		return a.savePlayerProfile()
	case 4:
		return ebiten.Termination
	case 5:
		a.openAchievements()
	}
	return nil
}
func (a *app) updatePauseMenu() error {
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
	a.textCentered(screen, "PAUSED", 85, 1)
	for index, label := range pauseMenuLabels {
		a.textCentered(screen, label, float64(125+index*30), .5)
	}
}
