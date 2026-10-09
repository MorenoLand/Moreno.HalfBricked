package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func TestZombieSpawnRiseTimeline(t *testing.T) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.spawnZombieAt(formats.SpawnType{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 90}, Strength: 100, Size: formats.Vec2{X: 30, Y: 30}, TurnSpeed: 12}, formats.Vec2{X: 300, Y: 100})
	z := &p.zombies[0]
	p.x, p.y = 100, 100
	// -1500 ms of waiting: invisible and motionless.
	for i := 0; i < 89; i++ {
		p.updateZombies()
	}
	if z.x != 300 || z.alpha > 2.0/255 {
		t.Fatalf("zombie moved or showed during the wait: x=%v alpha=%v", z.x, z.alpha)
	}
	// 1650 ms: the rise has begun, tall and thin (height up to 2x, width down to 0.25*0.797).
	for i := 0; i < 10; i++ {
		p.updateZombies()
	}
	if z.native.ai.state != 0 || z.size.Y <= z.native.sizeZ*1.5 || z.size.X >= z.native.sizeZ*.5 || z.x != 300 {
		t.Fatalf("rise squash wrong: state %d size %+v", z.native.ai.state, z.size)
	}
	// 2000 ms: standing and walking.
	for i := 0; i < 30; i++ {
		p.updateZombies()
	}
	if z.native.ai.state != 1 || z.size.Y != z.native.sizeZ || math.Abs(z.size.X-z.native.sizeZ*.796875) > 1e-9 || z.alpha != 1 {
		t.Fatalf("after the rise: state %d size %+v alpha %v", z.native.ai.state, z.size, z.alpha)
	}
	x := z.x
	for i := 0; i < 30; i++ {
		p.updateZombies()
	}
	if z.x >= x {
		t.Fatal("a risen zombie walks toward the player")
	}
}

// One shot raises the player's noise to 70 and every zombie within 70*alertRadius
// then tracks the player exactly; an unalerted one only drifts (retarget timer).
func TestFireNoiseAlertsZombies(t *testing.T) {
	if !zombieAlerted(300*300-1, 1, 300) || zombieAlerted(301*301, 1, 300) {
		t.Fatal("alert test is distance < noise * alertRadius")
	}
	p := modelRig()
	p.addFireNoise()
	if p.alertNoise != 70 {
		t.Fatalf("noise after one shot %v, want 70", p.alertNoise)
	}
	for i := 0; i < 10; i++ {
		p.addFireNoise()
	}
	if p.alertNoise != 350 {
		t.Fatalf("noise cap %v, want 350", p.alertNoise)
	}
	p.health = 0
	p.updateAlertNoise(1.0 / 60)
	if p.alertNoise != 0 {
		t.Fatal("death resets the noise")
	}
	p.health = 1
	for i := 0; i < 60; i++ {
		p.updateAlertNoise(1.0 / 60)
	}
	if math.Abs(p.alertNoise-.5) > 1e-6 {
		t.Fatalf("noise regrows .5/s, got %v after 1 s", p.alertNoise)
	}
}

// Heading turns by turn*error per frame (turnSpeed 12 -> 0.1).
func TestZombieTurnsByTurnFractionPerFrame(t *testing.T) {
	ai := zombieAI{turn: .1, desired: 10000, speedFactor: 1}
	if got := zombieHeadingStep(&ai, 0, false); got != 1000 {
		t.Fatalf("heading after one frame %d, want 1000", got)
	}
	ai.desired = 0
	if got := zombieHeadingStep(&ai, 100, false); got != 90 {
		t.Fatalf("turning back: %d, want 90", got)
	}
	// Weave: cos(phase)*amplitude*speedFactor is added to the desired heading.
	ai = zombieAI{turn: 1, desired: 0, speedFactor: 1, amplitude: 3000, phase: 0}
	if got := zombieHeadingStep(&ai, 0, false); got != 3000 {
		t.Fatalf("weave offset %d, want 3000", got)
	}
}

func TestZombieRetargetMovesTowardBearing(t *testing.T) {
	rng := weapons.NewNativeRNG()
	for i := 0; i < 50; i++ {
		ai := zombieAI{desired: 0}
		zombieRetarget(&ai, 20000, &rng)
		if ai.wander < .5 || ai.wander > .7 {
			t.Fatalf("retarget timer %v, want .5 + rnd(.2)", ai.wander)
		}
		// 30..50% toward the bearing, or -1.7x that away.
		toward := float64(int16(ai.desired)) / 20000
		if !(toward >= .29 && toward <= .51) && !(toward <= -.5 && toward >= -.86) {
			t.Fatalf("retarget moved %v of the error", toward)
		}
	}
}

func TestSpeedyZombieLeavesAfterimages(t *testing.T) {
	z := zombieState{x: 10, y: 20, size: formats.Vec2{X: 40, Y: 50}}
	z.native = zombieNative{kind: zombieKindSpeedy}
	z.native.ai.state = 1
	// The counter starts at 0, so the first ghost is taken on frame 1 and fades 6 per frame.
	for i := 0; i < 10; i++ {
		stepZombieTrail(&z)
	}
	if z.native.trail.ghosts[0].alpha != 0x80-9*6 || z.native.trail.ghosts[1].alpha != 0 {
		t.Fatalf("first ghost after 10 frames: %+v", z.native.trail.ghosts)
	}
	// The counter then runs 10 frames: the second ghost appears on frame 11.
	stepZombieTrail(&z)
	if z.native.trail.ghosts[1].alpha != 0x80 || z.native.trail.ghosts[0].alpha != 0x80-10*6 {
		t.Fatalf("ghosts after 11 frames: %+v", z.native.trail.ghosts)
	}
}

