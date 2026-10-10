package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

func pickupTestWeapons() formats.WeaponCatalog {
	return formats.WeaponCatalog{{GunType: "GRENADE", Ammo: 5}}
}

func TestPortalAlphaFadesInAndOut(t *testing.T) {
	a := 0.0
	for i := 0; i < 16; i++ {
		a = portalAlphaStep(a, false)
	}
	if a != 255 {
		t.Fatalf("alpha after 16 open frames = %v, want 255", a)
	}
	if first := portalAlphaStep(0, false); first != 16 {
		t.Fatalf("first open step = %v, want 16 (trunc 16.67)", first)
	}
	if got := portalAlphaStep(255, true); got != 238 {
		t.Fatalf("first close step = %v, want 238", got)
	}
	// native quirk: a step that would go below zero doubles the alpha
	if got := portalAlphaStep(10, true); got != 20 {
		t.Fatalf("closing alpha 10 -> %v, want 20", got)
	}
	if got := portalAlphaStep(0, true); got != 0 {
		t.Fatalf("closing alpha 0 -> %v", got)
	}
}

func TestPortalRotationSpeedTargets(t *testing.T) {
	play := &playState{portals: []portalState{{rotationSpeed: 0}}}
	for i := 0; i < 400; i++ {
		play.updatePortals()
		if len(play.portals) == 0 {
			break
		}
	}
	// at 2 s lifetime the late target 1.0 applies from remaining <= 1 s
	p := &playState{portals: []portalState{{age: 1.2, rotationSpeed: 1.2}}}
	for i := 0; i < 600; i++ {
		p.updatePortals()
		if len(p.portals) == 0 {
			break
		}
		if p.portals[0].age > 1.9 && p.portals[0].age < 2.1 {
			if p.portals[0].rotationSpeed >= 1.2 || p.portals[0].rotationSpeed < 1.0 {
				t.Fatalf("rotation speed at age %.2f = %.3f, want lerping toward 1.0", p.portals[0].age, p.portals[0].rotationSpeed)
			}
			return
		}
	}
	t.Fatal("portal never reached age 2")
}

func TestPickupDropLandsAfterNativeFall(t *testing.T) {
	p := &playState{scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "pickup", texture: "p_shotgun", drop: true, lift: pickupDropHeight}}}
	e := p.scriptEntities[1]
	frames := 0
	for ; frames < 200 && !e.landed; frames++ {
		p.stepPickupDrops(1.0 / 60)
	}
	want := int(math.Ceil(math.Sqrt(2*pickupDropHeight/pickupGravity) * 60))
	if frames < want-2 || frames > want+2 {
		t.Fatalf("first landing after %d frames, want about %d", frames, want)
	}
	if e.liftVel <= 0 {
		t.Fatalf("bounce speed %v, want upward", e.liftVel)
	}
	if e.lift != 0 {
		t.Fatalf("lift on landing = %v", e.lift)
	}
}

func TestPickupNotCollectedWhileFalling(t *testing.T) {
	p := &playState{x: 0, y: 0, weapons: pickupTestWeapons(), scriptEntities: map[int]*scriptEntity{1: {id: 1, kind: "pickup", texture: "p_grenade", drop: true, lift: pickupDropHeight}}}
	p.stepPickupDrops(1.0 / 60)
	p.updatePickups()
	if p.scriptEntities[1] == nil {
		t.Fatal("pickup collected while falling")
	}
	for i := 0; i < 120 && p.scriptEntities[1] != nil; i++ {
		p.stepPickupDrops(1.0 / 60)
		p.updatePickups()
	}
	if p.scriptEntities[1] != nil {
		t.Fatal("pickup never collected after landing")
	}
}

func TestFlameDirectionDecaysAndRises(t *testing.T) {
	proj, ok := weapons.NewGibFlame(0, 0, 0, 20, nil)
	if ok {
		t.Fatal("flame without rng must fail")
	}
	rng := weapons.NewNativeRNG()
	proj, ok = weapons.NewGibFlame(0, 0, 0, 20, &rng)
	if !ok || proj.EntityType != 0x16 || proj.WeaponType != 5 {
		t.Fatalf("gib flame projectile = %+v", proj)
	}
	p := &playState{}
	b := bullet{vx: 100, vy: 0, life: 1, projectile: &proj}
	b.projectile.Age = 0
	lift := b.projectile.Lift
	p.stepFlameFlight(&b)
	if math.Abs(b.vx-97.5) > 1e-4 {
		t.Fatalf("vx after one update = %v, want 97.5", b.vx)
	}
	if b.projectile.Lift != lift {
		t.Fatalf("lift moved on the first update: %v", b.projectile.Lift)
	}
	b.projectile.Age = 1.0 / 60
	p.stepFlameFlight(&b)
	if b.projectile.Lift <= lift {
		t.Fatalf("lift did not rise on the second update: %v", b.projectile.Lift)
	}
}

func TestFlameHitsEveryThirdUpdate(t *testing.T) {
	proj := weapons.NativeWeaponProjectile{EntityType: 0x16}
	live := 0
	for n := 1; n <= 9; n++ {
		proj.Age = float64(n) / 60
		if flameLive(bullet{projectile: &proj}) {
			live++
			if (n-1)%3 != 0 {
				t.Fatalf("update %d live", n)
			}
		}
	}
	if live != 3 {
		t.Fatalf("live updates in 9 = %d, want 3", live)
	}
}

