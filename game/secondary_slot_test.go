package game

import (
	"math"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestSecondaryPickupsReplaceTheSlot(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	for _, tc := range []struct {
		pickup, slot string
		ammo         int
	}{
		{"p_grenade", secondaryGrenade, 5},
		{"p_mine", secondaryMine, 3},
		{"p_bazooka", secondaryBazooka, 5},
		{"p_sentry", secondarySentry, 1},
		{"p_sentryFlamer", secondarySentry, 1},
	} {
		p.collectPickup(tc.pickup)
		if p.secondaryType != tc.slot || p.grenades != tc.ammo {
			t.Fatalf("%s: slot %q ammo %d, want %q %d", tc.pickup, p.secondaryType, p.grenades, tc.slot, tc.ammo)
		}
	}
	if p.secondaryWeapon != "SENTRY_FLAMER" {
		t.Fatalf("sentry variant %q", p.secondaryWeapon)
	}
	if p.weapon.GunType == "BAZOOKA" || p.weapon.GunType == "SENTRY" {
		t.Fatal("secondary pickups must not replace the primary weapon")
	}
}

// openDirection finds a unit direction with `length` px of free ground ahead of
// (x, y) so physics tests do not depend on the level's walls.
func openDirection(p *playState, x, y, length float64) (float64, float64) {
	for step := 0; step < 16; step++ {
		angle := float64(step) * math.Pi / 8
		dx, dy := math.Cos(angle), math.Sin(angle)
		free := true
		for d := 0.0; d <= length; d += 8 {
			if p.isSolid(x+dx*d, y+dy*d) {
				free = false
				break
			}
		}
		if free {
			return dx, dy
		}
	}
	return 1, 0
}

func hasSfx(p *playState, symbol string) bool {
	for _, queued := range p.sfxQueue {
		if queued == symbol {
			return true
		}
	}
	return false
}

// FUN_000a5e7c: a mine detonates when its age reaches the XML Life (1.5 s). The
// native class has no proximity trigger, so a zombie standing on it changes
// nothing and the mine does not wait for one.
func TestMineIsATimeBombWithNoProximityTrigger(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.collectPickup("p_mine")
	p.x, p.y = 300, 300
	if !p.fireSecondary(1, 0) || len(p.mines) != 1 {
		t.Fatal("mine not planted")
	}
	mine := p.mines[0]
	if mine.life != 1.5 {
		t.Fatalf("mine life %v, want the XML Life 1.5", mine.life)
	}
	p.zombies = []zombieState{{x: mine.x, y: mine.y, health: 1e9, size: formats.Vec2{X: 48, Y: 48}}}
	for frame := 0; frame < 89; frame++ {
		p.updateMines()
	}
	if len(p.mines) != 1 || len(p.zombieBlasts) != 0 {
		t.Fatalf("mine went off early or on contact: mines=%d blasts=%d", len(p.mines), len(p.zombieBlasts))
	}
	p.updateMines()
	if len(p.mines) != 0 || len(p.zombieBlasts) != 1 || !hasSfx(p, "SFX_MINE_EXPLODE") {
		t.Fatalf("mine should blast at 1.5 s: mines=%d blasts=%d sfx=%v", len(p.mines), len(p.zombieBlasts), p.sfxQueue)
	}
}

// FUN_000a5900: the rocket starts at age Life*.5, so it flies Life*.5 (0.5 s),
// and every native explosion starts through FUN_000a3ca8 = "mine_explode".
func TestBazookaRocketFliesHalfItsLifeAndExplodesWithTheMineSound(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.collectPickup("p_bazooka")
	if !p.fireSecondary(1, 0) || len(p.bullets) != 1 || !p.bullets[0].explodes() || p.grenades != 4 {
		t.Fatalf("rocket: bullets=%d ammo=%d", len(p.bullets), p.grenades)
	}
	weapon, _ := p.weapons.Find("BAZOOKA")
	if got := p.bullets[0].life; math.Abs(got-weapon.Life*.5) > 1e-9 {
		t.Fatalf("rocket flight %v, want Life*.5 = %v", got, weapon.Life*.5)
	}
	if p.bullets[0].explosionSound() != "SFX_MINE_EXPLODE" {
		t.Fatalf("rocket explosion sound %q", p.bullets[0].explosionSound())
	}
}

func TestSentryDeploysAtTheWeaponPositionAndPlaysItsSpawnSound(t *testing.T) {
	p := script125CachedHost(t, "world0_level0").play
	p.x, p.y = 300, 300
	p.collectPickup("p_sentryUzi")
	if !p.fireSecondary(1, 0) || len(p.sentries) != 1 || p.grenades != 0 {
		t.Fatalf("deploy: sentries=%d slot ammo=%d", len(p.sentries), p.grenades)
	}
	if s := p.sentries[0]; s.x != 300 || s.y != 300 {
		t.Fatalf("1.2.5 FUN_0010e0f8 places the turret at the weapon position, got %v,%v", s.x, s.y)
	}
	if !hasSfx(p, "SFX_SENTRY_SPAWN") {
		t.Fatalf("sentry spawn silent: %v", p.sfxQueue)
	}
}

func TestCowPatBombIsAThrownDynamiteWithThreeRounds(t *testing.T) {
	p := script125CachedHost(t, "world0_level1").play
	p.closeScript()
	p.collectPickup("p_cow_pat")
	if p.secondaryType != secondaryCowpat || p.grenades != 3 {
		t.Fatalf("slot %q ammo %d, want COWPATBOMB with 3 rounds", p.secondaryType, p.grenades)
	}
	if !p.fireSecondary(1, 0) || p.grenades != 2 || len(p.thrown) != 1 || !p.thrown[0].dynamite || len(p.bullets) != 0 {
		t.Fatal("the dynamite should be a thrown bouncing bomb, not a bullet")
	}
	if p.thrown[0].life != 2 || p.thrown[0].lift != 20 || p.thrown[0].fall != -70 {
		t.Fatalf("native launch lift 20, fall -70, fuse 2*Life: %+v", p.thrown[0])
	}
}
