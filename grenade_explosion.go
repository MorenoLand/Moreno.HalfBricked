package main

import "github.com/hajimehoshi/ebiten/v2"

const nativeGrenadeExplosionDuration = .75
const nativeGrenadeExplosionTexture = "Common0/Textures/explosion2_SD"

func grenadeExplosionFrame(age float64) (int, bool) {
	if !(age >= 0 && age <= nativeGrenadeExplosionDuration) {
		return 0, false
	}
	return int(float32(age)/float32(nativeGrenadeExplosionDuration)*float32(9)) % 9, true
}
func grenadeExplosionVertices(age, x, y, zoom, frontendX, frontendY float64, textureWidth, textureHeight int) ([4]ebiten.Vertex, bool) {
	frame, visible := grenadeExplosionFrame(age)
	if !visible || textureWidth <= 0 || textureHeight <= 0 || zoom <= 0 || frontendX <= 0 || frontendY <= 0 {
		return [4]ebiten.Vertex{}, false
	}
	u0 := float32(frame) * float32(.11)
	u1 := u0 + float32(.11)
	sourceX0, sourceX1, sourceY1 := u0*float32(textureWidth), u1*float32(textureWidth), float32(textureHeight)
	centerX, centerY := x*frontendX, y*frontendY-70*zoom*frontendX
	halfWidth, halfHeight := 60*zoom*frontendX, 120*zoom*frontendX
	x0, x1, y0, y1 := float32(centerX-halfWidth), float32(centerX+halfWidth), float32(centerY-halfHeight), float32(centerY+halfHeight)
	return [4]ebiten.Vertex{
		{DstX: x0, DstY: y0, SrcX: sourceX0, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: x1, DstY: y0, SrcX: sourceX1, SrcY: 0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: x0, DstY: y1, SrcX: sourceX0, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
		{DstX: x1, DstY: y1, SrcX: sourceX1, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1},
	}, true
}
func (a *app) drawNativeGrenadeExplosion(screen *ebiten.Image, explosion explosionState) {
	if screen == nil || a.play == nil || a.play.world == nil {
		return
	}
	if _, visible := grenadeExplosionFrame(explosion.age); !visible {
		return
	}
	texture, err := a.Texture(nativeGrenadeExplosionTexture)
	if err != nil {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	x, y := (explosion.x-world.CameraX)*zoom+world.ViewportX, (explosion.y-world.CameraY)*zoom+world.ViewportY
	frontendX, frontendY := a.renderScale()
	vertices, visible := grenadeExplosionVertices(explosion.age, x, y, zoom, frontendX, frontendY, texture.Bounds().Dx(), texture.Bounds().Dy())
	if !visible {
		return
	}
	screen.DrawTriangles(vertices[:], []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true})
}
