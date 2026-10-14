package game

import "math"

// T-Rex / President death sequence (G11). v7 FUN_000b9d2c (death hook, 1.2.5 FUN_0011be98) and FUN_000b9624 (state 6):
//
//   - The hook sets the timer +0x33c = 1500 (ms, 0x000b9de0), plays 0x2c T_rex_death, and records the boss time and
//     the boss counter (stats_writers.go).
//   - State 6, every frame: the body shows its Death sprite; outside survival the remaining zombies are killed
//     (FUN_0006ee40, robotKillSweep) and the portals closed (FUN_0006f0c0) every frame. The timer falls by
//     dt * 1000 (int). While it is above 0 the Death clip does not run; below 1000 ms two type-2 particles per frame
//     rise from the body at z = (1 - t / 1000) * 20 (literals 0x000b9868 = 1.0, 0x000b986c = 20.0). Once it is <= 0
//     the particles continue for 350 ms at z = 20 - t / 700 * 20 (0x000b9870 = 700.0, t negative: 20 -> 30), the
//     Death clip (rexdeath 4 frames at 5 fps, loop 0) plays and when it has finished (+0xc & 1) FUN_000a10c0(self, 3)
//     removes the body.
//   - Particle call FUN_000b346c(2, pos, heading 0, spread 0xff3a (0x000b9888), count 2, speed 30.0 (0x000b9880),
//     lift 10.0 (0x000b9884)).
const (
	rexDeathTimer       = 1.5   // 0x5dc ms
	rexDeathRiseWindow  = 1.0   // the particles start below 1000 ms
	rexDeathTailWindow  = 0.35  // -0x15e ms
	rexDeathHeight      = 20.0  // DAT_000b986c
	rexDeathTailDiv     = 700.0 // DAT_000b9870
	rexDeathParticles   = 2
	rexDeathSpread      = 0xff3a
	rexDeathSpeed       = 30.0
	rexDeathLift        = 10.0
	rexDeathClipDefault = 0.8 // 4 frames at 5 fps when the sprite is not in the catalog
)

// rexDeathClipLength is the playing time of the Death clip of the boss's sprite.
func (p *playState) rexDeathClipLength(z zombieState) float64 {
	if p.sprites != nil {
		if definition, ok := p.sprites.Find(z.texture); ok {
			if animation, ok := definition.Animation("Death"); ok && animation.FPS > 0 && animation.Frames > 0 {
				return float64(animation.Frames) / animation.FPS
			}
		}
	}
	return rexDeathClipDefault
}

// rexDeathSeconds is how long the dying body stays: the 1.5 s timer, then the Death clip.
func (p *playState) rexDeathSeconds(z zombieState) float64 {
	return rexDeathTimer + p.rexDeathClipLength(z)
}

// rexDeathStep runs one frame of state 6 for a dying rex (after deathAge advanced).
func (p *playState) rexDeathStep(z *zombieState) {
	if !p.isRexBoss(z) {
		return
	}
	if !p.isSurvival() {
		p.robotKillSweep(z)
		p.closePortals()
	}
	ms := int(float32(rexDeathTimer*1000) - float32(z.deathAge)*1000)
	height, ok := 0.0, false
	switch {
	case ms > 0 && ms < int(rexDeathRiseWindow*1000):
		height, ok = (1-float64(ms)/1000)*rexDeathHeight, true
	case ms <= 0 && ms > -int(math.Round(rexDeathTailWindow*1000)):
		height, ok = rexDeathHeight-float64(ms)/rexDeathTailDiv*rexDeathHeight, true
	}
	if !ok {
		return
	}
	if p.rng == nil {
		return
	}
	p.spawnParticles(2, z.x, z.y, height, 0, rexDeathSpread, rexDeathParticles, rexDeathSpeed, rexDeathLift)
}

// rexDeathFrame is the Death clip frame of a dying rex: it starts when the timer runs out and the last frame stays.
func (p *playState) rexDeathFrame(z zombieState, frames int, fps float64) (int, bool) {
	if !z.dying || !p.isRexBoss(&z) || z.deathAge < rexDeathTimer || frames <= 0 || fps <= 0 {
		return 0, false
	}
	frame := int((z.deathAge - rexDeathTimer) * fps)
	if frame >= frames {
		frame = frames - 1
	}
	return frame, true
}
