package game

import "testing"

func TestScoreTextUsesNativeCenteredUniformSize(t *testing.T) {
	x, y, scale := scoreTextGeometry(240, 17, 110, 24, .75, 2, 1.6875)
	if x != 370 || y != 4.6875 || scale != 1.5 {
		t.Fatalf("score geometry = (%g,%g,%g), want (370,4.6875,1.5)", x, y, scale)
	}
}
