package game

import "testing"

// Native rule (Research/native/combo-visibility-2026-10-09.md). The 1.2.5 (HD)
// draw FUN_000f73b8 serves the primary (kind 0x20), grenade-slot (kind 0x21) and
// pop-up classes alike, and every tracker starts with alpha byte 255 (white copied
// by FUN_000f5b7c in FUN_000f6c38), so a held count is drawn from pickup. v7 (SD)
// differs: the grenade-slot class answers a bare return at +0x18 (0x00099858).

func TestComboSecondaryCountDrawnWhileHeld(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.fillSecondarySlot(secondaryGrenade, "GRENADE")
	p.updateCombo(1.0 / 60)
	sec := p.combo.secondary
	if sec == nil {
		t.Fatal("a held grenade must own a secondary tracker")
	}
	sec.counter = 7
	p.updateCombo(1.0 / 60)
	popup, ok := sec.popup()
	if !ok || popup.Text != "x7" || popup.A != 255 {
		t.Fatalf("secondary count %+v shown %v, want x7 at alpha 255", popup, ok)
	}
}

func TestComboPrimaryCountDrawnFromPickupAtZero(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_uzi")
	p.updateCombo(1.0 / 60)
	tr := p.combo.primary
	if tr == nil {
		t.Fatal("no primary tracker after pickup")
	}
	popup, ok := tr.popup()
	if !ok || popup.Text != "x0" || popup.A != 255 {
		t.Fatalf("primary count after pickup: %+v shown %v, want x0 at alpha 255", popup, ok)
	}
}

func TestComboBothHeldCountsShowAtZeroLikeHD(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.fillSecondarySlot(secondaryGrenade, "GRENADE")
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	shown := 0
	for _, tr := range p.combo.trackers() {
		if popup, ok := tr.popup(); ok {
			shown++
			if popup.Text != "x0" {
				t.Fatalf("zero count text %q, want x0", popup.Text)
			}
		}
	}
	if shown != 2 {
		t.Fatalf("%d counts show with no hits, want both held trackers (1.2.5 draws each)", shown)
	}
}

func TestComboLiveCountsAnchorWithoutStacking(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.fillSecondarySlot(secondaryGrenade, "GRENADE")
	p.collectPickup("p_uzi")
	p.updateCombo(1.0 / 60)
	pri, sec := p.combo.primary, p.combo.secondary
	if pri == nil || sec == nil {
		t.Fatal("both trackers must be live")
	}
	priSize, secSize := pri.size, sec.size
	p.updateCombo(1.0 / 60)
	if x, y := p.comboAnchor(priSize); pri.posX != x || pri.posY != y {
		t.Fatalf("primary anchored at (%v,%v), want the plain anchor (%v,%v)", pri.posX, pri.posY, x, y)
	}
	if x, y := p.comboAnchor(secSize); sec.posX != x || sec.posY != y {
		t.Fatalf("secondary anchored at (%v,%v), want the plain anchor (%v,%v)", sec.posX, sec.posY, x, y)
	}
}
