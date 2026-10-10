package weapons

// Native flame projectile (entity kind 0x16, vtable 0x005bb310 in v7).
//
// Flame update FUN_000a6a58 (v7) / FUN_00106fe8 (1.2.5) and the post-init
// FUN_000a6cf8 share these literals in both builds (v7 0x000a6cb8.., 1.2.5
// 0x001071a0..): the direction vector is multiplied by 0.975 every update,
// the vertical speed grows by 50 per second (height += 50 * age * dt), the
// body grows by 5 px per second once its speed is <= 0.1, and the hit flags
// are live on every third update only (counter +0x80 cycles 2, 1, 0).
const (
	FlameDirDecay    = float32(0.975)
	FlameRiseAccel   = float32(50)
	FlameStopSpeed   = float32(0.1)
	FlameGrowRate    = float32(5)
	FlameHitPeriod   = 3
	GibFlameSpeed    = 100.0 // DAT_000a152c (v7) / DAT_0010074c (1.2.5)
	GibFlameWeapon   = 5     // +0x64 weapon class stored by the gib call, as the flamer's
	GibFlameBound    = 0xff3a
	GibFlameCountMin = 50 // 0x32 + Bounded(0x14)
	GibFlameCountVar = 20
)

// NewGibFlame is the flame a gibbed zombie spawns (FUN_000a10fc state 2 hit
// phase): heading Bounded(0xff3a) supplied by the caller, speed 100, hits the
// player and zombies, post-init life 0.7 + rnd(0.2) and width 20 + rnd(16) as
// every flame, starting height lift.
func NewGibFlame(x, y float64, heading uint16, lift float64, rng *NativeRNG) (NativeWeaponProjectile, bool) {
	shot := nativeWeaponShot{Direction: heading, WeaponType: GibFlameWeapon, Speed: GibFlameSpeed, Lift: lift}
	return NewNativeWeaponProjectile(shot, "FLAME", x, y, rng)
}
