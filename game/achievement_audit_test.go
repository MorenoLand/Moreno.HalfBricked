package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Audit tests for the false-unlock report (CARDIO with no zombies spawned). Native evidence:
// Research/native/achievement-audit-2026-10-10.md.

func auditCaches(t *testing.T) []ptCache {
	t.Helper()
	caches := ptCaches()
	if len(caches) == 0 {
		t.Skip("no data cache present")
	}
	return caches
}

func auditSurvival(t *testing.T, root string) *app {
	t.Helper()
	a := newHeadlessApp(t, root)
	a.page = 2
	if err := a.selectCaptureLevel("World0Survival0"); err != nil {
		t.Fatal(err)
	}
	if err := a.openPlay(); err != nil {
		t.Fatal(err)
	}
	return a
}

// Menu, stats, results and achievement pages, the sandbox and the pause menu own the frame: the tracker is neither
// ticked nor sampled behind them, however long they stay open.
func TestCardioDoesNotTickBehindMenuFramesAndSandbox(t *testing.T) {
	for _, c := range auditCaches(t) {
		a := auditSurvival(t, c.root)
		p := a.play
		p.closeScript()
		p.zombies = nil
		p.moveControl, p.shootControl = true, true
		before := p.achievementTracking
		reset := func() { a.page, a.sandboxOpen, a.resultsScreen, a.statsScreen = 2, false, nil, nil }
		run := func(label string) {
			for i := 0; i < 60*40; i++ {
				if err := a.Update(); err != nil {
					t.Fatalf("%s %s: %v", c.name, label, err)
				}
				if a.play == nil {
					t.Fatalf("%s %s: play vanished", c.name, label)
				}
			}
			if a.play.achievementTracking != before {
				t.Fatalf("%s %s: tracker moved behind a non-gameplay frame: %+v -> %+v", c.name, label, before, a.play.achievementTracking)
			}
			reset()
		}
		a.page = 6
		run("achievements page")
		a.statsScreen, a.page = newStatsMenu(&a.statistics), 4
		run("stats page")
		kills, highscore := int32(1), int32(1)
		a.resultsScreen, a.page = newResultsMenu(resultsData{Flags: 4, Score: 1, Kills: &kills, Highscore: &highscore}), 5
		run("results page")
		a.sandboxOpen = true
		run("sandbox")
		a.play.paused = true
		run("paused")
		for id, unlocked := range a.achievementUnlocks {
			if unlocked {
				t.Fatalf("%s: %s unlocked behind a non-gameplay frame", c.name, id)
			}
		}
	}
}

// The survival intro (a running script: 1.2.5 "GET READY...", the 1.2.1 tutorial that waits for both sticks) resets
// the no-kill timer every frame, so CARDIO cannot complete during it.
func TestCardioNeverCompletesDuringSurvivalIntro(t *testing.T) {
	for _, c := range auditCaches(t) {
		a := auditSurvival(t, c.root)
		a.inputHook = func() playerInput { return playerInput{} }
		p := a.play
		if p.scriptRuntime == nil {
			t.Skipf("%s: no survival intro script", c.name)
		}
		for frame := 0; frame < 60*5 && p.scriptRuntime != nil && !p.scriptRuntime.Done(); frame++ {
			if err := a.Update(); err != nil {
				t.Fatal(err)
			}
		}
		for id, unlocked := range a.achievementUnlocks {
			if unlocked {
				t.Fatalf("%s: %s unlocked during the survival intro", c.name, id)
			}
		}
	}
}

