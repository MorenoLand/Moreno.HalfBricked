package game

import "math"

// inDepthBand reports whether an entity at ground y belongs to the band being drawn (always true outside a banded draw).
func (a *app) inDepthBand(y float64) bool {
	if !a.depthBandActive {
		return true
	}
	return y >= a.depthBand.Lo && y < a.depthBand.Hi
}

// playerFeetOffset is how far Barry's drawn feet are below his position (his 64 px body is centred 25 px above it).
const playerFeetOffset = 7.0

// zombieFeetY is the bottom edge of a zombie's drawn sprite in world y: the sprite is centred zombieRenderAnchor of
// its height above the position, so the feet are a few pixels below it. Props are sorted by their bottom edge, so a
// zombie must be sorted by its feet too, or it pokes out under the prop it is drawn behind.
func zombieFeetY(z *zombieState) float64 {
	height := z.size.Y
	if height <= 0 {
		height = nativeZombieDefaultRenderSize
	}
	return z.y - zombieSpriteLift(*z, height) + height/2
}

// zombieSpriteLift is how far the sprite's centre is lifted above the zombie's position (native: +0x2d4 * +0x30).
// For the plain zombies +0x30 is the drawn height; the Don Zombie's car keeps its 2 * half size there while its
// drawn height is 0.6 of its width, so its sprite sits higher than a height-based lift would put it.
func zombieSpriteLift(z zombieState, renderHeight float64) float64 {
	if z.native.car.active {
		return carRenderAnchor * z.native.sizeZ
	}
	return renderHeight * zombieRenderAnchor
}

// scriptEntityFeetY is the bottom edge of a script-created sprite (CreateEntity / SpawnEntity). Those sprites are
// drawn centred on their position, so sorting them against props by the position alone puts a figure that stands in
// front of a prop (feet below its edge) behind it, with its head under the prop.
func (a *app) scriptEntityFeetY(e *scriptEntity) float64 {
	if e == nil || e.kind == "pickup" {
		if e == nil {
			return 0
		}
		return e.y
	}
	return e.y + a.scriptEntityDrawHeight(e)/2
}

// scriptEntityDrawHeight is the drawn height in world pixels, the same arithmetic as drawScriptEntity.
func (a *app) scriptEntityDrawHeight(e *scriptEntity) float64 {
	if e.sized {
		return math.Max(0, e.scaleY)
	}
	animation, has := a.spriteAnimationByIndex(e.texture, e.animation)
	path := commonSDTexture(e.texture)
	columns, rows := 5, 4
	if has {
		path = animation.Texture
		if animation.Angles > 0 {
			columns = animation.Angles
		}
		if animation.Frames > 0 {
			rows = animation.Frames
		}
	}
	texture, err := a.Texture(path)
	if err != nil || rows <= 0 {
		return 0
	}
	cell := float64(texture.Bounds().Dy()) / float64(rows)
	if has && columns == 1 && rows > 1 && texture.Bounds().Dx() > texture.Bounds().Dy() {
		cell = float64(texture.Bounds().Dy())
	}
	scale := e.scaleY
	if scale == 0 {
		scale = 1
	}
	return cell * scale / a.pack.TextureSourceScale(path)
}
