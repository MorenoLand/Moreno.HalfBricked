package game

import "testing"

func TestScriptEntityVFlipUsesHorizontalUVAxis(t *testing.T) {
	for _, test := range []struct {
		name                       string
		directionFlip, scriptVFlip bool
		wantX                      float64
	}{
		{"unflipped", false, false, 2},
		{"lab worker", false, true, -2},
		{"direction mirror", true, false, -2},
		{"combined mirrors", true, true, 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			x, y := scriptEntityRenderScale(2, 3, test.directionFlip, test.scriptVFlip)
			if x != test.wantX || y != 3 {
				t.Fatalf("scale = (%v, %v), want (%v, 3)", x, y, test.wantX)
			}
		})
	}
}
