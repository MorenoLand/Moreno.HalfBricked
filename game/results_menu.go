package game

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

type resultsAction uint8

const (
	resultsNone resultsAction = iota
	resultsMainMenu
	resultsContinue
	resultsReplay
)

type resultsData struct {
	Survival               bool
	Dead                   bool // story game over: Main Menu / Retry
	Flags                  uint32
	Score, LevelStartScore int32
	Highscore, Kills       *int32
}
type resultsMenu struct {
	Data                      resultsData
	Visible                   bool
	phase                     float32
	ready, closing, delivered bool
	pending                   resultsAction
}

func resultsVisible(survival bool, flags uint32) bool { return survival || flags&4 != 0 }

// replays reports whether the second button restarts the level (survival, or after dying).
func (d resultsData) replays() bool { return d.Survival || d.Dead }
func newResultsMenu(data resultsData) *resultsMenu {
	if data.Highscore != nil {
		value := *data.Highscore
		data.Highscore = &value
	}
	if data.Kills != nil {
		value := *data.Kills
		data.Kills = &value
	}
	return &resultsMenu{Data: data, Visible: data.Dead || resultsVisible(data.Survival, data.Flags)}
}
func (menu *resultsMenu) score() int32 {
	if menu.Data.Survival {
		return menu.Data.Score
	}
	return menu.Data.Score - menu.Data.LevelStartScore
}
func (menu *resultsMenu) update(dt float32, externalActive bool) resultsAction {
	if menu.delivered {
		return resultsNone
	}
	if !menu.Visible {
		if externalActive {
			return resultsNone
		}
		menu.delivered = true
		return resultsContinue
	}
	if dt > 0 && (menu.closing || !menu.ready) {
		menu.phase += dt * 3
	}
	if menu.phase > 1 || (menu.closing && menu.phase == 1) {
		menu.phase = 1
		if menu.closing {
			if externalActive {
				return resultsNone
			}
			menu.delivered = true
			return menu.pending
		}
		menu.ready = true
	}
	return resultsNone
}
func (menu *resultsMenu) hit(variables formats.FrontendVariables, x, y float64) resultsAction {
	if !menu.Visible || !menu.ready || menu.closing || menu.delivered {
		return resultsNone
	}
	for _, button := range []struct {
		name   string
		action resultsAction
	}{{"MENU", resultsMainMenu}, {"REPLAY", resultsContinue}} {
		position, posOK := variables.Vec2Value("ENDSCREEN_" + button.name + "_BOX_INNER_POS_VAR")
		size, sizeOK := variables.Vec2Value("ENDSCREEN_" + button.name + "_BOX_INNER_SIZE_VAR")
		if posOK && sizeOK && x >= position.X-size.X/2 && x <= position.X+size.X/2 && y >= position.Y-size.Y/2 && y <= position.Y+size.Y/2 {
			if button.name == "REPLAY" && menu.Data.replays() {
				return resultsReplay
			}
			return button.action
		}
	}
	return resultsNone
}
func (menu *resultsMenu) activate(action resultsAction) bool {
	if !menu.Visible || !menu.ready || menu.closing || menu.delivered || (action != resultsMainMenu && action != resultsContinue && action != resultsReplay) {
		return false
	}
	if (menu.Data.replays() && action == resultsContinue) || (!menu.Data.replays() && action == resultsReplay) {
		return false
	}
	menu.pending, menu.closing, menu.phase = action, true, 0
	return true
}
func (menu *resultsMenu) offset(variables formats.FrontendVariables, group string) formats.Vec2 {
	start, ok := variables.Vec2Value("ENDSCREEN_" + group + "_OFFSET_BEGIN_VAR")
	if !ok {
		return formats.Vec2{}
	}
	fraction := float64(1 - menu.phase)
	if menu.closing {
		fraction = float64(menu.phase)
	}
	return formats.Vec2{X: start.X * fraction, Y: start.Y * fraction}
}
func (a *app) drawResultsMenu(screen *ebiten.Image, menu *resultsMenu, gameTime float64) {
	if menu == nil || !menu.Visible {
		return
	}
	if ui := a.resultsUIScreen(); ui != nil {
		a.drawResultsMenuUI(screen, ui, menu, gameTime)
		return
	}
	a.drawResultsMenuLegacy(screen, menu, gameTime)
}

