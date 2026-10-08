package game

import (
	"strings"
	"testing"
)

// Every sound a weapon record names must resolve to a real clip, so no gun is silent.
func TestEveryWeaponSoundResolvesToAClip(t *testing.T) {
	host := script125CachedHost(t, "world0_level1")
	a := host.app
	files := a.pack.Manifest().Files
	exists := func(path string) bool {
		for key, value := range files {
			if strings.EqualFold(value, path) || strings.EqualFold(key, path) {
				return true
			}
		}
		return false
	}
	for _, gun := range []string{"BUZZSAW", "DUALPISTOL"} {
		if _, ok := a.weapons.Find(gun); !ok {
			t.Fatalf("%s is missing from the weapon catalog", gun)
		}
	}
	for _, weapon := range a.weapons {
		for label, symbol := range map[string]string{"start": weapon.SFXStart, "shoot": weapon.SFXShoot, "end": weapon.SFXEnd} {
			if symbol == "" || symbol == "0" {
				continue
			}
			if path := a.scriptSoundPath(symbol); !exists(path) {
				t.Errorf("%s %s %q resolves to %q, which is not in the pack", weapon.GunType, label, symbol, path)
			}
		}
	}
	for _, symbol := range []string{"SFX_BUZZSAW_HIT_1", "SFX_BUZZSAW_HIT_2", "SFX_BUZZSAW_HIT_3"} {
		if path := a.scriptSoundPath(symbol); !exists(path) {
			t.Errorf("%s resolves to %q, which is not in the pack", symbol, path)
		}
	}
}
