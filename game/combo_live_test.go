package game

import "testing"

// Every primary weapon's live tracker must follow its kills: the integer count rises.
func TestLiveTrackerRisesWithKillsForEachPrimaryWeapon(t *testing.T) {
	for _, pickup := range []string{"p_shotgun", "p_uzi", "p_minigun", "p_sniper", "p_flamer", "p_buzzsaw", "p_dual_pistol"} {
		r := testRig(t)
		p := r.p
		p.collectPickup(pickup)
		p.updateCombo(1.0 / 60)
		tr := p.combo.primary
		if tr == nil {
			t.Fatalf("%s: no live tracker", pickup)
		}
		if pickup == "p_flamer" {
			// Flame particles pay 0.01 only for hits the zombie survives (FUN_000a4bbc).
			p.zombies = nil
			for i := 0; i < 3; i++ {
				r.stack(1, p.x+20, p.y)
				p.zombies[i].health = 100
			}
			p.fire(1, 0)
			var peak float32
			for i := 0; i < 20; i++ { // the flamer drains 1 per second, so look at the peak
				r.tick(1)
				if tr.counter > peak {
					peak = tr.counter
				}
			}
			if peak <= 0 {
				t.Fatalf("flamer counter never rose")
			}
			continue
		}
		r.shootUntil(t, 3, 6, 2, func() bool { return false })
		if p.combo.primary != tr {
			t.Fatalf("%s: the live tracker was replaced while firing", pickup)
		}
		if int(tr.counter) < 1 {
			t.Fatalf("%s: counter %v did not rise with kills", pickup, tr.counter)
		}
		if popup, ok := tr.popup(); !ok || popup.Text == "x0" {
			t.Fatalf("%s: shown %q", pickup, popup.Text)
		}
	}
}
