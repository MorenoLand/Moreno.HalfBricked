package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// Native HUD health hearts (1.2.5 FUN_00133884): three hearts stacked upward from
// (20, 85+96), 32 logical pixels apart, each 24x24. Heart i is full below
// floor(health*3), partly filled at that index (cropped from the bottom) and
// empty above it. The fill texture is drawn first, then the outline.
const (
	healthHeartX      = 20.0
	healthHeartBaseY  = 85.0 + 96.0 - 20 // raised so the weapon button above the left stick stays clear
	healthHeartStep   = 32.0
	healthHeartSize   = 24.0
	healthHeartSource = 128.0
)

func healthHeartFills(health float64) [3]float64 {
	var fills [3]float64
	health = math.Max(0, math.Min(1, health))
	full := int(math.Floor(health * 3))
	for index := range fills {
		switch {
		case index < full:
			fills[index] = 1
		case index == full:
			fills[index] = (health - float64(full)/3) * 3
		}
	}
	return fills
}

func (a *app) drawHealthHearts(screen *ebiten.Image, health float64) {
	a.drawHealthHeartsAt(screen, health, healthHeartX, healthHeartBaseY, 1)
}

// drawHealthHeartsAt draws the three hearts with the lowest heart centred at
// (x, baseY); scale shrinks them for the co-op layout.
func (a *app) drawHealthHeartsAt(screen *ebiten.Image, health, x, baseY, scale float64) {
	outline, outlineErr := a.Texture("Common0/Textures/HealthHeart")
	fill, fillErr := a.Texture("Common0/Textures/HealthHeart_Fill")
	if outlineErr != nil || fillErr != nil {
		return
	}
	for index, level := range healthHeartFills(health) {
		y := baseY - float64(index)*healthHeartStep*scale
		size := healthHeartSize * scale
		if level > 0 {
			top := int(math.Round((1 - level) * healthHeartSource))
			source := fill.SubImage(image.Rect(0, top, int(healthHeartSource), int(healthHeartSource))).(*ebiten.Image)
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(size/healthHeartSource, size/healthHeartSource)
			options.GeoM.Translate(x-size/2, y-size/2+(1-level)*size)
			a.drawImage(screen, source, options)
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(size/healthHeartSource, size/healthHeartSource)
		options.GeoM.Translate(x-size/2, y-size/2)
		a.drawImage(screen, outline, options)
	}
}
