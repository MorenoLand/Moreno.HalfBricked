package game

import "testing"

func TestSlowMotionSkipsTicksThenEasesBack(t *testing.T) {
	p := &playState{}
	if p.slowMoSkipTick() {
		t.Fatal("no effect, no skipped ticks")
	}
	p.startSlowMo(slowMoWaveSecs)
	ticks, simulated := 0, 0
	for p.slowMoLeft > 0 {
		ticks++
		if !p.slowMoSkipTick() {
			simulated++
		}
	}
	if ticks < 40 || simulated >= ticks || simulated < ticks/3 {
		t.Fatalf("%d of %d ticks simulated", simulated, ticks)
	}
	if p.slowMoScale() != 1 || p.slowMoSkipTick() {
		t.Fatal("effect must end at full speed")
	}
}

func TestSlowMotionKeepsTheLongerEffect(t *testing.T) {
	p := &playState{}
	p.startSlowMo(slowMoFinalSecs)
	p.startSlowMo(slowMoWaveSecs)
	if p.slowMoTotal != slowMoFinalSecs {
		t.Fatal("a shorter effect replaced the final one")
	}
}
