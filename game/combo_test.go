package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func comboTiers() []formats.WeaponScoreMultiplier {
	return []formats.WeaponScoreMultiplier{{Threshold: 10, Text: "Poor", Multiplier: 1}, {Threshold: 20, Text: "OK", Multiplier: 2}, {Threshold: 30, Text: "Good", Multiplier: 4}, {Threshold: 50, Text: "Great", Multiplier: 6}, {Threshold: 80, Text: "AWESOME", Multiplier: 8}}
}

func newTestTracker(gun string, tiers []formats.WeaponScoreMultiplier) *comboTracker {
	t := &comboTracker{gun: gun, id: comboWeaponID(gun), tiers: tiers, rate: comboDecayRate(comboWeaponID(gun)), tier: -1, held: true, size: comboBaseSize, pulse: 1, alpha: 1, hold: 1}
	t.updateTier()
	return t
}

func TestComboWeaponIDsAndDecay(t *testing.T) {
	ids := map[string]int{"PISTOL": 0, "SHOTGUN": 1, "UZI": 2, "MINIGUN": 3, "SNIPER": 4, "FLAMER": 5, "BUZZSAW": 6, "DUALPISTOL": 7, "GRENADE": 8, "MINE": 9, "BAZOOKA": 10, "SENTRY": 11, "COWPATBOMB": 12}
	for gun, id := range ids {
		if comboWeaponID(gun) != id {
			t.Fatalf("%s id %d, want %d", gun, comboWeaponID(gun), id)
		}
	}
	rates := map[string]float32{"UZI": 1, "FLAMER": 1, "MINIGUN": 1.25, "PISTOL": 0, "SHOTGUN": 0, "SNIPER": 0, "GRENADE": 0}
	for gun, want := range rates {
		if got := comboDecayRate(comboWeaponID(gun)); got != want {
			t.Fatalf("%s decay %v, want %v", gun, got, want)
		}
	}
}

// The tier index follows the integer counter one step per update.
func TestComboTierThresholds(t *testing.T) {
	tr := newTestTracker("SHOTGUN", comboTiers())
	tr.counter = 9.99
	for i := 0; i < 8; i++ {
		tr.updateTier()
	}
	if tr.tier != -1 {
		t.Fatalf("9.99 is below the first threshold, tier %d", tr.tier)
	}
	tr.counter = 80
	tr.updateTier()
	if tr.tier != 0 {
		t.Fatalf("tiers climb one step per frame, tier %d", tr.tier)
	}
	for i := 0; i < 8; i++ {
		tr.updateTier()
	}
	if tr.tier != 4 || tr.tiers[tr.tier].Text != "AWESOME" {
		t.Fatalf("80 reaches AWESOME, tier %d", tr.tier)
	}
	tr.counter = 25
	for i := 0; i < 8; i++ {
		tr.updateTier()
	}
	if tr.tier != 1 {
		t.Fatalf("a falling counter steps the tier back down, tier %d", tr.tier)
	}
	empty := newTestTracker("PISTOL", nil)
	empty.counter = 500
	empty.updateTier()
	if empty.tier != -1 {
		t.Fatal("the pistol has no tiers")
	}
}

func TestComboAddOnlyWhileActiveAndFlashes(t *testing.T) {
	tr := newTestTracker("SHOTGUN", comboTiers())
	if v := tr.add(0.4); v != 0 || tr.flash != 0 {
		t.Fatalf("0.4 does not cross an integer: value %d flash %v", v, tr.flash)
	}
	if v := tr.add(0.7); v != 1 || tr.flash != comboFlashFull {
		t.Fatalf("crossing 1: value %d flash %v", v, tr.flash)
	}
	tr.state = comboFade
	if tr.add(5) != 0 || tr.counter > 1.2 {
		t.Fatal("a finished tracker ignores hits")
	}
}

