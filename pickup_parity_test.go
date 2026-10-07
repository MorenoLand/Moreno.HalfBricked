package main

import (
	"image"
	"os"
	"testing"
)

func TestPickupParityInitializerBindings(t *testing.T) {
	for _, v := range []struct {
		name       string
		kind, cell int
		secondary  bool
	}{{"p_shotgun", 0x26, 1, false}, {"p_uzi", 0x27, 2, false}, {"p_dual_pistol", 0x2c, 7, false}, {"p_grenade", 0x2d, 0, true}, {"p_mine", 0x2e, 1, true}, {"p_bazooka", 0x2f, 2, true}, {"p_sentry", 0x30, 3, true}, {"p_cow_pat", 0x31, 4, true}} {
		got, ok := pickupParityBinding(v.name)
		if !ok || got.Type != v.kind || got.Subtype != v.kind-0x25 || got.Cell != v.cell || got.Secondary != v.secondary {
			t.Fatalf("%s: %+v %v", v.name, got, ok)
		}
	}
	if _, ok := pickupParityBinding("p_rand_1"); ok {
		t.Fatal("random pickup requires resolved runtime subtype")
	}
}
func TestPickupParityCropAndGeometry(t *testing.T) {
	got, ok := pickupParityCell(image.Rect(4, 8, 260, 40), 2)
	if !ok || got != image.Rect(68, 8, 100, 40) {
		t.Fatalf("crop %v %v", got, ok)
	}
	for _, size := range []float32{50, 80} {
		x, y, w, h := pickupParityGeometry(100, 200, size, size, 5)
		if x != 100 || y != 195-size*0.375 || w != size || h != -size {
			t.Fatalf("geometry %v %v %v %v", x, y, w, h)
		}
	}
}
func TestPickupParityVoiceAssets(t *testing.T) {
	for _, symbol := range []string{"SFX_VO_SHOTGUN", "SFX_VO_SMG", "SFX_VO_MINIGUN", "SFX_VO_RIFLE", "SFX_VO_FLAMETHROWER", "SFX_VO_GRENADES", "SFX_VO_MINES", "SFX_VO_BAZOOKA"} {
		path, ok := pickupParityVoice("VO_Caveman", symbol)
		if !ok {
			t.Fatal(symbol)
		}
		if _, err := os.Stat("bin/data-cache/" + path); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	path, ok := pickupParityVoice("VO_Caveman", "SFX_VO_SMG")
	if !ok || path != "audio/sound/sfx/VO_Caveman_SMG.ogg" {
		t.Fatalf("voice %s %v", path, ok)
	}
	if _, ok := pickupParityVoice("VO_Caveman", "0"); ok {
		t.Fatal("sentinel is not a voice")
	}
}
