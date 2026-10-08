package game

import "testing"

func TestRandomPickupsResolveToRealWeapons(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	primary := map[string]bool{}
	for _, name := range randomPrimaryPickups {
		primary[name] = true
	}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		resolved, ok := p.resolveRandomPickup("P_RAND_1")
		if !ok || !primary[resolved] {
			t.Fatalf("p_rand_1 resolved to %q (%v), want a primary weapon", resolved, ok)
		}
		seen[resolved] = true
	}
	if len(seen) < 3 {
		t.Fatalf("p_rand_1 barely random: %v", seen)
	}
	for i := 0; i < 50; i++ {
		resolved, ok := p.resolveRandomPickup("P_RAND_2")
		if _, _, secondary := secondaryForPickup(resolved); !ok || !secondary {
			t.Fatalf("p_rand_2 resolved to %q, want a secondary pickup", resolved)
		}
	}
	for _, name := range []string{"p_rand_1", "p_rand_2", "p_rand_3", "p_rand_4", "p_rand_5", "p_rand_all"} {
		p.weapon, p.secondaryType, p.grenades = p.weapons[0], "", 0
		p.collectPickup(name)
		if p.weapon.GunType == "PISTOL" && p.grenades == 0 {
			t.Fatalf("%s granted nothing", name)
		}
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
