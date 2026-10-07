package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
)

type levelCarouselEntry struct {
	Info                       formats.LevelInfo
	CatalogIndex, World, Level int
}
type levelCarousel []levelCarouselEntry

func catalogLevelCarousel(levels []formats.LevelInfo, mode int) levelCarousel {
	if mode != 0 && mode != 1 {
		return nil
	}
	worlds, indices := map[int]int{}, map[int]int{}
	var result levelCarousel
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
func (levels levelCarousel) index(world, level int) int {
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
func (levels levelCarousel) selection(index int) (int, int, bool) {
	if index < 0 || index >= len(levels) || levels[index].Level < 0 {
		return 0, 0, false
	}
	return levels[index].World, levels[index].Level, true
}

type finiteCore struct {
	count, selected int
	position        float64
}

func newFiniteCore(count, selected int) finiteCore {
	core := finiteCore{count: max(0, count), selected: -1}
	core.selectIndex(selected)
	return core
}
func (core *finiteCore) selectIndex(index int) bool {
	if core.count == 0 {
		core.selected, core.position = -1, 0
		return false
	}
	index = min(max(index, 0), core.count-1)
	changed := core.selected != index
	core.selected, core.position = index, float64(index)
	return changed
}
func (core *finiteCore) move(delta int) bool {
	if core.count == 0 {
		return false
	}
	delta = min(max(delta, -core.selected), core.count-1-core.selected)
	return core.selectIndex(core.selected + delta)
}
func (core *finiteCore) scroll(deltaItems float64) bool {
	if core.count == 0 || math.IsNaN(deltaItems) || math.IsInf(deltaItems, 0) {
		return false
	}
	core.position = min(max(core.position+deltaItems, 0), float64(core.count-1))
	selected := int(math.Ceil(core.position - .5))
	changed := selected != core.selected
	core.selected = selected
	return changed
}
func (core *finiteCore) settle() { core.selectIndex(core.selected) }
func (core finiteCore) window(size int) (int, int) {
	if core.count == 0 || size <= 0 {
		return 0, 0
	}
	size = min(size, core.count)
	first := min(max(core.selected-size/2, 0), core.count-size)
	return first, first + size
}
