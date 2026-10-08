package weapons

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"testing"
)

func TestSecondaryWeaponDispatch(t *testing.T) {
	for cell, kind := range []string{"GRENADE", "MINE", "BAZOOKA"} {
		binding, ok := nativeSecondaryWeaponBinding(kind)
		if !ok || binding.GunType != kind || binding.IconCell != cell {
			t.Fatalf("%s: %+v, %v", kind, binding, ok)
		}
	}
	if _, ok := nativeSecondaryWeaponBinding("SHOTGUN"); ok {
		t.Fatal("primary accepted as secondary")
	}
}
func TestSecondaryPickupReplacesAmmoPool(t *testing.T) {
	s := nativeSecondaryInventory{}
	for _, weapon := range []formats.Weapon{{GunType: "GRENADE", Ammo: 5}, {GunType: "MINE", Ammo: 3}, {GunType: "MINE", Ammo: 3}, {GunType: "BAZOOKA", Ammo: 5}} {
		if err := s.equip(weapon); err != nil {
			t.Fatal(err)
		}
		if s.Ammo != weapon.Ammo || s.Weapon.GunType != weapon.GunType {
			t.Fatalf("pool did not replace: %+v", s)
		}
		if _, ok := s.discharge(0); !ok {
			t.Fatal("failed discharge")
		}
	}
	before := s
	if err := s.equip(formats.Weapon{GunType: "SHOTGUN", Ammo: 25}); err == nil {
		t.Fatal("primary accepted")
	}
	if s.Ammo != before.Ammo || s.Weapon.GunType != before.Weapon.GunType {
		t.Fatal("rejected pickup changed inventory")
	}
}
func TestSecondaryDischargeKeepsWeaponConfiguration(t *testing.T) {
	weapon := formats.Weapon{GunType: "BAZOOKA", Ammo: 1, RateOfFire: .5, Life: 1, Speed: 600, BulletType: "BAZOOKA", SFXShoot: "SFX_ROCKET_LAUNCH", SFXVO: "SFX_VO_BAZOOKA"}
	s := nativeSecondaryInventory{}
	if err := s.equip(weapon); err != nil {
		t.Fatal(err)
	}
	shot, ok := s.discharge(0xc123)
	if !ok || shot.Direction != 0xc123 || shot.AmmoConsumed != 1 || shot.Weapon.Life != 1 || shot.Weapon.Speed != 600 || shot.Weapon.BulletType != "BAZOOKA" || shot.Weapon.SFXShoot != weapon.SFXShoot || shot.Weapon.SFXVO != weapon.SFXVO || s.Ammo != 0 {
		t.Fatalf("configuration lost: %+v inventory=%+v", shot, s)
	}
	if _, ok := s.discharge(0); ok || s.Ammo != 0 {
		t.Fatal("empty slot fired")
	}
}
func TestSecondaryEmptyInventoryCannotFire(t *testing.T) {
	s := nativeSecondaryInventory{}
	if _, ok := s.discharge(0); ok {
		t.Fatal("empty slot fired")
	}
	s.Ammo = 5
	if _, ok := s.discharge(0); ok || s.Ammo != 5 {
		t.Fatal("unresolved slot fired or consumed ammo")
	}
}
func TestSecondaryMineNativeLifetime(t *testing.T) {
	p, ok := newNativeSecondaryProjectile(nativeSecondaryDischarge{Weapon: formats.Weapon{GunType: "MINE", BulletType: "MINE", Life: 1.5, Speed: 0}}, 100, 120)
	if !ok || p.EntityType != 0x15 || p.WeaponType != 9 || p.Width != 40 || p.Height != -40 || p.Lift != 0 || p.Direction != 0xbff4 {
		t.Fatalf("mine initialization: %+v", p)
	}
	if damage := p.targetContact(); damage != 0 {
		t.Fatalf("waiting mine damaged target: %d", damage)
	}
	if fired, removed := p.update(1); fired || removed || p.X != 100 || p.Y != 120 {
		t.Fatalf("waiting mine moved or exploded: %+v", p)
	}
	if fired, removed := p.update(.5); !fired || removed || p.State != 1 || p.Age != 0 {
		t.Fatalf("mine did not detonate at Life: %+v", p)
	}
	if damage := p.targetContact(); damage != 5 {
		t.Fatalf("explosion damage: %d", damage)
	}
	draw := p.drawGeometry()
	if draw.Texture != NativeGrenadeExplosionTexture || draw.Width != 120 || draw.Height != -240 || draw.Y != 50 {
		t.Fatalf("native mine explosion draw: %+v", draw)
	}
	p.update(.75)
	if p.State != 1 {
		t.Fatal("native strict expiration comparison lost")
	}
	p.update(.001)
	if p.State != 4 {
		t.Fatal("explosion did not finish")
	}
	if _, removed := p.update(.5); removed {
		t.Fatal("removed before native retirement threshold")
	}
	if _, removed := p.update(.001); !removed {
		t.Fatal("did not retire")
	}
}
func TestSecondaryBazookaNativeInitializationAndMotion(t *testing.T) {
	p, ok := newNativeSecondaryProjectile(nativeSecondaryDischarge{Weapon: formats.Weapon{GunType: "BAZOOKA", BulletType: "BAZOOKA", Life: 1, Speed: 600}, Direction: 0}, 100, 120)
	if !ok || p.EntityType != 0x13 || p.WeaponType != 10 || p.Life != 1 || p.Age != .5 || p.State != 0 || p.Width != 14 || p.Height != -28 || p.Texture != "Common0/Textures/bazooka_SD" {
		t.Fatalf("bazooka initialization: %+v", p)
	}
	if fired, removed := p.update(.1); fired || removed || p.X != 160 || p.Y != 120 || p.FallSpeed != 0 {
		t.Fatalf("bazooka motion: %+v", p)
	}
	if p.targetContact() != 5 || p.State != 1 || p.Speed != 600 {
		t.Fatal("rocket target contact should damage and detonate")
	}
	p.State, p.Lift, p.FallSpeed = 0, -1, 100
	p.groundContact()
	if p.State != 1 || p.FallSpeed != 0 {
		t.Fatalf("ground detonation: %+v", p)
	}
	p.State, p.Age = 0, .5
	if fired, _ := p.update(.5); !fired {
		t.Fatal("rocket did not detonate at native lifetime")
	}
}
func TestSecondaryNativeTargetEllipse(t *testing.T) {
	p := nativeSecondaryProjectile{NativeWeaponProjectile: NativeWeaponProjectile{X: 100, Y: 100, Width: 40}}
	if !p.touchesTarget(100, 120, 20) || p.touchesTarget(100, 140, 20) || p.touchesTarget(160, 100, 20) {
		t.Fatal("native target ellipse not respected")
	}
}
func TestSecondarySameTypeGrenadePickupReplaces(t *testing.T) {
	s := nativeSecondaryInventory{}
	weapon := formats.Weapon{GunType: "GRENADE", Ammo: 5}
	if err := s.equip(weapon); err != nil {
		t.Fatal(err)
	}
	s.discharge(0)
	if err := s.equip(weapon); err != nil {
		t.Fatal(err)
	}
	if s.Ammo != 5 {
		t.Fatalf("same-type pickup added ammo: %d", s.Ammo)
	}
}
