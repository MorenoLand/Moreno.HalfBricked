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
	if ticks < 60 || simulated >= ticks/2 || simulated < ticks/6 {
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

func TestSlowMotionDrawsBetweenTheLastTwoSteps(t *testing.T) {
	p := &playState{x: 0}
	p.startSlowMo(slowMoFinalSecs)
	for p.slowMoSkipTick() {
	}
	p.slowMoSkipTick() // first tick of the effect may or may not step; drive until a step has been captured
	p.x = 10           // the step moved the player from 0 to 10
	p.slowPrev.x = 0
	p.slowInterp, p.slowMoAccum = true, .25
	restore := p.applySlowInterp()
	if p.x != 2.5 {
		t.Fatalf("drawn x %v, want 2.5", p.x)
	}
	restore()
	if p.x != 10 {
		t.Fatalf("live x %v, want 10", p.x)
	}
}