func TestComboDecayRates(t *testing.T) {
	uzi := newTestTracker("UZI", comboTiers())
	pistol := newTestTracker("PISTOL", nil)
	mini := newTestTracker("MINIGUN", comboTiers())
	for _, tr := range []*comboTracker{uzi, pistol, mini} {
		tr.counter = 10
		for i := 0; i < 60; i++ {
			tr.update(1.0/60, false)
		}
	}
	if uzi.counter < 8.9 || uzi.counter > 9.1 {
		t.Fatalf("UZI drains 1/s: %v", uzi.counter)
	}
	if mini.counter < 8.7 || mini.counter > 8.8 {
		t.Fatalf("MINIGUN drains 1.25/s: %v", mini.counter)
	}
	if pistol.counter != 10 {
		t.Fatalf("pistol never drains: %v", pistol.counter)
	}
}

func runUntilBonus(tr *comboTracker, limit int) (*comboBonusEvent, int) {
	for i := 0; i < limit; i++ {
		if ev := tr.update(1.0/60, false); ev != nil {
			return ev, i
		}
	}
	return nil, limit
}

func TestComboReleasedTrackerShowsTierThenAddsToHUD(t *testing.T) {
	tr := newTestTracker("SHOTGUN", comboTiers())
	tr.counter = 35
	for i := 0; i < 5; i++ {
		tr.update(1.0/60, false)
	}
	if ev, _ := runUntilBonus(tr, 20); ev != nil || tr.state != comboActive {
		t.Fatal("a held tracker never delivers a bonus")
	}
	tr.held = false
	tr.update(1.0/60, false)
	if tr.text != "Good" || tr.state != comboHold || tr.bonus != 4 {
		t.Fatalf("released: text %q state %d bonus %d", tr.text, tr.state, tr.bonus)
	}
	ev, frames := runUntilBonus(tr, 600)
	if ev == nil || ev.amount != 4 {
		t.Fatalf("expected a +4 bonus, got %+v", ev)
	}
	// hold (0.375 s flash + 0.05 s) then a 1.5*0.75*t^2 flight: about 85 frames.
	if frames < 70 || frames > 100 {
		t.Fatalf("bonus arrived after %d frames", frames)
	}
	if tr.text != "+4" || tr.state != comboBonus {
		t.Fatalf("after arrival: %q state %d", tr.text, tr.state)
	}
	for i := 0; i < 200 && !tr.done; i++ {
		tr.update(1.0/60, false)
	}
	if !tr.done {
		t.Fatal("the +N text fades out and the tracker finishes")
	}
}

// The pistol and the sentry weapon never own a tracker (weapon vtable +0x2c
// returns 0 for their classes), so swapping the pistol out can never show
// "Epic Fail"; every other weapon does.
func TestComboOnlyWeaponsWithATrackerGetOne(t *testing.T) {
	for _, gun := range []string{"PISTOL", "SENTRY"} {
		if tr := newComboTracker(gun, 1, false, formats.WeaponCatalog{}); tr != nil {
			t.Fatalf("%s must not own a combo tracker", gun)
		}
	}
	for _, gun := range []string{"SHOTGUN", "UZI", "MINIGUN", "SNIPER", "FLAMER", "BUZZSAW", "DUALPISTOL", "GRENADE", "MINE", "BAZOOKA", "COWPATBOMB"} {
		if tr := newComboTracker(gun, 1, false, formats.WeaponCatalog{}); tr == nil {
			t.Fatalf("%s must own a combo tracker", gun)
		}
	}
}

func TestComboLowCountersGiveEpicFailOrFadingZero(t *testing.T) {
	low := newTestTracker("SHOTGUN", comboTiers())
	low.counter = 5 // above 1 but below the first threshold of 10
	low.held = false
	low.update(1.0/60, false)
	if low.text != comboEpicFail {
		t.Fatalf("below the first tier is Epic Fail, got %q", low.text)
	}
	if ev, _ := runUntilBonus(low, 300); ev != nil {
		t.Fatal("Epic Fail grants no multiplier")
	}
	if !low.done {
		t.Fatal("Epic Fail fades away after about a second")
	}
	// A counter below 1 is released straight into the fade state with its "x0" text
	// (FUN_00098d64 case 0 sets state 4 and leaves the text alone).
	empty := newTestTracker("SHOTGUN", comboTiers())
	empty.update(1.0/60, false)
	empty.held = false
	empty.update(1.0/60, false)
	popup, ok := empty.popup()
	if !ok || popup.Text != "x0" || empty.state != comboFade {
		t.Fatalf("released empty tracker: %+v ok=%v state %d", popup, ok, empty.state)
	}
	if ev, _ := runUntilBonus(empty, 300); ev != nil || !empty.done {
		t.Fatal("the fading x0 grants nothing and finishes")
	}
}

