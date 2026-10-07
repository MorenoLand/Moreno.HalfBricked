package main

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"os"
	"testing"
)

func TestNativeAchievementMotionUsesInputBeforeCollision(t *testing.T) {
	if x, y := nativeAchievementMotion(.1, 0); x != 0 || y != 0 {
		t.Fatal("native dead zone not applied")
	}
	x, y := nativeAchievementMotion(.2, 0)
	if x != float32(.4)*math.Float32frombits(0x3f4f5c29) || y != 0 {
		t.Fatal("fractional movement normalized away")
	}
	x, y = nativeAchievementMotion(1, 1)
	if x == 0 || x != y || math.Abs(float64(x*x+y*y)-.81*.81) > 1e-6 {
		t.Fatal("clamp/scale changed")
	}
	p := &playState{health: 1, combatKillCount: 20, achievementMotionX: x, achievementMotionY: y}
	a := &app{achievements: formats.AchievementCatalog{{ID: "camper", Type: "SPECIFIC", Check: "ge", SpecificType: "no_move", Total: 20}}}
	if err := a.updateGameplayAchievements(p); err != nil || a.achievementUnlocks["camper"] || p.achievementTracking.stationaryBaseline != 20 {
		t.Fatal("collision/animation flags replaced motion inputs")
	}
}
func TestLethalOwnedCombatCreditsBeforeCorpseRemoval(t *testing.T) {
	p := &playState{health: 1, maxHealth: 1}
	for i := 0; i < 20; i++ {
		p.zombies = append(p.zombies, zombieState{health: 100})
	}
	p.detonateGrenade(0, 0)
	if p.combatKillCount != 20 || p.levelKills != 0 || len(p.zombies) != 20 {
		t.Fatal("credit waited for corpse retirement")
	}
	p.detonateGrenade(0, 0)
	if p.combatKillCount != 20 {
		t.Fatal("dead targets credited twice")
	}
	a := &app{achievements: formats.AchievementCatalog{{ID: "camper", Type: "SPECIFIC", Check: "ge", SpecificType: "no_move", Total: 20}}}
	if err := a.updateGameplayAchievements(p); err != nil || !a.achievementUnlocks["camper"] {
		t.Fatal("real lethal events did not award CAMPER")
	}
}
func TestGameplayCardioSkipsPauseAndMenuSamples(t *testing.T) {
	p := &playState{health: 1}
	a := &app{achievements: formats.AchievementCatalog{{ID: "cardio", Type: "SPECIFIC", Check: "ge", SpecificType: "no_kills", Total: 30}}}
	for tick := 0; tick < 1800; tick++ {
		if err := a.updateGameplayAchievements(p); err != nil {
			t.Fatal(err)
		}
	}
	if a.achievementUnlocks["cardio"] {
		t.Fatal("float32 threshold rounded up")
	}
	p.paused = true
	before := p.achievementTracking
	if err := a.updateGameplayAchievements(p); err != nil || p.achievementTracking != before {
		t.Fatal("paused timer advanced")
	}
	a.awardLocalAchievements(nil)
	if a.achievementUnlocks["cardio"] {
		t.Fatal("stale menu sample awarded")
	}
	p.paused = false
	if err := a.updateGameplayAchievements(p); err != nil || !a.achievementUnlocks["cardio"] {
		t.Fatal("fresh active threshold missed")
	}
}

func TestLocalAchievementsUseSupportedScopesOnly(t *testing.T) {
	a := &app{statistics: newStatsData(), achievements: formats.AchievementCatalog{{ID: "world", Type: "STORY", Check: "e", Total: 0}, {ID: "total", Type: "SPECIFIC", Check: "ge", SpecificType: "total", Total: 50000}, {ID: "wave", Type: "SPECIFIC", Check: "ge", SpecificType: "wave", Total: 15}, {ID: "pickup", Type: "KILLS", Check: "ge", SpecificType: "shotgun", Total: 50}}}
	if a.awardLocalAchievements(&formats.LevelInfo{WorldIndex: 0, Flags: []string{"STORY"}}) {
		t.Fatal("ordinary level awarded world completion")
	}
	if !a.awardLocalAchievements(&formats.LevelInfo{WorldIndex: 0, Flags: []string{"STORY", "ENDWORLD"}}) || !a.achievementUnlocks["world"] {
		t.Fatal("world completion not awarded")
	}
	a.statistics.ZombiesKilled, a.statistics.HighestSurvivalWave = 50000, 15
	if a.awardLocalAchievements(nil) {
		t.Fatal("unmeasured stats substituted")
	}
	a.statistics.Available["Zombies Killed"], a.statistics.Available["Highest Survival Wave"] = true, true
	if !a.awardLocalAchievements(nil) || !a.achievementUnlocks["total"] || !a.achievementUnlocks["wave"] || a.achievementUnlocks["pickup"] {
		t.Fatal("achievement scope mismatch")
	}
	if a.awardLocalAchievements(nil) {
		t.Fatal("achievement awarded twice")
	}
	encoded, err := json.Marshal(playerProfile{AchievementUnlocks: a.achievementUnlocks})
	if err != nil {
		t.Fatal(err)
	}
	var restored playerProfile
	if err := json.Unmarshal(encoded, &restored); err != nil || !restored.AchievementUnlocks["world"] || !restored.AchievementUnlocks["total"] {
		t.Fatal("local unlock persistence lost")
	}
}
func TestAchievementsReturnToCallerWithoutResumingPlay(t *testing.T) {
	for _, page := range []int{2, 4} {
		p := &playState{paused: true}
		a := &app{page: page, play: p}
		a.openAchievements()
		if a.page != 6 || a.achievementsBackPage != page {
			t.Fatal("wrong achievement entry")
		}
		if err := a.closeAchievements(); err != nil || a.page != page || a.play != p || !p.paused {
			t.Fatal("achievement exit lost caller")
		}
	}
}
func TestOriginalCachedAchievementCatalog(t *testing.T) {
	root := "bin/web/data/data-cache"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("original asset cache unavailable")
	}
	p, err := content.NewPack(content.NewSource(root))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := p.Achievements()
	if err != nil || len(entries) != 33 {
		t.Fatalf("original catalog: %d entries %v", len(entries), err)
	}
}
