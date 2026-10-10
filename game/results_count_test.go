package game

import "testing"

func slideIn(m *resultsMenu) {
	for i := 0; i < 30 && !m.ready; i++ {
		m.update(1.0/60.0, false)
	}
}

// The original shows final values at once (enter routine 0x000cf030 sprintf's the
// totals once). With the port's count-up switched off that is exactly what happens.
func TestResultsCountUpOffIsTheNativeFinalValues(t *testing.T) {
	defer func(old bool) { resultsCountUp = old }(resultsCountUp)
	resultsCountUp = false
	kills, best := int32(135), int32(1210)
	menu := newResultsMenu(resultsData{Survival: true, Score: 1000, Kills: &kills, Highscore: &best})
	if menu.shownScore() != 1000 || menu.shownKills() != 135 || menu.shownHighscore() != 1210 || menu.counting() || menu.finishCount() {
		t.Fatal("native mode must show finals and never count")
	}
	if s, g := menu.popEffect(0); s != 1 || g != 0 {
		t.Fatal("no pop in native mode")
	}
}

// PORT ADDITION: numbers start at 0, count up after the slide-in, kills a little
// after score, and land exactly on the final values.
func TestResultsCountUpRunsAfterSlideInAndLandsOnFinals(t *testing.T) {
	kills, best := int32(135), int32(1210)
	menu := newResultsMenu(resultsData{Flags: 4, Score: 1450, LevelStartScore: 450, Kills: &kills, Highscore: &best})
	if menu.shownScore() != 0 || menu.shownKills() != 0 || menu.shownHighscore() != 0 {
		t.Fatal("must start at zero")
	}
	menu.update(.1, false)
	if menu.shownScore() != 0 {
		t.Fatal("must not count while sliding in")
	}
	slideIn(menu)
	if !menu.ready || !menu.counting() {
		t.Fatal("count should run once ready")
	}
	var prevScore int32
	for i := 0; i < 36; i++ { // 0.6 s
		menu.update(1.0/60.0, false)
		if menu.shownScore() < prevScore {
			t.Fatal("score must never go backwards")
		}
		prevScore = menu.shownScore()
	}
	mid := menu.shownScore()
	if mid <= 0 || mid >= 1000 {
		t.Fatalf("mid-count score %d", mid)
	}
	if float64(menu.shownKills())/135 >= float64(mid)/1000 {
		t.Fatal("kills must trail the score")
	}
	for i := 0; i < 60*2 && menu.counting(); i++ {
		menu.update(1.0/60.0, false)
	}
	if menu.counting() || menu.shownScore() != 1000 || menu.shownKills() != 135 || menu.shownHighscore() != 1210 {
		t.Fatalf("finals %d %d %d", menu.shownScore(), menu.shownKills(), menu.shownHighscore())
	}
}

func TestResultsCountUpDurationIsAboutOneAndAHalfSeconds(t *testing.T) {
	kills, best := int32(40), int32(900)
	menu := newResultsMenu(resultsData{Survival: true, Score: 900, Kills: &kills, Highscore: &best})
	slideIn(menu)
	frames := 0
	for menu.counting() && frames < 600 {
		menu.update(1.0/60.0, false)
		frames++
	}
	if seconds := float64(frames) / 60; seconds < 1.2 || seconds > 1.7 {
		t.Fatalf("count lasted %.2f s", seconds)
	}
}

// PORT ADDITION: the first press finishes the count and cannot press a button;
// the next press behaves natively.
func TestResultsFirstPressFinishesCountBeforeButtons(t *testing.T) {
	kills := int32(10)
	menu := newResultsMenu(resultsData{Survival: true, Score: 500, Kills: &kills})
	slideIn(menu)
	menu.update(.2, false)
	if !menu.finishCount() {
		t.Fatal("press during count must be consumed")
	}
	if menu.shownScore() != 500 || menu.shownKills() != 10 || menu.counting() {
		t.Fatal("finish must show finals")
	}
	if menu.finishCount() {
		t.Fatal("second press is not a skip")
	}
	if !menu.activate(resultsReplay) {
		t.Fatal("buttons must work after the skip")
	}
}