// A live tracker shows "x<count>" for as long as it lives; it does not fade by
// itself and it is drawn even at zero (FUN_00098d64 state 0, FUN_000993d0).
func TestComboLiveCountStaysVisible(t *testing.T) {
	tr := newTestTracker("SHOTGUN", comboTiers())
	tr.update(1.0/60, false)
	if popup, ok := tr.popup(); !ok || popup.Text != "x0" || popup.A != 255 {
		t.Fatalf("live count at zero: %+v %v", popup, ok)
	}
	tr.add(7.5)
	for i := 0; i < 60*30; i++ {
		tr.update(1.0/60, false)
	}
	if popup, ok := tr.popup(); !ok || popup.Text != "x7" || popup.A != 255 {
		t.Fatalf("a live count never fades: %+v %v", popup, ok)
	}
}

func TestComboPistolSwapIsSilent(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.updateCombo(1.0 / 60)
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	for _, popup := range p.combo.popups {
		if popup.text == comboEpicFail {
			t.Fatal("swapping the pistol out must not show Epic Fail")
		}
	}
}

func TestComboPlayerDeathEpicFailsAndResetsMultiplier(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.multiplier = 9
	p.updateCombo(1.0 / 60)
	p.combo.primary.counter = 40
	p.health = 0
	p.deathStarted = false
	p.updatePlayerDeath()
	if p.multiplier != 1 {
		t.Fatalf("death resets the multiplier to 1, got %d", p.multiplier)
	}
	p.updateCombo(1.0 / 60)
	if p.combo.primary != nil || len(p.combo.popups) < 2 {
		t.Fatal("death releases both trackers")
	}
	fails := 0
	for _, popup := range p.combo.popups {
		if popup.text == comboEpicFail {
			fails++
		}
	}
	if fails != 2 {
		t.Fatalf("every live tracker (primary and grenade slot) turns into Epic Fail on death, got %d", fails)
	}
	foundFail := false
	for _, popup := range p.combo.popups {
		if popup.text == comboEpicFail {
			foundFail = true
		}
	}
	if !foundFail {
		t.Fatal("expected an Epic Fail popup")
	}
	p.health = p.maxHealth
	p.updateCombo(1.0 / 60)
	if p.combo.primary == nil || p.combo.primary.counter != 0 {
		t.Fatal("a fresh tracker is created after respawn")
	}
}

func TestComboWeaponSwapReleasesTrackerAndRaisesHUD(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.multiplier = 1
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	tracker := p.combo.primary
	if tracker == nil || tracker.gun != "SHOTGUN" || len(tracker.tiers) != 5 {
		t.Fatalf("tracker %+v", tracker)
	}
	tracker.counter = 55
	for i := 0; i < 6; i++ {
		p.updateCombo(1.0 / 60)
	}
	p.collectPickup("p_shotgun") // picking the same gun again is a new pickup
	for i := 0; i < 400 && p.multiplier == 1; i++ {
		p.updateCombo(1.0 / 60)
	}
	if p.multiplier != 7 { // Great = +6 on top of x1
		t.Fatalf("multiplier %d, want 7", p.multiplier)
	}
	if p.combo.hudFlash <= 0 {
		t.Fatal("the HUD flashes on the bonus")
	}
	for _, name := range p.sfxQueue {
		if name == "SFX_MULTIPLIER_TICK_UP" {
			t.Fatal("no native caller plays SFX_MULTIPLIER_TICK_UP at the bonus")
		}
	}
	// Highest Multiplier is written by the per-frame stats pass.
	r.app.recordPlayStats(p, 0, 3)
	if r.app.statistics.HighestMultiplier != 7 || !r.app.statistics.Available["Highest Multiplier"] {
		t.Fatalf("highest multiplier %d", r.app.statistics.HighestMultiplier)
	}
}

