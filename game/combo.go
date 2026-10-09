package game

import (
	"math"
	"strconv"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Kill combo and score multiplier. Native evidence is in
// Research/native/combo-multiplier-2026-10-08.md. Short version (libmortargame
// v7, image base 0x10000):
//
//   - Every equipped weapon owns a combo tracker entity (created by the player's
//     equip routine FUN_0009458c). Its float counter (+0x84) is raised by the
//     projectile hit handlers through vtable slot 20 (FUN_00098b98) and drained
//     per second by a weapon-specific rate. The weapon XML <SM_Info> list picks
//     the tier (FUN_000989f8) from the integer part of the counter.
//   - When the weapon is replaced the tracker is released. Once released
//     (FUN_00098d64 case 0) it shows the tier text ("Epic Fail" below the first
//     tier), flies up to the HUD, and adds
//     the tier's multiplier to the HUD multiplier (FUN_000d065c).
//   - The HUD multiplier only ever rises; it resets to 1 when the player dies
//     (FUN_00094c6c -> FUN_000d06c0) and at level start.
//   - Only weapons whose class answers true to vtable +0x2c own a tracker. The
//     pistol (class 0x005bb620, +0x2c = FUN_000ac718 returns 0) and the sentry
//     weapon (class 0x005bbaf0, FUN_000ae184 returns 0) never get one, so a
//     pistol swap can never show "Epic Fail"; every other primary and secondary
//     weapon does (base FUN_000a8f70 returns 1).
//   - While a tracker is live (state 0) its count "x%d" (string 0x00562283) is
//     drawn every frame by FUN_000993d0 at the player's position pushed behind the
//     aim direction (FUN_00098d64), clamped to the screen below the HUD
//     multiplier; it never fades on its own. See Research/native/combo-achievements-followup-2026-10-08.md.

const (
	comboFlashFull     = 32768.0 // 0x8000, tracker +0x6c and HUD +0x90 flash timers
	comboFlashHalf     = 16384.0 // 0x4000
	comboFlashRate     = 65520.0 // units per second (DAT 0x00099174 / 0x000d0900)
	comboFlyAccel      = 1.5     // DAT_000991c4
	comboFadeRate      = 4.0     // DAT_000991cc
	comboBaseSize      = 18.0    // DAT_00099184
	comboInitialSize   = 24.0    // DAT_00098d54: +0x28 and +0x30 at init
	comboMaxSize       = 250.0   // DAT_000997b0: size clamp in the draw
	comboMargin        = 2.0     // DAT_000997bc: screen margin of the text
	comboAnchorDist    = 2.0     // update: position = player - 2*size*(cos, sin*squash)
	comboAnchorSquash  = 0.666   // DAT_00099188 = 0x3f2a7efa
	comboAnchorLift    = 25.0    // DAT_000997c4 (world units, subtracted before the camera mapping)
	comboFlyX          = 240.0   // DAT_000997b8
	comboHudTop        = 10.0    // DAT_000d071c: HUD multiplier centre = layout top + 10 + glyph/2
	comboHudMinGlyph   = 28.0    // DAT_000d0724 / DAT_000997c0
	comboSizePerCount  = 5.0     // DAT_00099180 (counter/5 + 18)
	comboSizeEase      = 0.1     // DAT_000991bc
	comboPulse         = 0.15    // DAT_0009917c
	comboHoldAfterFlag = 0.05    // DAT_000991a4
	comboEpicFail      = "Epic Fail"
	comboHitBullet     = 1.01 // bullet hit handlers FUN_000a521c / FUN_000a5378 (damage 0x65 / 100)
	comboHitFlame      = 0.01 // flame particle handler FUN_000a4bbc (damage 1 / 100), non-lethal hits only
	comboHitBlast      = 0.05 // blast handlers FUN_000a4cac / FUN_000a50f4 (damage 5 / 100), non-lethal hits only
)

// comboHudBlue is the idle HUD multiplier tint (FUN_000d0728: 0x4b, 0x4b, 0xff).
var comboHudBlue = [3]float64{75, 75, 255}

// comboStates mirror tracker +0x54.
const (
	comboActive = iota
	comboHold
	comboFly
	comboBonus
	comboFade
)

// comboWeaponID maps a catalog gun type to the native weapon enum (string table
// at 0x005627f4: PISTOL, SHOTGUN, UZI, MINIGUN, SNIPER, FLAMER, BUZZSAW,
// DUALPISTOL, GRENADE, MINE, BAZOOKA, SENTRY, COWPATBOMB). Ids below 8 are
// primary guns (FUN_000b1968).
func comboWeaponID(gun string) int {
	switch gun {
	case "PISTOL":
		return 0
	case "SHOTGUN":
		return 1
	case "UZI":
		return 2
	case "MINIGUN":
		return 3
	case "SNIPER":
		return 4
	case "FLAMER":
		return 5
	case "BUZZSAW":
		return 6
	case "DUALPISTOL":
		return 7
	case "GRENADE":
		return 8
	case "MINE":
		return 9
	case "BAZOOKA":
		return 10
	case "SENTRY", "SENTRY_SHOTGUN", "SENTRY_UZI", "SENTRY_FLAMER", "SENTRY_BAZOOKA":
		return 11
	case "COWPATBOMB":
		return 12
	}
	return -1
}

// comboDecayRate is the per-second counter drain passed to the tracker init by
// FUN_0009458c: 1.0 for id 2 (UZI) and 5 (FLAMER), 1.25 for id 3 (MINIGUN), 0
// for everything else.
func comboDecayRate(id int) float32 {
	switch id {
	case 2, 5:
		return 1
	case 3:
		return 1.25
	}
	return 0
}

type comboTracker struct {
	gun       string
	id        int
	serial    int
	secondary bool
	tiers     []formats.WeaponScoreMultiplier
	counter   float32
	rate      float32
	tier      int
	held      bool
	state     int
	flash     float64
	hold      float64 // +0x8c: hold timer, then fade alpha
	fly       float64 // +0x50: flight progress to the HUD
	velocity  float64 // +0x24
	size      float64 // +0x30
	pulse     float64 // +0xb... size multiplier from the flash
	text      string
	bonus     int
	color     [3]float64 // current text colour (+0x60..0x62)
	target    [3]float64 // tier colour (+0x64..0x66)
	alpha     float64    // 0..1
	posX      float64    // anchor while live (state 0), unclamped screen position
	posY      float64
	shownX    float64 // last clamped screen position drawn in state 0 (+0x58/+0x5c)
	shownY    float64
	placed    bool    // shownX/shownY have been set by a draw
	hudGlyph  float64 // HUD multiplier glyph size (+0x94), the fly target size and position
	done      bool
}

// comboHasTracker reports whether the weapon's class creates a combo tracker
// (weapon vtable +0x2c, called by FUN_0009458c): false for the pistol (id 0)
// and the sentry weapon (id 11).
func comboHasTracker(id int) bool { return id >= 0 && id != 0 && id != 11 }

func newComboTracker(gun string, serial int, secondary bool, catalog formats.WeaponCatalog) *comboTracker {
	id := comboWeaponID(gun)
	if !comboHasTracker(id) {
		return nil
	}
	t := &comboTracker{gun: gun, id: id, serial: serial, secondary: secondary, rate: comboDecayRate(id), tier: -1, held: true, size: comboInitialSize, pulse: 1, alpha: 1, hold: 1, color: [3]float64{255, 255, 255}, hudGlyph: comboHudMinGlyph, text: "x0"}
	if weapon, ok := catalog.Find(gun); ok {
		t.tiers = weapon.ScoreMultipliers
	} else if weapon, ok := catalog.Find(sentryBase(gun)); ok {
		t.tiers = weapon.ScoreMultipliers
	}
	t.updateTier()
	return t
}

func sentryBase(gun string) string {
	if comboWeaponID(gun) == 11 {
		return "SENTRY"
	}
	return gun
}

// add is FUN_00098b98: the counter only moves while the tracker is active, and
// every integer crossing restarts the flash timer. It returns the achievement
// value, the integer part of the counter.
func (t *comboTracker) add(amount float64) int {
	if t == nil || t.state != comboActive {
		return 0
	}
	before := t.counter
	t.counter += float32(amount)
	if int(before) < int(t.counter) {
		t.flash = comboFlashFull
	}
	return int(t.counter)
}

// updateTier is FUN_000989f8: the tier index chases the highest threshold the
// integer counter has reached, one step per frame, and the target colour runs
// from blue-cyan at the first tier to green at the last.
func (t *comboTracker) updateTier() {
	count := int(t.counter)
	n := len(t.tiers)
	if t.tier < 0 || t.tiers[t.tier].Threshold <= count {
		next := t.tier + 1
		if n > next && count >= t.tiers[next].Threshold {
			t.tier = next
		}
	} else {
		t.tier--
	}
	frac := math.NaN()
	if n > 0 {
		frac = (float64(t.tier) + 1) / float64(n)
		frac = math.Min(1, math.Max(0, frac))
	}
	hi, lo := 0, 512
	if !math.IsNaN(frac) {
		hi = int(frac * 512)
		lo = 512 - hi
	}
	t.target = [3]float64{0, math.Min(255, math.Max(0, float64(hi))), math.Min(255, math.Max(0, float64(lo)))}
}

func comboSin(flash float64) float64 {
	return math.Sin(flash * 2 * math.Pi / 65536)
}

// updateFlash runs the +0x6c timer: double speed above 0x4000, a 15% size
// pulse, and a white to tier-colour colour pulse.
func (t *comboTracker) updateFlash(dt float64) bool {
	if t.flash < 1 {
		return false
	}
	t.flash = float64(int(t.flash - dt*comboFlashRate))
	if t.flash > comboFlashHalf {
		t.flash = float64(int(t.flash - dt*comboFlashRate))
	}
	if t.flash < 1 {
		t.flash = 0
	}
	s := comboSin(t.flash)
	t.pulse = 1 + s*comboPulse
	for i := range t.color {
		t.color[i] = math.Min(255, math.Max(0, math.Floor(255+(t.target[i]-255)*s)))
	}
	return true
}

type comboBonusEvent struct {
	amount int
	color  [3]float64
}

// update advances the tracker by dt (FUN_00098d64). dead is the player health
// test (<= 0). A non-nil event is the HUD bonus delivered this frame; the
// tracker is finished when done is set.
func (t *comboTracker) update(dt float64, dead bool) *comboBonusEvent {
	if !t.updateFlash(dt) && t.state == comboActive {
		t.counter -= float32(dt) * t.rate
		if t.counter < 0 {
			t.counter = 0
		}
	}
	t.updateTier()
	switch t.state {
	case comboActive:
		t.text = "x" + strconv.Itoa(int(t.counter)) // sprintf("x%d", (int)counter)
		t.size += (float64(t.counter)/comboSizePerCount + comboBaseSize - t.size) * comboSizeEase
		if dead {
			// Any live tracker turns into "Epic Fail" the moment the player's health is 0.
			t.state, t.alpha, t.hold, t.flash, t.tier, t.counter, t.text = comboFade, 1, 1, comboFlashFull, -1, 0, comboEpicFail
		}
	case comboFade:
		t.fly = 0
		if t.hold <= 0 {
			t.alpha = 0
		} else {
			t.hold -= dt
			t.alpha = math.Min(1, math.Max(0, t.hold))
		}
	}
	if t.held {
		return nil
	}
	switch t.state {
	case comboActive:
		if t.counter >= 1 {
			t.state, t.flash = comboHold, comboFlashFull
			if t.tier < 0 {
				t.hold, t.state, t.tier, t.counter, t.text = 1, comboFade, -1, 0, comboEpicFail
			} else if t.tier < len(t.tiers) {
				t.bonus, t.text = t.tiers[t.tier].Multiplier, t.tiers[t.tier].Text
			}
		} else {
			t.state = comboFade
		}
	case comboHold:
		if t.flash > 0 {
			t.hold = comboHoldAfterFlag
			break
		}
		t.hold -= dt
		if t.hold < 0 {
			t.hold, t.velocity, t.state = 1, 0, comboFly
		}
	case comboFly:
		// The flight eases the size toward the HUD multiplier's glyph size (+0x94)
		// while the multiplier is above 0, else 28.
		t.size += (t.hudGlyph - t.size) * comboSizeEase
		t.velocity += dt * comboFlyAccel
		t.fly += dt * t.velocity * comboFlyAccel
		if t.fly >= 1 {
			t.state, t.fly, t.hold, t.flash = comboBonus, 1, 1, comboFlashFull
			t.text = "+" + strconv.Itoa(t.bonus)
			t.color = t.target
			return &comboBonusEvent{amount: t.bonus, color: t.target}
		}
	case comboBonus:
		if t.flash > 0 {
			break
		}
		t.hold -= dt * comboFadeRate
		t.alpha = math.Min(1, math.Max(0, t.hold))
		if t.hold < 0 {
			t.done = true
		}
	case comboFade:
		if t.hold < 0 {
			t.done = true
		}
	}
	return nil
}

// comboSystem is the per-play combo state: the live primary and secondary
// trackers, released trackers still animating, and the HUD multiplier display.
type comboSystem struct {
	primary, secondary *comboTracker
	secondarySerial    int
	popups             []*comboTracker
	hudFlash           float64
	hudColor           [3]float64
	hudScale           float64 // +0x94
	hudPulse           float64 // +0x98
	mirror             []wireCombo
	mirrorFlash        float64
}

// comboPopup is the drawable form of a tracker.
type comboPopup struct {
	Text       string
	X, Y, Size float64
	R, G, B, A uint8
}

func (t *comboTracker) visible() bool {
	return t.text != "" && t.alpha > 0
}

// comboHudGlyph is the HUD multiplier glyph size (+0x94) the popups aim for:
// the live value while the multiplier is above 0, else 28 (FUN_000d06e4).
func (c *comboSystem) comboHudGlyph(multiplier int) float64 {
	if multiplier > 0 && c.hudScale > 0 {
		return math.Max(c.hudScale, comboHudMinGlyph)
	}
	return comboHudMinGlyph
}

// comboFlyTarget is where released trackers fly (FUN_000993d0): x = 240
// (DAT_000997b8) and y = HUD multiplier centre (layout top 0 + 10 + glyph/2,
// FUN_000d06e4) plus the glyph size.
func comboFlyTarget(glyph float64) (float64, float64) {
	return comboFlyX, comboHudTop + glyph/2 + glyph
}

// comboTopBound is the upper screen clamp of the popups: just under the HUD
// multiplier (centre + glyph/2 + margin + half the text height).
func comboTopBound(glyph, size float64) float64 {
	return comboHudTop + glyph/2 + glyph/2 + comboMargin + size/2
}

// popup returns the on-screen text for a tracker. Live trackers sit at their
// anchor behind the player; released ones stay where they were last drawn
// (+0x58/+0x5c) and fly toward the HUD.
func (t *comboTracker) popup() (comboPopup, bool) {
	if !t.visible() {
		return comboPopup{}, false
	}
	x, y := t.posX, t.posY
	if t.state != comboActive && t.placed {
		x, y = t.shownX, t.shownY
	}
	if t.state == comboHold || t.state == comboFly || t.state == comboBonus {
		tx, ty := comboFlyTarget(t.hudGlyph)
		x += (tx - x) * t.fly
		y += (ty - y) * t.fly
	}
	size := math.Min(comboMaxSize, t.size*t.pulse)
	return comboPopup{Text: t.text, X: x, Y: y, Size: size, R: uint8(t.color[0]), G: uint8(t.color[1]), B: uint8(t.color[2]), A: uint8(math.Min(255, t.alpha*255))}, true
}

// comboHUD advances the HUD multiplier flash and scale (FUN_000d0728).
func (c *comboSystem) updateHUD(dt float64, multiplier int) {
	if c.hudScale == 0 {
		c.hudScale = comboHudTarget(multiplier)
	}
	c.hudPulse = 1
	if c.hudFlash > 0 {
		c.hudFlash = float64(int(c.hudFlash - dt*comboFlashRate))
		if c.hudFlash > comboFlashHalf {
			c.hudFlash = float64(int(c.hudFlash - dt*comboFlashRate))
		}
		if c.hudFlash < 1 {
			c.hudFlash = 0
		}
		c.hudPulse = 1 + comboSin(c.hudFlash)*comboPulse
	}
	if multiplier < 1 {
		c.hudScale *= 0.9
	} else {
		c.hudScale += (comboHudTarget(multiplier) - c.hudScale) * 0.1
	}
}

func comboHudTarget(multiplier int) float64 {
	return float64(min(multiplier, 100))/5 + 28
}

// hudTint is the multiplier colour: blue when idle, pulsing to the colour of
// the last bonus (HUD +0xa0 is overwritten by FUN_000d065c).
func (c *comboSystem) hudTint() [3]float64 {
	if c.hudFlash <= 0 {
		return comboHudBlue
	}
	s := comboSin(c.hudFlash)
	var out [3]float64
	for i := range out {
		out[i] = comboHudBlue[i] + (c.hudColor[i]-comboHudBlue[i])*s
	}
	return out
}

// release drops the player's hold on a tracker; it keeps animating as a popup.
func (c *comboSystem) release(t *comboTracker) {
	if t == nil {
		return
	}
	t.held = false
	c.popups = append(c.popups, t)
}

// comboAchievementValue is the SPECIFIC "combo" key submitted by the native
// adder: the integer part of the counter that was just raised.
const comboAchievementKey = "combo"

// syncCombo keeps one tracker per equipped weapon, replacing it (and releasing
// the old one) whenever the pickup serial or weapon changes.
func (p *playState) syncCombo() {
	c := &p.combo
	if p.weapon.GunType != "" && (c.primary == nil || c.primary.serial != p.achieve.pickupSerial || c.primary.gun != p.weapon.GunType) {
		c.release(c.primary)
		c.primary = newComboTracker(p.weapon.GunType, p.achieve.pickupSerial, false, p.weapons)
	}
	name := p.secondaryType
	if name == "" {
		name = secondaryGrenade
	}
	if p.grenades <= 0 {
		// Nothing to throw: no combo to count. (The original keeps an empty
		// grenade tracker whose "x0" sat beside Barry from the start of a level;
		// the user found that stuck "x0" confusing, so the port only tracks a
		// secondary while one is held.)
		if c.secondary != nil && c.secondary.counter >= 1 {
			c.release(c.secondary)
		}
		c.secondary = nil
		name = ""
	}
	if name != "" && (c.secondary == nil || c.secondary.serial != c.secondarySerial || c.secondary.gun != name) {
		c.release(c.secondary)
		c.secondary = newComboTracker(name, c.secondarySerial, true, p.weapons)
	}
}

// updateCombo runs every active frame from updateBulletsAndKills.
func (p *playState) updateCombo(dt float64) {
	c := &p.combo
	dead := p.health <= 0
	if dead {
		// FUN_00094c6c: the killing blow re-equips the pistol and grenades, so both
		// trackers are released and finish as "Epic Fail" popups.
		c.release(c.primary)
		c.release(c.secondary)
		c.primary, c.secondary = nil, nil
	} else {
		p.syncCombo()
	}
	glyph := c.comboHudGlyph(p.multiplier)
	for _, t := range c.trackers() {
		t.hudGlyph = glyph
		if t.state == comboActive {
			t.posX, t.posY = p.comboAnchor(t.size)
			if t == c.secondary && c.primary != nil && c.primary.state == comboActive && c.primary.visible() {
				// Both slots anchor on the same spot behind Barry; stack the
				// grenade's count under the weapon's so they don't overprint.
				t.posY += c.primary.size/2 + t.size/2 + 2
			}
		}
		if event := t.update(dt, dead); event != nil {
			p.applyComboBonus(event)
		}
	}
	live := c.popups[:0]
	for _, t := range c.popups {
		if !t.done {
			live = append(live, t)
		}
	}
	c.popups = live
	c.updateHUD(dt, p.multiplier)
}

func (c *comboSystem) trackers() []*comboTracker {
	list := make([]*comboTracker, 0, 2+len(c.popups))
	if c.primary != nil {
		list = append(list, c.primary)
	}
	if c.secondary != nil {
		list = append(list, c.secondary)
	}
	return append(list, c.popups...)
}

// comboAimDirection is the unit aim vector recovered from the port's sprite
// column (the original stores the exact heading in player +0x36; the port only
// keeps the nine-column facing, see barryDirection).
func (p *playState) comboAimDirection() (float64, float64) {
	angle := math.Pi/2 - float64(p.angle)*math.Pi/8
	dx, dy := math.Cos(angle), math.Sin(angle)
	if p.flipX {
		dx = -dx
	}
	return dx, dy
}

// comboAnchor is the unclamped screen position of a live tracker's count
// (FUN_00098d64 + FUN_000993d0): the player pushed back along the aim direction
// by 2*size world units (y squashed by 0.666), lifted 25 world units, then mapped
// through the camera. The caller clamps it to the screen with the text size.
func (p *playState) comboAnchor(size float64) (float64, float64) {
	dx, dy := p.comboAimDirection()
	wx := p.x - comboAnchorDist*size*dx
	wy := p.y - comboAnchorDist*size*dy*comboAnchorSquash - comboAnchorLift
	if p.world == nil {
		return float64(logicalWidth) / 2, float64(logicalHeight) / 2
	}
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return (wx-p.world.CameraX)*zoom + p.world.ViewportX, (wy-p.world.CameraY)*zoom + p.world.ViewportY
}

func (p *playState) applyComboBonus(event *comboBonusEvent) {
	p.multiplier += event.amount
	p.combo.hudFlash = comboFlashFull
	p.combo.hudColor = event.color
	// No sound: FUN_00098d64, FUN_000d065c and FUN_000d0728 (the whole bonus path)
	// contain no sound call, and no other caller of SFX_MULTIPLIER_TICK_UP/DOWN was found.
}

// resetMultiplier is FUN_000d06c0.
func (p *playState) resetMultiplier() {
	p.multiplier = 1
	p.combo.hudFlash = 0
}

// comboCredit raises the tracker that owns a shot. Primary shots carry the
// pickup serial; secondary shots use the equipped secondary tracker.
func (p *playState) comboCredit(origin killOrigin, amount float64) {
	var t *comboTracker
	switch origin.gun {
	case "GRENADE", "MINE", "ROCKET", "COWPAT":
		t = p.combo.secondary
	case "", "SENTRY", "EXPZOMBIE", "TREX", "TRAIN":
		return
	default:
		if t = p.combo.primary; t != nil && t.serial != origin.pickup {
			return
		}
	}
	if t == nil {
		return
	}
	if value := t.add(amount); value > 0 {
		p.achieve.noteCombo(value)
	}
}

func (a *achievementProgress) noteCombo(value int) {
	if a.best == nil {
		a.best = map[string]int32{}
	}
	if int32(value) > a.best[comboAchievementKey] {
		a.best[comboAchievementKey] = int32(value)
	}
}

// Zombie health model behind the per-hit credit. Zombies carry integer health
// (+0x2a8, maximum +0x2d0 = the spawn record's strength) and every projectile
// handler deals a fixed damage through the zombie's damage slot: bullets 0x65 =
// 101 (FUN_000a521c), flame particles 1 (FUN_000a4bbc), blast ticks 5
// (FUN_000a4cac, FUN_000a4dc4, FUN_000a4f50, FUN_000a50f4), the piercing blade
// handler 0x270f = 9999 (FUN_000a5338). The port applies the same damages (see
// zombie_model.go), so the credit is the native one without any conversion.

// comboBulletHit credits one bullet or flame particle hit on a living zombie.
// Native bullets credit 1.01 before the damage lands (so the lethal hit pays
// too); flame particles credit 0.01 only when the zombie survives the hit. Saw
// blades (entity 0x1b, FUN_000a5338) never come through here: stepSawBlade
// credits weapons.SawBladeHitCredit (1.01) per living target contacted, and
// blast ticks credit comboHitBlast per surviving 5 damage tick.
func (p *playState) comboBulletHit(b bullet, survived bool) {
	if b.projectile != nil && b.projectile.EntityType == 0x16 {
		if survived {
			p.comboCredit(b.origin, comboHitFlame)
		}
		return
	}
	p.comboCredit(b.origin, comboHitBullet)
}

// captureComboScene stages a UZI combo of 35 that is then swapped for the
// pistol, so the tier popup flies to the HUD (capture state play-combo).
func (a *app) captureComboScene() error {
	a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
	if err := a.openPlay(); err != nil {
		return err
	}
	p := a.play
	p.closeScript()
	p.dialogueIndex = len(p.dialogue)
	p.hudVisible, p.moveControl, p.shootControl = true, true, true
	p.collectPickup("p_uzi")
	p.syncCombo()
	p.combo.primary.counter = 35
	p.combo.primary.flash = comboFlashFull
	for i := 0; i < 6; i++ {
		p.updateCombo(1.0 / 60.0)
	}
	if pistol, ok := p.weapons.Find("PISTOL"); ok {
		p.equipWeapon(pistol)
	}
	return nil
}
