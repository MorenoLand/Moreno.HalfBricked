package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestAchievementProgressLabelShowsTheBestCount(t *testing.T) {
	entry := formats.Achievement{ID: "mow", Type: "KILLS", Check: "ge", SpecificType: "buzzsaw", Total: 50}
	a := &app{achievements: formats.AchievementCatalog{entry}, achievementUnlocks: map[string]bool{}}
	p := &playState{}
	p.achieve.best = map[string]int32{"buzzsaw": 12}
	a.play = p
	if !a.noteAchievementProgress(p) {
		t.Fatal("progress did not register")
	}
	if got := a.achievementProgressLabel(entry); got != "12/50" {
		t.Fatalf("label %q, want 12/50", got)
	}
	// A new run starts at 0, but the stored best stays.
	a.play = &playState{}
	if got := a.achievementProgressLabel(entry); got != "12/50" {
		t.Fatalf("label after a new run %q, want 12/50", got)
	}
	a.achievementUnlocks["mow"] = true
	if a.achievementProgressLabel(entry) != "" {
		t.Fatal("an unlocked achievement shows no progress")
	}
}

func TestOneShotAchievementsShowNoProgress(t *testing.T) {
	a := &app{achievementUnlocks: map[string]bool{}}
	if a.achievementProgressLabel(formats.Achievement{ID: "x", Type: "STORY", Check: "e", Total: 1}) != "" {
		t.Fatal("a one-shot achievement shows a count")
	}
}
