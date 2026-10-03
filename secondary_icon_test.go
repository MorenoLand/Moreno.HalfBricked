package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"testing"
)

func TestDesktopGrenadeIconCentersInSlot(t *testing.T) {
	x, y := secondaryIconPosition(416, 208, formats.Vec2{X: -12}, false)
	if x != 416 || y != 208 {
		t.Fatalf("icon center (%g,%g), want slot center (416,208)", x, y)
	}
}
func TestMobileGrenadeIconPreservesXMLSlotOffset(t *testing.T) {
	x, y := secondaryIconPosition(400, 220, formats.Vec2{X: -12}, true)
	if x != 388 || y != 220 {
		t.Fatalf("icon position (%g,%g), want native (388,220)", x, y)
	}
}
