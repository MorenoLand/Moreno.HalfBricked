package main

import (
	"math"
	"testing"
)

func TestTitleCockingFollowsRotatedShotgunAxis(t *testing.T) {
	a := &app{titleCocking: true}
	angle := titleBarryPieces[1].angle
	for _, sample := range []struct{ ticks, distance float64 }{{0, 0}, {4, 5}, {8, 10}, {12, 10}, {16, 10}, {20, 5}, {24, 0}, {32, 0}} {
		a.titleSoundElapsed = sample.ticks / 60
		x, y := a.titleCockingOffset(2)
		if math.Abs(x+math.Sin(angle)*sample.distance) > 1e-9 || math.Abs(y-math.Cos(angle)*sample.distance) > 1e-9 {
			t.Fatalf("tick %g moved off barrel axis: %g,%g", sample.ticks, x, y)
		}
		if math.Abs(x*math.Cos(angle)+y*math.Sin(angle)) > 1e-9 {
			t.Fatal("pump has perpendicular displacement")
		}
	}
	for _, index := range []int{0, 1, 3} {
		if x, y := a.titleCockingOffset(index); x != 0 || y != 0 {
			t.Fatal("other title piece moved")
		}
	}
	a.titleCocking = false
	if x, y := a.titleCockingOffset(2); x != 0 || y != 0 {
		t.Fatal("idle pump moved")
	}
}