func armedRig(t *testing.T, weapon string) (*playState, *zombieState) {
	p := tileRig(t, func(x, y int) bool { return false })
	p.zombieWeapons = formats.ZombieWeaponCatalog{{GunType: "PISTOL", RecoilSeconds: .25, RateOfFire: .25, Ammo: 99999999, AmmoPerShot: 1, Life: .75, Speed: 600, BulletType: "NORMAL"}, {GunType: "VOMIT", RecoilSeconds: .8, RateOfFire: .8, SpreadUnits: 17 * 182, Ammo: 10, AmmoPerShot: 4, Life: 0, Speed: 250, BulletType: "VENOM"}}
	p.x, p.y = 100, 200
	p.health, p.maxHealth = 1, 1
	p.spawnZombieAt(formats.SpawnType{Name: "armed_zombie", Speed: formats.Vec2{X: 90, Y: 90}, Strength: 100, Size: formats.Vec2{X: 30, Y: 30}, TurnSpeed: 12, Weapon: weapon}, formats.Vec2{X: 220, Y: 200})
	z := &p.zombies[0]
	z.native.ai.riseMS = zombieRiseDoneMS - 1
	z.native.ai.sightRamp = 1
	return p, z
}

// armed_zombie with a gun: interval x2.25, timer starts at -0.75; in sight and
// line it brakes (state 2), waits (3), fires a bullet that hurts for 0.1.
func TestArmedZombieStopsAndShootsThePlayer(t *testing.T) {
	p, z := armedRig(t, "PISTOL")
	gun := z.native.ai.gun
	if gun.rounds != 2*99999999 || math.Abs(gun.interval-.25*2.25) > 1e-9 || gun.timer != -.75 || gun.kind != 0x10 {
		t.Fatalf("gun %+v", gun)
	}
	sawBrake, fired := false, false
	for i := 0; i < 400 && !fired; i++ {
		p.updateZombies()
		if z.native.ai.state == 2 || z.native.ai.state == 3 {
			sawBrake = true
		}
		fired = len(p.zombieShots) > 0 || p.health < 1
	}
	if !sawBrake || !fired {
		t.Fatalf("armed zombie never braked/fired (state %d, shots %d)", z.native.ai.state, len(p.zombieShots))
	}
	if z.native.ai.gun.rounds != 2*99999999-1 {
		t.Fatalf("rounds %d after one shot", z.native.ai.gun.rounds)
	}
	for i := 0; i < 120 && p.health >= 1; i++ {
		p.updateZombies()
	}
	if p.health > .95 || p.health < .8 {
		t.Fatalf("one bullet hit costs 0.1 health, health %v", p.health)
	}
}

// VOMIT / CURVER / DYNOMITE projectiles are unresolved: the zombie keeps walking.
func TestUnresolvedZombieGunsKeepWalking(t *testing.T) {
	p, z := armedRig(t, "VOMIT")
	if z.native.ai.gun.rounds != 0 {
		t.Fatal("venom is not implemented, the zombie must not stop for it")
	}
	for i := 0; i < 60; i++ {
		p.updateZombies()
	}
	if z.native.ai.state != 1 || len(p.zombieShots) != 0 {
		t.Fatalf("state %d shots %d", z.native.ai.state, len(p.zombieShots))
	}
}

func TestSnapshotCarriesZombieTintGhostsAndShots(t *testing.T) {
	host := coopTestPlay(t)
	host.zombies = []zombieState{{x: 10, y: 20, size: formats.Vec2{X: 40, Y: 50}, health: 50}}
	host.zombies[0].native = zombieNative{kind: zombieKindSpeedy, brightness: .9}
	host.zombies[0].native.trail.ghosts[0] = zombieGhost{x: 5, y: 6, w: 40, h: 50, alpha: 100}
	host.zombieShots = []zombieShot{{projectile: weapons.NativeWeaponProjectile{X: 7, Y: 8, EntityType: 0x10, Width: 28}}}
	msg, ok := decodeWire(encodeWire(wireMsg{T: "snap", Snap: host.snapshot(1)}))
	if !ok {
		t.Fatal("decode")
	}
	guest := coopTestPlay(t)
	guest.applySnapshot(msg.Snap)
	z := guest.zombies[0]
	if z.native.brightness != .9 || z.native.kind != zombieKindSpeedy || z.native.trail.ghosts[0].alpha != 100 || z.native.trail.ghosts[0].x != 5 {
		t.Fatalf("guest zombie lost tint/ghost: %+v", z.native)
	}
	if len(guest.bullets) != 1 || guest.bullets[0].projectile == nil || guest.bullets[0].projectile.Width != 28 {
		t.Fatalf("zombie shot not mirrored: %+v", guest.bullets)
	}
}

func TestZombieTintDarkensOnHit(t *testing.T) {
	z := zombieState{native: zombieNative{brightness: .9}}
	if zombieTint(z) != float32(.9) {
		t.Fatalf("tint %v", zombieTint(z))
	}
	z.hitFlash = .05
	if got, want := zombieTint(z), float32(0x32)/255*float32(.9); got != want {
		t.Fatalf("flash tint %v, want %v", got, want)
	}
	if zombieTint(zombieState{}) != 1 {
		t.Fatal("staged zombies are unmodified")
	}
}
