package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"

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
	if z.native.car.active {
		return carRenderAnchor * z.native.sizeZ
	}
	return renderHeight * zombieRenderAnchor
}
