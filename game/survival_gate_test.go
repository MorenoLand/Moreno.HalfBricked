package game

import "testing"

// A survival map opens when the story level that unlocks it (unlockLevels, the ENDWORLD level) is completed,
// whatever the STARTUNLOCKED flag says; being inside the world is not enough.
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
	a.unlocked["World0Survival0"], a.unlocked["World4Survival0"] = true, true
	check("World0Survival0", false)
	a.unlocked["World4Level0"], a.unlocked["World4Level1"] = true, true
	check("World4Survival0", false) // inside world 4 but its last level is not finished
	a.completed = map[string]bool{"World0Level2": true}
	check("World0Survival0", true)
	a.unlocked["World4Level2"] = true
	a.completed["World4Level2"] = true
	check("World4Survival0", true)
}
