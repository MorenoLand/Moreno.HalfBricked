package main

import (
	"encoding/json"
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"os"
	"testing"
)

func selectorTestApp(t *testing.T) *app {
	t.Helper()
	data, err := os.ReadFile("bin/data-cache/pack.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Levels []formats.LevelInfo `json:"levels"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}
	return &app{levels: catalog.Levels, page: 2, unlocked: map[string]bool{}, silent: true}
}
func TestLevelSelectorAllLaunchMappings(t *testing.T) {
	a := selectorTestApp(t)
	for mode, count := range []int{21, 12} {
		a.mode, a.world, a.level = mode, 0, 0
		levels := a.syncLevelCarousel()
		for index := 0; index < count; index++ {
			a.setCursor(index)
			selected := a.filteredLevels()[a.level]
			if selected.ID != levels[index].Info.ID || a.carouselCore.selected != index {
				t.Fatalf("mode %d index %d launches %s", mode, index, selected.ID)
			}
			x := int(a.levelCardX(index))
			if a.levelHit(x, 100) != index {
				t.Fatalf("mode %d selected card %d inaccessible at %d", mode, index, x)
			}
		}
		a.move(1)
		if a.carouselCore.selected != count-1 {
			t.Fatal("last endpoint wrapped")
		}
		for index := count - 2; index >= 0; index-- {
			a.move(-1)
			if a.carouselCore.selected != index {
				t.Fatalf("backward navigation %d", index)
			}
		}
		a.move(-1)
		if a.carouselCore.selected != 0 {
			t.Fatal("first endpoint wrapped")
		}
		for index := 1; index < count; index++ {
			a.move(1)
			if a.carouselCore.selected != index || a.filteredLevels()[a.level].ID != levels[index].Info.ID || a.world != levels[index].World || a.level != levels[index].Level {
				t.Fatalf("forward keyboard mode %d index %d maps to world %d level %d", mode, index, a.world, a.level)
			}
		}
	}
}
func TestLevelSelectorSwipeDoesNotActivate(t *testing.T) {
	a := selectorTestApp(t)
	for _, info := range a.levels {
		a.unlocked[info.ID] = true
	}
	for mode := 0; mode < 2; mode++ {
		a.mode, a.world, a.level = mode, 0, 0
		levels := a.syncLevelCarousel()
		for swipe := 0; swipe < 12; swipe++ {
			for _, event := range [][4]int{{390, 100, 1, 1}, {90, 100, 0, 1}, {90, 100, 0, 0}} {
				if handled, err := a.updateLevelSelectPointer(event[0], event[1], event[2] != 0, event[3] != 0); !handled || err != nil {
					t.Fatalf("swipe: handled=%v err=%v", handled, err)
				}
			}
			if a.play != nil || a.page != 2 || a.selectorPointerDown || a.selectorDragging {
				t.Fatal("swipe activated or retained capture")
			}
			expected := min((swipe+1)*2, len(levels)-1)
			if a.carouselCore.selected != expected || a.filteredLevels()[a.level].ID != levels[expected].Info.ID || a.world != levels[expected].World || a.level != levels[expected].Level {
				t.Fatalf("drag mode %d swipe %d mapping world %d level %d", mode, swipe, a.world, a.level)
			}
		}
		if a.carouselCore.selected != a.carouselCore.count-1 {
			t.Fatal("swipe failed to reach last catalog card")
		}
		a.updateLevelSelectPointer(90, 100, true, true)
		a.updateLevelSelectPointer(390, 200, false, true)
		a.updateLevelSelectPointer(390, 200, false, false)
		if a.carouselCore.selected != a.carouselCore.count-3 {
			t.Fatal("captured swipe outside card row lost")
		}
		a.setCursor(0)
		for _, event := range [][4]int{{390, 100, 1, 1}, {90, 100, 0, 1}, {390, 100, 0, 1}, {390, 100, 0, 0}} {
			if handled, err := a.updateLevelSelectPointer(event[0], event[1], event[2] != 0, event[3] != 0); !handled || err != nil {
				t.Fatalf("returning drag: handled=%v err=%v", handled, err)
			}
		}
		if a.play != nil || a.page != 2 || a.carouselCore.selected != 0 {
			t.Fatal("drag returning to pressed unlocked card activated on release")
		}
	}
}
func TestLevelSelectorClickAndExternalSelection(t *testing.T) {
	a := selectorTestApp(t)
	a.syncLevelCarousel()
	a.updateLevelSelectPointer(240, 100, true, true)
	if a.level != 0 {
		t.Fatal("press selected before release")
	}
	a.updateLevelSelectPointer(240, 100, false, false)
	if a.carouselCore.selected != 1 || a.level != 1 || a.play != nil {
		t.Fatal("locked card click mapping")
	}
	a.world, a.level = 4, 2
	levels := a.syncLevelCarousel()
	if levels[a.carouselCore.selected].Info.ID != a.filteredLevels()[2].ID {
		t.Fatal("external continuation selection lost")
	}
	a.leaveLevelSelect()
	if a.carouselReady || a.selectorPointerDown {
		t.Fatal("selector state leaked on exit")
	}
}

type selectorCaptureGame struct {
	app      *app
	err      error
	capture  *engine.Capture
	stage    int
	label    string
	prepared bool
}

func (game *selectorCaptureGame) Update() error {
	if game.err != nil {
		return game.err
	}
	if game.prepared {
		return nil
	}
	a := game.app
	switch game.stage {
	case 0:
		game.label = "story-first"
	case 1:
		for index := 0; index < 3; index++ {
			a.move(1)
		}
		game.label = "story-world1"
	case 2:
		a.move(100)
		game.label = "story-last"
	case 3:
		a.setCursor(0)
		a.updateLevelSelectPointer(390, 100, true, true)
		a.updateLevelSelectPointer(-60, 100, false, true)
		a.updateLevelSelectPointer(-60, 100, false, false)
		if a.filteredLevels()[a.level].ID != "World1Level0" || a.play != nil {
			return fmt.Errorf("runtime cross-world drag mapping or accidental launch")
		}
		game.label = "story-swipe"
	case 4:
		a.mode, a.world, a.level = 1, 0, 0
		a.syncLevelCarousel()
		game.label = "survival-first"
	case 5:
		a.move(100)
		game.label = "survival-last"
	case 6:
		a.updateLevelSelectPointer(90, 100, true, true)
		a.updateLevelSelectPointer(390, 100, false, true)
		a.updateLevelSelectPointer(390, 100, false, false)
		game.label = "survival-swipe"
	case 7:
		a.mode, a.world, a.level = 0, 0, 0
		a.syncLevelCarousel()
		if _, err := a.updateLevelSelectPointer(90, 100, true, true); err != nil {
			return err
		}
		if _, err := a.updateLevelSelectPointer(90, 100, false, false); err != nil {
			return err
		}
		if a.play == nil || a.play.levelInfo.ID != a.filteredLevels()[a.level].ID {
			return fmt.Errorf("runtime click did not launch selected level")
		}
		game.label = "story-click-launch"
	default:
		return ebiten.Termination
	}
	levels := a.syncLevelCarousel()
	if levels[a.carouselCore.selected].Info.ID != a.filteredLevels()[a.level].ID {
		return fmt.Errorf("runtime launch mapping")
	}
	game.prepared = true
	return nil
}
func (game *selectorCaptureGame) Draw(screen *ebiten.Image) {
	game.app.Draw(screen)
	if !game.prepared {
		return
	}
	if err := game.capture.Save(screen, game.label); err != nil {
		game.err = err
	}
	game.stage++
	game.prepared = false
}
func (game *selectorCaptureGame) Layout(width, height int) (int, int) {
	return game.app.Layout(width, height)
}
func runSelectorRuntimeCapture(directory string) error {
	a, err := newApp("bin/data-cache", false, false, true)
	if err != nil {
		return err
	}
	defer a.sound.Close()
	a.profileWritable, a.titleScreen, a.startupFrames, a.page = false, false, 0, 2
	capture, err := engine.NewCapture(directory, 0)
	if err != nil {
		return err
	}
	ebiten.SetWindowSize(960, 540)
	if err := ebiten.RunGame(&selectorCaptureGame{app: a, capture: capture}); err != nil && err != ebiten.Termination {
		return err
	}
	return nil
}
func TestMain(m *testing.M) {
	if directory := os.Getenv("AOZ_SELECTOR_CAPTURE"); directory != "" {
		if err := runSelectorRuntimeCapture(directory); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}
