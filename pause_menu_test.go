package main

import (
	"github.com/hajimehoshi/ebiten/v2"
	"testing"
)

func TestPauseMenuActionsPreserveGame(t *testing.T) {
	p := &playState{paused: true, score: 123}
	a := &app{play: p, page: 2, statistics: newStatsData()}
	for _, index := range []int{1, 2} {
		if err := a.activatePauseMenu(index); err != nil || a.play != p || !p.paused || p.score != 123 || a.page != index+2 {
			t.Fatalf("action %d lost paused game", index)
		}
	}
	if a.statsScreen == nil {
		t.Fatal("stats not opened")
	}
	if err := a.activatePauseMenu(4); err != ebiten.Termination {
		t.Fatal("desktop exit did not terminate")
	}
	if err := a.activatePauseMenu(0); err != nil || p.paused {
		t.Fatal("resume failed")
	}
}
func TestPauseMenuHitRegions(t *testing.T) {
	for index := range pauseMenuLabels {
		if pauseMenuHit(240, 130+index*30) != index {
			t.Fatalf("row %d", index)
		}
	}
	for _, point := range [][2]int{{139, 130}, {341, 130}, {240, 119}, {240, 265}} {
		if pauseMenuHit(point[0], point[1]) != -1 {
			t.Fatalf("outside hit %v", point)
		}
	}
}
func TestPauseQuitToMenuClosesPlay(t *testing.T) {
	a := &app{play: &playState{paused: true}, page: 2}
	if err := a.activatePauseMenu(3); err != nil || a.play != nil || a.page != 0 {
		t.Fatal("quit-to-menu did not release game")
	}
}
