package game

import (
	"encoding/json"
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/stats"
	"log"
	"math"
)

type playerProfile struct {
	Options            optionsSettings  `json:"options"`
	Stats              stats.StatsData  `json:"stats"`
	Unlocked           map[string]bool  `json:"unlocked"`
	NewDismissed       map[string]bool  `json:"newDismissed"`
	AchievementUnlocks map[string]bool  `json:"achievementUnlocks,omitempty"`
	DeathFreeLevels    map[string]bool  `json:"deathFreeLevels,omitempty"`
	Highscores         map[string]int32 `json:"highscores,omitempty"`
}

func (a *app) loadPlayerProfile() error {
	data, err := readPlayerProfile()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	var profile playerProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return fmt.Errorf("player profile: %w", err)
	}
	if err := a.options.RestoreSettings(profile.Options); err != nil {
		return err
	}
	a.statistics = profile.Stats
	a.newDismissed = profile.NewDismissed
	a.achievementUnlocks = profile.AchievementUnlocks
	a.achieveSession.deathFree = profile.DeathFreeLevels
	a.highscores = profile.Highscores
	if a.statistics.Available == nil {
		a.statistics.Available = map[string]bool{}
	}
	for id, unlocked := range profile.Unlocked {
		a.unlocked[id] = unlocked
	}
	a.awardLocalAchievements(nil)
	return nil
}
func (a *app) savePlayerProfile() error {
	if !a.profileWritable {
		return nil
	}
	data, err := json.MarshalIndent(playerProfile{Options: a.options.Settings(), Stats: a.statistics, Unlocked: a.unlocked, NewDismissed: a.newDismissed, AchievementUnlocks: a.achievementUnlocks, DeathFreeLevels: a.achieveSession.deathFree, Highscores: a.highscores}, "", "  ")
	if err != nil {
		return err
	}
	return writePlayerProfile(data)
}
func (a *app) updateStatsClock() {
	if a.statistics.Available == nil {
		a.statistics.Available = map[string]bool{}
	}
	a.statsClock += float32(1.0 / 60.0)
	if a.statsClock >= 1 {
		a.statistics.TimePlayed++
		a.statistics.Available["Total Time Played"] = true
		a.statsClock = 0
	}
}
func (a *app) recordPlayStats(p *playState, previousKills, previousLives int) {
	if a.statistics.Available == nil {
		a.statistics.Available = map[string]bool{}
	}
	if math.Hypot(p.x-p.statsPositionX, p.y-p.statsPositionY) > 32 {
		a.statistics.DistanceTravelled++
		a.statistics.Available["Distance Travelled"] = true
		p.statsPositionX, p.statsPositionY = p.x, p.y
	}
	if p.levelKills > previousKills {
		a.statistics.ZombiesKilled += int32(p.levelKills - previousKills)
		a.statistics.Available["Zombies Killed"] = true
	}
	if p.lives < previousLives {
		a.statistics.LivesLost += int32(previousLives - p.lives)
		a.statistics.Available["Lives Lost"] = true
	}
	if p.multiplier > int(a.statistics.HighestMultiplier) {
		a.statistics.HighestMultiplier = int32(p.multiplier)
		a.statistics.Available["Highest Multiplier"] = true
	}
	if a.mode == 1 && p.waveIndex+1 > int(a.statistics.HighestSurvivalWave) {
		a.statistics.HighestSurvivalWave = int32(p.waveIndex + 1)
		a.statistics.Available["Highest Survival Wave"] = true
	}
	if a.mode == 1 {
		a.statistics.BestSurvivalScore = max(a.statistics.BestSurvivalScore, int32(p.score))
		a.statistics.Available["Best Survival Level Score"] = true
	} else {
		a.statistics.BestStoryScore = max(a.statistics.BestStoryScore, int32(max(0, p.score-p.levelStartScore)))
		a.statistics.Available["Best Story Level Score"] = true
	}
	if a.awardLocalAchievements(nil) {
		if err := a.savePlayerProfile(); err != nil {
			log.Printf("achievements: %v", err)
		}
	}
}
