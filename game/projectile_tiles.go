package game

import "math"

// Projectile versus level geometry (libmortargame.so v7).
//
// The shared projectile update FUN_000a5f74 (every entity kind 0x10..0x1d: normal,
// through and multi bullets, bazooka rocket, grenade/mine/dynamite, flame, venom,
// spinner, saw blade, rex shockwave) ends a projectile in state 0 when
//
//	age >= life  ||  FUN_000be2c4(level, (int)(x * 1/32), (int)(y * 1/32)) == 1
//	             ||  lift < 0
//
// FUN_000be2c4 returns the collision-layer cell, or -1 outside the level. So only
// collision value 1 stops a projectile and only under its centre: value 2 (the
// pits, water and the void around the arena, which still block walking because
// FUN_000be3c0 is called with its blocking flag set for zombies and the player)
// is flown over, the level edge is not a wall (-1), and spawn markers (3..16) are
// open ground. See Research/native/projectile-tile-collision-2026-10-09.md.
//
// Projectiles that bounce or stop (grenade/dynamite FUN_000a7218/FUN_000a700c,
// flame FUN_000a6a58) call FUN_000be3c0 with flag 0, i.e. the same value-1-only
// set, and resolve against a circle instead of a point.

// projectileBlockedAt is the centre test of FUN_000a5f74.
func (p *playState) projectileBlockedAt(x, y float64) bool {
	if p.world == nil {
		return false
	}
	tx, ty := int(float32(x)*(1.0/32)), int(float32(y)*(1.0/32))
	return inLevel(p, tx, ty) && p.collisionValue(tx, ty) == 1
}

// projectileTilePush is FUN_000be3c0 with blocking flag 0: the displacement to add
// to (x, y) so a projectile body of radius r leaves the value-1 tiles.
func (p *playState) projectileTilePush(x, y, r float64) (float64, float64) {
	if p.world == nil || math.IsNaN(r) {
		return 0, 0
	}
	return p.nativeTilePushMode(x, y, r, false)
}

// flameWallRadius is the FUN_000a6a58 push radius: width (+0x28) * 0.5
// (literal DAT_000a6cd0 = 0x3f000000).
const flameWallRadius = .5

// bulletInWall is the tile rule of a player bullet for one frame, after it moved.
// Every kind ends on a value-1 tile under its centre (FUN_000a5f74). The flame
// (kind 0x16, FUN_000a6a58) never reaches that test: while its speed factor is
// above 0.1 it calls FUN_000be3c0(pos, width * .5, flag 0), subtracts the push
// and zeroes the speed factor, so it stops against the wall and lives out its
// lifetime there.
func (p *playState) bulletInWall(b *bullet) bool {
	if b.projectile != nil && b.projectile.EntityType == 0x16 && (b.vx != 0 || b.vy != 0) {
		radius := b.projectile.Width * flameWallRadius
		if pushX, pushY := p.projectileTilePush(b.x, b.y, radius); pushX != 0 || pushY != 0 {
			b.x += pushX
			b.y += pushY
			b.vx, b.vy = 0, 0
			b.projectile.X, b.projectile.Y = b.x, b.y
			b.projectile.VX, b.projectile.VY = 0, 0
		}
	}
	return p.projectileBlockedAt(b.x, b.y)
}
