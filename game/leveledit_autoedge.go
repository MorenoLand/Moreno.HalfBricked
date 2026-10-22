package game

import (
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Auto-edge (experimental). The tilesets have no written rules for which edge tile goes beside which, so the rules are
// learned from the game's own levels: for every cell of every level that uses the same tileset, remember which tile stood
// there given the tiles to its north, east, south and west. After a stroke, the cells around what was painted are given the
// tile that most often stood there in the game's levels, when the levels agree clearly enough.

const (
	autoOutside = ^uint32(1) // beside the edge of the map
	autoMask    = 0x3ffff    // the tile number with its mirror flags
)

type autoStats struct {
	full  map[[4]uint32]map[uint32]int // (north, east, south, west) -> tile -> count
	pairV map[[2]uint32]map[uint32]int // (north, south)
	pairH map[[2]uint32]map[uint32]int // (west, east)
}

func newAutoStats() *autoStats {
	return &autoStats{full: map[[4]uint32]map[uint32]int{}, pairV: map[[2]uint32]map[uint32]int{}, pairH: map[[2]uint32]map[uint32]int{}}
}

func autoNorm(value uint32) uint32 {
	if value == editorEmpty {
		return editorEmpty
	}
	return value & autoMask
}

func autoNeighbours(cells []uint32, width, height, x, y int) (n, e, s, w uint32) {
	at := func(cx, cy int) uint32 {
		if cx < 0 || cy < 0 || cx >= width || cy >= height {
			return autoOutside
		}
		return autoNorm(cells[cy*width+cx])
	}
	return at(x, y-1), at(x+1, y), at(x, y+1), at(x-1, y)
}

func bump[K comparable](m map[K]map[uint32]int, key K, tile uint32) {
	inner := m[key]
	if inner == nil {
		inner = map[uint32]int{}
		m[key] = inner
	}
	inner[tile]++
}

// learn adds one layer of one level to the statistics.
func (s *autoStats) learn(cells []uint32, width, height int) {
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			n, e, sv, w := autoNeighbours(cells, width, height, x, y)
			tile := autoNorm(cells[y*width+x])
			bump(s.full, [4]uint32{n, e, sv, w}, tile)
			bump(s.pairV, [2]uint32{n, sv}, tile)
			bump(s.pairH, [2]uint32{w, e}, tile)
		}
	}
}

func bestOf(counts map[uint32]int) (tile uint32, count, total int) {
	for candidate, n := range counts {
		total += n
		if n > count || (n == count && candidate < tile) {
			tile, count = candidate, n
		}
	}
	return
}

// predict is the tile that most often stands between these neighbours, and how sure the levels are of it (ok is false
// when they do not agree well enough to change anything).
func (s *autoStats) predict(n, e, sv, w uint32) (tile uint32, ok bool) {
	if tile, count, total := bestOf(s.full[[4]uint32{n, e, sv, w}]); total >= 3 && float64(count)/float64(total) >= .6 {
		return tile, true
	}
	merged := map[uint32]int{}
	for t, c := range s.pairV[[2]uint32{n, sv}] {
		merged[t] += c
	}
	for t, c := range s.pairH[[2]uint32{w, e}] {
		merged[t] += c
	}
	if tile, count, total := bestOf(merged); total >= 8 && float64(count)/float64(total) >= .6 {
		return tile, true
	}
	return 0, false
}

// autoStatsFor learns from the game's levels of this tileset (once per tileset and layer, then remembered).
func (a *app) autoStatsFor(kind formats.LayerKind) *autoStats {
	e := a.editor
	key := strings.ToLower(e.level.Tileset) + "|" + string(kind)
	if stats, ok := e.autoCache[key]; ok {
		return stats
	}
	stats := newAutoStats()
	for _, info := range a.levels {
		if info.ID == editorTestID || a.customLevels[info.ID] {
			continue
		}
		level, err := a.pack.Load(info.ID)
		if err != nil || !strings.EqualFold(level.Tileset, e.level.Tileset) {
			continue
		}
		stats.learn(level.Layers[kind], level.Width, level.Height)
	}
	if e.autoCache == nil {
		e.autoCache = map[string]*autoStats{}
	}
	e.autoCache[key] = stats
	return stats
}

// autoEdge looks at the cells around the ones just changed and gives them the tile the levels expect there.
func (a *app) autoEdge() {
	e := a.editor
	if len(e.changedCells) == 0 {
		return
	}
	changed := e.changedCells
	e.changedCells = nil
	for _, kind := range []formats.LayerKind{formats.LayerG, formats.LayerD, formats.LayerH, formats.LayerHB} {
		if e.locked[kind] {
			continue
		}
		own := map[[2]int]bool{}
		var around [][2]int
		for _, cell := range changed {
			if cell.kind == kind {
				own[[2]int{cell.x, cell.y}] = true
			}
		}
		if len(own) == 0 {
			continue
		}
		seen := map[[2]int]bool{}
		for cell := range own {
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					next := [2]int{cell[0] + dx, cell[1] + dy}
					if !own[next] && !seen[next] && next[0] >= 0 && next[1] >= 0 && next[0] < e.level.Width && next[1] < e.level.Height {
						seen[next] = true
						around = append(around, next)
					}
				}
			}
		}
		stats := a.autoStatsFor(kind)
		cells := e.level.Layers[kind]
		for _, cell := range around {
			n, ea, s, w := autoNeighbours(cells, e.level.Width, e.level.Height, cell[0], cell[1])
			tile, ok := stats.predict(n, ea, s, w)
			index := cell[1]*e.level.Width + cell[0]
			if ok && tile != autoNorm(cells[index]) && tile != autoOutside {
				cells[index] = tile // the same undo step as the stroke that caused it
			}
		}
	}
}
