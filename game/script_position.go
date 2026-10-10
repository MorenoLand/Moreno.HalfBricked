package game

import "math"

// randomPositionWithin is the GetPositionWithinRadius(x, y, minRadius, maxRadius[, clearance]) callback
// (1.2.5 closure 0x0013f3f8 -> FUN_00122828, the level's random-position search):
//
//   - draw r = rand(0..maxRadius-minRadius) (FUN_000dd3ac) and a 16-bit angle < 0xfff0 (FUN_000e624c);
//   - candidate = centre - (cos, sin)(angle) * (minRadius + r) from the 4096-entry sine table;
//   - accept it when the straight segment candidate -> centre crosses no solid/hazard cell
//     (FUN_00122714: 4 px steps, 0.25 * length iterations) and a circle of the clearance radius
//     (default 32, FUN_0012122c with hazards blocking) overlaps no solid tile;
//   - after 100 failed tries both radii shrink by 0.9 per try, after 200 the last candidate is returned.
func (p *playState) randomPositionWithin(centerX, centerY, minRadius, maxRadius, clearance float64) (float64, float64) {
	if p.world == nil {
		// No level is loaded, so there is no map to search: the callback returns the
		// cleared pair (0, 0) recorded from the ARMv7 result contract (PROJECT.md, 2026-09-20).
		return 0, 0
	}
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	pick := func() (float64, float64) {
		extent := float64(zombieRandom(p.rng, float32(maxRadius-minRadius)))
		angle := zombieBounded(p.rng, 0xfff0)
		index := int(angle >> 4)
		sine := math.Sin(2 * math.Pi * float64(index) / 4096)
		cosine := math.Sin(2 * math.Pi * float64((index+0x400)&0xfff) / 4096)
		return centerX - cosine*(minRadius+extent), centerY - sine*(minRadius+extent)
	}
	x, y := pick()
	for tries := 0; ; {
		blocked := p.segmentBlocked(x, y, centerX, centerY)
		if !blocked {
			_, _, blocked = p.collisionDisplacement(x, y, clearance, tileSize)
		}
		if !blocked {
			return x, y
		}
		tries++
		if tries > 100 {
			if tries > 200 {
				return x, y
			}
			minRadius *= .9
			maxRadius *= .9
		}
		x, y = pick()
	}
}

// segmentBlocked is FUN_00122714: march from (x, y) toward (toX, toY) in 4 px steps and report a
// solid or hazard collision cell (values 1 and 2) on the way.
func (p *playState) segmentBlocked(x, y, toX, toY float64) bool {
	dx, dy := toX-x, toY-y
	length := math.Hypot(dx, dy)
	if length == 0 {
		return false
	}
	stepX, stepY := dx/length*4, dy/length*4
	tileSize := float64(p.tileSize)
	if tileSize <= 0 {
		tileSize = 32
	}
	for i := 0; float64(i) < length*.25; i++ {
		if playerCollisionBlocks(p.collisionValue(int(math.Floor(x/tileSize)), int(math.Floor(y/tileSize)))) {
			return true
		}
		x += stepX
		y += stepY
	}
	return false
}

// scriptWalkPatience is how long a scripted zombie walk may fail to get closer to its target before the port ends it.
const scriptWalkPatience = 3.0

// scriptWalkGivesUp tracks the closest distance a walking script zombie has reached and reports true once it has not
// improved by half a pixel for scriptWalkPatience seconds.
func (p *playState) scriptWalkGivesUp(entity *scriptEntity, distance, dt float64) bool {
	if entity.walkBest == 0 {
		entity.walkBest = math.Inf(1) // a walk that was not started through WalkZombieTo
	}
	if distance < entity.walkBest-.5 {
		entity.walkBest, entity.walkStalled = distance, 0
		return false
	}
	entity.walkStalled += dt
	return entity.walkStalled >= scriptWalkPatience
}
