package game

// The level select's "Best Deaths" and "Best Kills" rows. The 1.2.5 info screen (FUN_000d65f4, strings at 0x85970a
// "Best Score", 0x859715 "Best Deaths", 0x859721 "Best Kills") reads three numbers of the level record (+8 score,
// +0x10 deaths, +0xc kills) and shows "0" for a value still at the 2147483648.0 sentinel (0xd682c, string 0x85973e),
// else "%d". What WRITES the deaths and kills fields was not traced, so the update rule here is a PORT DECISION:
// kills keep the highest count of a finished or lost run, deaths keep the fewest deaths of a level that was
// completed. The score row keeps the existing highscores map.
type levelBest struct {
	Kills     int32 `json:"kills,omitempty"`
	Deaths    int32 `json:"deaths,omitempty"`
	HaveDeath bool  `json:"haveDeaths,omitempty"` // Deaths is set (a completed run exists)
}

// recordLevelBests folds a finished or lost run into the level's bests.
func (a *app) recordLevelBests(levelID string, data resultsData) {
	if a.play == nil {
		return
	}
	if a.levelBests == nil {
		a.levelBests = map[string]levelBest{}
	}
	best := a.levelBests[levelID]
	changed := false
	if data.Kills != nil && *data.Kills > best.Kills {
		best.Kills, changed = *data.Kills, true
	}
	if !data.Dead {
		deaths := int32(a.play.achieve.deaths)
		if !best.HaveDeath || deaths < best.Deaths {
			best.Deaths, best.HaveDeath, changed = deaths, true, true
		}
	}
	if changed {
		a.levelBests[levelID] = best
		_ = a.savePlayerProfile()
	}
}
