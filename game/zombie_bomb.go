package game

import "math"

// Zombie DYNOMITE (G06): the zombie weapon record DYNOMITE (DLC1_ZombieWeapons.xml: Recoil / Rate 1000, Spread 5,
// Ammo 5, 1 per shot, Life 1, Speed 350, Bullet_Type DYNOMITE) fires entity kind 0x1d, the class built by v7
// FUN_000a643c (1.2.5 FUN_00106988) that Mad Dog's grenade also uses (the post-init FUN_000a5a7c plays 0x3d md_grenade).
//
// Native evidence (v7 addresses; the 1.2.5 twins FUN_00105fa8 / FUN_00107708 / FUN_0010467c carry the same literals):
//   - Fire: FUN_0009f7e4 as for every zombie bullet (lift 0.46875 * body height, fall (lift - 23.6) / (distance / speed),
//     hit-player 1, hit-entities 0, owner = the zombie), then the post-init FUN_000a5a7c: state 2, the bounce flag
//     +0x80, life doubled, width/height/depth x 4.0 (10 -> 40; DAT_000a5ae4), hit-entities +0x59 = 1, sound 0x3d.
//   - Motion FUN_000a700c (called by FUN_000a5f74 after age += dt, lift -= dt * fall): fall += dt * 205 (+0x84,
//     0x434d0000) and, in states 2/3, fall += dt * 500 (DAT_000a7200); the bomb moves dir * speed * dt; while the fall
//     speed is positive a wall tile pushes it out (FUN_000be3c0, flag 0, radius = width), the push is subtracted from
//     the position and the heading, the heading is renormalised, speed *= 0.75 and sound 0x13 plays. The bomb
//     detonates (vtable +0x54 FUN_000a4050) when the bounce counter (+0x88, starts at 4) is below 1 and its lift is
//     below 10.0 (DAT_000a71fc).
//   - Landing (lift < 0 -> FUN_000a4dc4 with no target): lift = 0, bounces - 1, speed *= 0.5 (DAT_000a4f34), fall *=
//     -0.2 (DAT_000a4f38). While the bomb overlaps the player (+0x58 flag) its speed is multiplied by 0.95 per frame
//     (DAT_000a4f3c, bounce flag set).
//   - Detonation FUN_000a4050: state 1, sound 0xc (grenade_explode); the explosion is the shared FUN_000a5bb4 blast
//     (radius 160 -> 0 in .15 s, over at .75 s). It hurts the player at dt * 1.5 per frame (DAT_000a4f40) and every
//     zombie but its owner by takeDamage(5) (FUN_000a4dc4 with a target).
//   - Draw FUN_000a7d44 (the grenade draw): the "Dynomite" texture, turned to its heading.
//
// VOMIT (kind 0x17, Life 0): the earlier reading that FUN_000a4288 removes it on the first update was wrong: the init
// hook FUN_000a4210 overwrites the life (1.4 + rnd(0.5)), so the zombie's VOMIT is the rex's spit (rex_ai.go).
const (
	zombieBombSpeedBoost  = 205.0 // +0x84 (0x434d0000)
	zombieBombGravity     = 500.0 // DAT_000a7200
	zombieBombBounces     = 4     // +0x88
	zombieBombDetonateLow = 10.0  // DAT_000a71fc
	zombieBombWallSpeed   = .75   // DAT_000a7204
	zombieBombLandSpeed   = .5    // DAT_000a4f34
	zombieBombLandFall    = -.2   // DAT_000a4f38
	zombieBombPlayerDrag  = .95   // DAT_000a4f3c
	zombieBombScale       = 4.0   // DAT_000a5ae4
	zombieBombBaseSize    = 10.0  // DAT_000a4978
	zombieBombPlayerRate  = 1.5   // DAT_000a4f40
	zombieBombKind        = 0x1d
	zombieBombFireSound   = "SFX_MD_GRENADE"      // id 0x3d
	zombieBombBlastSound  = "SFX_GRENADE_EXPLODE" // id 0xc
)

