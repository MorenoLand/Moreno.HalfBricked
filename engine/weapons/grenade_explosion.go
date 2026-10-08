package weapons

import "github.com/hajimehoshi/ebiten/v2"

const NativeGrenadeExplosionDuration = .75
const NativeGrenadeExplosionTexture = "Common0/Textures/explosion2_SD"

func GrenadeExplosionFrame(age float64) (int, bool) {
	if !(age >= 0 && age <= NativeGrenadeExplosionDuration) {
		return 0, false
	}
	return int(float32(age)/float32(NativeGrenadeExplosionDuration)*float32(9)) % 9, true
}
func GrenadeExplosionVertices(age, x, y, zoom, frontendX, frontendY float64, textureWidth, textureHeight int) ([4]ebiten.Vertex, bool) {
	frame, visible := GrenadeExplosionFrame(age)
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
