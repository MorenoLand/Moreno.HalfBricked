package main

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"strings"
)

type nativeSecondaryBinding struct {
	GunType        string
	IconCell       int
	WeaponType     int
	ProjectileType uint8
}
type nativeSecondaryInventory struct {
	Weapon  formats.Weapon
	Binding nativeSecondaryBinding
	Ammo    int
}
type nativeSecondaryDischarge struct {
	Weapon       formats.Weapon
	Direction    uint16
	AmmoConsumed int
}

func nativeSecondaryWeaponBinding(gunType string) (nativeSecondaryBinding, bool) {
	switch strings.ToUpper(strings.TrimSpace(gunType)) {
	case "GRENADE":
		return nativeSecondaryBinding{GunType: "GRENADE", IconCell: 0, WeaponType: 8, ProjectileType: 0x14}, true
	case "MINE":
		return nativeSecondaryBinding{GunType: "MINE", IconCell: 1, WeaponType: 9, ProjectileType: 0x15}, true
	case "BAZOOKA":
		return nativeSecondaryBinding{GunType: "BAZOOKA", IconCell: 2, WeaponType: 10, ProjectileType: 0x13}, true
	default:
		return nativeSecondaryBinding{}, false
	}
}
func (s *nativeSecondaryInventory) equip(weapon formats.Weapon) error {
	binding, ok := nativeSecondaryWeaponBinding(weapon.GunType)
	if !ok {
		return fmt.Errorf("native secondary dispatch unresolved for %s", weapon.GunType)
	}
	*s = nativeSecondaryInventory{Weapon: weapon, Binding: binding, Ammo: weapon.Ammo}
	return nil
}
func (s *nativeSecondaryInventory) discharge(direction uint16) (nativeSecondaryDischarge, bool) {
	if s.Ammo < 1 {
		return nativeSecondaryDischarge{}, false
	}
	if _, ok := nativeSecondaryWeaponBinding(s.Weapon.GunType); !ok {
		return nativeSecondaryDischarge{}, false
	}
	s.Ammo--
	return nativeSecondaryDischarge{Weapon: s.Weapon, Direction: direction, AmmoConsumed: 1}, true
}

type nativeSecondaryProjectile struct {
	nativeWeaponProjectile
	State            int
	Speed, FallSpeed float32
}

func newNativeSecondaryProjectile(shot nativeSecondaryDischarge, x, y float64) (nativeSecondaryProjectile, bool) {
	binding, ok := nativeSecondaryWeaponBinding(shot.Weapon.GunType)
	if !ok || shot.Weapon.BulletType != binding.GunType || binding.GunType == "GRENADE" {
		return nativeSecondaryProjectile{}, false
	}
	p := nativeSecondaryProjectile{nativeWeaponProjectile: nativeWeaponProjectile{X: x, Y: y, Life: float64(float32(shot.Weapon.Life)), Lift: 20, Direction: shot.Direction, WeaponType: binding.WeaponType, EntityType: binding.ProjectileType}, State: 2, Speed: float32(shot.Weapon.Speed)}
	if binding.GunType == "MINE" {
		p.Lift, p.Width, p.Height, p.Direction, p.Texture = 0, 40, -40, 0xbff4, "Common0/Textures/mine_SD"
	} else {
		p.State, p.Age = 0, float64(float32(p.Life)*.5)
		p.Width, p.Height, p.Texture = 14, -28, "Common0/Textures/bazooka_SD"
	}
	p.VX, p.VY = nativeWeaponVelocity(shot.Direction, float64(p.Speed))
	return p, true
}
func (p *nativeSecondaryProjectile) update(dt float32) (detonated, removed bool) {
	p.Age = float64(float32(p.Age) + dt)
	p.Lift = float64(float32(p.Lift) - dt*p.FallSpeed)
	if p.State == 2 || p.State == 0 {
		if p.EntityType == 0x13 && p.State == 0 {
			p.X = float64(float32(p.X) + dt*float32(p.VX))
			p.Y = float64(float32(p.Y) + dt*float32(p.VY))
		}
		if float32(p.Age) < float32(p.Life) {
			return false, false
		}
		p.State, p.Age, p.FallSpeed = 1, 0, 0
		return true, false
	}
	if p.State == 1 {
		if float32(p.Age) < math.Float32frombits(0x3e19999a) {
			p.Width = float64(dt*160 - (float32(p.Age)/.75)*dt*160*.5)
		} else {
			p.Width = 0
		}
		if float32(p.Age) > .75 {
			p.State, p.Age = 4, 0
		}
		return false, false
	}
	return false, float32(p.Age) > .5
}
func (p *nativeSecondaryProjectile) groundContact() {
	if p.EntityType != 0x13 || p.Lift >= 0 || p.State != 0 {
		return
	}
	p.State, p.Age, p.FallSpeed = 1, 0, 0
}
func (p *nativeSecondaryProjectile) targetContact() int {
	if p.EntityType == 0x13 && p.State == 0 {
		p.State, p.Age, p.FallSpeed = 1, 0, 0
	}
	if p.State != 1 {
		return 0
	}
	return 5
}
func (p *nativeSecondaryProjectile) wallContact(dx, dy float32) {
	if p.EntityType != 0x13 || p.State != 0 {
		return
	}
	p.State, p.Age, p.FallSpeed = 1, 0, 0
}
func (p nativeSecondaryProjectile) drawGeometry() nativeWeaponProjectileDraw {
	if p.State == 1 {
		frame, ok := grenadeExplosionFrame(p.Age)
		if !ok {
			return nativeWeaponProjectileDraw{}
		}
		u := float32(frame) * float32(.11)
		return nativeWeaponProjectileDraw{X: p.X, Y: p.Y - 70, Width: 120, Height: -240, U0: float64(u), U1: float64(u + float32(.11)), Texture: nativeGrenadeExplosionTexture}
	}
	if p.State == 4 {
		return nativeWeaponProjectileDraw{}
	}
	return p.nativeWeaponProjectile.drawGeometry()
}
func (p nativeSecondaryProjectile) touchesTarget(x, y, width float32) bool {
	dx, dy := x-float32(p.X), (y-float32(p.Y))/math.Float32frombits(0x3f2a7efa)
	radius := float32(p.Width) + width*math.Float32frombits(0x3e99999a)
	return dx*dx+dy*dy < radius*radius
}
