package weapons

import "math"

// Buzzsaw (weapon class 6) and dual pistol (class 7) behaviour recovered from
// libmortargame.so(v7) (image base 0x10000). The evidence is itemised in
// Research/native/buzzsaw-dualpistol-2026-10-08.md; every constant names its
// source address.

// SAW_BLADE projectile (entity kind 0x1b, vtable 0x005bb4f0).
const (
	// SawBladeHitDamage is the damage the collision handler 0x000a5338 passes to
	// the zombie's takeDamage (vtable +0x5c): the literal 0x270f at 0x000a542c.
	SawBladeHitDamage = 9999
	// SawBladeHitCredit is the combo credit added per valid hit by 0x000a5338
	// (tracker slot 20 with the literal 0x3f8147ae = 1.01 at 0x000a5420).
	SawBladeHitCredit = 1.01
	// SawBladeRadius is the projectile's +0x28 after every move step (0x000a6970
	// writes the literal 0x42280000 = 42.0 at 0x000a69c4 into +0x28/+0x2c).
	SawBladeRadius = 42.0
	// SawBladeQueryScale is the grid query half extent factor (0x000a63d8 = .5):
	// the pass queries +-(42 * .5) around the blade.
	SawBladeQueryScale = float32(.5)
	// SawBladeBodyScale scales the target's +0x28 body size in the overlap test
	// (0x000a63d4 = .3); the vertical axis is divided by 0x000a63d0 = 0.6667.
	SawBladeBodyScale = .3
	SawBladeYSquash   = 0x3f2a7efa // float32 bits of 0.6667
)

// SawBladeOverlap is the ellipse test of the projectile pass 0x000a5f74 for a
// zombie of the given body width: dx^2 + (dy/0.6667)^2 < (42 + .3*width)^2.
func SawBladeOverlap(dx, dy, bodyWidth float64) bool {
	squash := float64(math.Float32frombits(SawBladeYSquash))
	reach := float64(float32(SawBladeRadius) + float32(bodyWidth)*float32(SawBladeBodyScale))
	dy /= squash
	return dx*dx+dy*dy < reach*reach
}

// SawBladeHitSound returns the clip symbol 0x000a5338 plays on a valid hit:
// FUN_000ca58c(0x51, 0x53) selects from the half-open id range, i.e. sound id
// 0x51 (chainsaw_rev_1) or 0x52 (chainsaw_rev_2); id 0x53 (rev_3) is never played.
func SawBladeHitSound(random uint32) string {
	if uint64(random)*2>>32 == 0 {
		return "SFX_BUZZSAW_HIT_1"
	}
	return "SFX_BUZZSAW_HIT_2"
}

// GunVisual is the weapon object state shared by the buzzsaw and dual pistol
// classes (offsets of the native weapon object in comments).
type GunVisual struct {
	Spin         float32 // +0x10, milliseconds: buzzsaw spin phase 0..101, dual flash timer 160..0
	Timer        float32 // +0x14, seconds since the last shot
	Blink        float32 // +0x18, seconds until the low-ammo blink toggles
	BlinkOn      bool    // +0x20, selects the sprite's second animation and white tint
	Right        bool    // +0x4c (dual): selects the 0x005d9c18 table instead of 0x005d9cd8
	HandX, HandY float32 // +0x50/+0x54 (dual): offset of the hand that fired last
}

// AttachBuzzsaw is 0x000a9eb8: the fire timer starts at the rate (ready), the
// spin phase and blink state are cleared.
func (g *GunVisual) AttachBuzzsaw(rate float32) {
	g.Spin, g.Timer, g.Blink, g.BlinkOn = 0, rate, 0, false
}

// AttachDual is 0x000abf44: timer, flash timer and blink flag are cleared. The
// hand flag lives in the constructor (0x000ac068) and starts cleared.
func (g *GunVisual) AttachDual() {
	g.Spin, g.Timer, g.Blink, g.BlinkOn = 0, 0, 0, false
}

// BuzzsawStep is the per-frame update 0x000a9f0c. It returns true when the
// weapon spawns its three blades this frame: ammo remains and the timer has
// reached the rate. The native code never resets the timer, so once the rate has
// elapsed (it starts at the rate) a volley leaves every frame and costs one ammo.
// The trigger is not consulted; 0x000947e0 calls the update every frame.
func (g *GunVisual) BuzzsawStep(dt, rate float32, ammo, maxAmmo int) bool {
	spin := g.Spin - dt*1000
	if spin < 0 || spin > 100 {
		if spin > 100 {
			spin = spin - 101 + 0
		} else {
			spin = 101 - (0 - spin)
		}
	}
	g.Spin = spin
	g.Timer += dt
	if g.Blink > 0 {
		g.Blink -= dt
	} else {
		max := uint32(maxAmmo)
		if int32(max>>1) < int32(ammo) {
			g.BlinkOn = false
		} else {
			interval := float32(0.15)
			if int32(max>>2) < int32(ammo) {
				interval = float32(0.3)
			}
			g.BlinkOn = !g.BlinkOn
			g.Blink = interval
		}
	}
	return ammo > 0 && rate <= g.Timer
}

