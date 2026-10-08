package game

import (
	"image"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// Per-body state and drawing of the buzzsaw and dual pistol weapon objects
// (weapon classes 6 and 7). Evidence: Research/native/buzzsaw-dualpistol-2026-10-08.md.

// gunState is the native weapon object's mutable fields for the two weapons that
// need them (weapons.GunVisual) plus the class it was attached for.
type gunState struct {
	weapons.GunVisual
	kind    string
	maxAmmo int
	stale   bool // a weapon was equipped since the last tick: attach a fresh object
}

const (
	sawBladeTexture = "Common1/Textures/Characters/Saw_Blade_SD"
	// The player steps its walk-bob index every 120 ms (0x00096818: +0x260 is
	// reloaded with 0x78 and counts down in milliseconds).
	gunBobPeriod = .12
)

// fireGateOpen is whether the trigger handlers may call fire this frame. The
// dual pistol gates inside fire (its own timer, see DualStep) and the buzzsaw
// never fires from the trigger.
func (p *playState) fireGateOpen() bool {
	switch p.weapon.GunType {
	case "MINIGUN", "DUALPISTOL":
		return true
	case "BUZZSAW":
		return false
	}
	return p.shootCooldown <= 0
}

// tickGun runs the buzzsaw / dual pistol weapon update once per frame
// (0x000a9f0c / 0x000aadac, called by 0x000947e0 after movement). primary is
// true for player 1, whose sound the host plays.
func (p *playState) tickGun(primary bool) {
	const dt = float32(1.0 / 60.0)
	g := &p.gun
	kind := p.weapon.GunType
	if g.kind != kind || g.stale {
		previous := g.kind
		g.kind, g.stale = kind, false
		g.GunVisual = weapons.GunVisual{}
		g.maxAmmo = p.weapon.Ammo
		if record, ok := p.weapons.Find(kind); ok {
			g.maxAmmo = record.Ammo
		}
		switch kind {
		case "BUZZSAW":
			g.AttachBuzzsaw(float32(p.weapon.RateOfFire))
		case "DUALPISTOL":
			g.AttachDual()
		}
		if primary {
			p.sawAudioSwitch(previous == "BUZZSAW", kind == "BUZZSAW")
		}
		return
	}
	rate := float32(p.weapon.RateOfFire)
	switch kind {
	case "BUZZSAW":
		if g.BuzzsawStep(dt, rate, p.weapon.Ammo, g.maxAmmo) {
			p.spawnSawVolley()
		}
		if primary {
			p.sawAudioTick()
		}
	case "DUALPISTOL":
		g.DualStep(dt, rate, p.gunHeld, p.weapon.Ammo, g.maxAmmo)
	}
	p.gunHeld = false
	if (kind == "BUZZSAW" || kind == "DUALPISTOL") && p.weapon.Ammo <= 0 {
		// 0x000947e0: a weapon without ammo is replaced by the pistol at once.
		if pistol, ok := p.weapons.Find("PISTOL"); ok {
			p.equipWeapon(pistol)
		}
	}
}

// muzzleColumn is the facing column (0..15) of an aim vector, as muzzleTransform
// indexes the native direction tables.
func muzzleColumn(dx, dy float64) int {
	column, flipX := barryDirection(dx, dy)
	if flipX && column != 0 {
		column = 16 - column
	}
	return column
}

// gunDirection is the 0..15 direction index of a drawn body (+0x34).
func gunDirection(angle int, flipX bool) int {
	if angle < 0 {
		angle = 0
	} else if angle > 8 {
		angle = 8
	}
	if flipX && angle != 0 {
		return 16 - angle
	}
	return angle
}

// gunBlinks is whether the weapon's sprite is currently in its low-ammo blink
// frame (the +0x20 flag selects the sprite's second animation and a white tint).
func (p *playState) gunBlinks() bool {
	return p.gun.BlinkOn && (p.weapon.GunType == "BUZZSAW" || p.weapon.GunType == "DUALPISTOL")
}

// drawGunFlare draws the buzzsaw's Saw_Blade quad and the dual pistol's muzzle
// flare. The native player draw calls the weapon twice: +0x5c before the body
// (the buzzsaw while facing directions 5..11, the dual pistol for 6..10) and
// +0x60 after it (the buzzsaw's quad precedes the gun sprite, the dual pistol's
// flare follows the gun).
func (a *app) drawGunFlare(screen *ebiten.Image, x, y, scale float64, angle int, flipX bool, stage gunStage) {
	p := a.play
	if p == nil {
		return
	}
	dir := gunDirection(angle, flipX)
	switch p.weapon.GunType {
	case "BUZZSAW":
		want := gunStageAfterBody
		if dir >= 5 && dir <= 11 {
			want = gunStageBehind
		}
		if stage != want {
			return
		}
		offset := weapons.GunFlareOffsets[dir]
		bob := weapons.GunBobOffsets[int(p.time/gunBobPeriod)&3]
		a.drawFlareQuad(screen, sawBladeTexture, x+float64(offset[0])*scale, y+(float64(offset[1]+bob)-weapons.GunFlareAnchorY)*scale, scale, 0, weapons.BuzzsawFlareLeftHalf(p.gun.Spin))
	case "DUALPISTOL":
		if !weapons.DualFlashActive(p.gun.Spin) {
			return
		}
		want := gunStageAfterGun
		if dir >= 6 && dir <= 10 {
			want = gunStageBehind
		}
		if stage != want {
			return
		}
		nudgeX, nudgeY := weapons.DualFlashNudge(dir)
		cx := x + (float64(p.gun.HandX)+nudgeX)*scale
		cy := y + (float64(p.gun.HandY)+nudgeY-weapons.GunFlareAnchorY)*scale
		a.drawFlareQuad(screen, commonSDTexture(p.weapon.TextureFlare), cx, cy, scale, weapons.DualFlashRotationDegrees(dir), weapons.DualFlashLeftHalf(p.gun.Spin))
	}
}

type gunStage int

const (
	gunStageBehind gunStage = iota
	gunStageAfterBody
	gunStageAfterGun
)

// drawFlareQuad draws one half of a two-frame strip as a 45 x 30 quad centred on
// (cx, cy), rotated by rotation degrees (0x0006f798 / 0x00093724).
func (a *app) drawFlareQuad(screen *ebiten.Image, name string, cx, cy, scale, rotation float64, firstHalf bool) {
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	half := bounds.Dx() / 2
	if half <= 0 || bounds.Dy() <= 0 {
		return
	}
	left := bounds.Min.X
	if !firstHalf {
		left += half
	}
	source := texture.SubImage(image.Rect(left, bounds.Min.Y, left+half, bounds.Max.Y)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(half)/2, -float64(bounds.Dy())/2)
	options.GeoM.Scale(weapons.GunFlareWidth*scale/float64(half), weapons.GunFlareHeight*scale/float64(bounds.Dy()))
	options.GeoM.Rotate(rotation * math.Pi / 180)
	options.GeoM.Translate(cx, cy)
	a.drawImage(screen, source, options)
}
