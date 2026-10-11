package game

// PORT ADDITION (not in the original binary): slow motion on the kill that ends a wave.
//
// Under the all-dead wave rule a wave ends the moment its last zombie dies. That moment starts a short slow-down:
// the simulation runs on only some of the 60 Hz ticks (the rest are skipped), holds at slowMoHoldScale and eases back
// to full speed. The kill that ends the final wave, and so the level, gets a longer one. Switch it off with
// -slow-motion=false.
var portSlowMo = true

const (
	slowMoHoldScale = 0.3  // fraction of ticks simulated at the bottom of the effect
	slowMoWaveSecs  = 0.7  // real seconds of the effect for an ordinary wave
	slowMoFinalSecs = 1.8  // real seconds of the effect for the last wave of a level
	slowMoHoldShare = 0.55 // part of the effect spent at the hold scale before easing back
)

// startSlowMo begins the effect for the given real duration. A longer one in progress is kept.
func (p *playState) startSlowMo(seconds float64) {
	if !portSlowMo || seconds <= p.slowMoLeft {
		return
	}
	p.slowMoLeft, p.slowMoTotal = seconds, seconds
}

// slowMoScale is the fraction of ticks that are simulated now (1 outside the effect).
func (p *playState) slowMoScale() float64 {
	if p.slowMoLeft <= 0 || p.slowMoTotal <= 0 {
		return 1
	}
	elapsed := (p.slowMoTotal - p.slowMoLeft) / p.slowMoTotal
	if elapsed <= slowMoHoldShare {
		return slowMoHoldScale
	}
	ease := (elapsed - slowMoHoldShare) / (1 - slowMoHoldShare)
	return slowMoHoldScale + (1-slowMoHoldScale)*ease
}

// slowMoSkipTick reports whether this 60 Hz tick is skipped. It advances the effect in real time.
func (p *playState) slowMoSkipTick() bool {
	if p.slowMoLeft <= 0 {
		return false
	}
	p.slowMoAccum += p.slowMoScale()
	p.slowMoLeft -= 1.0 / 60.0
	if p.slowMoLeft <= 0 {
		p.slowMoLeft, p.slowMoAccum = 0, 0
		return false
	}
	if p.slowMoAccum >= 1 {
		p.slowMoAccum--
		return false
	}
	return true
}
