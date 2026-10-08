package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"strings"
	"testing"
)

func resultsTestVariables(t *testing.T) formats.FrontendVariables {
	t.Helper()
	v, err := formats.ParseVariables(strings.NewReader(`<Variables><Vec2 name="ENDSCREEN_MENU_BOX_INNER_POS_VAR" value="392,162"/><Vec2 name="ENDSCREEN_MENU_BOX_INNER_SIZE_VAR" value="116,42"/><Vec2 name="ENDSCREEN_REPLAY_BOX_INNER_POS_VAR" value="392,228"/><Vec2 name="ENDSCREEN_REPLAY_BOX_INNER_SIZE_VAR" value="116,48"/><Vec2 name="ENDSCREEN_MAIN_OFFSET_BEGIN_VAR" value="200,0"/><Vec2 name="ENDSCREEN_SCORE_OFFSET_BEGIN_VAR" value="0,-120"/></Variables>`))
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestResultsNativeVisibility(t *testing.T) {
	for _, row := range []struct {
		survival bool
		flags    uint32
		want     bool
	}{{false, 1, false}, {false, 0x10, false}, {false, 0x11, false}, {false, 4, true}, {false, 0x14, true}, {true, 0, true}} {
		if got := resultsVisible(row.survival, row.flags); got != row.want {
			t.Fatalf("%+v: %v", row, got)
		}
	}
}
func TestResultsScoreAndUnknownData(t *testing.T) {
	best, kills := int32(300), int32(12)
	m := newResultsMenu(resultsData{Flags: 4, Score: 400, LevelStartScore: 150, Highscore: &best, Kills: &kills})
	best, kills = 999, 999
	if m.score() != 250 || *m.Data.Highscore != 300 || *m.Data.Kills != 12 {
		t.Fatal(m.Data)
	}
	m = newResultsMenu(resultsData{Survival: true, Score: 400, LevelStartScore: 150})
	if m.score() != 400 || m.Data.Highscore != nil || m.Data.Kills != nil {
		t.Fatal(m.Data)
	}
}
func TestResultsActionsAndExitGate(t *testing.T) {
	v := resultsTestVariables(t)
	for _, survival := range []bool{false, true} {
		m := newResultsMenu(resultsData{Survival: survival, Flags: 4})
		if m.hit(v, 392, 162) != resultsNone || m.activate(resultsMainMenu) {
			t.Fatal("input before entry")
		}
		m.update(.34, false)
		want := resultsContinue
		if survival {
			want = resultsReplay
		}
		if m.hit(v, 392, 228) != want || m.hit(v, 392, 162) != resultsMainMenu || m.hit(v, 0, 0) != resultsNone {
			t.Fatal("native input rectangles")
		}
		if !m.activate(want) || m.activate(want) {
			t.Fatal("activation must be once")
		}
		if m.update(.34, true) != resultsNone || m.update(0, false) != want || m.update(1, false) != resultsNone {
			t.Fatal("external gate or repeated transition")
		}
	}
}
func TestResultsAutomaticContinuationIsNotMenu(t *testing.T) {
	m := newResultsMenu(resultsData{Flags: 0x10})
	if m.update(1, true) != resultsNone || m.update(0, false) != resultsContinue || m.update(1, false) != resultsNone {
		t.Fatal("ENDSTORY-only continuation")
	}
}
func TestResultsOffsetNativeSpeed(t *testing.T) {
	m := newResultsMenu(resultsData{Flags: 4})
	v := resultsTestVariables(t)
	if m.offset(v, "SCORE") != (formats.Vec2{Y: -120}) {
		t.Fatal("score entry offset")
	}
	m.update(1.0/6, false)
	if m.offset(v, "MAIN") != (formats.Vec2{X: 100}) {
		t.Fatal("entry speed")
	}
	m.update(.2, false)
	if !m.activate(resultsMainMenu) {
		t.Fatal("menu selection")
	}
	m.update(1.0/6, false)
	if m.offset(v, "MAIN") != (formats.Vec2{X: 100}) {
		t.Fatal("exit speed")
	}
}

// The native enter routine (1.2.5 0x000cf030) sprintf's the final totals straight
// into the text components; there is no count-up to animate.
func TestResultsShowFinalValuesImmediately(t *testing.T) {
	kills := int32(135)
	menu := newResultsMenu(resultsData{Survival: true, Score: 1000, Kills: &kills})
	if menu.shownScore() != 1000 || menu.shownKills() != 135 {
		t.Fatalf("values %d %d", menu.shownScore(), menu.shownKills())
	}
	if newResultsMenu(resultsData{Survival: true, Score: 5}).shownKills() != 0 {
		t.Fatal("absent kills row must stay zero")
	}
}
