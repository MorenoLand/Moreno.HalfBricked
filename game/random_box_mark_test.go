package game

import "testing"

func TestLifePopupFliesToTheCounterAndExpires(t *testing.T) {
	p := &playState{}
	p.lifePopups = []lifePopup{{x: 100, y: 120}}
	x0, _, _, _ := lifePopupPose(p.lifePopups[0])
	if x0 != 100 {
		t.Fatalf("starts at the crate, x %v", x0)
	}
	end := lifePopup{x: 100, y: 120, age: lifePopupTotal - .01}
	x, y, _, alpha := lifePopupPose(end)
	if x < lifeCounterX-3 || y > lifeCounterY+3 || alpha > .2 {
		t.Fatalf("arrives at the counter faded: %.1f,%.1f alpha %.2f", x, y, alpha)
	}
	for i := 0; i < 70; i++ {
		p.updateLifePopups(1.0 / 60.0)
	}
	if len(p.lifePopups) != 0 {
		t.Fatal("popup outlived its flight")
	}
}
