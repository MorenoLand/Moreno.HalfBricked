package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

func (a *app) drawNativeGrenadeExplosion(screen *ebiten.Image, explosion explosionState) {
	if screen == nil || a.play == nil || a.play.world == nil {
		return
	}
	if _, visible := weapons.GrenadeExplosionFrame(explosion.age); !visible {
		return
	}
	texture, err := a.Texture(weapons.NativeGrenadeExplosionTexture)
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
	vertices, visible := weapons.GrenadeExplosionVertices(explosion.age, x, y, zoom, frontendX, frontendY, texture.Bounds().Dx(), texture.Bounds().Dy())
	if !visible {
		return
	}
	screen.DrawTriangles(vertices[:], []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true})
}
