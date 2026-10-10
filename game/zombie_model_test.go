package game

import (
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func modelRig() *playState {
	p := &playState{
		world:    &viewer.Viewer{Level: formats.Level{Width: 64, Height: 64, Layers: map[formats.LayerKind][]uint32{formats.LayerC: openLayer(64 * 64)}}, Zoom: 1},
		x:        100,
		y:        200,
		tileSize: 32, health: 1, maxHealth: 1, lives: 3, hudVisible: true, multiplier: 1,
		scriptEntities: map[int]*scriptEntity{}, scriptNextEntity: 1,
	}
	rng := weapons.NewNativeRNG()
	p.rng = &rng
	return p
}

func waveEntry(name string, strength float64, size formats.Vec2) formats.SpawnType {
	return formats.SpawnType{Name: name, Chance: 1, Speed: formats.Vec2{X: 90, Y: 95}, Strength: strength, Size: size, TurnSpeed: 12, Texture: "cavezombie"}
}

// pistolBullet is a NORMAL bullet (entity 0x10, width 10) at a point.
func pistolBullet(x, y float64) bullet {
	return bullet{x: x, y: y, life: 1, projectile: &weapons.NativeWeaponProjectile{EntityType: 0x10, Width: 10, Life: 1}}
}

// A pistol bullet deals 101 (FUN_000a521c): strength 100 dies in one hit, 300 in
// 3, 500 in 5, 700 in 7, 1000 in 10 (ceil(strength / 101)).
func TestPistolHitsToKillFollowStrength(t *testing.T) {
	for _, c := range []struct {
		strength float64
		hits     int
	}{{100, 1}, {300, 3}, {500, 5}, {700, 7}, {1000, 10}} {
		p := modelRig()
		p.spawnZombieAt(waveEntry("zombie", c.strength, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
		z := &p.zombies[0]
		z.native.ai.state = 1
		hits := 0
		for !z.dying && hits < 50 {
			p.bullets = append(p.bullets, pistolBullet(z.x, z.y))
			p.updateBulletsAndKills()
			hits++
		}
		if hits != c.hits {
			t.Fatalf("strength %v died after %d hits, want %d", c.strength, hits, c.hits)
		}
		if got, want := z.health, c.strength-float64(c.hits*zombieBulletDamage); got != want {
			t.Fatalf("strength %v health after kill = %v, want %v", c.strength, got, want)
		}
		if len(p.bullets) != 0 {
			t.Fatalf("a 0x10 bullet is consumed by its hit, %d left", len(p.bullets))
		}
	}
}

// The zombie keeps its max health; a surviving hit starts the 0.1 s flash.
func TestZombieHitFlashAndSurvivalSound(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 300, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
	riseZombies(p)
	z := &p.zombies[0]
	if z.native.maxHealth != 300 || z.health != 300 {
		t.Fatalf("health %v / max %v, want 300", z.health, z.native.maxHealth)
	}
	p.bullets = append(p.bullets, pistolBullet(z.x, z.y))
	p.updateBulletsAndKills()
	if z.health != 199 || z.hitFlash != zombieDamageFlash || zombieDamageFlash != .1 {
		t.Fatalf("after one hit: health %v flash %v", z.health, z.hitFlash)
	}
	if len(p.sfxQueue) != 1 || !strings.HasPrefix(p.sfxQueue[0], "SFX_ZOMBIE_HURT_") {
		t.Fatalf("surviving hit queues zombie_hurt_1/2, got %v", p.sfxQueue)
	}
	// Flame particles deal 1 and play no hurt sound (only kind 0x10 does).
	p.sfxQueue = nil
	flame := bullet{x: z.x, y: z.y, life: 1, projectile: &weapons.NativeWeaponProjectile{EntityType: 0x16, Width: 20}}
	p.bullets = append(p.bullets, flame)
	p.updateBulletsAndKills()
	if z.health != 198 || len(p.sfxQueue) != 0 {
		t.Fatalf("flame: health %v sounds %v, want 198 and none", z.health, p.sfxQueue)
	}
}

// Score is (rounded points / 20) * multiplier when the death finalises, so a
// 300 strength zombie pays 15 at multiplier 1 and nothing before it dies.
func TestBigZombieScoreComesFromStrength(t *testing.T) {
	p := modelRig()
	p.multiplier = 2
	p.spawnZombieAt(waveEntry("zombie", 300, formats.Vec2{X: 35, Y: 35}), formats.Vec2{X: 400, Y: 400})
	riseZombies(p)
	z := &p.zombies[0]
	if z.rawPoints != 300 {
		t.Fatalf("raw points %d, want the strength 300", z.rawPoints)
	}
	for i := 0; i < 3; i++ {
		p.bullets = append(p.bullets, pistolBullet(z.x, z.y))
		p.updateBulletsAndKills()
		if i < 2 && p.score != 0 {
			t.Fatalf("score %d before the kill", p.score)
		}
	}
	for i := 0; i < 20; i++ {
		p.updateZombies()
		p.updateBulletsAndKills()
	}
	if want := (300 / 20) * 2; p.score != want {
		t.Fatalf("score %d, want %d", p.score, want)
	}
}

// shieldDirection is the bullet heading whose heading + 0x3ffc lies diff units
// behind the zombie facing (FUN_00072a40 argument order).
func shieldDirection(facing uint16, diff int) uint16 {
	return facing - zombieShieldBulletOffset - uint16(diff)
}

// Shield zombies (type 3) take damage/10 from a type 0x10 bullet inside the arc of
// 0x1c6f around their heading + 0x3ffc and the full 101 outside it.
func TestShieldZombieReducesFrontalBulletDamage(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("shield_zombie", 1000, formats.Vec2{X: 40, Y: 40}), formats.Vec2{X: 400, Y: 400})
	riseZombies(p)
	z := &p.zombies[0]
	if z.native.kind != zombieKindShield {
		t.Fatalf("native kind %d, want 3", z.native.kind)
	}
	z.native.facing = 0x2000
	shielded := bullet{x: z.x, y: z.y, life: 1, projectile: &weapons.NativeWeaponProjectile{EntityType: 0x10, Width: 10, Direction: shieldDirection(0x2000, 0x1c6f)}}
	p.bullets = append(p.bullets, shielded)
	p.updateBulletsAndKills()
	if z.health != 990 || z.hitFlash != zombieShieldFlash {
		t.Fatalf("shielded hit: health %v flash %v, want 990 and 0.05", z.health, z.hitFlash)
	}
	open := bullet{x: z.x, y: z.y, life: 1, projectile: &weapons.NativeWeaponProjectile{EntityType: 0x10, Width: 10, Direction: shieldDirection(0x2000, 0x1c70)}}
	p.bullets = append(p.bullets, open)
	p.updateBulletsAndKills()
	if z.health != 990-101 {
		t.Fatalf("unshielded hit: health %v, want %v", z.health, 990-101)
	}
	// A piercing child (kind 0x11) is never reduced.
	z.health = 1000
	child := bullet{x: z.x, y: z.y, life: 1, projectile: &weapons.NativeWeaponProjectile{EntityType: 0x11, Width: 16.5, Direction: shieldDirection(0x2000, 0)}}
	p.bullets = append(p.bullets, child)
	p.updateBulletsAndKills()
	if z.health != 1000-101 {
		t.Fatalf("0x11 hit on a shield zombie: health %v, want %v", z.health, 1000-101)
	}
}

// The hit test is the ellipse of FUN_000a5f74: reach = bullet width + 0.3 * body
// width, y squashed by 0.666, so a larger zombie is hit from further away.
func TestBulletReachGrowsWithZombieSize(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 29, Y: 29}), formats.Vec2{X: 400, Y: 400})
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 45, Y: 45}), formats.Vec2{X: 800, Y: 400})
	riseZombies(p)
	small, big := &p.zombies[0], &p.zombies[1]
	b := pistolBullet(small.x+10+zombieBodyScale*small.size.X+1, small.y)
	if p.bulletReachesZombie(b, small) {
		t.Fatal("a bullet just outside 10 + 0.3*width must miss")
	}
	b = pistolBullet(big.x+10+zombieBodyScale*small.size.X+1, big.y)
	if !p.bulletReachesZombie(b, big) {
		t.Fatal("the same offset hits the bigger body")
	}
	// y is squashed: dy of 0.666*reach is inside, reach is outside.
	reach := 10 + zombieBodyScale*big.size.X
	if !p.bulletReachesZombie(pistolBullet(big.x, big.y+.66*reach), big) || p.bulletReachesZombie(pistolBullet(big.x, big.y+.67*reach), big) {
		t.Fatal("vertical reach must be 0.666 of the horizontal reach")
	}
}