func TestComboScoreUsesMultiplier(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.hudVisible = true
	p.multiplier = 5
	p.zombies = []zombieState{{health: -1, rawPoints: 100, dying: true, deathAge: zombieDeathDelay}}
	p.updateBulletsAndKills()
	if p.score != (100/20)*5 {
		t.Fatalf("score %d, want %d", p.score, 25)
	}
}

func TestComboBulletHitsAndKillsAreCredited(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	r.stack(1, p.x+20, p.y)
	p.fire(1, 0)
	r.tick(10)
	if p.combo.primary.counter < 1 {
		t.Fatalf("a bullet hit credits 1.01, counter %v", p.combo.primary.counter)
	}
	if p.achieve.best["combo"] < 1 {
		t.Fatal("the achievement value is the integer counter")
	}
	// A shot fired from an earlier pickup no longer feeds the new tracker.
	origin := p.primaryOrigin(1)
	p.collectPickup("p_uzi")
	p.updateCombo(1.0 / 60)
	before := p.combo.primary.counter
	p.comboCredit(origin, 5)
	if p.combo.primary.counter != before {
		t.Fatal("stale pickup credit must be ignored")
	}
	// Flame particles pay 0.01 per surviving hit (damage 1).
	p.comboBulletHit(bullet{origin: p.primaryOrigin(2), projectile: &weapons.NativeWeaponProjectile{EntityType: 0x16}}, 50, 1)
	p.comboBulletHit(bullet{origin: p.primaryOrigin(2), projectile: &weapons.NativeWeaponProjectile{EntityType: 0x16}}, 1, 1)
	if d := p.combo.primary.counter - before; d < 0.0099 || d > 0.0101 {
		t.Fatalf("flame credit %v", d)
	}
	// Bullets credit 1.01 for every native 101-damage hit the zombie would have
	// taken, lethal one included: 100 health = 1 hit, 300 = 3, 3000 hit with the
	// port's 500 damage = 5 (of the 30 native hits).
	for _, c := range []struct{ health, damage, want float64 }{{100, 500, 1.01}, {300, 500, 3.03}, {3000, 500, 5.05}, {101, 500, 1.01}, {102, 500, 2.02}} {
		p.collectPickup("p_uzi")
		p.updateCombo(1.0 / 60)
		p.comboBulletHit(bullet{origin: p.primaryOrigin(3)}, c.health, c.damage)
		if got := float64(p.combo.primary.counter); math.Abs(got-c.want) > 1e-4 {
			t.Fatalf("bullet credit health %v damage %v = %v, want %v", c.health, c.damage, got, c.want)
		}
	}
	// A blast pays 0.05 for every 5-damage tick that leaves the zombie alive: 0.95 for 100 health.
	if p.combo.secondary == nil {
		t.Fatal("the grenade slot owns a tracker")
	}
	p.combo.secondary.counter = 0
	p.comboBlastKill(killOrigin{gun: "GRENADE", shot: 1}, 100)
	if got := float64(p.combo.secondary.counter); math.Abs(got-0.95) > 1e-4 {
		t.Fatalf("blast kill credit %v, want 0.95", got)
	}
	p.combo.secondary.counter = 0
	p.comboBlastKill(killOrigin{gun: "GRENADE", shot: 1}, 300)
	if got := float64(p.combo.secondary.counter); math.Abs(got-2.95) > 1e-3 {
		t.Fatalf("blast kill credit for 300 health %v, want 2.95", got)
	}
	// Enemy-owned blasts and trains have no tracker.
	before = p.combo.primary.counter
	p.comboCredit(killOrigin{gun: "EXPZOMBIE"}, 5)
	p.comboCredit(killOrigin{gun: "TRAIN"}, 5)
	if d := p.combo.primary.counter - before; d > 0.0101 {
		t.Fatalf("hazard credit leaked into the tracker: %v", d)
	}
}

