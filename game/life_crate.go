package game

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The extra-life crate (P_HEALTH, "Lives + 1" in the native pickup handler) used to be an empty box. It now shows
// the HUD's life icon (Barry's face, the one next to the "x3" counter) with a "+1" under it, so the crate reads as
// what it gives. Port addition (the crate art has no icon of its own); -mystery-mark=false turns it off with the
// other crate marks.
const lifeCrateIconSize = 24.0

func isLifeCrate(name string) bool { return strings.EqualFold(strings.TrimSpace(name), "p_health") }

// drawLifeCrateIcon draws the icon on the crate whose centre is (x, y - size * 0.375) and reports whether the pickup
// is the extra-life crate.
func (a *app) drawLifeCrateIcon(screen *ebiten.Image, entity *scriptEntity, x, y, zoom float64) bool {
	if !portMysteryMark || !isLifeCrate(entity.texture) {
		return false
	}
	centreY := y - 50*0.375*zoom
	if icon, err := a.Texture("Common0/Textures/LifeIcon"); err == nil {
		w, h := float64(icon.Bounds().Dx()), float64(icon.Bounds().Dy())
		if w > 0 && h > 0 {
			scale := lifeCrateIconSize * zoom / h
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Translate(-w/2, -h/2)
			options.GeoM.Scale(scale, scale)
			options.GeoM.Translate(x, centreY-4*zoom)
			a.drawImage(screen, icon, options)
		}
	}
	text := "+1"
	scale := .4 * zoom
	a.text(screen, text, x-a.fontTextWidth(text, scale)/2, centreY+6*zoom, scale)
	return true
}