// drawResultsMenuLegacy is the XML-variable layout used when the cache has no
// compiled EndScreen (the older version).
func (a *app) drawResultsMenuLegacy(screen *ebiten.Image, menu *resultsMenu, gameTime float64) {
	if texture, err := a.Texture("Common0/Textures/portal_menu_SD"); err == nil {
		bounds := texture.Bounds()
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		op.GeoM.Scale(864/float64(bounds.Dx()), 864/float64(bounds.Dy()))
		op.GeoM.Rotate(gameTime / 3)
		op.GeoM.Translate(336, 96)
		a.drawImage(screen, texture, op)
	}
	scoreOffset, mainOffset := menu.offset(a.variables, "SCORE"), menu.offset(a.variables, "MAIN")
	if texture, err := a.Texture("Common0/Textures/TutorialBoxes"); err == nil {
		for _, box := range []struct {
			name   string
			offset formats.Vec2
		}{{"SCORE", scoreOffset}, {"STATS", mainOffset}, {"MENU", mainOffset}, {"REPLAY", mainOffset}} {
			for style, part := range []string{"OUTER", "INNER"} {
				position, posOK := a.variables.Vec2Value("ENDSCREEN_" + box.name + "_BOX_" + part + "_POS_VAR")
				size, sizeOK := a.variables.Vec2Value("ENDSCREEN_" + box.name + "_BOX_" + part + "_SIZE_VAR")
				if posOK && sizeOK {
					position.X, position.Y = position.X+box.offset.X, position.Y+box.offset.Y
					if box.name == "MENU" || box.name == "REPLAY" {
						style = 2
					}
					a.drawOptionsBoxStyle(screen, texture, position, size, style)
				}
			}
		}
	}
	menuLabel, nextLabel := "Main Menu", "Continue"
	if menu.Data.Survival {
		menuLabel, nextLabel = "Menu", "Replay"
	}
	if menu.Data.Dead {
		menuLabel, nextLabel = "Main Menu", "Retry"
	}
	for _, label := range []struct {
		text, position string
		offset         formats.Vec2
	}{
		{"Score", "ENDSCREEN_SCORE_TITLE_POS", scoreOffset}, {fmt.Sprintf("%d", menu.shownScore()), "ENDSCREEN_SCORE_AMOUNT_POS", scoreOffset},
		{menuLabel, "ENDSCREEN_MENU_BOX_INNER_POS_VAR", mainOffset}, {nextLabel, "ENDSCREEN_REPLAY_BOX_INNER_POS_VAR", mainOffset},
	} {
		a.drawLegacyResultsText(screen, label.text, label.position, label.offset, false, false)
	}
	for _, row := range []struct {
		label, slot string
		value       *int32
	}{{"Highscore:", "B", menu.Data.Highscore}, {"Zombie Kills:", "A", menu.Data.Kills}} {
		if row.value == nil {
			continue
		}
		a.drawLegacyResultsText(screen, row.label, "ENDSCREEN_STAT_TITLE_POS_"+row.slot, mainOffset, true, false)
		shown := *row.value
		if row.slot == "A" {
			shown = menu.shownKills()
		}
		a.drawLegacyResultsText(screen, fmt.Sprintf("%d", shown), "ENDSCREEN_STAT_NUM_POS_"+row.slot, mainOffset, true, true)
	}
}
func (a *app) drawLegacyResultsText(screen *ebiten.Image, text, positionName string, offset formats.Vec2, small, right bool) {
	position, ok := a.variables.Vec2Value(positionName)
	if !ok || a.font == nil || a.font.LineHeight == 0 {
		return
	}
	name := "ENDSCREEN_TEXT_SCALE_NORMAL_VAR"
	if small {
		name = "ENDSCREEN_TEXT_SCALE_SMALL_VAR"
	}
	size, ok := a.variables.FloatValue(name)
	if !ok {
		return
	}
	scale := size / float64(a.font.LineHeight)
	x, y := position.X+offset.X, position.Y+offset.Y
	if right {
		x -= a.fontTextWidth(text, scale)
	} else if !small {
		x -= a.fontTextWidth(text, scale) / 2
	}
	if !small {
		y -= size / 2
	}
	a.drawFont(screen, a.font, text, x, y, scale)
}