// SUPERSIZED COMBO is the integer counter of one weapon tracker reaching 100.
func TestSupersizedComboNeedsHundredOnOneTracker(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := pack.Achievements()
		if err != nil {
			t.Fatal(err)
		}
		weapons, err := pack.Weapons()
		if err != nil {
			t.Fatal(err)
		}
		var entry formats.Achievement
		for _, candidate := range catalog {
			if candidate.Type == "SPECIFIC" && candidate.SpecificType == "combo" {
				entry = candidate
			}
		}
		if entry.Total != 100 {
			t.Fatalf("%s: combo achievement %+v", root, entry)
		}
		r := newAchievementRig(t, formats.AchievementCatalog{entry}, weapons)
		r.p.collectPickup("p_shotgun")
		r.p.updateCombo(1.0 / 60)
		// 99 kills on the pickup, then the pickup is swapped: the new tracker starts at 0.
		for i := 0; i < 99; i++ {
			r.p.comboCredit(r.p.primaryOrigin(i), 1)
		}
		r.tick(2)
		if r.unlocked(entry.ID) || r.p.achieve.best["combo"] != 99 {
			t.Fatalf("%s: 99 must not unlock (best %d)", root, r.p.achieve.best["combo"])
		}
		r.p.collectPickup("p_shotgun")
		r.p.updateCombo(1.0 / 60)
		for i := 0; i < 99; i++ {
			r.p.comboCredit(r.p.primaryOrigin(i), 1)
		}
		r.tick(2)
		if r.unlocked(entry.ID) {
			t.Fatalf("%s: counters of different pickups must not be added", root)
		}
		r.p.comboCredit(r.p.primaryOrigin(1), 1)
		r.tick(2)
		if !r.unlocked(entry.ID) {
			t.Fatalf("%s: 100 on one pickup unlocks SUPERSIZED COMBO", root)
		}
	}
}

// Both content caches carry the same tier tables (pistol has none, dual pistols
// end in CRACK SHOT!, the flamer has a sixth INCENDIARY tier).
func TestComboTierTablesInBothCaches(t *testing.T) {
	roots := achievementCaches()
	if len(roots) == 0 {
		t.Skip("original asset cache unavailable")
	}
	for _, root := range roots {
		pack, err := content.NewPack(content.NewSource(root))
		if err != nil {
			t.Fatal(err)
		}
		weapons, err := pack.Weapons()
		if err != nil {
			t.Fatal(err)
		}
		check := func(gun string, count int, last string, lastMultiplier int) {
			w, ok := weapons.Find(gun)
			if !ok {
				t.Logf("%s: %s not in this cache", root, gun)
				return
			}
			if len(w.ScoreMultipliers) != count {
				t.Fatalf("%s %s: %d tiers, want %d", root, gun, len(w.ScoreMultipliers), count)
			}
			if count > 0 {
				got := w.ScoreMultipliers[count-1]
				if got.Text != last || got.Multiplier != lastMultiplier {
					t.Fatalf("%s %s: last tier %+v", root, gun, got)
				}
			}
		}
		check("PISTOL", 0, "", 0)
		check("SHOTGUN", 5, "AWESOME", 8)
		check("FLAMER", 6, "INCENDIARY", 12)
		check("SNIPER", 5, "AWESOME", 8)
		check("GRENADE", 4, "AWESOME", 8)
		check("DUALPISTOL", 5, "CRACK SHOT!", 8)
		for _, gun := range []string{"SHOTGUN", "UZI", "MINIGUN", "SNIPER", "FLAMER", "BUZZSAW", "DUALPISTOL", "GRENADE", "MINE", "BAZOOKA"} {
			if tr := newComboTracker(gun, 1, false, weapons); tr != nil && len(tr.tiers) == 0 && gun != "BUZZSAW" && gun != "DUALPISTOL" {
				t.Fatalf("%s: %s has no tiers", root, gun)
			}
		}
	}
}

