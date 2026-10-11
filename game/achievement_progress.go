package game

import (
	"fmt"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Achievement progress shown on locked rows ("12/50"). PORT ADDITION: the original's list shows no counts; the
// values come from the same counters the unlock checks read (playAchievementMet). The best value reached is kept in
// the profile so the row shows how far you got in earlier runs too.

// achievementCountable reports whether the achievement has a count worth showing.
func achievementCountable(entry formats.Achievement) bool {
	if entry.Total <= 1 || entry.Check != "ge" {
		return false
	}
	switch {
	case entry.Type == "KILLS", entry.Type == "KILLS_SHIELD":
		return true
	case entry.Type == "SPECIFIC" && (entry.SpecificType == "combo" || entry.SpecificType == "wave" || entry.SpecificType == "total"):
		return true
	}
	return false
}

// achievementLiveValue is the count of the play in progress for a countable achievement.
func (a *app) achievementLiveValue(p *playState, entry formats.Achievement) int {
	if entry.Type == "SPECIFIC" && entry.SpecificType == "total" {
		if a.statistics.Available["Zombies Killed"] {
			return int(a.statistics.ZombiesKilled)
		}
		return 0
	}
	if p == nil || p.achievementsTainted() {
		return 0
	}
	switch {
	case entry.Type == "KILLS":
		return int(p.achieve.best[entry.SpecificType])
	case entry.Type == "KILLS_SHIELD":
		return int(p.achieve.shieldKills[entry.SpecificType])
	case entry.SpecificType == "combo":
		return int(p.achieve.best["combo"])
	case entry.SpecificType == "wave":
		if p.isSurvival() {
			return int(p.achieve.waveAdvances)
		}
	}
	return 0
}

// noteAchievementProgress raises the stored best of every countable, locked achievement. It reports whether any
// value changed, so the caller can save the profile.
func (a *app) noteAchievementProgress(p *playState) bool {
	changed := false
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] || !achievementCountable(entry) {
			continue
		}
		if v := int32(min(a.achievementLiveValue(p, entry), entry.Total)); v > a.achievementBest[entry.ID] {
			if a.achievementBest == nil {
				a.achievementBest = map[string]int32{}
			}
			a.achievementBest[entry.ID] = v
			changed = true
		}
	}
	return changed
}

// achievementProgressLabel is "n/total" for a locked, countable achievement and "" otherwise.
func (a *app) achievementProgressLabel(entry formats.Achievement) string {
	if a.achievementUnlocks[entry.ID] || !achievementCountable(entry) {
		return ""
	}
	best := int(a.achievementBest[entry.ID])
	if live := a.achievementLiveValue(a.play, entry); live > best {
		best = min(live, entry.Total)
	}
	return fmt.Sprintf("%d/%d", best, entry.Total)
}