// A bullet pass ends at the first target for kinds 0x10, 0x11 and 0x18.
func TestBulletStopsAtFirstTarget(t *testing.T) {
	p := modelRig()
	for i := 0; i < 3; i++ {
		p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 29, Y: 29}), formats.Vec2{X: 400, Y: 400})
	}
	riseZombies(p)
	p.bullets = append(p.bullets, pistolBullet(400, 400))
	p.updateBulletsAndKills()
	dead := 0
	for _, z := range p.zombies {
		if z.dying {
			dead++
		}
	}
	if dead != 1 {
		t.Fatalf("%d zombies hit by one bullet, want 1", dead)
	}
}

func TestZombieSpawnRecordRollsRanges(t *testing.T) {
	rng := weapons.NewNativeRNG()
	entry := formats.SpawnType{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 300, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12}
	seenSpeed, seenSize := map[int]bool{}, map[int]bool{}
	for i := 0; i < 200; i++ {
		r := rollZombieSpawnRecord(entry, &rng)
		if r.Speed < 90 || r.Speed > 94 || r.HalfSize < 29 || r.HalfSize > 30 || r.Strength != 300 || r.TurnSpeed != 12 || r.Kind != zombieKindPlain {
			t.Fatalf("record %+v outside the declared ranges", r)
		}
		seenSpeed[r.Speed], seenSize[r.HalfSize] = true, true
	}
	if len(seenSpeed) != 5 || len(seenSize) != 2 {
		t.Fatalf("rolls cover speeds %v sizes %v, want 90..94 and 29..30", seenSpeed, seenSize)
	}
	// A point range draws nothing and keeps the value.
	before := rng
	r := rollZombieSpawnRecord(formats.SpawnType{Name: "zombie", Speed: formats.Vec2{X: 80, Y: 80}, Size: formats.Vec2{X: 40, Y: 40}, Strength: 500, TurnSpeed: 12, DeviateAmount: 17, AlertRadius: 250, DeviateCycleSpeed: 45}, &rng)
	if r.Speed != 80 || r.HalfSize != 40 || before != rng {
		t.Fatalf("point ranges: %+v, rng advanced %t", r, before != rng)
	}
	if got := nativeZombieTypes["speedy_zombie"]; got != zombieKindSpeedy {
		t.Fatalf("speedy kind %d", got)
	}
	if rollZombieSpawnRecord(formats.SpawnType{Name: "armed_zombie", Weapon: "VOMIT"}, &rng).Gun != 10 {
		t.Fatal("VOMIT is zombie gun 10")
	}
}