func TestStoppedFlameGrows(t *testing.T) {
	proj := weapons.NativeWeaponProjectile{EntityType: 0x16, Width: 20, Height: -20, Age: 1.0 / 60}
	b := bullet{projectile: &proj}
	p := &playState{}
	p.growFlame(&b)
	if proj.Width <= 20 {
		t.Fatalf("stopped flame width %v did not grow", proj.Width)
	}
	proj2 := weapons.NativeWeaponProjectile{EntityType: 0x16, Width: 20, Height: -20, Age: 1.0 / 60}
	moving := bullet{vx: 100, projectile: &proj2}
	p.growFlame(&moving)
	if proj2.Width != 20 {
		t.Fatalf("moving flame width changed to %v", proj2.Width)
	}
}

func TestGibZombieSpawnsHostileFlame(t *testing.T) {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng}
	z := &zombieState{x: 50, y: 60, dying: true, deathState: zombieDeathGib, deathDelay: 1.5}
	z.native.sizeZ = 60
	p.stepGibFlames(z)
	if len(p.bullets) != 1 {
		t.Fatalf("flames after first gib frame = %d, want 1 (countdown starts at 0)", len(p.bullets))
	}
	f := p.bullets[0]
	if !f.hostile || f.projectile.EntityType != 0x16 || f.projectile.Lift != float64(float32(.35)*float32(60)) {
		t.Fatalf("flame = %+v lift %v", f, f.projectile.Lift)
	}
	if p.gibFlameCountdown < 50 || p.gibFlameCountdown >= 70 {
		t.Fatalf("countdown = %d, want 50..69", p.gibFlameCountdown)
	}
	for i := 0; i < 49; i++ {
		p.stepGibFlames(z)
	}
	if len(p.bullets) != 1 {
		t.Fatalf("flames after 49 more frames = %d, want 1", len(p.bullets))
	}
}

func TestGibFlameHurtsPlayerOnLiveUpdateOnly(t *testing.T) {
	proj := weapons.NativeWeaponProjectile{EntityType: 0x16, Width: 20, Age: 1.0 / 60}
	p := &playState{x: 0, y: 0, health: 1}
	p.flameHitPlayers(bullet{x: 0, y: 0, projectile: &proj, hostile: true})
	if p.health >= 1 {
		t.Fatalf("health = %v, want damage", p.health)
	}
}

func TestSnapshotCarriesGibStateAndPickupLift(t *testing.T) {
	host := coopTestPlay(t)
	guest := coopTestPlay(t)
	host.zombies = []zombieState{{x: 10, y: 20, texture: "Zombie/a", animation: "walk", dying: true, deathState: zombieDeathGib, deathAge: .3}}
	host.gibBodies = []zombieState{{x: 30, y: 40, texture: "Zombie/a", animation: "walk", gibbed: true, presentAge: .25, deathState: zombieDeathGib}}
	host.spawnPickup("p_shotgun", formats.Vec2{X: 70, Y: 80})
	for _, e := range host.scriptEntities {
		if e.kind == "pickup" {
			e.lift = 123.4
		}
	}
	raw := encodeWire(wireMsg{T: "snap", Snap: host.snapshot(1)})
	msg, ok := decodeWire(raw)
	if !ok || msg.Snap == nil {
		t.Fatal("snapshot did not decode")
	}
	guest.applySnapshot(msg.Snap)
	if len(guest.zombies) != 1 || guest.zombies[0].deathState != zombieDeathGib {
		t.Fatalf("guest zombie death state lost: %+v", guest.zombies)
	}
	if len(guest.gibBodies) != 1 || !guest.gibBodies[0].gibbed || guest.gibBodies[0].presentAge != .25 {
		t.Fatalf("guest gib bodies: %+v", guest.gibBodies)
	}
	found := false
	for _, e := range guest.scriptEntities {
		if e.kind == "pickup" && math.Abs(e.lift-123.4) < .01 {
			found = true
		}
	}
	if !found {
		t.Fatal("pickup lift not carried")
	}
}

// Shotgun: aabdc (v7 vtable header 0x005bb714) / 0x0010fe24 (1.2.5) store 7 in the class
// before any fire; the loop of FUN_000abac0 / FUN_0011051c reads it and draws
// Bounded(2 * spread) per pellet.
func TestShotgunVolleyIsSevenPelletsWithOwnSpreadDraws(t *testing.T) {
	rng := weapons.NewNativeRNG()
	w := formats.Weapon{GunType: "SHOTGUN", Ammo: 16, SpreadUnits: 19 * 182, Life: .25, Speed: 800}
	volley, err := weapons.NativePrimaryVolley(w, 1000, &rng)
	if err != nil {
		t.Fatal(err)
	}
	if len(volley.Shots) != 7 || volley.AmmoConsumed != 1 {
		t.Fatalf("shots %d ammo %d, want 7 pellets for one round", len(volley.Shots), volley.AmmoConsumed)
	}
	distinct := map[uint16]bool{}
	for _, shot := range volley.Shots {
		distinct[shot.Direction] = true
		d := int(int16(shot.Direction - 1000))
		if d < -19*182 || d >= 19*182 {
			t.Fatalf("pellet direction %d outside +-spread", shot.Direction)
		}
	}
	if len(distinct) < 4 {
		t.Fatalf("only %d distinct pellet headings", len(distinct))
	}
}
