package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
	"testing"
)

func TestPrimaryShotUsesNativeSpawnPointAndRenderLift(t *testing.T) {
	p := &playState{x: 100, y: 100, weapon: formats.Weapon{Speed: 600, Life: .75, RateOfFire: .25}}
	if !p.fire(1, 0) || len(p.bullets) != 1 {
		t.Fatal("primary shot was not created")
	}
	b := p.bullets[0]
	if b.x != 127 || b.y != 111 || b.y-nativeProjectileRenderAnchor != 91 {
		t.Fatalf("primary spawn/render = (%g,%g)/%g, want (127,111)/91", b.x, b.y, b.y-nativeProjectileRenderAnchor)
	}
	distance := math.Hypot(69, -11)
	if math.Abs(b.vx-600*69/distance) > 1e-9 || math.Abs(b.vy-600*-11/distance) > 1e-9 {
		t.Fatalf("primary velocity = (%g,%g), want native 96-unit aim convergence", b.vx, b.vy)
	}
	if p.flash != .16 || p.shootCooldown != .25 {
		t.Fatalf("primary timers = %g/%g, want .16/.25", p.flash, p.shootCooldown)
	}
}
func TestSecondaryShotUsesNativeForwardPoint(t *testing.T) {
	p := &playState{x: 100, y: 100, grenades: 2, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Speed: 350, Life: 1, RateOfFire: .25}}}
	if !p.fireSecondary(0, 1) || len(p.thrown) != 1 {
		t.Fatal("secondary shot was not created")
	}
	b := p.thrown[0]
	if b.x != 94 || b.y != 124 || p.grenades != 1 {
		t.Fatalf("secondary = %#v, ammo %d, want native (94,124) and one consumed", b, p.grenades)
	}
	distance := math.Hypot(6, 72)
	if math.Abs(b.dirX*b.speed-350*6/distance) > 1e-9 || math.Abs(b.dirY*b.speed-350*72/distance) > 1e-9 {
		t.Fatalf("secondary velocity = (%g,%g), want forward to (100,196)", b.dirX*b.speed, b.dirY*b.speed)
	}
}
func TestActiveStickRetainsInputDirection(t *testing.T) {
	p := &playState{stick: 2}
	dx, dy := p.projectileDirection(0, 1, -6, 24)
	if dx != 0 || dy != 1 {
		t.Fatalf("stick direction = (%g,%g), want (0,1)", dx, dy)
	}
}
func TestGrenadeSlotKeepsIndependentCooldown(t *testing.T) {
	p := &playState{grenades: 1, shootCooldown: .125, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Speed: 350, Life: 1, RateOfFire: .5}}}
	if !p.fireSecondary(0, 1) {
		t.Fatal("secondary slot failed while the primary slot was cooling down")
	}
	if p.shootCooldown != .125 || p.secondaryShootCooldown != .5 {
		t.Fatalf("slot cooldowns = %g/%g, want independent .125/.5", p.shootCooldown, p.secondaryShootCooldown)
	}
}

func TestBuzzsawFansThreeBladesPerAmmoAndDualPistolFiresOneMulti(t *testing.T) {
	rng := weapons.NewNativeRNG()
	saw := formats.Weapon{GunType: "BUZZSAW", Ammo: 10, SpreadUnits: 6370, Life: .1, Speed: 300}
	volley, err := weapons.NativePrimaryVolley(saw, 1000, &rng)
	if err != nil || len(volley.Shots) != 3 || volley.AmmoConsumed != 1 {
		t.Fatalf("buzzsaw volley %+v err %v", volley, err)
	}
	if volley.Shots[2].Direction != 1000+6370 || volley.Shots[1].Direction != 1000 {
		t.Fatalf("the middle blade should fly straight and the last one +spread")
	}
	dual := formats.Weapon{GunType: "DUALPISTOL", Ammo: 24, Life: .75, Speed: 700}
	volley, err = weapons.NativePrimaryVolley(dual, 0, &rng)
	if err != nil || len(volley.Shots) != 1 || volley.AmmoConsumed != 1 || volley.Shots[0].WeaponType != 7 {
		t.Fatalf("dual pistol volley %+v err %v", volley, err)
	}
}
