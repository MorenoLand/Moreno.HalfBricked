package carousel

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
)

type levelCarouselEntry struct {
	Info                       formats.LevelInfo
	CatalogIndex, World, Level int
}
type LevelCarousel []levelCarouselEntry

func CatalogLevelCarousel(levels []formats.LevelInfo, mode int) LevelCarousel {
	if mode != 0 && mode != 1 {
		return nil
	}
	worlds, indices := map[int]int{}, map[int]int{}
	var result LevelCarousel
	flag := "STORY"
	if mode == 1 {
		flag = "SURVIVAL"
	}
	for index, info := range levels {
		world, exists := worlds[info.WorldIndex]
		if !exists {
			world = len(worlds)
			worlds[info.WorldIndex] = world
		}
		matches, survival := false, false
		for _, value := range info.Flags {
			if value == flag {
				matches = true
			}
			if value == "SURVIVAL" {
				survival = true
			}
		}
		local := -1
		if (mode == 1) == survival {
			local = indices[info.WorldIndex]
			indices[info.WorldIndex]++
		}
		if !matches {
			continue
		}
		result = append(result, levelCarouselEntry{Info: info, CatalogIndex: index, World: world, Level: local})
	}
	return result
}
func (levels LevelCarousel) Index(world, level int) int {
	if world < 0 || level < 0 {
		return -1
	}
	for index, entry := range levels {
		if entry.World == world && entry.Level == level {
			return index
		}
	}
	return -1
}
func (levels LevelCarousel) Selection(index int) (int, int, bool) {
	if index < 0 || index >= len(levels) || levels[index].Level < 0 {
		return 0, 0, false
	}
	return levels[index].World, levels[index].Level, true
}

type FiniteCore struct {
	Count, Selected int
	Position        float64
}

func NewFiniteCore(count, selected int) FiniteCore {
	core := FiniteCore{Count: max(0, count), Selected: -1}
	core.SelectIndex(selected)
	return core
}
func (core *FiniteCore) SelectIndex(index int) bool {
	if core.Count == 0 {
		core.Selected, core.Position = -1, 0
		return false
	}
	index = min(max(index, 0), core.Count-1)
	changed := core.Selected != index
	core.Selected, core.Position = index, float64(index)
	return changed
}
func (core *FiniteCore) Move(delta int) bool {
	if core.Count == 0 {
		return false
	}
	delta = min(max(delta, -core.Selected), core.Count-1-core.Selected)
	return core.SelectIndex(core.Selected + delta)
}
func (core *FiniteCore) Scroll(deltaItems float64) bool {
	if core.Count == 0 || math.IsNaN(deltaItems) || math.IsInf(deltaItems, 0) {
		return false
	}
	core.Position = min(max(core.Position+deltaItems, 0), float64(core.Count-1))
	selected := int(math.Ceil(core.Position - .5))
	changed := selected != core.Selected
	core.Selected = selected
	return changed
}
func (core *FiniteCore) Settle() { core.SelectIndex(core.Selected) }
func (core FiniteCore) window(size int) (int, int) {
	if core.Count == 0 || size <= 0 {
		return 0, 0
	}
	size = min(size, core.Count)
	first := min(max(core.Selected-size/2, 0), core.Count-size)
	return first, first + size
}
