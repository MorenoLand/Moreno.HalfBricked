package game

// inDepthBand reports whether an entity at ground y belongs to the band being drawn (always true outside a banded draw).
func (a *app) inDepthBand(y float64) bool {
	if !a.depthBandActive {
		return true
	}
	return y >= a.depthBand.Lo && y < a.depthBand.Hi
}
