package game

import (
	"fmt"
	"runtime"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// perfOverlay is the frame-time readout of the browser build: open the page with ?perf (or set it from the
// console) to show the update and draw CPU times and the frame rates in the top-left corner. It is for finding
// where a slow machine loses its time; it is off otherwise and costs nothing.
var perfOverlay bool

// audioDisabled (?noaudio in the browser) starts the game with no sound system at all, to tell whether audio
// decoding is what holds the frame rate down.
var audioDisabled bool

type perfCounters struct {
	update, draw float64 // smoothed milliseconds
	// Frame spikes: the longest gap between two draws in the last second, and the garbage collections in it.
	lastDraw     time.Time
	windowStart  time.Time
	windowWorst  float64
	shownWorst   float64
	windowGCBase uint32
	shownGC      uint32
}

func (a *app) perfTrackUpdate(start time.Time) {
	if !perfOverlay {
		return
	}
	a.perf.update += (float64(time.Since(start).Microseconds())/1000 - a.perf.update) * .1
}

func (a *app) perfTrackDraw(start time.Time) {
	if !perfOverlay {
		return
	}
	a.perf.draw += (float64(time.Since(start).Microseconds())/1000 - a.perf.draw) * .1
}

func (a *app) drawPerfOverlay(screen *ebiten.Image) {
	if !perfOverlay {
		return
	}
	now := time.Now()
	p := &a.perf
	if !p.lastDraw.IsZero() {
		if gap := float64(now.Sub(p.lastDraw).Microseconds()) / 1000; gap > p.windowWorst {
			p.windowWorst = gap
		}
	}
	p.lastDraw = now
	if now.Sub(p.windowStart) >= time.Second {
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		p.shownWorst, p.shownGC = p.windowWorst, ms.NumGC-p.windowGCBase
		p.windowWorst, p.windowStart, p.windowGCBase = 0, now, ms.NumGC
	}
	line := fmt.Sprintf("fps %.0f tps %.0f  update %.2f  draw %.2f  worst gap %.0f ms  gc/s %d  %dx%d", ebiten.ActualFPS(), ebiten.ActualTPS(), a.perf.update, a.perf.draw, p.shownWorst, p.shownGC, screen.Bounds().Dx(), screen.Bounds().Dy())
	a.drawRect(screen, 0, 0, 480, 14, colorDark)
	a.text(screen, line, 3, 0, .32)
}
