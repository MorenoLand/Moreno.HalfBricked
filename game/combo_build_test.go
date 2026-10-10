package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

// setTestBuild gives the rig a level from the given cache build (see waveBuild).
func setTestBuild(p *playState, build string) {
	if p.world == nil {
		p.world = &viewer.Viewer{}
	}
	p.world.Level.WaveBuild = build
}

// The held counts are one shared count on both builds: the live counter stands for both slots and shows their
// hits added together.
func TestComboCountIsOneSharedCountOnBothBuilds(t *testing.T) {
	for _, build := range []string{content.WaveBuildV7, content.WaveBuild125} {
		r := testRig(t)
		p := r.p
		setTestBuild(p, build)
		p.fillSecondarySlot(secondaryGrenade, "GRENADE")
		p.collectPickup("p_shotgun")
		p.updateCombo(1.0 / 60)
		if p.combo.primary == nil || p.combo.secondary == nil {
			t.Fatalf("%s: both held slots must own a tracker", build)
		}
		p.combo.primary.counter, p.combo.secondary.counter = 3, 4
		p.updateCombo(1.0 / 60)
		live := 0
		for _, tr := range p.comboShownTrackers() {
			if tr == p.combo.primary || tr == p.combo.secondary {
				live++
			}
		}
		if live != 1 {
			t.Fatalf("%s: %d live counters drawn, want one", build, live)
		}
		popup, ok := p.combo.shownPopup(p.combo.liveCounter())
		if !ok || popup.Text != "x7" {
			t.Fatalf("%s: shared counter %+v shown %v, want x7", build, popup, ok)
		}
	}
}

// Holding a weapon shows no counter; it appears once hits are counted.
func TestComboCounterHiddenUntilThereIsACount(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	if len(p.comboShownTrackers()) != 0 {
		t.Fatal("a counter is drawn before any hit")
	}
	p.combo.primary.counter = 2
	if len(p.comboShownTrackers()) != 1 {
		t.Fatal("the counter must show once the count is above 0")
	}
}
