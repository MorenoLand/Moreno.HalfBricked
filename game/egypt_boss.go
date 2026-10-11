package game

import (
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// The Egyptian boss (boss_egyptian, native zombie type 12). Its post-init FUN_000b5198 (libmortargame.so v7):
//
//   - looks the level's prop up by its texture name "Sarcophagusclosed" (FUN_000bf978) and puts the boss on it;
//   - swaps that prop's texture for "sarcophagus" (sarcophagus_HD.png, the open coffin, same 256x128 size) and lifts
//     the prop 32 px and the boss 10 px (literals 32.0 / 10.0);
//   - sets +0x27c (speed) = 0 and +0x2e4 (gun) = 0: the boss does not walk. It sits in its open coffin and attacks
//     from there (FUN_000b5930: flame bursts, a ground slam and gun bursts); the sprite is only the upper body
//     because the lower half is the coffin.
//
// The flame / slam / gun attack cycle of FUN_000b5930 is UNRESOLVED (not ported); the boss stays where it is.
const (
	egyptCoffinClosed = "Sarcophagusclosed"
	egyptCoffinOpen   = "sarcophagus"
	egyptCoffinLift   = 32.0
	egyptBossLift     = 10.0
)

// egyptBossCoffin opens the level's sarcophagus and returns where the boss stands.
func (p *playState) egyptBossCoffin(point formats.Vec2) (formats.Vec2, bool) {
	if p.world == nil {
		return point, false
	}
	for i := range p.world.Level.Props {
		prop := &p.world.Level.Props[i]
		if strings.EqualFold(prop.Texture, egyptCoffinClosed) {
			at := formats.Vec2{X: prop.X, Y: prop.Y - egyptBossLift}
			prop.Texture = egyptCoffinOpen
			prop.Y -= egyptCoffinLift
			return at, true
		}
	}
	return point, false
}