// A sandbox run (F1 menu, cheat toggles) never awards, not even the timer based ones, which would otherwise be
// reachable by freezing the waves or removing every zombie.
func TestSandboxRunNeverAwards(t *testing.T) {
	catalog := formats.AchievementCatalog{
		{ID: "cardio", Type: "SPECIFIC", Check: "ge", SpecificType: "no_kills", Total: 30},
		{ID: "camper", Type: "SPECIFIC", Check: "ge", SpecificType: "no_move", Total: 20},
		{ID: "world", Type: "STORY", Check: "e", Total: 0},
	}
	for index, mark := range []func(p *playState){
		func(p *playState) { p.achieve.sandboxUsed = true },
		func(p *playState) { p.cheats.freezeWaves = true },
		func(p *playState) { p.cheats.god = true },
	} {
		r := newAchievementRig(t, catalog, nil)
		mark(r.p)
		for i := 0; i < 60*40; i++ {
			if err := r.app.updateGameplayAchievements(r.p); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 25; i++ {
			r.p.creditKill(killOrigin{gun: "PISTOL"})
		}
		_ = r.app.updateGameplayAchievements(r.p)
		r.app.awardLocalAchievements(&formats.LevelInfo{WorldIndex: 0, Flags: []string{"STORY", "ENDWORLD"}})
		if len(r.app.achievementUnlocks) != 0 {
			t.Fatalf("variant %d: sandbox run awarded %v", index, r.app.achievementUnlocks)
		}
	}
	// The same run without the sandbox does award: the gate is the only difference.
	r := newAchievementRig(t, catalog, nil)
	for i := 0; i < 60*31 && !r.unlocked("cardio"); i++ {
		_ = r.app.updateGameplayAchievements(r.p)
	}
	if !r.unlocked("cardio") {
		t.Fatal("clean run did not unlock CARDIO")
	}
}

// Death resets the no-kill timer (1.2.5 FUN_000e5a3c: health <= 0 zeroes the accumulator).
func TestCardioResetsOnDeath(t *testing.T) {
	r := newAchievementRig(t, formats.AchievementCatalog{{ID: "cardio", Type: "SPECIFIC", Check: "ge", SpecificType: "no_kills", Total: 30}}, nil)
	for i := 0; i < 60*20; i++ {
		_ = r.app.updateGameplayAchievements(r.p)
	}
	r.p.health = 0
	_ = r.app.updateGameplayAchievements(r.p)
	r.p.health = 1
	for i := 0; i < 60*29; i++ {
		_ = r.app.updateGameplayAchievements(r.p)
	}
	if r.unlocked("cardio") {
		t.Fatal("timer survived a death")
	}
}

// IMMUNITY IDOL follows the native wave counter, which keeps counting while the wave table loops. On the shipped
// survival tables the index never reaches 15, so the old waveIndex+1 rule could not unlock it.
func TestImmunityIdolCountsLoopedWavesOnShippedLevels(t *testing.T) {
	for _, c := range auditCaches(t) {
		a := auditSurvival(t, c.root)
		p := a.play
		p.closeScript()
		p.dialogueIndex = len(p.dialogue)
		var idol formats.Achievement
		for _, entry := range a.achievements {
			if entry.Type == "SPECIFIC" && entry.SpecificType == "wave" {
				idol = entry
			}
		}
		if idol.ID == "" {
			t.Fatal("no wave achievement in catalog")
		}
		maxIndex := 0
		for frame := 0; frame < 60*60*60 && !a.achievementUnlocks[idol.ID]; frame++ {
			p.zombies = nil
			p.updateWaves()
			maxIndex = max(maxIndex, p.waveIndex)
			if err := a.updateGameplayAchievements(p); err != nil {
				t.Fatal(err)
			}
		}
		if !a.achievementUnlocks[idol.ID] || int(p.achieve.waveAdvances) != idol.Total {
			t.Fatalf("%s: counter %d unlocked=%v", c.name, p.achieve.waveAdvances, a.achievementUnlocks[idol.ID])
		}
		if maxIndex >= idol.Total {
			t.Fatalf("%s: wave index reached %d; the table is expected to loop", c.name, maxIndex)
		}
	}
}

// Story levels have no wave hook (the story mode's advance hook is a no-op): "wave" is never submitted there.
func TestWaveAchievementIsSurvivalOnly(t *testing.T) {
	r := newAchievementRig(t, formats.AchievementCatalog{{ID: "idol", Type: "SPECIFIC", Check: "ge", SpecificType: "wave", Total: 15}}, nil)
	r.p.achieve.waveAdvances = 40
	_ = r.app.updateGameplayAchievements(r.p)
	if r.unlocked("idol") {
		t.Fatal("story level awarded the survival wave achievement")
	}
}
