package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

// inDepthBand reports whether an entity at raw native position y belongs to the band being drawn (always true
// outside a banded draw). The band holds y-buckets, not pixels: native submits every entity at its own position y
// (no feet offset, Research/native/prop-entity-draw-order-2026-10-12.md) into a 1.3 px bucket, and an entity in the
// bucket of a tile row or prop is drawn after it.
func (a *app) inDepthBand(y float64) bool {
	if !a.depthBandActive {
		return true
	}
	return a.depthBand.Contains(y)
}

// depthBucket is the native y-bucket of an entity at raw position y (a.depthBand's camera, or the live camera).
func (a *app) depthBucket(y float64) int {
	if a.depthBandActive {
		return a.depthBand.Bucket(y)
	}
	if a.play == nil || a.play.world == nil {
		return viewer.DepthBucket(y, 0, 0)
	}
	return viewer.DepthBucket(y, a.play.world.CameraCentreY(), a.play.world.NativeZoom())
}


// zombieSpriteLift is how far the sprite's centre is lifted above the zombie's position (native: +0x2d4 * +0x30).
// For the plain zombies +0x30 is the drawn height; the Don Zombie's car keeps its 2 * half size there while its
// drawn height is 0.6 of its width, so its sprite sits higher than a height-based lift would put it.
func zombieSpriteLift(z zombieState, renderHeight float64) float64 {
	if z.native.kind == zombieKindRobot {
		// FUN_000bb0e0: +0x2d4 = 0.75; the teleport raises it to 2 * t + 0.75 (FUN_000bb610).
		if z.native.robot.active && z.native.robot.lift > 0 {
			return z.native.robot.lift * z.native.sizeZ
		}
		return robotRenderAnchor * z.native.sizeZ
	}
	if z.native.car.active {
		return carRenderAnchor * z.native.sizeZ
	}
	return renderHeight * zombieRenderAnchor
}

// scriptSpriteDepthY is the key a script sprite registers with. A sprite made with CreateEntity(..., true) keys at
// y + 0.3 * height (the sprite is drawn centred on y), any other entity at its raw y. The draw position does not
// change. Research/native/script-sprite-depth-2026-10-14.md (v7 FUN_00099f60, 1.2.5 FUN_000f8488).
func (a *app) scriptSpriteDepthY(e *scriptEntity) float64 {
	if e == nil || e.kind != "sprite" || !e.feet {
		if e == nil {
			return 0
		}
		return e.y
	}
	return e.y + .3*a.scriptEntityDrawHeight(e)
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

// robotBodyAimPoint reports the ground position of a living robot boss when the world point (x, y) lies inside his
// drawn body (centred zombieSpriteLift above his position).
func (p *playState) robotBodyAimPoint(x, y float64) (float64, float64, bool) {
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.native.kind != zombieKindRobot || z.health <= 0 || z.dying {
			continue
		}
		centreY := z.y - zombieSpriteLift(*z, z.size.Y)
		if math.Abs(x-z.x) <= z.size.X/2 && math.Abs(y-centreY) <= z.size.Y/2 {
			return z.x, z.y, true
		}
	}
	return 0, 0, false
}
