package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/formats"

// spawnLevelPickups puts down the crates a level made in the editor places for the wave that is starting (wave 0 is the
// start of the level; waves count from 1). Crates with no wave, or wave 1, are there from the start.
func (p *playState) spawnLevelPickups(wave int) {
	if p.world == nil {
		return
	}
	for _, pickup := range p.world.Level.Pickups {
		atStart := pickup.Wave <= 1
		if (wave == 0 && atStart) || (wave > 1 && pickup.Wave == wave) {
			p.spawnPickup(pickup.Name, formats.Vec2{X: pickup.X, Y: pickup.Y})
		}
	}
}
