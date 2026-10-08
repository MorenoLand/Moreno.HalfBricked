package game

// The native entity grid (spatial hash) that projectile hit passes walk.
//
// Evidence (libmortargame.so v7, image base 0x10000):
//
//   - Construction: the scene constructor FUN_0006f270 allocates a 0x14-byte grid
//     object and calls FUN_0009145c(grid, 16.0, 16.0) (literal 0x0006f49c =
//     0x41800000). The init stores the cell size at +0xc/+0x10, computes
//     cols = int(2240.0 / cellW) = 140 and rows = int(1216.0 / cellH) = 76
//     (literals 0x000914d8 = 0x450c0000, 0x000914dc = 0x44980000) and allocates
//     cols*rows cells of 0x104 bytes: 64 entity pointers and a last-index word.
//   - Registration: FUN_000a17cc (called after every zombie move, FUN_000a18e4 and
//     FUN_000a1b08) calls FUN_00091918(grid, entity, extent, cells, count, lastCell)
//     with extent = scene+0x509dc->+0x28 * 0.3. scene+0x509dc is the player
//     (script GetPlayerX, FUN_000daf90, reads +0x10 of it) and the player
//     constructor FUN_00094a94 stores +0x28 = 0x42800000 = 64.0. Every entity is
//     therefore registered over +-19.2 px of its own position, whatever its size.
//   - FUN_00091918 recomputes the cell list only when the cell of the entity's
//     CENTRE (FUN_00091690) differs from the one stored at entity+0x310 (or the
//     list is empty); otherwise the old list stays. The cell list is the box
//     [x-ext, x+ext] x [y-ext, y+ext] from FUN_000917f4: each bound is
//     int(value/cell) truncated toward zero, <1 -> 0, clamped to cols-1 / rows-1.
//   - Query: FUN_000916f0(grid, pos, extent) returns the same box with the same
//     clamps. The projectile pass (FUN_000a5f74) loops every cell of the query
//     box and every entity listed in it without de-duplication, so an entity
//     registered in n of the queried cells is visited n times.
const (
	entityGridCell       = 16.0
	entityGridCols       = 140
	entityGridRows       = 76
	entityGridPlayerBody = 64.0
	entityGridBodyScale  = float32(0.3)
)

// entityGridRegisterExtent is 64.0 * 0.3 evaluated in float32 (FUN_000a17cc).
var entityGridRegisterExtent = float32(entityGridPlayerBody) * entityGridBodyScale

// entityGridRange is a box of grid cells, inclusive.
type entityGridRange struct{ x0, x1, y0, y1 int }

// entityGridRegistration is the per-entity state FUN_00091918 keeps (cells at
// +0x60, count at +0x50, last centre cell at +0x310).
type entityGridRegistration struct {
	valid  bool
	center int
	cells  entityGridRange
}

func entityGridBound(value float32, limit int) int {
	index := int(value / entityGridCell)
	if index < 1 {
		return 0
	}
	if index >= limit-1 {
		return limit - 1
	}
	return index
}

// entityGridBox is FUN_000917f4 / FUN_000916f0.
func entityGridBox(x, y float64, extent float32) entityGridRange {
	fx, fy := float32(x), float32(y)
	return entityGridRange{
		x0: entityGridBound(fx-extent, entityGridCols),
		x1: entityGridBound(fx+extent, entityGridCols),
		y0: entityGridBound(fy-extent, entityGridRows),
		y1: entityGridBound(fy+extent, entityGridRows),
	}
}

// entityGridCenter is FUN_00091690: the cell index of a point.
func entityGridCenter(x, y float64) int {
	cx := int(float32(x) / entityGridCell)
	if cx > entityGridCols-1 {
		cx = entityGridCols - 1
	}
	if cx < 0 {
		cx = 0
	}
	cy := int(float32(y) / entityGridCell)
	if cy > entityGridRows-1 {
		cy = entityGridRows - 1
	}
	if cy < 0 {
		cy = 0
	}
	return cy*entityGridCols + cx
}

// register mirrors FUN_00091918's early-out: the cell list is rebuilt only when
// the centre cell changed.
func (r *entityGridRegistration) register(x, y float64) {
	center := entityGridCenter(x, y)
	if r.valid && r.center == center {
		return
	}
	r.valid, r.center = true, center
	r.cells = entityGridBox(x, y, entityGridRegisterExtent)
}

// visits is how many times a projectile pass over the query box lists the
// entity: once per registered cell inside the box.
func (r *entityGridRegistration) visits(query entityGridRange) int {
	if !r.valid {
		return 0
	}
	width := min(r.cells.x1, query.x1) - max(r.cells.x0, query.x0) + 1
	height := min(r.cells.y1, query.y1) - max(r.cells.y0, query.y0) + 1
	if width <= 0 || height <= 0 {
		return 0
	}
	return width * height
}

// refreshEntityGrid registers every zombie at its current position. The native
// registration runs right after each zombie move, so the port runs it once per
// tick before any projectile pass reads it.
func (p *playState) refreshEntityGrid() {
	for index := range p.zombies {
		zombie := &p.zombies[index]
		zombie.grid.register(zombie.x, zombie.y)
	}
}

// zombieGridVisits is the number of times the projectile pass at (x, y) with
// the given query extent lists this zombie.
func (p *playState) zombieGridVisits(zombie *zombieState, x, y float64, extent float32) int {
	zombie.grid.register(zombie.x, zombie.y)
	return zombie.grid.visits(entityGridBox(x, y, extent))
}
