package weapons

import (
	"math"
	"testing"
)

func TestBuzzsawFiresEveryFrameOnceAttachedAndNeverResetsItsTimer(t *testing.T) {
	var g GunVisual
	g.AttachBuzzsaw(0.03)
	ammo := 700
	for frame := 0; frame < 700; frame++ {
		if !g.BuzzsawStep(1.0/60, 0.03, ammo, 700) {
			t.Fatalf("frame %d: the saw did not fire (timer %v)", frame, g.Timer)
		}
		ammo--
	}
	if g.BuzzsawStep(1.0/60, 0.03, 0, 700) {
		t.Fatal("the saw fired without ammo")
	}
	if g.Timer < 700.0/60 {
		t.Fatalf("timer %v was reset; 0x000a9f0c never writes it", g.Timer)
	}
}

func TestBuzzsawSpinPhaseCyclesBetweenZeroAnd101(t *testing.T) {
	var g GunVisual
	g.AttachBuzzsaw(0.03)
	g.BuzzsawStep(1.0/60, 0.03, 700, 700)
	if want := 101 - 1000.0/60; math.Abs(float64(g.Spin)-want) > 1e-3 {
		t.Fatalf("first phase %v, want %v (0 - dt*1000 wraps to 101 - 16.67)", g.Spin, want)
	}
	seen := map[bool]int{}
	for i := 0; i < 600; i++ {
		g.BuzzsawStep(1.0/60, 0.03, 700, 700)
		if g.Spin < 0 || g.Spin > 101 {
			t.Fatalf("phase %v left 0..101", g.Spin)
		}
		seen[BuzzsawFlareLeftHalf(g.Spin)]++
	}
	if seen[true] == 0 || seen[false] == 0 {
		t.Fatalf("the blade strip never alternated halves: %v", seen)
	}
}

func TestLowAmmoBlinkThresholds(t *testing.T) {
	// Buzzsaw: steady above half the maximum, then toggling every .3 s above a
	// quarter and every .15 s at or below it.
	var g GunVisual
	g.AttachBuzzsaw(0.03)
	g.BuzzsawStep(1.0/60, 0.03, 600, 700)
	if g.BlinkOn || g.Blink != 0 {
		t.Fatalf("blinking above half ammo: %+v", g)
	}
	g.BuzzsawStep(1.0/60, 0.03, 300, 700)
	if !g.BlinkOn || g.Blink != 0.3 {
		t.Fatalf("300/700 buzzsaw: want on with .3 s, got %+v", g)
	}
	g = GunVisual{}
	g.BuzzsawStep(1.0/60, 0.03, 100, 700)
	if !g.BlinkOn || g.Blink != 0.15 {
		t.Fatalf("100/700 buzzsaw: want on with .15 s, got %+v", g)
	}
	// Dual pistol: thresholds one halving lower (a quarter and an eighth).
	g = GunVisual{}
	g.DualStep(1.0/60, 0.35, true, 12, 24)
	if g.BlinkOn {
		t.Fatalf("dual blinking at half ammo: %+v", g)
	}
	g.DualStep(1.0/60, 0.35, true, 5, 24)
	if !g.BlinkOn || g.Blink != 0.3 {
		t.Fatalf("5/24 dual: want on with .3 s, got %+v", g)
	}
	g = GunVisual{}
	g.DualStep(1.0/60, 0.35, true, 3, 24)
	if !g.BlinkOn || g.Blink != 0.15 {
		t.Fatalf("3/24 dual: want on with .15 s, got %+v", g)
	}
}

func TestDualPistolTimerPegsToHalfTheRateWhenReleased(t *testing.T) {
	var g GunVisual
	g.AttachDual()
	g.DualStep(1.0/60, 0.35, false, 24, 24)
	if g.Timer != 0.175 {
		t.Fatalf("released timer %v, want rate/2", g.Timer)
	}
	frames := 0
	for !g.DualReady(0.35) {
		g.DualStep(1.0/60, 0.35, true, 24, 24)
		frames++
	}
	if frames < 10 || frames > 11 {
		t.Fatalf("first shot after %d held frames, want about .175 s (10.5 frames)", frames)
	}
	g.DualShot(4)
	if g.Timer != 0 || g.Spin != DualFlashStart {
		t.Fatalf("shot left timer %v flash %v", g.Timer, g.Spin)
	}
	g.DualStep(1.0/60, 0.35, false, 23, 24)
	if g.Timer != 0.175 || g.Spin >= DualFlashStart {
		t.Fatalf("release after a shot: timer %v flash %v", g.Timer, g.Spin)
	}
}