// DualStep is the per-frame update 0x000aadac. held is the trigger state of this
// weapon's slot: released, the timer is pegged to half the rate; held, it
// counts up.
func (g *GunVisual) DualStep(dt, rate float32, held bool, ammo, maxAmmo int) {
	spin := g.Spin - dt*1000
	if spin < 0 {
		spin = 0
	}
	g.Spin = spin
	if held {
		g.Timer += dt
	} else {
		g.Timer = rate * 0.5
	}
	if g.Blink > 0 {
		g.Blink -= dt
		return
	}
	max := uint32(maxAmmo)
	if int32(max>>2) < int32(ammo) {
		g.BlinkOn = false
		return
	}
	interval := float32(0.15)
	if int32(max>>3) < int32(ammo) {
		interval = float32(0.3)
	}
	g.BlinkOn = !g.BlinkOn
	g.Blink = interval
}

// DualReady is the "can fire" slot 0x000a8f98: the timer has reached the rate.
func (g *GunVisual) DualReady(rate float32) bool { return g.Timer >= rate }

// DualFlashStart is the flash timer written by the dual pistol's fire function
// (0x000aa174 stores the literal 0x43200000 = 160.0 at 0x000aa338 into +0x10).
const DualFlashStart = 160

// DualShot is the hand bookkeeping of 0x000aa174 for the shot fired at
// direction index dir (0..15): the hand offset is read from the table selected
// by the current hand flag, the timer is reset, the flash timer starts and the
// hand flag toggles. It returns the offset the bullet spawns at.
func (g *GunVisual) DualShot(dir int) (x, y float32) {
	if dir < 0 || dir > 15 {
		dir = 0
	}
	table := &DualHandOffsets[0]
	if g.Right {
		table = &DualHandOffsets[1]
	}
	g.HandX, g.HandY = table[dir][0], table[dir][1]
	g.Timer = 0
	g.Spin = DualFlashStart
	g.Right = !g.Right
	return g.HandX, g.HandY
}

// DualHandOffsets are the per-direction muzzle offsets of the two hands (x, y;
// the z field is 0 everywhere), written by static initialiser _INIT_49
// (0x000ac798). [0] is the table used while the hand flag is clear
// (0x005d9cd8), [1] the table used while it is set (0x005d9c18). The flag starts
// clear, so the first shot of a pickup uses table [0].
var DualHandOffsets = [2][16][2]float32{
	{{-8, 26}, {3, 26}, {9, 22}, {17, 18}, {25, 12}, {29, 3}, {26, -8}, {23, -16}, {10, -15}, {-22, -14}, {-24, -10}, {-28, 2}, {-28, 9}, {-19, 17}, {-9, 21}, {-4, 24}},
	{{4, 26}, {20, 26}, {26, 22}, {27, 15}, {32, 7}, {28, -5}, {11, -17}, {6, -20}, {-7, -15}, {-9, -21}, {-18, -18}, {-26, -8}, {-32, 4}, {-30, 14}, {-26, 19}, {-21, 24}},
}

// GunFlareOffsets are the per-direction anchors of the weapon flare quad drawn by
// the buzzsaw (0x000ac3f0 / 0x000ac284, table 0x005d9e58 written by _INIT_49).
var GunFlareOffsets = [16][2]float32{
	{-4, 21}, {6, 21}, {11, 22}, {22, 21}, {26, 16}, {22, 10}, {20, 1}, {18, -4}, {4, -4}, {-16, -4}, {-20, 1}, {-19, 10}, {-27, 16}, {-22, 21}, {-14, 21}, {-5, 21},
}

// GunBobOffsets is the int table 0x00567314 the flare quad adds to its y by the
// weapon's walk-bob index (+0x2c, which the player steps every 120 ms).
var GunBobOffsets = [4]float32{0, 1, 1, 0}

const (
	// GunFlareWidth/Height are the quad size literals 0x42340000 / 0x41f00000
	// (45.0 and 30.0) at 0x000ac540 / 0x000ac544 (and 0x000a8f00/4, 0x000ac704/8).
	GunFlareWidth, GunFlareHeight = 45.0, 30.0
	// GunFlareAnchorY is the literal 0x41a00000 = 20.0 subtracted from the weapon y.
	GunFlareAnchorY = 20.0
)

// BuzzsawFlareLeftHalf is whether the Saw_Blade strip's first half is shown:
// 0x000ac3f0/0x000ac284 use u = 0..0.5 while the spin phase is above 50 and
// u = 0.5..1 otherwise.
func BuzzsawFlareLeftHalf(spin float32) bool { return spin > 50 }

// DualFlashLeftHalf is the same choice for the dual pistol's muzzle flare
// (0x000ac548): the first half while the flash timer is above 80.
func DualFlashLeftHalf(spin float32) bool { return spin > 80 }

// DualFlashActive is the draw condition of 0x000ac548: 0 < timer < 160.
func DualFlashActive(spin float32) bool { return spin > 0 && spin < DualFlashStart }

// DualFlashRotationDegrees and DualFlashNudge are the rotation and the (x, y)
// nudge applied to the dual pistol flash quad for direction index dir
// (0x000ac548: degrees = 450 - 22.5*dir; nudge = (-15*cos(a), 10*sin(a)) with
// a = (450 + 22.5*dir) degrees).
func DualFlashRotationDegrees(dir int) float64 {
	return float64(uint16(int(450 - float64(dir)*22.5)))
}
func DualFlashNudge(dir int) (x, y float64) {
	units := uint16(int(450+float64(dir)*22.5) * 0xb6)
	radians := float64(units) / 65536 * 2 * math.Pi
	return math.Cos(radians) * -15, math.Sin(radians) * 10
}
