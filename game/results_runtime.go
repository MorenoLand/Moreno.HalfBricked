package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// openDeathResults shows the TV results screen when the last life is lost in
// story mode; its second button retries the level.
func (a *app) openDeathResults() {
	if a.resultsScreen != nil || a.play == nil {
		return
	}
	kills := int32(a.play.levelKills)
	data := resultsData{Dead: true, Flags: nativeCatalogLevelFlags(a.play.levelInfo.Flags), Score: int32(a.play.score), LevelStartScore: int32(a.play.levelStartScore), Kills: &kills}
	best := a.recordHighscore(a.play.levelInfo.ID, data)
	data.Highscore = &best
	a.resultsScreen = newResultsMenu(data)
	a.page = 5
	a.stopWeaponPlayback()
}

func (a *app) openLevelResults() {
	if a.resultsScreen != nil || a.play == nil {
		return
	}
	kills := int32(a.play.levelKills)
	data := resultsData{Survival: a.mode == 1, Flags: nativeCatalogLevelFlags(a.play.levelInfo.Flags), Score: int32(a.play.score), LevelStartScore: int32(a.play.levelStartScore), Kills: &kills}
	best := a.recordHighscore(a.play.levelInfo.ID, data)
	data.Highscore = &best
	a.resultsScreen = newResultsMenu(data)
	a.page = 5
	a.stopWeaponPlayback()
}

// recordHighscore stores the best score per level and returns it.
func (a *app) recordHighscore(levelID string, data resultsData) int32 {
	score := data.Score
	if !data.Survival {
		score -= data.LevelStartScore
	}
	if a.highscores == nil {
		a.highscores = map[string]int32{}
	}
	best := a.highscores[levelID]
	if score > best {
		best = score
		a.highscores[levelID] = best
		_ = a.savePlayerProfile()
	}
	return best
}

func (a *app) updateResultsMenu() error {
	menu := a.resultsScreen
	pressed := inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	if pressed {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			menu.activate(resultsMainMenu)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if menu.Data.replays() {
				menu.activate(resultsReplay)
			} else {
				menu.activate(resultsContinue)
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			x, y := a.pointer()
			menu.activate(menu.hit(a.variables, float64(x), float64(y)))
		}
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
