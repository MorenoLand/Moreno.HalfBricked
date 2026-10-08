package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"github.com/hajimehoshi/ebiten/v2"
	"testing"
)

func TestPauseMenuActionsPreserveGame(t *testing.T) {
	p := &playState{paused: true, score: 123}
	a := &app{play: p, page: 2, statistics: stats.NewStatsData()}
	for _, index := range []int{1, 2} {
		if err := a.activatePauseMenu(index); err != nil || a.play != p || !p.paused || p.score != 123 || a.page != index+2 {
			t.Fatalf("action %d lost paused game", index)
		}
	}
	if a.statsScreen == nil {
		t.Fatal("stats not opened")
	}
	if err := a.activatePauseMenu(4); err != nil || a.confirm == nil {
		t.Fatal("quitting to the desktop must ask first")
	}
	if err := a.confirm.yes(); err != ebiten.Termination {
		t.Fatal("confirming the desktop exit did not terminate")
	}
	a.confirm = nil
	if err := a.activatePauseMenu(0); err != nil || p.paused {
		t.Fatal("resume failed")
	}
}
func TestPauseMenuHitRegions(t *testing.T) {
	for row, action := range pauseMenuOrder {
		if pauseMenuHit(240, pauseMenuTop+5+row*pauseMenuStride) != action {
			t.Fatalf("row %d is not action %d", row, action)
		}
	}
	for _, point := range [][2]int{{139, 130}, {341, 130}, {240, 111}, {240, 112 + 7*26}} {
		if pauseMenuHit(point[0], point[1]) != -1 {
			t.Fatalf("outside hit %v", point)
		}
	}
}
func TestPauseQuitToMenuClosesPlay(t *testing.T) {
	a := &app{play: &playState{paused: true}, page: 2}
	if err := a.activatePauseMenu(3); err != nil || a.play == nil || a.confirm == nil {
		t.Fatal("quitting to the menu must ask first and keep the game running")
	}
	if err := a.confirm.yes(); err != nil || a.play != nil || a.page != 0 {
		t.Fatal("quit-to-menu did not release game")
	}
}

func TestPauseMenuNamesTheLevelOrSurvival(t *testing.T) {
	a := &app{play: &playState{levelInfo: formats.LevelInfo{DisplayName: "Prehistoric: Level 1.1"}}}
	if got := a.pauseLevelLabel(); got != "Prehistoric: Level 1.1" {
		t.Fatalf("story label %q", got)
	}
	a.play.levelInfo.Flags = []string{"SURVIVAL"}
	if got := a.pauseLevelLabel(); got != "Survival" {
		t.Fatalf("survival label %q", got)
	}
}
