package game

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
	return z.y + (.5-zombieAnchor(*z))*height
}

// zombieAnchor is the fraction of its height the sprite is lifted above the position (+0x2d4): 0.35 for the plain
// zombies, 0.2 for the Don Zombie's car (FUN_000b732c stores 0.2 into +0x2d4).
func zombieAnchor(z zombieState) float64 {
	if z.native.car.active {
		return carRenderAnchor
	}
	return zombieRenderAnchor
}
