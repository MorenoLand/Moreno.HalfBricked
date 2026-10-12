package game

import (
	"fmt"
	"image/color"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// Browser build: a loading screen while the files a screen or level needs are still downloading. Without it a
// click made the game fetch them on the spot, one blocking request at a time, and the frame rate fell to a few
// frames per second until they were in. The gate holds the click (or the first frame) until every file of the set
// has arrived, showing a progress bar, then continues with the held action.
type loadingGate struct {
	label string
	paths func() []string // re-evaluated every frame (a level's file list is only known once its JSON is in)
	total int
	next  func() error
	began time.Time
}

// loadingGiveUp is how long the gate waits before it goes on with whatever has not arrived (those files are then
// fetched on demand, as before the gate existed): a stuck download must never trap the player on this screen.
const loadingGiveUp = 45 * time.Second

// beginLoading starts the gate when files of the set are still missing and reports whether it did; the caller then
// returns without doing its action, which next performs later. Outside the browser it never gates.
func (a *app) beginLoading(label string, paths func() []string, next func() error) bool {
	if runtime.GOOS != "js" || a.silent || a.pack == nil || a.loading != nil {
		return false
	}
	list := paths()
	a.pack.Prefetch(list)
	missing := a.pack.PrefetchPending(list)
	if missing == 0 {
		return false
	}
	a.loading = &loadingGate{label: label, paths: paths, total: max(1, len(list)), next: next, began: time.Now()}
	return true
}

// updateLoading runs instead of the normal update while the gate is up.
func (a *app) updateLoading() error {
	gate := a.loading
	list := gate.paths()
	a.pack.Prefetch(list) // names that appeared since the last frame (the level's textures after its JSON)
	gate.total = max(gate.total, len(list))
	if a.pack.PrefetchPending(list) > 0 && time.Since(gate.began) < loadingGiveUp {
		return nil
	}
	a.loading = nil
	if gate.next != nil {
		return gate.next()
	}
	return nil
}

func (a *app) drawLoading(screen *ebiten.Image) {
	gate := a.loading
	screen.Fill(color.RGBA{8, 10, 16, 255})
	list := gate.paths()
	missing := a.pack.PrefetchPending(list)
	total := max(gate.total, len(list), 1)
	done := float64(total-missing) / float64(total)
	if done < 0 {
		done = 0
	}
	title := gate.label
	a.text(screen, title, 240-a.fontTextWidth(title, .5)/2, 120, .5)
	a.drawRect(screen, 90, 160, 300, 10, color.RGBA{40, 46, 62, 255})
	a.drawRect(screen, 90, 160, 300*done, 10, color.RGBA{214, 64, 52, 255})
	detail := fmt.Sprintf("%d of %d files", total-missing, total)
	a.text(screen, detail, 240-a.fontTextWidth(detail, .32)/2, 178, .32)
}

// levelLoadPaths is the file set of the level selected in the level select: its JSON, entry script and music, and
// once the JSON is in, the tileset atlas and the common and world textures.
func (a *app) levelLoadPaths() func() []string {
	levels := a.filteredLevels()
	if a.level < 0 || a.level >= len(levels) {
		return func() []string { return nil }
	}
	info := levels[a.level]
	early := a.levelEarlyPaths(info)
	var full []string
	return func() []string {
		if full != nil {
			return full
		}
		if a.pack.LevelLoaded(info.ID) || a.pack.PrefetchReady(early[0]) {
			if level, err := a.pack.Load(info.ID); err == nil {
				full = append(append([]string(nil), early...), a.levelAssetPaths(level, info)...)
				return full
			}
		}
		return early
	}
}
