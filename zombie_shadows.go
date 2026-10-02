package main

import (
	"image"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
)

type zombieShadow struct {
	x, y, width, height float64
	alpha               uint8
}

func zombieShadowGeometry(zombie zombieState, world *viewer.Viewer, inner image.Rectangle) (zombieShadow, bool) {
	if world == nil || !(zombie.alpha > 0) {
		return zombieShadow{}, false
	}
	alpha := uint8(math.Min(1, zombie.alpha)*255) >> 1
	if alpha == 0 {
		return zombieShadow{}, false
	}
	width, height := float32(zombie.size.X), float32(zombie.size.Y)
	if width <= 0 {
		width = nativeZombieDefaultRenderSize
	}
	if height <= 0 {
		height = nativeZombieDefaultRenderSize
	}
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	x, y := (zombie.x-world.CameraX)*zoom+world.ViewportX, (zombie.y-world.CameraY)*zoom+world.ViewportY
	bodyY := y - float64(float32(height*float32(zombieRenderAnchor)))*zoom
	halfWidth, halfHeight := float64(width)*zoom/2, float64(height)*zoom/2
	left, top, right, bottom := x-halfWidth, bodyY-halfHeight, x+halfWidth, bodyY+halfHeight
	if !(right > 0 && left < logicalWidth && bottom > 0 && top < logicalHeight && right > float64(inner.Min.X) && left < float64(inner.Max.X) && bottom > float64(inner.Min.Y) && top < float64(inner.Max.Y)) {
		return zombieShadow{}, false
	}
	return zombieShadow{x: x, y: y, width: float64(float32(width*float32(.8))) * zoom, height: float64(float32(float32(width*float32(.666))*float32(.8))) * zoom, alpha: alpha}, true
}
func (a *app) zombieShadowTexture(name string) *ebiten.Image {
	if texture := a.images[strings.ToLower(strings.TrimSuffix(name, ".tex"))]; texture != nil {
		return texture
	}
	if a.pack == nil {
		return nil
	}
	texture, err := a.Texture(name)
	if err != nil {
		return nil
	}
	return texture
}
func (a *app) zombieShadows() []zombieShadow {
	if a.play == nil || a.play.world == nil || a.pack == nil && a.images == nil {
		return nil
	}
	limit := 511
	playerTexture := "Common0/Textures/Characters/barryidle_SD"
	if a.play.moving {
		playerTexture = "Common0/Textures/Characters/barryrun_SD"
	}
	if a.zombieShadowTexture(playerTexture) != nil {
		limit--
	}
	shadows := make([]zombieShadow, 0, min(len(a.play.zombies), limit))
	for _, zombie := range a.play.zombies {
		shadow, visible := zombieShadowGeometry(zombie, a.play.world, image.Rect(0, 0, logicalWidth, logicalHeight))
		if !visible {
			continue
		}
		name := zombie.texture
		if name == "" {
			name = "cavezombie"
		}
		animationName := zombie.animation
		if zombie.bossRage || zombie.rexRageTimer > 0 {
			animationName = "Rage"
		}
		path := commonSDTexture(name)
		if animation, ok := a.spriteAnimation(name, animationName); ok {
			path = animation.Texture
		}
		if a.zombieShadowTexture(path) == nil {
			continue
		}
		shadows = append(shadows, shadow)
		if len(shadows) == limit {
			break
		}
	}
	return shadows
}
func (a *app) drawZombieShadows(screen *ebiten.Image) {
	if a.play == nil || a.play.world == nil || screen == nil {
		return
	}
	shadows := a.zombieShadows()
	if len(shadows) == 0 {
		return
	}
	texture := a.zombieShadowTexture("Common0/Textures/shadow_SD")
	if texture == nil {
		return
	}
	w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	if w <= 0 || h <= 0 {
		return
	}
	for _, shadow := range shadows {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-w/2, -h/2)
		options.GeoM.Scale(shadow.width/w, shadow.height/h)
		options.GeoM.Translate(shadow.x, shadow.y)
		options.ColorScale.ScaleAlpha(float32(shadow.alpha) / 255)
		a.drawImage(screen, texture, options)
	}
}
