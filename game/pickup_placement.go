package game

import (
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Pickup spawner placement (G16). v7 FUN_000bec48 (1.2.5 FUN_00121b50), the branch for type ids 0x23..0x3b
// (1.2.5 0x23..0x40):
//
//	try 1..10: the point is a marker-tile point as for every spawner (nativeSpawnPoint). It is accepted when its
//	  tile holds a spawn marker (FUN_000c1124 -> FUN_000be2c4 value > 2) and no pickup is listed in its 16 px entity
//	  grid cell (FUN_00091690 / FUN_00091680: any entity whose type byte is 0x23..0x3b); otherwise a new marker point
//	  is rolled.
//	try 11..20: the point is a random map cell, (rnd(W - 6) + 5) * 32 + 16 by (rnd(H - 6) + 5) * 32 + 16 (x then y;
//	  literals 0x000bf03c = 32.0, 0x000bf040 = 16.0), with the same acceptance test.
//	after the 20th failed try nothing spawns (the spawner's count has already been spent).
//
// A pickup is listed in the grid over +-size (its +0x28 = 50, or 80 for the special crate) of its position, registered
// by FUN_000928f4 in the bounce branch, i.e. from its first landing on.
//
// UNRESOLVED: 1.2.5 skips the pickup record when the mode object's vtable +0x20 returns non-zero (the meaning of that
// virtual was not read); the port does not skip it.
const (
	pickupPlacementMarkerTries = 10
	pickupPlacementTotalTries  = 20
	pickupPlacementMapMargin   = 6
	pickupPlacementMapOffset   = 5
	pickupPlacementTile        = 32.0
	pickupPlacementHalfTile    = 16.0
)

func isPickupSpawnName(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "p_")
}

// pickupCellTaken reports whether a registered pickup is listed in the grid cell of (x, y).
func (p *playState) pickupCellTaken(x, y float64) bool {
	cx := entityGridBound(float32(x), entityGridCols)
	cy := entityGridBound(float32(y), entityGridRows)
	for _, e := range p.scriptEntities {
		if e == nil || e.kind != "pickup" || (e.drop && !e.registered) {
			continue
		}
		box := entityGridBox(e.x, e.y, float32(pickupDrawSize(e.texture)))
		if cx >= box.x0 && cx <= box.x1 && cy >= box.y0 && cy <= box.y1 {
			return true
		}
	}
	return false
}

// pickupPointAcceptable is the acceptance test of one try.
func (p *playState) pickupPointAcceptable(pt formats.Vec2) bool {
	if p.world == nil {
		return true
	}
	tx, ty := int(float32(pt.X)*(1.0/32)), int(float32(pt.Y)*(1.0/32))
	if !inLevel(p, tx, ty) || p.collisionValue(tx, ty) <= 2 {
		return false
	}
	return !p.pickupCellTaken(pt.X, pt.Y)
}

// nativePickupPoint runs the retry loop from the first rolled point. ok is false when all 20 tries fail.
func (p *playState) nativePickupPoint(points []formats.Vec2, first formats.Vec2) (formats.Vec2, bool) {
	if p.world == nil || p.rng == nil {
		return first, true
	}
	point := first
	for try := 1; try <= pickupPlacementTotalTries; try++ {
		if p.pickupPointAcceptable(point) {
			return point, true
		}
		if try < pickupPlacementMarkerTries {
			point = p.nativeSpawnPoint(points)
			continue
		}
		width, height := p.world.Level.Width, p.world.Level.Height
		x := int(zombieBounded(p.rng, uint32(max(width-pickupPlacementMapMargin, 0)))) + pickupPlacementMapOffset
		y := int(zombieBounded(p.rng, uint32(max(height-pickupPlacementMapMargin, 0)))) + pickupPlacementMapOffset
		point = formats.Vec2{X: float64(float32(x)*pickupPlacementTile + pickupPlacementHalfTile), Y: float64(float32(y)*pickupPlacementTile + pickupPlacementHalfTile)}
	}
	return formats.Vec2{}, false
}
