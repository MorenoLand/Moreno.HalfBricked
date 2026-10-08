package game

import "image"

func zombieSpriteScale(width, height float64, source image.Rectangle, zoom, frontendX, frontendY float64) (float64, float64) {
	x, y := .5*zoom, .5*zoom
	if width > 0 {
		x = width / float64(source.Dx()) * zoom
	}
	if height > 0 {
		y = height / float64(source.Dy()) * zoom
	}
	return x, y * frontendX / frontendY
}
