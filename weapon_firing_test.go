package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"testing"
)

func TestNativeWeaponDirectionLookup(t *testing.T) {
	for _, tc := range []struct {
		x, y  float64
		angle uint16
	}{{1, 0, 0}, {1, 1, 0x2000}, {0, 1, 0x4000}, {-1, 1, 0x6000}, {-1, 0, 0x8000}, {-1, -1, 0xa000}, {0, -1, 0xc000}, {1, -1, 0xe000}, {128, 1, 81}, {1, 128, 0x4000 - 81}, {0, 0, 0}} {
		if got := nativeWeaponDirection(tc.x, tc.y); got != tc.angle {
			t.Fatalf("(%v,%v)=%d want %d", tc.x, tc.y, got, tc.angle)
		}
	}
}

func TestNativeWeaponVelocityBits(t *testing.T) {
	for _, tc := range []struct {
		index int
		bits  uint32
	}{{0, 0}, {1, 0x3ac90fca}, {128, 0x3e47c5b7}, {512, 0x3f3504eb}, {1024, 0x3f800000}, {2048, 0x36321480}, {3072, 0xbf800000}, {4095, 0xbac9c1df}} {
		_, y := nativeWeaponVelocity(uint16(tc.index<<4), 1)
		if got := math.Float32bits(float32(y)); got != tc.bits {
			t.Fatalf("index%d: %08x want%08x", tc.index, got, tc.bits)
		}
	}
	x, y := nativeWeaponVelocity(16, 600)
	x2, y2 := nativeWeaponVelocity(31, 600)
	if x != x2 || y != y2 {
		t.Fatal("low direction bits affected LUT velocity")
	}
}

func TestNativePrimaryVolleyShotgun(t *testing.T) {
	rng, expected := newNativeRNG(), newNativeRNG()
	w := formats.Weapon{GunType: "SHOTGUN", Ammo: 16, SpreadUnits: 19 * 182, Life: .25, Speed: 800}
	v, err := nativePrimaryVolley(w, 65000, &rng)
	if err != nil || len(v.Shots) != 7 || v.AmmoConsumed != 1 {
		t.Fatalf("volley=%+v err=%v", v, err)
	}
	for _, shot := range v.Shots {
		angle := uint16(uint32(65000) + expected.bounded(19*182*2) - 19*182)
		if shot.Direction != angle || shot.WeaponType != 1 || shot.Life != .25 || shot.Speed != 800 || shot.Lift != 20 {
			t.Fatalf("shot=%+v expected angle=%d", shot, angle)
		}
	}
	if rng.state != expected.state {
		t.Fatal("native RNG call count changed")
	}
}

func TestNativePrimaryVolleyAmmoAndDispatch(t *testing.T) {
	for _, name := range []string{"PISTOL", "SHOTGUN", "UZI", "MINIGUN", "SNIPER", "FLAMER"} {
		rng := newNativeRNG()
		v, err := nativePrimaryVolley(formats.Weapon{GunType: name}, 0, &rng)
		if err != nil {
			t.Fatal(err)
		}
		if name == "PISTOL" {
			if len(v.Shots) != 1 || v.AmmoConsumed != 0 {
				t.Fatal(v)
			}
		} else if len(v.Shots) != 0 {
			t.Fatal(v)
		}
	}
	if _, err := nativePrimaryVolley(formats.Weapon{GunType: "RPG", Ammo: 1}, 0, nil); err == nil {
		t.Fatal("invented RPG dispatch")
	}
	if _, err := nativePrimaryVolley(formats.Weapon{GunType: "UZI", Ammo: 1}, 0, nil); err == nil {
		t.Fatal("missing RNG accepted")
	}
}
