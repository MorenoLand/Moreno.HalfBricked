package main

import (
	"fmt"
	"strconv"
	"strings"
)

type statsData struct {
	BestStoryScore, BestSurvivalScore, HighestSurvivalWave, HighestMultiplier                                             int32
	ShotgunKills, UziKills, FlamerKills, MinigunKills, RifleKills, BuzzsawKills, GrenadeKills, MineKills                  int32
	BestTRexTime, BestGangsterTime, BestEgyptTime, BestJapanTime, BestFutureTime, BestBonusTime                           int32
	TimePlayed, TimesPlayed, ZombiesKilled, ZombiesIgnited, LivesLost, StoryLevelsPlayed, SurvivalLevelsPlayed            int32
	DistanceTravelled, GrenadesTossed, SentryGunsUsed, SentryGunKills, ShotsFired, BossesKilled, FruitSliced, ScreenViews int32
	Available                                                                                                             map[string]bool
}

func newStatsData() statsData {
	return statsData{BestTRexTime: -1, BestGangsterTime: -1, BestEgyptTime: -1, BestJapanTime: -1, BestFutureTime: -1, BestBonusTime: -1, Available: map[string]bool{}}
}
func statsInteger(value int32) string {
	if value == -1 {
		return "Not Set"
	}
	s := strconv.FormatInt(int64(value), 10)
	start := 0
	if strings.HasPrefix(s, "-") {
		start = 1
	}
	for i := len(s) - 3; i > start; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
func statsTime(value int32) string {
	if value == -1 {
		return "Not Set"
	}
	if value < 0 {
		value = 0
	}
	return fmt.Sprintf("%20d:%02d:%02d", value/3600, value/60%60, value%60)
}
func (d *statsData) bossTotal() int32 {
	var total int32
	for _, value := range []int32{d.BestTRexTime, d.BestGangsterTime, d.BestEgyptTime, d.BestJapanTime, d.BestFutureTime, d.BestBonusTime} {
		if value != -1 {
			total += value
		}
	}
	return total
}
