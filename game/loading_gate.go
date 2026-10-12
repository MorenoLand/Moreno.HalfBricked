package game

import (
	"fmt"
	"image/color"
	"math"
	"runtime"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"

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

// loadingTips cycle under the bar (a new one every few seconds).
var loadingTips = []string{
	"HEADSHOTS DON'T COUNT MORE. KEEP SHOOTING.",
	"MINES GO OFF ON A TIMER. PLANT THEM BEHIND YOU.",
	"A SHIELD STOPS EVERYTHING, BOSSES INCLUDED.",
	"GRAB THE HEARTS. THEY ARE YOUR EXTRA LIVES.",
	"KEEP MOVING. YOUR HEALTH GROWS BACK IF YOU DO.",
	"BIGGER COMBOS, BIGGER SCORE. DON'T STOP KILLING.",
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// drawLoading is the loading screen: the menu's turning portal, the logo, a panel with what is loading, a bar with
// a moving shine, the file and megabyte counts, and a tip.
func (a *app) drawLoading(screen *ebiten.Image) {
	gate := a.loading
	a.menuTime += 1.0 / 60.0 // Update does not run while the gate is up; the portal and the shine keep moving
	screen.Fill(color.RGBA{10, 8, 14, 255})
	a.drawBackdrop(screen)
	a.drawMainMenuBanner(screen)

	list := gate.paths()
	missing := a.pack.PrefetchPending(list)
	total := max(gate.total, len(list), 1)
	var bytesTotal, bytesLeft int64
	var sized bool
	pending := map[string]bool{}
	if missing > 0 {
		for _, path := range list {
			pending[path] = true
		}
	}
	for _, path := range list {
		if size, ok := a.pack.FileSize(path); ok {
			sized = true
			bytesTotal += size
			if pending[path] && a.pack.PrefetchPending([]string{path}) > 0 {
				bytesLeft += size
			}
		}
	}
	done := float64(total-missing) / float64(total)
	if sized && bytesTotal > 0 {
		done = float64(bytesTotal-bytesLeft) / float64(bytesTotal)
	}
	done = min(1, max(0, done))

	subtitle, tip := "", loadingTips[int(a.menuTime/4)%len(loadingTips)]
	if levels := a.filteredLevels(); gate.label == "LOADING LEVEL" && a.level >= 0 && a.level < len(levels) {
		subtitle = strings.ToUpper(levels[a.level].DisplayName)
		if d := strings.ToUpper(strings.NewReplacer("\\n", " ", "\n", " ").Replace(levels[a.level].Description)); d != "" {
			tip = d
		}
	}
	if boxes, err := a.Texture("Common0/Textures/TutorialBoxes"); err == nil {
		a.drawOptionsBoxStyle(screen, boxes, formats.Vec2{X: 240, Y: 214}, formats.Vec2{X: 380, Y: 130}, 0)
	} else {
		a.drawRect(screen, 50, 150, 380, 130, color.RGBA{20, 22, 32, 235})
	}
	a.textCentered(screen, gate.label, 156, .5)
	if subtitle != "" {
		a.textCentered(screen, subtitle, 180, .36)
	}

	const barX, barY, barW, barH = 80.0, 204.0, 320.0, 14.0
	a.drawRect(screen, barX-2, barY-2, barW+4, barH+4, color.RGBA{0, 0, 0, 255})
	a.drawRect(screen, barX, barY, barW, barH, color.RGBA{38, 42, 58, 255})
	fill := barW * done
	a.drawRect(screen, barX, barY, fill, barH, color.RGBA{214, 64, 52, 255})
	a.drawRect(screen, barX, barY, fill, barH/3, color.RGBA{244, 120, 98, 255}) // top highlight
	if fill > 4 {
		// A soft band sweeps along the filled part.
		bandW := 36.0
		x := math.Mod(a.menuTime*90, fill+bandW) - bandW
		x0, x1 := max(x, 0), min(x+bandW, fill)
		if x1 > x0 {
			a.drawRect(screen, barX+x0, barY, x1-x0, barH, color.RGBA{255, 255, 255, 46})
		}
	}
	percent := fmt.Sprintf("%d%%", int(done*100))
	a.text(screen, percent, barX+barW+8, barY-1, .36)
	counts := fmt.Sprintf("%d of %d files", total-missing, total)
	if sized {
		counts += fmt.Sprintf("   %s of %s", formatBytes(bytesTotal-bytesLeft), formatBytes(bytesTotal))
	}
	a.textCentered(screen, counts, 230, .32)
	a.textCentered(screen, tip, 254, .3)
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
