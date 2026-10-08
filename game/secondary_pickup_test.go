package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestSecondaryGrenadeActualPickupReplacesPool(t *testing.T) {
	primary := formats.Weapon{GunType: "SHOTGUN", Ammo: 16}
	p := playState{weapon: primary, grenades: 2, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Ammo: 5}}}
	p.collectPickup("p_grenade")
	if p.grenades != 5 || p.weapon.GunType != primary.GunType || p.weapon.Ammo != primary.Ammo {
		t.Fatalf("pickup changed wrong inventory: grenades=%d primary=%+v", p.grenades, p.weapon)
	}
	p.grenades--
	p.collectPickup("p_grenade")
	if p.grenades != 5 {
		t.Fatalf("same-type pickup added ammo: %d", p.grenades)
	}
}