// size is the half size: the body is 2*size tall and 0.796875 of that wide.
func TestWaveZombieSizeComesFromTheSizeAttribute(t *testing.T) {
	p := modelRig()
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 40, Y: 40}), formats.Vec2{X: 400, Y: 400})
	p.spawnZombieAt(waveEntry("zombie", 100, formats.Vec2{X: 45, Y: 45}), formats.Vec2{X: 500, Y: 400})
	standard := waveEntry("zombie", 100, formats.Vec2{X: 29, Y: 31})
	p.spawnZombieAt(standard, formats.Vec2{X: 600, Y: 400})
	forty, forty5, std := p.zombies[0], p.zombies[1], p.zombies[2]
	if forty.size.Y != 80 || math.Abs(forty.size.X-80*.796875) > 1e-9 {
		t.Fatalf("size 40 body = %v x %v, want 63.75 x 80", forty.size.X, forty.size.Y)
	}
	if forty5.size.Y != 90 {
		t.Fatalf("size 45 body height %v, want 90", forty5.size.Y)
	}
	if std.size.Y != 58 && std.size.Y != 60 {
		t.Fatalf("size 29..31 body height %v, want 58 or 60", std.size.Y)
	}
	if want := zombieBodyScale * forty.size.X; math.Abs(zombieCollisionRadius(forty)-want) > 1e-9 {
		t.Fatalf("collision radius %v, want 0.3 * body width = %v", zombieCollisionRadius(forty), want)
	}
	if forty.native.maxHealth != 100 || forty5.native.sizeZ != 90 {
		t.Fatalf("native record not kept: %+v %+v", forty.native, forty5.native)
	}
}
