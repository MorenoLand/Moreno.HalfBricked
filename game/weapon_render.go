package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
	"math"
)

func (a *app) drawWeaponProjectile(screen *ebiten.Image, projectile weapons.NativeWeaponProjectile) {
	draw := projectile.DrawGeometry()
	texture, err := a.Texture(draw.Texture)
	if err != nil {
		return
	}
	world := a.play.world
	zoom := world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	sx, sy := a.renderScale()
	x := ((draw.X-world.CameraX)*zoom + world.ViewportX) * sx
	y := ((projectile.Y-world.CameraY)*zoom+world.ViewportY)*sy - projectile.Lift*zoom*sx
	w, h := draw.Width*zoom*sx, -draw.Height*zoom*sx
	cos, sin := math.Cos(draw.Rotation), math.Sin(draw.Rotation)
	var vertices [4]ebiten.Vertex
	for i, corner := range [4]struct{ x, y, u, v float64 }{{-w / 2, -h / 2, draw.U0, 0}, {w / 2, -h / 2, draw.U1, 0}, {-w / 2, h / 2, draw.U0, 1}, {w / 2, h / 2, draw.U1, 1}} {
		vertices[i] = ebiten.Vertex{DstX: float32(x + corner.x*cos - corner.y*sin), DstY: float32(y + corner.x*sin + corner.y*cos), SrcX: float32(corner.u * float64(texture.Bounds().Dx())), SrcY: float32(corner.v * float64(texture.Bounds().Dy())), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}
	}
	screen.DrawTriangles(vertices[:], []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true})
}
