package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"testing"
)

func TestEndWorldOutroEntersNativeResultsOnce(t *testing.T) {
	p := &playState{health: 1, score: 450, levelStartScore: 200, levelKills: 5, exitScriptStarted: true, levelInfo: formats.LevelInfo{Flags: []string{"STORY", "ENDWORLD"}, NextLevel: "World1Level0", UnlockLevels: []string{"World0Survival1"}}}
	runtime, err := scripting.New("return", &playScriptHost{play: p}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	p.scriptRuntime = runtime
	a := &app{play: p}
	if err := a.updateLevelCompletion(); err != nil {
		t.Fatal(err)
	}
	if a.page != 5 || a.resultsScreen == nil || !a.resultsScreen.Visible || a.resultsScreen.score() != 250 || *a.resultsScreen.Data.Kills != 5 {
		t.Fatal("ENDWORLD froze instead of entering results")
	}
	original := a.resultsScreen
	if !a.unlocked["World1Level0"] || !a.unlocked["World0Survival1"] {
		t.Fatal("results opened before completion unlocks")
	}
	a.openLevelResults()
	if a.resultsScreen != original {
		t.Fatal("results reinitialized")
	}
}
func TestSurvivalGameOverEntersReplayResults(t *testing.T) {
	a := &app{mode: 1, play: &playState{score: 123, health: 0, lives: -1}}
	if err := a.updateLevelCompletion(); err != nil {
		t.Fatal(err)
	}
	if a.resultsScreen == nil || !a.resultsScreen.Data.Survival || a.resultsScreen.score() != 123 {
		t.Fatal("survival game-over did not enter replay results")
	}
}
