package game

// Player-fired explosions share the exploding zombie's blast class.
//
// Evidence (libmortargame.so v7, image base 0x10000; details in
// Research/native/secondary-weapons-2026-10-08.md):
//
//   - GRENADE (bullet kind 0x14, vtable 0x005bb190), MINE (0x15, 0x005bb2b0),
//     BAZOOKA (0x13, 0x005bb130) and the dynamite COWPAT (0x1e, 0x005bb250) all run
//     the same explosion state: their detonation slot +0x54 is FUN_000a3ca8 (age 0,
//     state 1, plays sound id 8 = "mine_explode") and their update calls
//     FUN_000a5bb4(self, dt, 0, 1.0). The last argument is the radius scale and is
//     1.0 for every class, so the radius is the exploding zombie's
//     160*(1 - age/.75*.5) for the first .15 s and 0 afterwards, and the state
//     ends at .75 s (zombie_blast constants in exploding_zombie.go).
//   - The hit pass is FUN_000a5f74 (the same ellipse test and entity-grid walk) and
//     the damage handlers FUN_000a4cac (mine), FUN_000a4f50 (grenade, dynamite) and
//     FUN_000a50f4 (rocket) all call takeDamage(5) on a zombie while in state 1.
//   - The projectile constructors FUN_000adbe8 / FUN_000ada64 / FUN_000ad8e0 /
//     FUN_000ad5f0 (grenade, mine, bazooka, dynamite fire) and the sentry's death
//     (FUN_0009acb8) pass hit-player = 0, hit-entities = 1 to FUN_000a4868, so none
//     of these blasts hurts the player (the player branch of the handlers, 1.5/s for
//     grenades and dynamite, 3/s for mines, is reached only with hit-player = 1,
//     which only the exploding zombie sets).

// spawnBlast starts a mine-class blast at (x, y): the explosion2 animation, the
// "mine_explode" sound and a damaging blast projectile (updateZombieBlasts).
// harmless is the hit-player flag inverted.
func (p *playState) spawnBlast(x, y float64, sound string, origin killOrigin, harmless bool) {
	if sound != "" {
		p.sfxQueue = append(p.sfxQueue, sound)
	}
	p.explosions = append(p.explosions, explosionState{x: x, y: y})
	id := origin.shot
	if id == 0 {
		id = p.newShot()
	}
	p.zombieBlasts = append(p.zombieBlasts, zombieBlast{x: x, y: y, id: id, origin: origin, harmless: harmless})
}

// Rocket body: FUN_000a5900 sets width 14 (literal 0x000a5988). A rocket in
// flight (state 0) that overlaps a zombie runs FUN_000a50f4, which calls
// takeDamage(5) once per entity-grid listing and then detonates.
const rocketBodyWidth = 14.0

func (p *playState) rocketContactDamage(b bullet, zombie *zombieState) {
	if zombie.health <= 0 || zombie.dying || zombie.invulnerable {
		return
	}
	visits := p.zombieGridVisits(zombie, b.x, b.y, float32(rocketBodyWidth)*zombieBlastQueryScale)
	for visit := 0; visit < visits && zombie.health > 0; visit++ {
		zombie.health -= zombieBlastDamage
		zombie.hitFlash = zombieHitFlashDuration
		if zombie.health > 0 {
			p.comboCredit(b.origin, comboHitBlast)
		}
	}
	if zombie.health <= 0 {
		p.creditKill(b.origin)
		zombie.dying = true
		zombie.deathAge = 0
	}
}
