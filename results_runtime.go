package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (a *app) openLevelResults() {
	if a.resultsScreen != nil || a.play == nil {
		return
	}
	kills := int32(a.play.levelKills)
	a.resultsScreen = newResultsMenu(resultsData{Survival: a.mode == 1, Flags: nativeCatalogLevelFlags(a.play.levelInfo.Flags), Score: int32(a.play.score), LevelStartScore: int32(a.play.levelStartScore), Kills: &kills})
	a.page = 5
	a.stopWeaponPlayback()
}
func (a *app) updateResultsMenu() error {
	menu := a.resultsScreen
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		menu.activate(resultsMainMenu)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		if menu.Data.Survival {
			menu.activate(resultsReplay)
		} else {
			menu.activate(resultsContinue)
		}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		x, y := a.pointer()
		menu.activate(menu.hit(a.variables, float64(x), float64(y)))
	}
	action := menu.update(1.0/60.0, false)
	if action == resultsNone {
		return nil
	}
	a.resultsScreen = nil
	switch action {
	case resultsMainMenu:
		if a.play != nil {
			a.play.closeScript()
		}
		a.play, a.page = nil, 0
		a.setMenuMusic()
		return a.savePlayerProfile()
	case resultsReplay:
		if a.play != nil {
			a.play.closeScript()
		}
		a.page = 2
		return a.openPlay()
	case resultsContinue:
		a.page = 2
		return a.continueStoryLevel(a.play.levelInfo)
	}
	return nil
}
