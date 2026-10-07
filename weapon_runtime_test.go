package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"testing"
)

func TestPrimaryPickupSelectsNativeProjectileAndConsumesAmmo(t *testing.T) {
	for _, item := range []struct {
		gun, bullet string
		entity      uint8
	}{{"UZI", "NORMAL", 0x10}, {"MINIGUN", "NORMAL", 0x10}, {"SNIPER", "MULTI", 0x12}, {"FLAMER", "FLAME", 0x16}} {
		w := formats.Weapon{GunType: item.gun, BulletType: item.bullet, Ammo: 2, Speed: 600, Life: 1, RateOfFire: .1}
		p := &playState{weapons: formats.WeaponCatalog{w}}
		p.collectPickup("p_" + item.gun)
		if item.gun == "MINIGUN" {
			p.spinAudio.SpinTick(weaponBindings(w), true, .1, false)
		}
		if !p.fire(1, 0) || len(p.bullets) != 1 || p.bullets[0].projectile == nil || p.bullets[0].projectile.EntityType != item.entity || p.weapon.Ammo != 1 {
			t.Fatalf("%s did not use native projectile/ammo", item.gun)
		}
	}
}
func TestEmptyPrimaryReturnsToOriginalPistol(t *testing.T) {
	p := &playState{weapon: formats.Weapon{GunType: "SHOTGUN", BulletType: "NORMAL", Speed: 800, Life: .25, RateOfFire: .7}, weapons: formats.WeaponCatalog{{GunType: "PISTOL", BulletType: "NORMAL", Speed: 600, Life: .75, RateOfFire: .25, Ammo: 99999999}}}
	if !p.fire(1, 0) || p.weapon.GunType != "PISTOL" || len(p.bullets) != 1 {
		t.Fatal("empty primary did not return to pistol")
	}
}
func TestPickupQueuesOriginalBarryVoice(t *testing.T) {
	p := &playState{levelInfo: formats.LevelInfo{VoiceoverPrefix: "VO_Caveman"}, weapons: formats.WeaponCatalog{{GunType: "SHOTGUN", SFXVO: "SFX_VO_SHOTGUN"}, {GunType: "UZI", SFXVO: "SFX_VO_SMG"}}}
	p.collectPickup("p_shotgun")
	p.collectPickup("p_uzi")
	if len(p.pickupVoices) != 2 || p.pickupVoices[0] != "audio/sound/sfx/VO_Caveman_Shotgun.ogg" || p.pickupVoices[1] != "audio/sound/sfx/VO_Caveman_SMG.ogg" {
		t.Fatal("pickup lost original VO bindings")
	}
}
func TestMinigunRequiresNativeTimerAndResetsAfterShot(t *testing.T) {
	w := formats.Weapon{GunType: "MINIGUN", BulletType: "NORMAL", Ammo: 2, Speed: 600, Life: .75, RateOfFire: .03, SFXStart: "SFX_MINIGUN_SPIN_UP", SFXShoot: "SFX_MINIGUN", SFXEnd: "SFX_MINIGUN_SPIN_DOWN"}
	p := &playState{}
	p.equipWeapon(w)
	if p.fire(1, 0) {
		t.Fatal("fresh minigun bypassed eligibility")
	}
	p.spinEvents.Add(p.spinAudio.SpinTick(weaponBindings(w), true, .02, false))
	if p.fire(1, 0) {
		t.Fatal("minigun fired before rate threshold")
	}
	p.spinEvents.Add(p.spinAudio.SpinTick(weaponBindings(w), true, .02, false))
	if !p.fire(1, 0) || p.spinAudio.SpinTimer != 0 || p.weapon.Ammo != 1 {
		t.Fatal("native minigun timer/ammo not applied")
	}
	p.spinEvents.Add(p.spinAudio.SpinTick(weaponBindings(w), false, .02, false))
	events := p.spinEvents.Drain()
	if len(events) < 3 || events[len(events)-1].Sound != w.SFXEnd {
		t.Fatal("release did not schedule spin-down")
	}
}
