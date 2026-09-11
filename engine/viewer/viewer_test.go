package viewer

import "testing"

func TestPropSourceBoundsPreserveReversedUVs(t *testing.T) {
	x0, y0, x1, y1 := propSourceBounds(64, 0, 1, 64, 128, 64)
	if x0 != 64 || y0 != 0 || x1 != 1 || y1 != 64 {
		t.Fatalf("reversed prop UVs = %.0f,%.0f -> %.0f,%.0f, want 64,0 -> 1,64", x0, y0, x1, y1)
	}
}

func TestPropSourceBoundsUsesTextureForDegenerateUVs(t *testing.T) {
	x0, y0, x1, y1 := propSourceBounds(0, 0, 0, 64, 128, 64)
	if x0 != 0 || y0 != 0 || x1 != 128 || y1 != 64 {
		t.Fatalf("degenerate prop UVs = %.0f,%.0f -> %.0f,%.0f, want 0,0 -> 128,64", x0, y0, x1, y1)
	}
}