// fireZombieBomb launches one DYNOMITE bomb from a zombie (one pellet of FUN_0009f7e4).
func (p *playState) fireZombieBomb(z *zombieState, heading uint16, lift, fall float64, gun *zombieGun) {
	p.sfxQueue = append(p.sfxQueue, zombieBombFireSound)
	p.thrown = append(p.thrown, thrownBomb{x: z.x, y: z.y, lift: lift, fall: fall, dirX: cosU16(heading), dirY: sinU16(heading),
		speed: gun.speed, life: gun.life * 2, size: zombieBombBaseSize * zombieBombScale, bounce: true,
		zombieBomb: true, bounces: zombieBombBounces, ownerID: z.scriptID,
		origin: killOrigin{}}) // no weapon tracker: the fire call passes none (FUN_000a4868 argument 12 = 0)
}

// stepZombieBomb advances one bomb a 1/60 s frame; false means it is gone.
func (p *playState) stepZombieBomb(b *thrownBomb, dt float64) bool {
	b.age += dt
	b.lift -= dt * b.fall // FUN_000a5f74
	// FUN_000a700c
	b.fall += dt * zombieBombSpeedBoost
	b.fall += dt * zombieBombGravity
	b.x += b.dirX * b.speed * dt
	b.y += b.dirY * b.speed * dt
	if b.fall > 0 {
		if pushX, pushY := p.projectileTilePush(b.x, b.y, b.size); pushX != 0 || pushY != 0 {
			b.x += pushX
			b.y += pushY
			b.dirX += pushX
			b.dirY += pushY
			if length := math.Hypot(b.dirX, b.dirY); length > 0 {
				b.dirX, b.dirY = b.dirX/length, b.dirY/length
			}
			b.speed = float64(float32(b.speed) * float32(zombieBombWallSpeed))
			p.sfxQueue = append(p.sfxQueue, thrownBounceSound)
		}
	}
	if b.bounces < 1 && b.lift < zombieBombDetonateLow {
		p.detonateZombieBomb(b)
		return false
	}
	// FUN_000a4dc4, the landing branch.
	if b.lift < 0 {
		b.lift = 0
		b.bounces--
		b.speed *= zombieBombLandSpeed
		b.fall *= zombieBombLandFall
	}
	// FUN_000a5f74's player pass: overlapping the player slows the bomb (bounce flag).
	for _, target := range p.livingPlayers() {
		reach := zombieBlastBodyHalf + zombieBlastPlayerScale*b.size
		if zombieBlastEllipse(target.x-b.x, target.y-b.y, reach) {
			b.speed *= zombieBombPlayerDrag
		}
	}
	return true
}

// detonateZombieBomb is FUN_000a4050: sound 0xc and the blast, hurting the player at 1.5/s and every zombie except
// its owner.
func (p *playState) detonateZombieBomb(b *thrownBomb) {
	p.spawnBlast(b.x, b.y, zombieBombBlastSound, b.origin, false)
	last := &p.zombieBlasts[len(p.zombieBlasts)-1]
	last.playerRate = zombieBombPlayerRate
	last.ownerID = b.ownerID
}

// fireZombieVenom is one pellet of a zombie's VOMIT volley: a venom blob at the zombie plus 16 px along its facing,
// flying along the pellet heading at the record's speed (the rex's fire code, rex_ai.go).
func (p *playState) fireZombieVenom(z *zombieState, heading uint16, gun *zombieGun) {
	facing := z.native.facing
	radians := float64(heading) * 2 * math.Pi / 65536
	height := z.native.sizeZ * rexVenomHeightFrac
	p.rex.venom = append(p.rex.venom, p.newRexVenom(z.x+cosU16(facing)*rexVenomMuzzle, z.y+sinU16(facing)*rexVenomMuzzle, radians, height, gun.speed, false))
}
