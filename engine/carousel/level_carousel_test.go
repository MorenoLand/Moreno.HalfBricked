package carousel

import (
	"encoding/json"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"os"
	"testing"
)

func TestLevelCarouselCatalogAndSelection(t *testing.T) {
	levels := []formats.LevelInfo{{ID: "survival", WorldIndex: 7, Flags: []string{"SURVIVAL"}}, {ID: "a", WorldIndex: 2, Flags: []string{"STORY"}}, {ID: "other", WorldIndex: 9}, {ID: "b", WorldIndex: 7, Flags: []string{"STORY"}}, {ID: "c", WorldIndex: 2, Flags: []string{"STORY"}}, {ID: "d", WorldIndex: 9, Flags: []string{"STORY"}}}
	story := CatalogLevelCarousel(levels, 0)
	if len(story) != 4 {
		t.Fatalf("story count = %d", len(story))
	}
	for index, expected := range [][3]int{{1, 0, 1}, {0, 0, 3}, {1, 1, 4}, {2, 1, 5}} {
		world, level, ok := story.Selection(index)
		if !ok || world != expected[0] || level != expected[1] || story[index].CatalogIndex != expected[2] || story.Index(world, level) != index {
			t.Fatalf("entry %d = %+v", index, story[index])
		}
	}
	if len(CatalogLevelCarousel(levels, 1)) != 1 || CatalogLevelCarousel(levels, 2) != nil || story.Index(5, 0) != -1 {
		t.Fatal("invalid mode/world or survival filtering")
	}
	for _, index := range []int{-1, len(story)} {
		if _, _, ok := story.Selection(index); ok {
			t.Fatalf("accepted index %d", index)
		}
	}
}
func TestLevelCarouselMixedFlagsCannotLaunchThroughStorySelection(t *testing.T) {
	levels := CatalogLevelCarousel([]formats.LevelInfo{{ID: "mixed", Flags: []string{"STORY", "SURVIVAL"}}}, 0)
	if len(levels) != 1 {
		t.Fatal("native mode membership lost")
	}
	if _, _, ok := levels.Selection(0); ok || levels.Index(0, -1) != -1 {
		t.Fatal("incompatible parent selection accepted")
	}
}
func TestLevelCarouselExistingCatalog(t *testing.T) {
	data, err := os.ReadFile("../../bin/data-cache/pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Levels []formats.LevelInfo `json:"levels"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	for mode, count := range []int{21, 12} {
		levels := CatalogLevelCarousel(catalog.Levels, mode)
		if len(levels) != count {
			t.Fatalf("mode %d count = %d, want %d", mode, len(levels), count)
		}
		core := NewFiniteCore(len(levels), 0)
		for index, entry := range levels {
			if levels.Index(entry.World, entry.Level) != index {
				t.Fatalf("mode %d selection %d", mode, index)
			}
			core.SelectIndex(index)
			first, end := core.window(3)
			if index < first || index >= end {
				t.Fatalf("mode %d index %d outside [%d,%d)", mode, index, first, end)
			}
		}
	}
}
func TestFiniteCoreScrollAndBounds(t *testing.T) {
	core := NewFiniteCore(21, 0)
	if core.Scroll(.5) || core.Selected != 0 {
		t.Fatal("midpoint must retain first nearest item")
	}
	if !core.Scroll(.01) || core.Selected != 1 {
		t.Fatal("drag failed to cross nearest item")
	}
	core.Scroll(19)
	if core.Selected != 20 {
		t.Fatal("later worlds unreachable")
	}
	if core.Scroll(100) || core.Position != 20 {
		t.Fatal("upper endpoint wrapped")
	}
	core.Scroll(-100)
	if core.Selected != 0 || core.Position != 0 {
		t.Fatal("lower endpoint wrapped")
	}
	for _, delta := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if core.Scroll(delta) || core.Position != 0 {
			t.Fatal("accepted nonfinite input")
		}
	}
	core.Move(int(^uint(0) >> 1))
	if core.Selected != 20 {
		t.Fatal("large forward delta")
	}
	core.Move(-int(^uint(0)>>1) - 1)
	if core.Selected != 0 {
		t.Fatal("large backward delta")
	}
	core.Scroll(3.25)
	core.Settle()
	if core.Position != 3 || core.Selected != 3 {
		t.Fatal("settle failed")
	}
}
func TestFiniteCoreEmptyAndWindow(t *testing.T) {
	for _, count := range []int{-1, 0} {
		core := NewFiniteCore(count, 4)
		if core.Selected != -1 || core.Move(1) || core.Scroll(1) {
			t.Fatal("empty carousel selected an item")
		}
		if first, end := core.window(3); first != 0 || end != 0 {
			t.Fatal("empty window")
		}
	}
	core := NewFiniteCore(4, 100)
	if first, end := core.window(3); first != 1 || end != 4 {
		t.Fatal("last card window")
	}
	if first, end := core.window(100); first != 0 || end != 4 {
		t.Fatal("oversized window")
	}
	if first, end := core.window(0); first != 0 || end != 0 {
		t.Fatal("invalid window")
	}
}
func TestLevelCarouselOriginalUnlockMetadata(t *testing.T) {
	file, err := os.Open("../../bin/web/data/data-cache/source/World0/Xml/World0_Levels.xml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	levels, err := formats.ParseLevelCatalogXML(file, "World0/Xml/World0_Levels.xml")
	if err != nil {
		t.Fatal(err)
	}
	story := CatalogLevelCarousel(levels, 0)
	if len(story) != 3 || story[0].Info.NextLevel != story[1].Info.ID || story[1].Info.NextLevel != story[2].Info.ID {
		t.Fatal("original successor chain")
	}
	for index, entry := range story {
		initial := false
		for _, flag := range entry.Info.Flags {
			if flag == "STARTUNLOCKED" {
				initial = true
			}
		}
		if initial != (index == 0) {
			t.Fatalf("original initial unlock %s = %v", entry.Info.ID, initial)
		}
	}
}
