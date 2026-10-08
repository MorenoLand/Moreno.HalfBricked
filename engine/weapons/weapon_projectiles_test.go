package weapons

import "testing"

func TestNativeWeaponMultiThroughLifecycle(t *testing.T) {
	for _, count := range []int{3, 6} {
		remaining, spawns := count, 0
		for tick := 0; tick < count; tick++ {
			next, spawn, remove := NativeWeaponMultiTick(remaining)
			if !spawn || remove {
				t.Fatalf("count%d tick%d", count, tick)
			}
			remaining = next
			spawns++
		}
		_, spawn, remove := NativeWeaponMultiTick(remaining)
		if spawn || !remove || spawns != count {
			t.Fatal("incorrect MULTI lifetime")
		}
	}
	if retained, confirmed := nativeWeaponNormalHitRetention(0x11); !retained || !confirmed {
		t.Fatal("THROUGH removed on hit")
	}
	if retained, confirmed := nativeWeaponNormalHitRetention(0x10); retained || !confirmed {
		t.Fatal("NORMAL retained on hit")
	}
	if _, confirmed := nativeWeaponNormalHitRetention(0x12); confirmed {
		t.Fatal("MULTI container assigned ordinary hit callback")
	}
}

func TestNativeWeaponFlameHitDispatch(t *testing.T) {
	speed, damage := nativeWeaponFlameHit(450)
	if speed != float64(float32(450)*float32(.85)) || damage != 1 {
		t.Fatalf("speed%v damage%d", speed, damage)
	}
}

func TestNativeWeaponProjectileNormalAndMulti(t *testing.T) {
	shot := nativeWeaponShot{Direction: 0, WeaponType: 4, Life: 1, Speed: 600, Lift: 20}
	p, ok := NewNativeWeaponProjectile(shot, "MULTI", 100, 200, nil)
	if !ok || p.Penetration != 6 || p.VX != 600 || p.VY != 0 {
		t.Fatalf("%+v", p)
	}
	d := p.DrawGeometry()
	if d.Texture != "" {
		t.Fatal("MULTI container rendered")
	}
	d = NativeWeaponMultiChild(p).DrawGeometry()
	if d.Y != 180 || d.Width != -16.5 || d.Height != -16.5 || d.U1 != 1 {
		t.Fatalf("%+v", d)
	}
	if _, ok = NewNativeWeaponProjectile(shot, "BAZOOKA", 0, 0, nil); ok {
		t.Fatal("unresolved rocket treated as normal")
	}
}

func TestNativeWeaponProjectileFlameGeometry(t *testing.T) {
	rng := NewNativeRNG()
	p, ok := NewNativeWeaponProjectile(nativeWeaponShot{WeaponType: 5, Speed: 450, Lift: 20}, "FLAME", 0, 0, &rng)
	if !ok || p.EntityType != 0x16 || p.Life < .7 || p.Life > .9 || p.Width < 20 || p.Width > 36 {
		t.Fatalf("%+v", p)
	}
	p.Age, p.VX = .5, -1
	d := p.DrawGeometry()
	if d.U0 != .75 || d.U1 != .5 || d.Rotation != 0 || d.Height != -d.Width {
		t.Fatalf("%+v", d)
	}
}
