package formats

import (
	"strings"
	"testing"
)

func TestParseZombieWeaponsUsesNativeUnits(t *testing.T) {
	catalog, err := ParseZombieWeapons(strings.NewReader(`<WeaponData><Weapon><Gun_Type>PISTOL</Gun_Type><Recoil>250</Recoil><Rate_Of_Fire>250</Rate_Of_Fire><Spread>19</Spread><Ammo>16</Ammo><Ammo_Per_Shot>7</Ammo_Per_Shot><Life>0.25</Life><Speed>800</Speed><Bullet_Type>NORMAL</Bullet_Type></Weapon></WeaponData>`))
	if err != nil || len(catalog) != 1 {
		t.Fatalf("ParseZombieWeapons() = %#v, %v", catalog, err)
	}
	weapon := catalog[0]
	if weapon.GunType != "PISTOL" || weapon.RecoilSeconds != .25 || weapon.RateOfFire != .25 || weapon.SpreadUnits != 3458 || weapon.Ammo != 16 || weapon.AmmoPerShot != 7 || weapon.Life != .25 || weapon.Speed != 800 || weapon.BulletType != "NORMAL" {
		t.Fatalf("parsed zombie weapon = %#v", weapon)
	}
}
