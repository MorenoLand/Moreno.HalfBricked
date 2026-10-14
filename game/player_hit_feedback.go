package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Player damage feedback (G07). Native: v7 FUN_00094c6c, 1.2.5 FUN_000f2208 (the damage routine), v7 FUN_000962f4 /
// 1.2.5 FUN_000f2964 (the per-frame dead state).
//
// A hit that has a source object and lands while the bleed counter rests (+0x9c < -699, 1.2.5 +0xc4) and the
// player is not downed:
//
//   - starts a camera shake at the source's position: duration 0.3 s (DAT_00095074 / 1.2.5 DAT_000f25b0 = 0x3e99999a),
//     amplitudes 1.0 x 1.0, angle multiplier 1.0 (v7, FUN_000be1fc) / 8.0 (1.2.5, FUN_00095d38 via FUN_000f5be8);
//   - v7 only: five type-1 particles (FUN_000b346c at 0x94e44) at the player + 16 px along the shake angle ^ 0x8000
//     (cos/sin * 64 * 0.25), height 10, heading = that angle, spread 0x1554, speed 20, lift 10. 1.2.5 FUN_000f2208 has
//     no particle call.
//
// While the player is dead and the respawn timer (+0x98, set to 2.0 at death; 1.2.5 +0xc0) is above 1.5, both builds
// spawn one type-1 particle per frame at the player's position, height 10, heading 0, spread 0xff3a, speed 20,
// lift 10 (v7 0x9644c, 1.2.5 FUN_000f2964).
//
// The port keeps its own red hurt tint (coop.go); the owner has not ruled on it.
const (
	hitShakeDuration  = 0.3
	hitSpurtCount     = 5
	hitSpurtOffset    = 64 * 0.25
	hitSpurtHeight    = 10.0
	hitSpurtSpeed     = 20.0
	hitSpurtLift      = 10.0
	hitSpurtSpread    = 0x1554
	deadSpurtSpread   = 0xff3a
	deadSpurtAbove    = 1.5
	deadSpurtHeight   = 10.0
)

// hitOrigin is the position of the object that dealt the damage when the caller passes one without a zombie.
type hitOrigin struct {
	x, y float64
	set  bool
}

// damagePlayerAt is damagePlayer for a source object at (x, y) (blast, wave, projectile, boss body).
func (p *playState) damagePlayerAt(index int, amount, x, y float64) {
	p.hitFrom = hitOrigin{x: x, y: y, set: true}
	p.damagePlayer(index, amount)
	p.hitFrom = hitOrigin{}
}

// playerHitFeedback runs in the damage routine before the bleed trigger, with the same rest test as the hurt sound.
func (p *playState) playerHitFeedback(v *playerVitals, px, py float64, source *zombieState) {
	if p.quietHit || v.bleeding || (v.bleedInit && v.bleedCtr > bleedRest) {
		return
	}
	var ox, oy float64
	switch {
	case source != nil:
		ox, oy = source.x, source.y
	case p.hitFrom.set:
		ox, oy = p.hitFrom.x, p.hitFrom.y
	default:
		return
	}
	p.startCameraShake(ox, oy, hitShakeDuration, 1, 1, cameraShakeAngleMul(p.waveBuild()))
	if p.waveBuild() != content.WaveBuildV7 {
		return
	}
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
	angle := p.shake.angle ^ 0x8000
	x := px + cosU16(angle)*hitSpurtOffset
	y := py + sinU16(angle)*hitSpurtOffset
	p.spawnParticles(particleKindSpurt, x, y, hitSpurtHeight, angle, hitSpurtSpread, hitSpurtCount, hitSpurtSpeed, hitSpurtLift)
}

// deadSpurt is the per-frame spurt of the dead state while the respawn timer is above 1.5.
func (p *playState) deadSpurt() {
	if p.deathTimer <= deadSpurtAbove {
		return
	}
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
	p.spawnParticles(particleKindSpurt, p.x, p.y, deadSpurtHeight, 0, deadSpurtSpread, 1, hitSpurtSpeed, hitSpurtLift)
}
