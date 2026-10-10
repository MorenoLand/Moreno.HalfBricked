package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// The native initialiser (0x00092da0 + FUN_000b188c) maps p_rand_N to the loaded
// weapons whose XML <Value> is N, p_rand_3 to the hover pickup and everything with
// no match to the shotgun.
func TestRandomPickupsFollowTheWeaponValueGroups(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	for name, want := range map[string]string{
		"P_RAND_1": "P_BUZZSAW", "P_RAND_2": "P_SENTRY", "P_RAND_3": "P_HOVER", "P_RAND_4": "P_COW_PAT",
		"P_RAND_5": "P_DUAL_PISTOL", "P_RAND_6": "P_SHOTGUN", "P_RAND_7": "P_SHOTGUN", "P_RAND_8": "P_SHOTGUN",
	} {
		for i := 0; i < 20; i++ {
			if got, ok := p.resolveRandomPickup(name); !ok || got != want {
				t.Fatalf("%s resolved to %q (%v), want %s", name, got, ok, want)
			}
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 300; i++ {
		got, ok := p.resolveRandomPickup("p_rand_all")
		if !ok {
			t.Fatal("p_rand_all unresolved")
		}
		seen[got] = true
	}
	for _, want := range []string{"P_BUZZSAW", "P_DUAL_PISTOL", "P_SENTRY", "P_COW_PAT"} {
		if !seen[want] {
			t.Fatalf("p_rand_all never gave %s: %v", want, seen)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("p_rand_all pool %v", seen)
	}
	if _, ok := p.resolveRandomPickup("p_shotgun"); ok {
		t.Fatal("a plain pickup is not random")
	}
}

// The crate on the floor is already the resolved pickup (the roll happens at spawn).
func TestRandomPickupIsRolledWhenItSpawns(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.scriptEntities = map[int]*scriptEntity{}
	p.spawnPickup("p_rand_1", formats.Vec2{X: 10, Y: 10})
	p.spawnPickup("p_rand_3", formats.Vec2{X: 20, Y: 10})
	got := map[string]bool{}
	for _, e := range p.scriptEntities {
		got[e.texture] = true
	}
	if !got["p_buzzsaw"] || !got["p_hover"] {
		t.Fatalf("spawned %v", got)
	}
}

// p_health is a 1UP (FUN_000928f4 type 0x23: lives + 1, text "Lives + 1"), p_hover
// the 2 s speed boost.
func TestHealthPickupIsAnExtraLifeAndHoverIsASpeedBoost(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.health, p.lives = .4, 2
	p.collectPickup("p_health")
	if p.lives != 3 || p.health != .4 {
		t.Fatalf("p_health: lives %d health %v", p.lives, p.health)
	}
	if p.vitals.walkSpeedFactor() != 1 {
		t.Fatal("no boost before the pickup")
	}
	p.collectPickup("p_rand_3")
	if f := p.vitals.walkSpeedFactor(); f != 2 {
		t.Fatalf("boost factor %v", f)
	}
	for i := 0; i < 60*3; i++ {
		p.stepVitals(1.0 / 60)
	}
	if p.vitals.walkSpeedFactor() != 1 {
		t.Fatal("boost must wear off")
	}
}

func TestPickupReachMatchesTheNativeDistanceTest(t *testing.T) {
	if r := pickupReach("p_shotgun"); math.Abs(r-27.95) > .01 {
		t.Fatalf("reach %v", r)
	}
	if r := pickupReach("p_bazooka"); math.Abs(r-44.72) > .01 {
		t.Fatalf("special crate reach %v", r)
	}
}

func TestBuzzsawAndDualPistolPickupsAreEquippable(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	for pickup, gun := range map[string]string{"p_buzzsaw": "BUZZSAW", "p_dual_pistol": "DUALPISTOL"} {
		p.collectPickup(pickup)
		if p.weapon.GunType != gun {
			t.Fatalf("%s equipped %q, want %s", pickup, p.weapon.GunType, gun)
		}
	}
}