func TestComboWireRoundTrip(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.updateCombo(1.0 / 60)
	p.combo.primary.counter = 35
	for i := 0; i < 4; i++ {
		p.updateCombo(1.0 / 60)
	}
	p.collectPickup("p_uzi")
	p.updateCombo(1.0 / 60)
	p.multiplier = 5
	snapshot := p.snapshot(1)
	hasGood := func(list []wireCombo) bool {
		for _, w := range list {
			if w.Text == "Good" {
				return true
			}
		}
		return false
	}
	if snapshot.Multiplier != 5 || !hasGood(snapshot.Combo) {
		t.Fatalf("snapshot %+v", snapshot.Combo)
	}
	guest := &playState{weapons: p.weapons}
	guest.applySnapshot(snapshot)
	popups := guest.comboPopups()
	found := false
	for _, popup := range popups {
		found = found || popup.Text == "Good"
	}
	if guest.multiplier != 5 || !found {
		t.Fatalf("guest popups %+v mult %d", popups, guest.multiplier)
	}
}

// The live count sits behind the player: player minus 2*size along the aim
// direction (y squashed by 0.666), lifted 25 world units (FUN_00098d64).
func TestRunningCountSitsBehindThePlayer(t *testing.T) {
	p := &playState{world: &viewer.Viewer{Zoom: 1}, x: 300, y: 200}
	p.angle, p.flipX = barryDirection(1, 0) // aiming right
	x, y := p.comboAnchor(20)
	if math.Abs(x-(300-2*20)) > 1e-6 || math.Abs(y-(200-25)) > 1e-6 {
		t.Fatalf("aiming right: anchor %.3f,%.3f", x, y)
	}
	p.angle, p.flipX = barryDirection(-1, 0) // aiming left
	x, _ = p.comboAnchor(20)
	if math.Abs(x-(300+2*20)) > 1e-6 {
		t.Fatalf("aiming left: anchor x %.3f", x)
	}
	p.angle, p.flipX = barryDirection(0, 1) // aiming down
	x, y = p.comboAnchor(20)
	if math.Abs(x-300) > 1e-6 || math.Abs(y-(200-2*20*0.666-25)) > 1e-6 {
		t.Fatalf("aiming down: anchor %.3f,%.3f", x, y)
	}
	// The camera maps world units to screen pixels.
	p.world = &viewer.Viewer{Zoom: 2, CameraX: 100, CameraY: 50, ViewportX: 3}
	p.angle, p.flipX = barryDirection(1, 0)
	x, y = p.comboAnchor(20)
	if math.Abs(x-((300-40-100)*2+3)) > 1e-6 || math.Abs(y-((200-25-50)*2)) > 1e-6 {
		t.Fatalf("camera mapping: anchor %.3f,%.3f", x, y)
	}
}

// The fly target is x 240 and one glyph below the HUD multiplier's centre
// (10 + glyph/2); the upper clamp stays just under the HUD multiplier.
func TestComboFlyTargetAndTopBound(t *testing.T) {
	x, y := comboFlyTarget(28)
	if x != 240 || y != 10+14+28 {
		t.Fatalf("target %v,%v", x, y)
	}
	x, y = comboFlyTarget(48)
	if x != 240 || y != 10+24+48 {
		t.Fatalf("grown HUD target %v,%v", x, y)
	}
	if got := comboTopBound(28, 40); got != 10+28+2+20 {
		t.Fatalf("top bound %v", got)
	}
	c := &comboSystem{}
	if c.comboHudGlyph(0) != 28 || c.comboHudGlyph(1) != 28 {
		t.Fatal("an unset HUD glyph is the 28 minimum")
	}
	c.hudScale = 40
	if c.comboHudGlyph(5) != 40 || c.comboHudGlyph(0) != 28 {
		t.Fatalf("glyph %v / %v", c.comboHudGlyph(5), c.comboHudGlyph(0))
	}
}

func TestComboInitialSizeIs24(t *testing.T) {
	tr := newComboTracker("SHOTGUN", 1, false, formats.WeaponCatalog{})
	if tr.size != 24 {
		t.Fatalf("FUN_00098ca0 stores 24.0 in +0x28/+0x30, got %v", tr.size)
	}
}