func TestResultsSkipDuringSlideInAndGoldPopOnLanding(t *testing.T) {
	kills := int32(10)
	menu := newResultsMenu(resultsData{Survival: true, Score: 500, Kills: &kills})
	menu.update(.1, false)
	if !menu.finishCount() || menu.shownScore() != 500 {
		t.Fatal("press while sliding in finishes the count too")
	}
	slideIn(menu)
	menu.update(.05, false)
	if scale, gold := menu.popEffect(0); scale <= 1 || gold <= 0 {
		t.Fatalf("landed number should pop: %v %v", scale, gold)
	}
	menu.update(1, false)
	if scale, gold := menu.popEffect(0); scale != 1 || gold != 0 {
		t.Fatal("pop must settle")
	}
}

func TestResultsTickSoundIsSoftAndRateLimited(t *testing.T) {
	kills, best := int32(135), int32(1210)
	menu := newResultsMenu(resultsData{Survival: true, Score: 2000, Kills: &kills, Highscore: &best})
	slideIn(menu)
	ticks := 0
	for i := 0; i < 60*3; i++ {
		menu.update(1.0/60.0, false)
		if menu.takeTick() {
			ticks++
		}
	}
	if ticks < 8 || ticks > 30 {
		t.Fatalf("%d ticks over the count", ticks)
	}
	if resultsTickVolume > .3 {
		t.Fatal("tick must be quiet")
	}
}

// Final values stay right for story (score minus level start), survival and death
// results, and guests (rebuilt from the wire) count too.
func TestResultsCountFinalsForStorySurvivalDeadAndGuest(t *testing.T) {
	kills, best := int32(7), int32(300)
	for _, data := range []resultsData{
		{Flags: 4, Score: 900, LevelStartScore: 400, Kills: &kills, Highscore: &best},
		{Survival: true, Score: 900, Kills: &kills, Highscore: &best},
		{Dead: true, Flags: 4, Score: 650, LevelStartScore: 400, Kills: &kills, Highscore: &best},
	} {
		menu := newResultsMenu(data)
		slideIn(menu)
		for i := 0; i < 60*3; i++ {
			menu.update(1.0/60.0, false)
		}
		want := data.Score - data.LevelStartScore
		if data.Survival {
			want = data.Score
		}
		if menu.shownScore() != want || menu.shownKills() != 7 || menu.shownHighscore() != 300 {
			t.Fatalf("%+v shows %d %d %d", data, menu.shownScore(), menu.shownKills(), menu.shownHighscore())
		}
	}
	app := &app{}
	host := newResultsMenu(resultsData{Survival: true, Score: 1290, Kills: &kills, Highscore: &best})
	guest := newResultsMenu(app.wireResultsFrom(host).data())
	if guest.shownScore() != 0 {
		t.Fatal("guest screen must start from zero too")
	}
	slideIn(guest)
	for i := 0; i < 60*3; i++ {
		guest.update(1.0/60.0, false)
	}
	if guest.shownScore() != 1290 || guest.shownKills() != 7 {
		t.Fatal("guest finals")
	}
}

func TestResultsZeroRowsAndAbsentRowsDoNotBlockTheSkip(t *testing.T) {
	zero := int32(0)
	menu := newResultsMenu(resultsData{Survival: true, Score: 0, Kills: &zero})
	slideIn(menu)
	menu.update(.01, false)
	for i := 0; i < 200 && menu.counting(); i++ {
		menu.update(1.0/60.0, false)
	}
	if menu.counting() {
		t.Fatal("zero rows must land")
	}
	if s, _ := menu.popEffect(1); s != 1 {
		t.Fatal("a zero must not pop")
	}
}
