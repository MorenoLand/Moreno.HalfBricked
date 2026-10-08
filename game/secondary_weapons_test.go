package game

import (
	"math"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func firstFreeRoom(t *testing.T) (*playState, float64, float64) {
	t.Helper()
	p := script125CachedHost(t, "world0_level0").play
	return p, p.x, p.y
}

func sentryTarget(p *playState, s sentryState, distance float64) {
	p.zombies = []zombieState{{x: s.x + distance, y: s.y, health: 1e9, size: formats.Vec2{X: 48, Y: 48}}}
}

func TestSentryGunsCloneThePlayersWeapons(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	want := map[string]string{"SENTRY_SHOTGUN": "SHOTGUN", "SENTRY_UZI": "UZI", "SENTRY_FLAMER": "FLAMER", "SENTRY_BAZOOKA": "BAZOOKA"}
	for variant, gun := range want {
		p.sentries = nil
		if !p.deploySentry(x, y, variant) {
			t.Fatalf("%s refused", variant)
		}
		s := p.sentries[0]
		record, _ := p.weapons.Find(gun)
		if s.weapon.GunType != gun || s.ammo != record.Ammo {
			t.Fatalf("%s: gun %q ammo %d, want the %s record with %d rounds", variant, s.weapon.GunType, s.ammo, gun, record.Ammo)
		}
		if math.Abs(s.rangePx-record.Life*record.Speed*.5) > 1e-3 {
			t.Fatalf("%s: range %v, want Life*Speed*.5 = %v", variant, s.rangePx, record.Life*record.Speed*.5)
		}
	}
}

func TestSentryDropsItsHeadThenFiresOnlyAtTargetsInRange(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.deploySentry(x, y, "SENTRY_UZI")
	sentryTarget(p, p.sentries[0], 100)
	for frame := 0; frame < 44; frame++ {
		p.updateSentries()
	}
	if p.sentries[0].state != sentryStateDeploy || len(p.bullets) != 0 {
		t.Fatalf("still deploying at 44 frames: state=%d bullets=%d", p.sentries[0].state, len(p.bullets))
	}
	p.updateSentries()
	p.updateSentries()
	if p.sentries[0].state != sentryStateActive || p.sentries[0].lift != sentryRestLift {
		t.Fatalf("head not raised: state=%d lift=%v", p.sentries[0].state, p.sentries[0].lift)
	}
	for frame := 0; frame < 240 && len(p.bullets) == 0; frame++ {
		p.updateSentries()
	}
	if len(p.bullets) == 0 {
		t.Fatal("sentry never fired at a zombie inside its range")
	}
	// Outside Life*Speed*.5 nothing is acquired.
	p.sentries = nil
	p.bullets = nil
	p.deploySentry(x, y, "SENTRY_UZI")
	sentryTarget(p, p.sentries[0], p.sentries[0].rangePx+1)
	for frame := 0; frame < 300; frame++ {
		p.updateSentries()
	}
	if len(p.bullets) != 0 || p.sentries[0].hasTarget {
		t.Fatal("sentry shot at a zombie beyond its range")
	}
}

// The lifetime is the cloned weapon's ammo: one round per shot, then 1 s of
// dying and a harmless blast.
func TestSentryDiesWhenItsAmmoRunsOut(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.deploySentry(x, y, "SENTRY_BAZOOKA")
	record, _ := p.weapons.Find("BAZOOKA")
	sentryTarget(p, p.sentries[0], 150)
	p.sfxQueue = nil
	frames := 0
	for ; len(p.sentries) > 0 && frames < 60*60; frames++ {
		p.updateSentries()
	}
	if len(p.sentries) != 0 {
		t.Fatal("sentry never ran out of ammo")
	}
	if len(p.bullets) != record.Ammo {
		t.Fatalf("fired %d rockets, want %d (one per ammo)", len(p.bullets), record.Ammo)
	}
	if !hasSfx(p, "SFX_SENTRY_DEATH") || !hasSfx(p, "SFX_MINE_EXPLODE") || len(p.zombieBlasts) != 1 {
		t.Fatalf("death: sfx=%v blasts=%d", p.sfxQueue, len(p.zombieBlasts))
	}
	if blast := p.zombieBlasts[0]; !blast.harmless || blast.origin.gun != "SENTRY" {
		t.Fatalf("death blast must be a harmless sentry-owned blast: %+v", blast)
	}
	minimum := int((float64(record.Ammo-1)*record.RateOfFire + sentryDyingTime + 0.75) * 60)
	if frames < minimum {
		t.Fatalf("lived %d frames, less than deploy + %d rounds at %v s + 1 s dying", frames, record.Ammo, record.RateOfFire)
	}
}

func TestSentryKeepsItsTargetUntilItDiesOrLeavesTheRange(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.deploySentry(x, y, "SENTRY_UZI")
	p.sentries[0].state, p.sentries[0].lift = sentryStateActive, sentryRestLift
	p.zombies = []zombieState{{x: x + 150, y: y, health: 1e9, size: formats.Vec2{X: 48, Y: 48}}}
	p.updateSentries()
	if !p.sentries[0].hasTarget || p.sentries[0].targetIdx != 0 {
		t.Fatal("no target acquired")
	}
	p.zombies = append(p.zombies, zombieState{x: x + 40, y: y + 10, health: 1e9, size: formats.Vec2{X: 48, Y: 48}})
	p.updateSentries()
	if p.sentries[0].targetIdx != 0 {
		t.Fatal("a nearer zombie must not steal a held target (the search only runs without one)")
	}
	p.zombies[0].health = 0
	p.updateSentries()
	p.updateSentries()
	if !p.sentries[0].hasTarget || p.sentries[0].targetIdx != 1 {
		t.Fatalf("dead target should be replaced by the nearest: has=%v idx=%d", p.sentries[0].hasTarget, p.sentries[0].targetIdx)
	}
}

func TestSentryFacingColumnsAndMirroredCells(t *testing.T) {
	if sentryColumn(0x47ce) != 0 {
		t.Fatal("start angle 0x47ce must be column 0")
	}
	at := func(offset int) int { return sentryColumn(uint16(0x47ce + offset)) }
	if at(-0x1000) != 1 || at(-0x8000) != 8 || at(0x1000) != 15 {
		t.Fatalf("columns: %d %d %d", at(-0x1000), at(-0x8000), at(0x1000))
	}
	for column, want := range map[int][2]int{0: {0, 0}, 8: {8, 0}, 9: {7, 1}, 15: {1, 1}} {
		cell, mirrored := sentryCell(column)
		flag := 0
		if mirrored {
			flag = 1
		}
		if cell != want[0] || flag != want[1] {
			t.Fatalf("column %d -> cell %d mirrored %v", column, cell, mirrored)
		}
	}
	// 10% per frame, truncated toward zero, so it stops within 9 units.
	if step := int(float32(angleDifference(100, 0)) * sentryTurnRate); step != 10 {
		t.Fatalf("turn step %d", step)
	}
	if step := int(float32(angleDifference(9, 0)) * sentryTurnRate); step != 0 {
		t.Fatalf("a 9 unit error should not turn, got %d", step)
	}
}

func TestPlainSentryPickupRollsOneOfFourVariants(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	seen := map[string]bool{}
	for i := 0; i < 80; i++ {
		p.sentries = nil
		p.collectPickup("p_sentry")
		if !p.fireSecondary(1, 0) {
			t.Fatal("deploy refused")
		}
		seen[p.sentries[0].weapon.GunType] = true
		p.x, p.y = x, y
		p.secondaryShootCooldown = 0
	}
	if len(seen) != 4 {
		t.Fatalf("variants seen %v, want all of SHOTGUN UZI FLAMER BAZOOKA", seen)
	}
}

// ---- blasts ------------------------------------------------------------------

func TestPlayerBlastsHurtZombiesFivePerTickAndNeverThePlayer(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.health = 1
	p.x, p.y = x, y
	p.zombies = []zombieState{{x: x + 20, y: y, health: 100000, size: formats.Vec2{X: 48, Y: 48}}}
	before := p.zombies[0].health
	p.spawnBlast(x+20, y, zombieBlastSound, killOrigin{gun: "GRENADE", shot: 7}, true)
	p.updateZombieBlasts()
	lost := before - p.zombies[0].health
	if lost <= 0 || math.Mod(lost, 5) != 0 {
		t.Fatalf("first tick damage %v, want a multiple of 5 (one takeDamage(5) per grid listing)", lost)
	}
	for i := 0; i < 80; i++ {
		p.updateZombieBlasts()
	}
	if p.health != 1 {
		t.Fatalf("a player-fired blast must not hurt the player (hit-player 0), health %v", p.health)
	}
	if len(p.zombieBlasts) != 0 {
		t.Fatal("the blast should end after .75 s")
	}
}

func TestBlastKillIsCreditedToTheFiringWeapon(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.zombies = []zombieState{{x: x + 30, y: y, health: 100, size: formats.Vec2{X: 48, Y: 48}}}
	p.spawnBlast(x+30, y, zombieBlastSound, killOrigin{gun: "MINE", shot: 9}, true)
	for i := 0; i < 50 && !p.zombies[0].dying; i++ {
		p.updateZombieBlasts()
	}
	if !p.zombies[0].dying || p.achieve.counts[scopeKey{"mine", 9}] != 1 {
		t.Fatalf("mine kill not credited: dying=%v counts=%v", p.zombies[0].dying, p.achieve.counts)
	}
}

func TestExplodingZombieBlastStillHurtsThePlayer(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.x, p.y = x, y
	p.health = 1
	p.spawnBlast(x, y, zombieBlastSound, killOrigin{gun: "EXPZOMBIE", shot: 3}, false)
	for i := 0; i < 20; i++ {
		p.updateZombieBlasts()
	}
	if p.health >= 1 {
		t.Fatal("the exploding zombie sets hit-player and must hurt Barry")
	}
}

// ---- thrown bombs --------------------------------------------------------------

func throwOnOpenGround(t *testing.T, dynamite bool) (*playState, float64, float64) {
	t.Helper()
	p, x, y := firstFreeRoom(t)
	dx, dy := openDirection(p, x, y, 330)
	p.thrown = nil
	p.throwBomb(x, y, dx, dy, 350, 1, dynamite, killOrigin{gun: "GRENADE", shot: 5})
	return p, dx, dy
}

func TestGrenadeLandsBouncesAndDetonatesAfterTheThirdBounce(t *testing.T) {
	p, _, _ := throwOnOpenGround(t, false)
	if b := p.thrown[0]; b.lift != 20 || b.fall != -70 || b.life != 2 || b.size != 20 {
		t.Fatalf("launch state %+v", b)
	}
	landed, bounces := -1, 0
	last := p.thrown[0].speed
	for frame := 0; len(p.thrown) > 0 && frame < 600; frame++ {
		p.updateThrown()
		if len(p.thrown) == 0 {
			break
		}
		if b := p.thrown[0]; b.speed < last-1e-9 {
			bounces++
			if landed < 0 {
				landed = frame + 1
			}
			if math.Abs(b.speed-last*.5) > 1e-6 {
				t.Fatalf("bounce %d: speed %v, want half of %v", bounces, b.speed, last)
			}
		}
		last = p.thrown[0].speed
	}
	// z(t) = 20 + 70t - 250t^2 reaches 0 at t = 0.456 s = frame 28.
	if landed < 27 || landed > 29 {
		t.Fatalf("first landing at frame %d, want about 28", landed)
	}
	if bounces != 3 || len(p.zombieBlasts) != 1 || !hasSfx(p, "SFX_MINE_EXPLODE") {
		t.Fatalf("bounces=%d blasts=%d sfx=%v: 175 -> 87.5 -> 43.75 < 52.5 detonates the update after the third landing", bounces, len(p.zombieBlasts), p.sfxQueue)
	}
	if !hasSfx(p, thrownBounceSound) {
		t.Fatal("a landing at fall speed > 70 plays sound 0x13")
	}
}

func TestGrenadeDoesNotExplodeOnZombieContact(t *testing.T) {
	p, dx, dy := throwOnOpenGround(t, false)
	b := p.thrown[0]
	p.zombies = []zombieState{{x: b.x + dx*60, y: b.y + dy*60, health: 1e9, size: formats.Vec2{X: 48, Y: 48}}}
	startSpeed := b.speed
	for frame := 0; frame < 12; frame++ {
		p.updateThrown()
	}
	if len(p.thrown) != 1 || len(p.zombieBlasts) != 0 {
		t.Fatalf("contact exploded the grenade: thrown=%d blasts=%d", len(p.thrown), len(p.zombieBlasts))
	}
	if p.thrown[0].speed >= startSpeed {
		t.Fatalf("the bounce flag slows it to .95 per listing over a zombie, speed %v", p.thrown[0].speed)
	}
}

func TestDynamiteSpawnsThreeChildBombsThenBlasts(t *testing.T) {
	p, _, _ := throwOnOpenGround(t, true)
	if p.thrown[0].size != 32 || !p.thrown[0].dynamite {
		t.Fatalf("dynamite state %+v", p.thrown[0])
	}
	for frame := 0; frame < 600 && len(p.zombieBlasts) == 0; frame++ {
		p.updateThrown()
	}
	if len(p.zombieBlasts) != 1 || len(p.thrown) != dynamiteChildren {
		t.Fatalf("blasts=%d children=%d, want 1 blast and %d children", len(p.zombieBlasts), len(p.thrown), dynamiteChildren)
	}
	for _, child := range p.thrown {
		if !child.child || child.bounce || child.lift != dynamiteChildLift || child.fall != dynamiteChildFall {
			t.Fatalf("child state %+v", child)
		}
		if child.speed < 175 || child.speed >= 525 || child.life < 2 || child.life >= 10 {
			t.Fatalf("child speed %v life %v outside the native ranges [175,525) and [2,10)", child.speed, child.life)
		}
	}
	for frame := 0; frame < 900 && len(p.thrown) > 0; frame++ {
		p.updateThrown()
	}
	if len(p.thrown) != 0 || len(p.zombieBlasts) != 1+dynamiteChildren {
		t.Fatalf("children should each blast once and spawn nothing: thrown=%d blasts=%d", len(p.thrown), len(p.zombieBlasts))
	}
}

// ---- rocket contact ----------------------------------------------------------------

func TestRocketContactHurtsFiveBeforeTheBlast(t *testing.T) {
	p, x, y := firstFreeRoom(t)
	p.zombies = []zombieState{{x: x, y: y, health: 100, size: formats.Vec2{X: 48, Y: 48}}}
	p.rocketContactDamage(bullet{x: x, y: y, origin: killOrigin{gun: "ROCKET", shot: 2}}, &p.zombies[0])
	lost := 100 - p.zombies[0].health
	if lost <= 0 || math.Mod(lost, 5) != 0 {
		t.Fatalf("contact damage %v, want a multiple of 5", lost)
	}
}

// ---- shield -------------------------------------------------------------------------

func TestShieldPickupLastsFifteenSecondsAndBlocksAllPlayerDamage(t *testing.T) {
	p, _, _ := firstFreeRoom(t)
	p.health = 1
	p.collectPickup("p_shield")
	if p.achieve.shieldTimer != 15 {
		t.Fatalf("shield timer %v, want 15 (DAT_00092d34)", p.achieve.shieldTimer)
	}
	p.achieve.shieldTimer = 4
	p.collectPickup("p_shield")
	if p.achieve.shieldTimer != 15 {
		t.Fatal("a second pickup replaces the timer instead of adding to it")
	}
	p.damagePlayer(0, .5)
	if p.health != 1 {
		t.Fatalf("shielded player lost health: %v", p.health)
	}
	p.spawnBlast(p.x, p.y, zombieBlastSound, killOrigin{gun: "EXPZOMBIE", shot: 1}, false)
	for i := 0; i < 20; i++ {
		p.updateZombieBlasts()
	}
	if p.health != 1 {
		t.Fatal("blast damage must also be blocked")
	}
	found := false
	for _, path := range p.pickupVoices {
		if strings.HasSuffix(path, "Shield.ogg") {
			found = true
		}
	}
	if !found {
		t.Fatalf("SFX_VO_SHIELD voice not queued: %v", p.pickupVoices)
	}
}

func TestShieldSpriteFramesAndFinalBlink(t *testing.T) {
	if shieldFrame(15) != 0 || shieldFrame(14.9) != 3 || shieldFrame(14.8) != 2 || shieldFrame(14.74) != 1 || shieldFrame(14.6) != 0 {
		t.Fatalf("frames %d %d %d %d %d", shieldFrame(15), shieldFrame(14.9), shieldFrame(14.8), shieldFrame(14.74), shieldFrame(14.6))
	}
	if !shieldVisible(10) || !shieldVisible(3.05) {
		t.Fatal("solid above 3 s")
	}
	if shieldVisible(2.95) == shieldVisible(2.85) {
		t.Fatal("must alternate every .1 s in the last 3 s")
	}
}
