package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Menu text reacts to the pointer with a light lift and a warmer colour, easing
// in and out (a port addition).

// hoverAmount eases a 0..1 hover value for a menu row towards its target.
func (a *app) hoverAmount(key string, hovered bool) float64 {
	if a.hover == nil {
		a.hover = map[string]float64{}
	}
	target := 0.0
	if hovered {
		target = 1
	}
	value := a.hover[key]
	value += (target - value) * .28
	if math.Abs(value-target) < .01 {
		value = target
	}
	a.hover[key] = value
	return value
}

// drawHoverText draws text centred on centerX at y (the same units as textCentered),
// lifting a couple of pixels and warming to gold as amount goes from 0 to 1.
func (a *app) drawHoverText(screen *ebiten.Image, text string, centerX, y, scale, amount float64) {
	x := centerX - a.fontTextWidth(text, scale)/2
	if amount <= 0 {
		a.text(screen, text, x, y, scale) // centred on centerX, not on the screen
		return
	}
	lift := 3 * amount
	r, g, b := float32(1), float32(1-.12*amount), float32(1-.45*amount)
	a.drawComboGlyphs(screen, text, x, y-lift+1, scale, 0, 0, 0, float32(.45*amount))
	a.drawComboGlyphs(screen, text, x, y-lift, scale, r, g, b, 1)
}
