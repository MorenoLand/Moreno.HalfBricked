package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
)

type nativeWeaponProjectile struct {
	X, Y, VX, VY, Age, Life, Lift, Width, Height float64
	Direction                                    uint16
	WeaponType                                   int
	EntityType                                   uint8
	Penetration                                  int
	Texture                                      string
}
type nativeWeaponProjectileDraw struct {
	X, Y, Width, Height, Rotation, U0, U1 float64
	Texture                               string
}

func newNativeWeaponProjectile(shot nativeWeaponShot, bulletType string, x, y float64, rng *nativeRNG) (nativeWeaponProjectile, bool) {
	p := nativeWeaponProjectile{X: x, Y: y, Direction: shot.Direction, WeaponType: shot.WeaponType, EntityType: 0x10, Life: shot.Life, Lift: shot.Lift, Width: 10, Height: -20, Texture: "Common0/Textures/Bullet_SD"}
	p.VX, p.VY = nativeWeaponVelocity(shot.Direction, shot.Speed)
	switch bulletType {
	case "NORMAL":
	case "MULTI":
		p.EntityType, p.Height = 0x12, -10
		p.Penetration = 3
		if shot.WeaponType == 4 {
			p.Penetration = 6
		}
	case "FLAME":
		p.EntityType = 0x16
		if rng == nil {
			return nativeWeaponProjectile{}, false
		}
		p.Life = float64(nativeWeaponRandomFloat(rng, .2) + float32(.7))
		p.Width = float64(nativeWeaponRandomFloat(rng, 16) + float32(20))
		p.Height, p.Texture = -p.Width, "Common0/Textures/flame_animlarge_SD"
	default:
		return nativeWeaponProjectile{}, false
	}
	return p, true
}

func (p nativeWeaponProjectile) drawGeometry() nativeWeaponProjectileDraw {
	if p.EntityType == 0x12 {
		return nativeWeaponProjectileDraw{}
	}
	d := nativeWeaponProjectileDraw{X: p.X, Y: p.Y - p.Lift, Width: -p.Width, Height: p.Height, Rotation: float64(float32(p.Direction)/182+90) * math.Pi / 180, U1: 1, Texture: p.Texture}
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

func nativeWeaponMultiTick(remaining int) (next int, spawnThrough, remove bool) {
	if remaining < 1 {
		return remaining, false, true
	}
	return remaining - 1, true, false
}

func nativeWeaponMultiChild(parent nativeWeaponProjectile) nativeWeaponProjectile {
	parent.EntityType, parent.Penetration, parent.Age = 0x11, 0, 0
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
