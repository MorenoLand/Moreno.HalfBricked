package game

import "testing"

// A primary crate's icon must name the weapon it gives.
func TestPrimaryPickupIconMatchesTheWeaponItGives(t *testing.T) {
	want := map[string]string{"p_shotgun": "SHOTGUN", "p_uzi": "UZI", "p_minigun": "MINIGUN", "p_sniper": "SNIPER", "p_flamer": "FLAMER", "p_buzzsaw": "BUZZSAW", "p_dual_pistol": "DUALPISTOL"}
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	for name, gun := range want {
		if _, ok := pickupPrimaryCell(name); !ok {
			t.Fatalf("%s has no icon", name)
		}
		p.collectPickup(name)
		if p.weapon.GunType != gun {
			t.Errorf("%s gave %s, want %s", name, p.weapon.GunType, gun)
		}
		cell, _ := primaryWeaponIconCell(p.weapon)
		crateCell, _ := pickupPrimaryCell(name)
		if cell != crateCell {
			t.Errorf("%s: the crate shows cell %d but the weapon button shows cell %d", name, crateCell, cell)
		}
	}
}
