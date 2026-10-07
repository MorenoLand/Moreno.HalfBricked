package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"testing"
)

func TestShotgunPickupFiresNativeVolleyAndSound(t *testing.T) {
	w := formats.Weapon{GunType: "SHOTGUN", BulletType: "NORMAL", Ammo: 16, Speed: 800, Life: .25, RateOfFire: .7, SpreadUnits: 19 * 182, SFXStart: "SFX_SHOTGUN_1", SFXEnd: "SFX_SHOTGUN_2"}
	p := &playState{weapons: formats.WeaponCatalog{w}}
	p.collectPickup("p_shotgun")
	if !p.fire(1, 0) || len(p.bullets) != 7 || p.weapon.Ammo != 15 || p.shotSound != "SFX_SHOTGUN_1" {
		t.Fatalf("volley=%d ammo=%d sound=%s", len(p.bullets), p.weapon.Ammo, p.shotSound)
	}
	if p.bullets[0].vy == p.bullets[1].vy {
		t.Fatal("pellets have no native spread")
	}
}
