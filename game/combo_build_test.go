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

// Both held counts are drawn on both builds (the count display is left unchanged on request).
func TestComboBothCountsShownOnBothBuilds(t *testing.T) {
	for _, build := range []string{content.WaveBuildV7, content.WaveBuild125} {
		r := testRig(t)
		p := r.p
		setTestBuild(p, build)
		p.fillSecondarySlot(secondaryGrenade, "GRENADE")
		p.collectPickup("p_shotgun")
		p.updateCombo(1.0 / 60)
		if p.combo.secondary == nil {
			t.Fatalf("%s: a held grenade must own a secondary tracker", build)
		}
		found := false
		for _, tr := range p.comboShownTrackers() {
			if tr == p.combo.secondary {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: the grenade-slot count is not drawn", build)
		}
	}
}
