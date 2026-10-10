package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"reflect"
	"testing"
)

func TestNewPickupCollectionPreservesCatalog(t *testing.T) {
	for _, kind := range []string{"SHOTGUN", "UZI", "MINIGUN", "SNIPER", "FLAMER"} {
		weapon := formats.Weapon{GunType: kind, Ammo: 16, Spread: 12, SFXShoot: "original", TextureGun: "original"}
		p := playState{weapons: formats.WeaponCatalog{weapon}}
		p.collectPickup("p_" + kind)
		if !reflect.DeepEqual(p.weapon, weapon) {
			t.Fatalf("%s lost catalog data", kind)
		}
	}
	p := playState{health: 1, maxHealth: 5, lives: 3, grenades: 2, weapons: formats.WeaponCatalog{{GunType: "GRENADE", Ammo: 5}}}
	p.collectPickup("p_grenade")
	p.collectPickup("p_health")
	// p_health is the native 1UP (FUN_000928f4 type 0x23: lives + 1), not a heal.
	if p.grenades != 5 || p.health != 1 || p.lives != 4 {
		t.Fatalf("grenades %d health %v lives %d", p.grenades, p.health, p.lives)
	}
}
func TestNewPickupNativeCellsAndDimensions(t *testing.T) {
	for i, name := range []string{"p_shotgun", "p_uzi", "p_minigun", "p_sniper", "p_flamer", "p_buzzsaw", "p_dual_pistol"} {
		cell, ok := pickupPrimaryCell(name)
		if !ok || cell != i+1 || pickupDrawSize(name) != 50 {
			t.Fatalf("%s: cell %d known %v size %v", name, cell, ok, pickupDrawSize(name))
		}
	}
	if _, ok := pickupPrimaryCell("p_pistol"); ok {
		t.Fatal("unregistered pickup")
	}
	for _, name := range []string{"p_cow_pat", "p_bazooka", "p_sentry"} {
		if pickupDrawSize(name) != 80 {
			t.Fatal(name)
		}
	}
}
