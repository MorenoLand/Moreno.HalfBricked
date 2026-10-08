package game

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"testing"
)

func TestPlayerProfileKeepsOptionsAndMeasuredStats(t *testing.T) {
	menu := newOptionsMenu(true, true)
	menu.activate(optionsHidden, nil)
	menu.activate(optionsInvert, nil)
	data := stats.NewStatsData()
	data.ZombiesKilled, data.TimePlayed = 9, 12
	data.Available["Zombies Killed"] = true
	profile := playerProfile{Options: menu.Settings(), Stats: data, Unlocked: map[string]bool{"World0Level1": true}, NewDismissed: map[string]bool{"World0Level0": true}}
	encoded, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	var restored playerProfile
	if err := json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	var restoredMenu optionsMenu
	if err := restoredMenu.RestoreSettings(restored.Options); err != nil {
		t.Fatal(err)
	}
	if restoredMenu.controls.Visible || restoredMenu.controls.Normal || restored.Stats.ZombiesKilled != 9 || restored.Stats.TimePlayed != 12 || !restored.Stats.Available["Zombies Killed"] || !restored.Unlocked["World0Level1"] {
		t.Fatal("profile lost control settings or measured counters")
	}
	if !restored.NewDismissed["World0Level0"] || restored.NewDismissed["World0Level1"] {
		t.Fatal("profile lost per-level NEW suppression")
	}
}
func TestStatsCountersUseNativeClockAndDistanceThreshold(t *testing.T) {
	a := &app{statistics: stats.NewStatsData()}
	for i := 0; i < 61; i++ {
		a.updateStatsClock()
	}
	if a.statistics.TimePlayed != 1 {
		t.Fatal("time counter did not use whole seconds")
	}
	p := &playState{x: 32, score: 12, levelStartScore: 5}
	a.recordPlayStats(p, 0, 0)
	if a.statistics.DistanceTravelled != 0 || a.statistics.BestStoryScore != 7 {
		t.Fatal("distance boundary or per-level best score changed")
	}
	p.x = 33
	a.recordPlayStats(p, 0, 0)
	if a.statistics.DistanceTravelled != 1 || p.statsPositionX != 33 {
		t.Fatal("native distance threshold did not update reference")
	}
}
