package game

import (
	"fmt"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// perfOverlay is the frame-time readout of the browser build: open the page with ?perf (or set it from the
// console) to show the update and draw CPU times and the frame rates in the top-left corner. It is for finding
// where a slow machine loses its time; it is off otherwise and costs nothing.
var perfOverlay bool

type perfCounters struct {
	update, draw float64 // smoothed milliseconds
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
	line := fmt.Sprintf("fps %.0f tps %.0f  update %.2f ms  draw %.2f ms  %dx%d", ebiten.ActualFPS(), ebiten.ActualTPS(), a.perf.update, a.perf.draw, screen.Bounds().Dx(), screen.Bounds().Dy())
	a.drawRect(screen, 0, 0, 300, 10, colorDark)
	a.text(screen, line, 2, 0, .22)
}
