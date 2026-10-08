package weapons

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
)

type NativeWeaponProjectile struct {
	X, Y, VX, VY, Age, Life, Lift, Width, Height float64
	Direction                                    uint16
	WeaponType                                   int
	EntityType                                   uint8
	Penetration                                  int
	// ChildType is the entity kind a MULTI container spawns each tick (its +0x84).
	// Zero means the post-init default 0x11 (0x000a57b0).
	ChildType uint8
	Texture   string
}
type nativeWeaponProjectileDraw struct {
	X, Y, Width, Height, Rotation, U0, U1 float64
	Texture                               string
}

func NewNativeWeaponProjectile(shot nativeWeaponShot, bulletType string, x, y float64, rng *NativeRNG) (NativeWeaponProjectile, bool) {
	p := NativeWeaponProjectile{X: x, Y: y, Direction: shot.Direction, WeaponType: shot.WeaponType, EntityType: 0x10, Life: shot.Life, Lift: shot.Lift, Width: 10, Height: -20, Texture: "Common0/Textures/Bullet_SD"}
	p.VX, p.VY = nativeWeaponVelocity(shot.Direction, shot.Speed)
	switch bulletType {
	case "NORMAL":
	case "MULTI":
		p.EntityType, p.Height = 0x12, -10
		// Post-init 0x000a57b0: +0x84 = 0x11 and +0x80 = 6 for weapon type 4 (the
		// sniper), 3 otherwise.
		p.Penetration = 3
		if shot.WeaponType == 4 {
			p.Penetration = 6
		}
		if shot.WeaponType == 7 {
			// The dual pistol (0x000aa174) stores +0x80 = 3 before init (equal to the
			// default) and overwrites +0x84 with 0x10 afterwards: its children are
			// plain bullets (consumed on the first hit), not the piercing 0x11 kind.
			p.ChildType = 0x10
		}
	case "SAW_BLADE":
		// Entity 0x1b (vtable 0x005bb4f0). Its draw slot (5) is the empty stub
		// 0x000a3bf4, so the blade is never drawn; the visible saw is the weapon's
		// Saw_Blade flare quad (gun_visuals.go). Collision: see game/saw_blade.go.
		p.EntityType, p.Width, p.Height = 0x1b, 44, -14
		p.Texture = "Common1/Textures/Characters/Saw_Blade_SD"
	case "FLAME":
		p.EntityType = 0x16
		if rng == nil {
			return NativeWeaponProjectile{}, false
		}
		p.Life = float64(NativeWeaponRandomFloat(rng, .2) + float32(.7))
		p.Width = float64(NativeWeaponRandomFloat(rng, 16) + float32(20))
		p.Height, p.Texture = -p.Width, "Common0/Textures/flame_animlarge_SD"
	default:
		return NativeWeaponProjectile{}, false
	}
	return p, true
}

func (p NativeWeaponProjectile) DrawGeometry() nativeWeaponProjectileDraw {
	if p.EntityType == 0x12 || p.EntityType == 0x1b {
		// The multi-hit container and the saw blades are not drawn themselves (the
		// blade's draw slot is the empty stub 0x000a3bf4); the buzzsaw's visible
		// disc is the Saw_Blade flare quad on the weapon.
		return nativeWeaponProjectileDraw{}
	}
	d := nativeWeaponProjectileDraw{X: p.X, Y: p.Y - p.Lift, Width: -p.Width, Height: p.Height, Rotation: float64(float32(p.Direction)/182+90) * math.Pi / 180, U1: 1, Texture: p.Texture}
	if p.EntityType == 0x1b {
		d.Width, d.Rotation = p.Width, 0
		d.U0 = math.Floor(p.Age*20) * .5
		d.U0 -= math.Floor(d.U0)
		d.U1 = d.U0 + .5
	}
	if p.WeaponType == 5 {
		d.Width = p.Width
		d.Rotation = 0
		d.U0 = math.Floor(p.Age*4) * .25
		d.U1 = d.U0 + .25
		if p.VX < 0 {
			d.U0, d.U1 = d.U1, d.U0
		}
	}
	return d
}

func nativeWeaponAmmoFallback(weapon formats.Weapon) bool {
	return weapon.GunType != "PISTOL" && weapon.Ammo <= 0
}

func NativeWeaponMultiTick(remaining int) (next int, spawnThrough, remove bool) {
	if remaining < 1 {
		return remaining, false, true
	}
	return remaining - 1, true, false
}

func NativeWeaponMultiChild(parent NativeWeaponProjectile) NativeWeaponProjectile {
	kind := parent.ChildType
	if kind == 0 {
		kind = 0x11
	}
	parent.EntityType, parent.Penetration, parent.Age, parent.ChildType = kind, 0, 0, 0
	parent.Width = float64(float32(parent.Width) * float32(1.65))
	parent.Height = float64(float32(parent.Height) * float32(1.65))
	return parent
}

func nativeWeaponNormalHitRetention(entityType uint8) (retain, confirmed bool) {
	switch entityType {
	case 0x10:
		return false, true
	case 0x11:
		return true, true
	default:
		return false, false
	}
}

func nativeWeaponFlameHit(speed float64) (nextSpeed float64, damage int) {
	return float64(float32(speed) * float32(.85)), 1
}
