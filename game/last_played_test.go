package game

import "testing"

func TestLevelSelectOpensOnTheLastPlayedLevel(t *testing.T) {
	a := newHeadlessApp(t, "bin/data-cache")
	for _, mode := range []int{0, 1} {
		a.mode = mode
		levels := []string{}
		for world := range a.worlds() {
			a.world = world
			for _, item := range a.filteredLevels() {
				levels = append(levels, item.ID)
			}
		}
		target := levels[len(levels)-1]
		a.noteLastPlayed(target)
		a.world, a.level = 0, 0
		a.restoreLastPlayed()
		got := a.filteredLevels()[a.level].ID
		if got != target {
			t.Fatalf("mode %d: opened on %s, want %s", mode, got, target)
		}
	}
}
