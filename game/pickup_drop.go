package game

// Pickup drop and bounce (libmortargame.so v7 FUN_000928f4, init 0x00092da0; 1.2.5
// FUN_000ef948, init 0x000eed84..0x000eeec8).
//
// A pickup is created at height +0x1c (v7) / +0x18 (1.2.5) = 250 (literal 0x00092f80,
// 1.2.5 0x000eeef8) with vertical speed 0 (the zero vector at 0x005d10d4/8). Every update:
//
//	speed  -= dt * 700          (DAT_00092cfc, 1.2.5 DAT_000efcb8)
//	height += speed * dt
//	if height < 0: height = 0, speed *= -0.25 (DAT_00092d04, 1.2.5 DAT_000efcc4) and
//	               only then the player test runs: (2*dx)^2 + (2*dy)^2 + (2*dz)^2 <
//	               size^2 * 1.25 (double 0x00092cf0), size 50 or 80.
//
// So a pickup cannot be collected while it falls or while a bounce is in the air; once it
// rests it re-enters the ground branch every frame (speed gains -11.7 per frame and the
// bounce restores a quarter of it). The crate and the weapon icon are drawn with
// y - size * 0.375 - height (FUN_000930c8). The off-screen arrow FUN_000930c8 draws while the
// pickup is outside the camera (+0x240 / +0x244, "markerweapon") lives in offscreen_markers.go.
const (
	pickupDropHeight  = 250.0
	pickupGravity     = 700.0
	pickupBounceScale = -0.25
)

// stepPickupDrops advances the drop of every pickup by one 60 Hz frame. It runs once per
// frame (updateZombies); updatePickups only reads the landed flag.
func (p *playState) stepPickupDrops(dt float64) {
	for _, e := range p.scriptEntities {
		if e == nil || e.kind != "pickup" || !e.drop {
			continue
		}
		speed := float32(e.liftVel) - float32(dt)*float32(pickupGravity)
		height := float32(e.lift) + speed*float32(dt)
		e.landed = false
		if height < 0 {
			height = 0
			speed *= float32(pickupBounceScale)
			e.landed = true
		}
		e.lift, e.liftVel = float64(height), float64(speed)
	}
	// The off-screen arrows share the 60 Hz step (offscreen_markers.go).
	p.stepPickupMarkers(dt)
	p.stepZombieMarkers(dt)
}
