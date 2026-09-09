package main

import "testing"

func TestPortalCellMatchesNativeBounds(t *testing.T) {
	for _, test := range []struct {
		x, y         float64
		wantX, wantY int
	}{
		{-1, -1, 0, 0},
		{364, 530, 5, 8},
		{730, 660, 11, 10},
		{9999, 9999, 0x22, 0x12},
	} {
		gotX, gotY := portalCell(test.x, test.y)
		if gotX != test.wantX || gotY != test.wantY {
			t.Fatalf("portalCell(%.1f,%.1f)=(%d,%d), want (%d,%d)", test.x, test.y, gotX, gotY, test.wantX, test.wantY)
		}
	}
}
