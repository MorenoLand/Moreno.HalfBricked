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

// The held counts are one shared count on both builds: the pop-up shows their hits added together, and it is not a
// permanent badge: no live tracker is drawn.
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
		for i := 0; i < 30; i++ { // the pop-up counts up to the shared count
			p.updateCombo(1.0 / 60)
		}
		for _, tr := range p.comboShownTrackers() {
			if tr == p.combo.primary || tr == p.combo.secondary {
				t.Fatalf("%s: a live tracker is drawn as a permanent badge", build)
			}
		}
		popup, ok := p.killPopPopup()
		if !ok || popup.Text != "x7" {
			t.Fatalf("%s: shared pop-up %+v shown %v, want x7", build, popup, ok)
		}
	}
}

// Holding a weapon shows nothing; the count pops up when hits are counted and goes away again.
func TestComboCounterPopsUpOnHitsAndFadesAway(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	if _, ok := p.killPopPopup(); ok {
		t.Fatal("a count is drawn before any hit")
	}
	p.combo.primary.counter = 2
	p.updateCombo(1.0 / 60)
	if popup, ok := p.killPopPopup(); !ok || popup.A == 0 {
		t.Fatalf("no pop-up after a hit: %+v %v", popup, ok)
	}
	// Barry turning round must not move it: it is anchored where the kill happened.
	before, _ := p.killPopPopup()
	p.angle = (p.angle + 4) % 9
	after, _ := p.killPopPopup()
	if before.X != after.X || before.Y != after.Y {
		t.Fatalf("the pop-up follows Barry's facing: %v,%v -> %v,%v", before.X, before.Y, after.X, after.Y)
	}
	for i := 0; i < 120; i++ {
		p.combo.primary.counter = 2 // the count stays, no new hits
		p.updateCombo(1.0 / 60)
	}
	if _, ok := p.killPopPopup(); ok {
		t.Fatal("the pop-up is still on screen two seconds after the last hit")
	}
}
