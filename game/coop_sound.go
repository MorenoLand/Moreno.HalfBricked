package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"

// playCoopShots plays the shot sounds of players 2 and up. Player 1's weapon sound goes through playWeaponSound (one
// tracked loop for the minigun and flamer); the others only had their bullets before, so their shooting was silent. On
// the online host these go out to the guest with every other sound (net_guest_menu.go).
func (a *app) playCoopShots() {
	p := a.play
	if p == nil || len(p.coopShots) == 0 {
		return
	}
	if p.coopShotAt == nil {
		p.coopShotAt = map[string]float64{}
	}
	for _, symbol := range p.coopShots {
		// A looping weapon sound (minigun, flamer) must not restart every frame while the trigger is held.
		if weapons.NativeWeaponSoundLoopSamples(symbol) >= 0 || symbol == "SFX_FLAMETHROWER" {
			if last, ok := p.coopShotAt[symbol]; ok && p.time-last < .25 {
				continue
			}
			p.coopShotAt[symbol] = p.time
		}
		a.playSound(a.scriptSoundPath(symbol), .8)
	}
	p.coopShots = p.coopShots[:0]
}
