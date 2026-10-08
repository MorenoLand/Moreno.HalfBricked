package game

import "testing"

func TestStoryGameOverOnlyAfterLastLifeAndDeathAnimation(t *testing.T) {
	p := &playState{health: 0, lives: 0}
	if !p.storyGameOver(0) {
		t.Fatal("expected game over at zero lives")
	}
	if p.storyGameOver(1) {
		t.Fatal("survival uses the results menu")
	}
	p.deathTimer = .5
	if p.storyGameOver(0) {
		t.Fatal("death animation still running")
	}
	p.deathTimer, p.lives = 0, 1
	if p.storyGameOver(0) {
		t.Fatal("lives remain")
	}
}

func TestPlayerBloodPopAnimatesDuringDeath(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.scriptRuntime.Close()
	p.scriptRuntime = nil
	p.health, p.lives = 0, 3
	p.Update(0, 0, false, false, false)
	if len(p.bloodPops) != 1 {
		t.Fatalf("death should start one blood pop, got %d", len(p.bloodPops))
	}
	start := p.bloodPops[0].age
	for frame := 0; frame < 10; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if len(p.bloodPops) == 1 && p.bloodPops[0].age <= start {
		t.Fatalf("blood pop frozen at age %v while the player is dead", p.bloodPops[0].age)
	}
	for frame := 0; frame < 120; frame++ {
		p.Update(0, 0, false, false, false)
	}
	if len(p.bloodPops) != 0 {
		t.Fatalf("blood pop never finished: %d left", len(p.bloodPops))
	}
}

func TestStoryDeathOpensTheResultsScreenWithRetry(t *testing.T) {
	data := resultsData{Dead: true}
	menu := newResultsMenu(data)
	if !menu.Visible {
		t.Fatal("the TV screen must show after dying in story mode")
	}
	menu.ready = true
	if !menu.activate(resultsReplay) || menu.pending != resultsReplay {
		t.Fatal("Retry should restart the level")
	}
	menu = newResultsMenu(data)
	menu.ready = true
	if menu.activate(resultsContinue) {
		t.Fatal("there is no Continue after dying")
	}
}

func TestOutOfLivesDeathBloodsOnlyOnce(t *testing.T) {
	p := &playState{health: 0, lives: 0}
	for frame := 0; frame < 60*7; frame++ {
		p.updatePlayerDeath()
	}
	if len(p.bloodPops) != 1 {
		t.Fatalf("a single death left %d blood pops", len(p.bloodPops))
	}
}
