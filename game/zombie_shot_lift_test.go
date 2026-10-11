package game

import (
	"math"
	"testing"
)

// The zombie bullet keeps the player bullet's 10 x -20 size; 0.46875 * body height is its visual lift, which falls
// at (lift - 23.6) / (distance / speed) per second (FUN_0009f7e4 -> FUN_000a4868 +0x78 / +0x7c).
func TestZombieShotIsNormalSizedAndFallsToTheTarget(t *testing.T) {
	fall := zombieShotFall(30, 600, 120)
	if want := (30 - 23.6) / (120.0 / 600.0); math.Abs(fall-want) > 1e-3 {
		t.Fatalf("fall %v, want %v", fall, want)
	}
	if zombieShotFall(20, 600, 120) >= 0 {
		t.Fatal("a bullet fired below the floor height must rise, not fall")
	}
}

func TestZombieGunSoundTable(t *testing.T) {
	for _, gun := range []string{"PISTOL", "SHOTGUN", "UZI", "MINIGUN", "SNIPER"} {
		if zombieGunSounds[gun] == "" {
			t.Fatalf("no fire sound for %s", gun)
		}
	}
}
