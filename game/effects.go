package game

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func (a *app) drawBloodPopSprite(screen *ebiten.Image, x, y, scale float64, variant int, age float64) {
	if variant < 0 || variant > 2 {
		return
	}
	animation, ok := zombieDeathAnimation(a.sprites, variant)
	if !ok || animation.Frames <= 0 || animation.FPS <= 0 {
		return
	}
	texture, err := a.Texture(animation.Texture)
	if err != nil {
		return
	}
	frames := animation.Frames
	cellWidth := texture.Bounds().Dx() / frames
	cellHeight := texture.Bounds().Dy()
	frame := int(math.Floor(age * animation.FPS))
	if cellWidth <= 0 || cellHeight <= 0 || frame < 0 || frame >= frames {
		return
	}
	source := texture.SubImage(image.Rect(frame*cellWidth, 0, (frame+1)*cellWidth, cellHeight)).(*ebiten.Image)
	if scale <= 0 {
		scale = 1
	}
	scale /= a.pack.TextureSourceScale(animation.Texture)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(cellWidth)/2, -float64(cellHeight)/2)
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, source, options)
}