func TestDualPistolHandsAlternateStartingWithTheClearFlagTable(t *testing.T) {
	var g GunVisual
	g.AttachDual()
	x, y := g.DualShot(0)
	if x != -8 || y != 26 || !g.Right {
		t.Fatalf("first shot offset (%v,%v) right=%v, want the 0x005d9cd8 entry (-8,26)", x, y, g.Right)
	}
	x, y = g.DualShot(0)
	if x != 4 || y != 26 || g.Right {
		t.Fatalf("second shot offset (%v,%v) right=%v, want the 0x005d9c18 entry (4,26)", x, y, g.Right)
	}
	// Spot checks of both tables against the decoded literal pool (_INIT_49).
	for _, c := range []struct {
		table, dir int
		x, y       float32
	}{{0, 4, 25, 12}, {1, 4, 32, 7}, {0, 8, 10, -15}, {1, 8, -7, -15}, {0, 15, -4, 24}, {1, 15, -21, 24}} {
		if got := DualHandOffsets[c.table][c.dir]; got != [2]float32{c.x, c.y} {
			t.Errorf("table %d dir %d = %v, want (%v,%v)", c.table, c.dir, got, c.x, c.y)
		}
	}
}

func TestDualFlashGeometry(t *testing.T) {
	if !DualFlashActive(160-1) || DualFlashActive(0) || DualFlashActive(160) {
		t.Fatal("the flash draws while 0 < timer < 160")
	}
	if !DualFlashLeftHalf(100) || DualFlashLeftHalf(80) {
		t.Fatal("the first half shows while the timer is above 80")
	}
	if got := DualFlashRotationDegrees(4); got != 360 {
		t.Fatalf("facing right rotates %v, want 360 (a quad authored pointing right)", got)
	}
	x, y := DualFlashNudge(4)
	if math.Abs(x-15) > .05 || math.Abs(y) > .05 {
		t.Fatalf("facing right nudge (%v,%v), want (15,0)", x, y)
	}
	x, y = DualFlashNudge(0)
	if math.Abs(x) > .05 || math.Abs(y-10) > .05 {
		t.Fatalf("facing down nudge (%v,%v), want (0,10)", x, y)
	}
}

func TestDualChildrenAreConsumedBulletsAndSniperChildrenPierce(t *testing.T) {
	dual, ok := NewNativeWeaponProjectile(nativeWeaponShot{WeaponType: 7, Life: .75, Speed: 700}, "MULTI", 0, 0, nil)
	if !ok || dual.EntityType != 0x12 || dual.Penetration != 3 || dual.ChildType != 0x10 {
		t.Fatalf("dual projectile %+v", dual)
	}
	if child := NativeWeaponMultiChild(dual); child.EntityType != 0x10 {
		t.Fatalf("dual child kind %#x, want 0x10 (+0x84 overwritten after init)", child.EntityType)
	}
	sniper, _ := NewNativeWeaponProjectile(nativeWeaponShot{WeaponType: 4, Life: 1, Speed: 800}, "MULTI", 0, 0, nil)
	if child := NativeWeaponMultiChild(sniper); sniper.Penetration != 6 || child.EntityType != 0x11 {
		t.Fatalf("sniper children %#x x%d, want 0x11 x6", child.EntityType, sniper.Penetration)
	}
}

func TestSawBladeContactRules(t *testing.T) {
	if !SawBladeOverlap(42+.3*48-1, 0, 48) || SawBladeOverlap(42+.3*48+1, 0, 48) {
		t.Fatal("horizontal reach is 42 + .3 * width")
	}
	reach := 42 + .3*48
	if !SawBladeOverlap(0, reach*.6667-1, 48) || SawBladeOverlap(0, reach*.6667+1, 48) {
		t.Fatal("vertical reach is squashed by 0.6667")
	}
	if SawBladeHitSound(0) != "SFX_BUZZSAW_HIT_1" || SawBladeHitSound(0xffffffff) != "SFX_BUZZSAW_HIT_2" || SawBladeHitSound(0x7fffffff) != "SFX_BUZZSAW_HIT_1" || SawBladeHitSound(0x80000000) != "SFX_BUZZSAW_HIT_2" {
		t.Fatal("the hit sound range [0x51,0x53) selects rev_1 or rev_2 only")
	}
}
