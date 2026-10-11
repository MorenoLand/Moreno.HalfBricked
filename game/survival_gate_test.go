package game

import "testing"

// A world's survival maps stay shut until the world is reached in story mode (port addition).
func TestSurvivalMapsFollowStoryProgress(t *testing.T) {
	a := newHeadlessApp(t, "bin/data-cache")
	check := func(id string, want bool) {
		t.Helper()
		for _, item := range a.levels {
			if item.ID == id {
				if got := a.levelUnlocked(item); got != want {
					t.Fatalf("%s unlocked = %v, want %v", id, got, want)
				}
				return
			}
		}
		t.Fatalf("level %s missing", id)
	}
	check("World0Survival0", true)
	for _, item := range a.levels {
		if item.ID == "World3Survival0" && a.unlocked[item.ID] {
			check("World3Survival0", false)
		}
	}
	a.unlocked["World3Level0"] = true
	check("World3Survival0", a.unlocked["World3Survival0"])
}
