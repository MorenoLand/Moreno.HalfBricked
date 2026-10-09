package game

import "math"

// Zombie and boss versus level geometry: FUN_000a17cc -> FUN_000be3c0.
//
// Evidence (libmortargame.so v7): every zombie-class entity (plain, speedy,
// smart, exploding, bosses; the rex overrides the move slot with a tail call to
// the same FUN_000a18e4) moves straight along its heading
// (FUN_000a18e4: pos += (cos h, sin h) * speed * dt * speedFactor) and then
// FUN_000a17cc asks FUN_000be3c0(level, pos, 0.3 * body width, &push, 1) for a
// push-out and subtracts it from the position. There is no path search for these
// classes (only the smart zombie's own FUN_0009c168 builds paths), no stuck timer,
// no sidestep and no despawn: a blocked zombie keeps pushing into the wall and is
// pushed back out along the axis of least penetration every frame. What frees it
// is the heading itself (the weave of deviateAmount/deviateCycleSpeed, the wander
// retarget while not alerted, the turn rate).
//
// FUN_000be3c0: tiles with collision value 1, and value 2 because the last
// argument is 1 for zombies and for the player, block. The scan box is
// [x-r, x+r] x [y-0.666r, y+0.666r] in 32 px tiles. A single overlapping tile is
// resolved as a circle of radius r against the tile box in y-squashed space
// (y / 0.666, FUN_001be118); several tiles use the axis rule below with tiles
// that have exactly one blocked vertical (horizontal) neighbour widened to 32 half
// extent. FUN_001be118 itself was not decompiled; the circle-versus-box push is
// the geometric reading of its call (UNRESOLVED: exact rounding at corners).
const zombieTileHalf = 16.0

// nativeTilePush returns the displacement to add to (x, y) so that a body of
// radius r leaves the blocking tiles.
func (p *playState) nativeTilePush(x, y, r float64) (float64, float64) {
	return p.nativeTilePushMode(x, y, r, true)
}

// nativeTilePushMode is FUN_000be3c0; walkers pass the blocking flag (values 1 and
// 2 block), projectiles pass 0 (only value 1 blocks).
func (p *playState) nativeTilePushMode(x, y, r float64, blockTwo bool) (float64, float64) {
	tile := float64(p.tileSize)
	if tile <= 0 {
		tile = 32
	}
	blocks := func(tx, ty int) bool {
		if !inLevel(p, tx, ty) {
			return false
		}
		value := p.collisionValue(tx, ty)
		return value == 1 || (blockTwo && value == 2)
	}
	ry := r * tileCollisionYSquash
	minX, maxX := int(math.Max(0, (x-r)/tile)), int(math.Min(float64(p.world.Level.Width-1), (x+r)/tile))
	minY, maxY := int(math.Max(0, (y-ry)/tile)), int(math.Min(float64(p.world.Level.Height-1), (y+ry)/tile))
	hits := 0
	var pushX, pushY float64
	var lastX, lastY int
	allSameX, allSameY := true, true
	for tx := minX; tx <= maxX; tx++ {
		for ty := minY; ty <= maxY; ty++ {
			if !blocks(tx, ty) {
				continue
			}
			up, down := blocks(tx, ty-1), blocks(tx, ty+1)
			left, right := blocks(tx-1, ty), blocks(tx+1, ty)
			cx, cy := (float64(tx)+.5)*tile, (float64(ty)+.5)*tile
			halfY, halfX := zombieTileHalf, zombieTileHalf
			if up && !down {
				cy -= zombieTileHalf
				halfY = tile
			} else if !up && down {
				cy += zombieTileHalf
				halfY = tile
			}
			if left && !right {
				cx -= zombieTileHalf
				halfX = tile
			} else if !left && right {
				cx += zombieTileHalf
				halfX = tile
			}
			dx, dy := x-cx, y-cy
			if math.Abs(dx) >= halfX+r || math.Abs(dy) >= halfY+ry {
				continue
			}
			penX, penY := halfX+r-math.Abs(dx), halfY+ry-math.Abs(dy)
			// FUN_000be3c0 writes only the axis of least penetration of each tile and
			// keeps the other component from earlier tiles.
			if penX < penY {
				pushX = math.Copysign(penX, dx)
				if dx == 0 {
					pushX = penX
				}
			} else {
				pushY = math.Copysign(penY, dy)
				if dy == 0 {
					pushY = penY
				}
			}
			if hits > 0 {
				allSameX = allSameX && tx == lastX
				allSameY = allSameY && ty == lastY
			}
			lastX, lastY = tx, ty
			hits++
		}
	}
	switch {
	case hits == 0:
		return 0, 0
	case hits == 1:
		return p.circleBoxPush(x, y, r, lastX, lastY, tile)
	}
	if allSameX || allSameY {
		// Tiles in one row or column ("bVar4"): keep only the larger axis.
		if math.Abs(pushX) <= math.Abs(pushY) {
			pushX = 0
		} else {
			pushY = 0
		}
	}
	return math.Max(-tile, math.Min(tile, pushX)), math.Max(-tile, math.Min(tile, pushY))
}

func inLevel(p *playState, tx, ty int) bool {
	return tx >= 0 && ty >= 0 && tx < p.world.Level.Width && ty < p.world.Level.Height
}

// circleBoxPush pushes a circle of radius r out of one 32 px tile, measured in
// the y-squashed space of FUN_000be3c0 (box half extents 16 and 16/0.666).
func (p *playState) circleBoxPush(x, y, r float64, tx, ty int, tile float64) (float64, float64) {
	cx, cy := (float64(tx)+.5)*tile, (float64(ty)+.5)*tile
	lx, ly := x-cx, (y-cy)/tileCollisionYSquash
	hx, hy := tile/2, tile/2/tileCollisionYSquash
	nx, ny := math.Max(-hx, math.Min(hx, lx)), math.Max(-hy, math.Min(hy, ly))
	ox, oy := lx-nx, ly-ny
	dist := math.Hypot(ox, oy)
	var px, py float64
	switch {
	case dist > 1e-9:
		if dist >= r {
			return 0, 0
		}
		px, py = ox/dist*(r-dist), oy/dist*(r-dist)
	default:
		// Centre inside the box: leave through the nearest face.
		gapX, gapY := hx-math.Abs(lx)+r, hy-math.Abs(ly)+r
		if gapX < gapY {
			px = math.Copysign(gapX, lx)
		} else {
			py = math.Copysign(gapY, ly)
		}
	}
	return px, py * tileCollisionYSquash
}
