package game

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// FUN_000f1990 (1.2.5): a primary-weapon kill counts for the achievement scopes
// only while the weapon that fired is the one equipped now, never for the
// pistol, and the counter is zeroed by every primary equip (FUN_000f1bcc).
func TestPrimaryKillsCountOnlyForTheEquippedWeapon(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_shotgun")
	p.creditKill(killOrigin{gun: "UZI", shot: 1, pickup: p.achieve.pickupSerial})
	p.creditKill(killOrigin{gun: "PISTOL", shot: 2, pickup: p.achieve.pickupSerial})
	if p.achieve.best["smg"] != 0 || p.achieve.best["shotgun"] != 0 {
		t.Fatalf("kills by a weapon that is not equipped must not count: %v", p.achieve.best)
	}
	p.creditKill(killOrigin{gun: "SHOTGUN", shot: 3, pickup: p.achieve.pickupSerial})
	p.creditKill(killOrigin{gun: "SHOTGUN", shot: 4, pickup: p.achieve.pickupSerial})
	if p.achieve.best["shotgun"] != 2 {
		t.Fatalf("two shotgun kills over two shots: %v", p.achieve.best)
	}
}

// The rifle is a primary weapon like the others: BOOM HEADSHOT counts the kills
// of one pickup, not of one shot.
func TestRifleKillsAccumulateOverShotsOfOnePickup(t *testing.T) {
	r := testRig(t)
	p := r.p
	p.collectPickup("p_sniper")
	for shot := 1; shot <= 4; shot++ {
		for i := 0; i < 3; i++ {
			p.creditKill(killOrigin{gun: "SNIPER", shot: shot, pickup: p.achieve.pickupSerial})
		}
	}
	if p.achieve.best["rifle"] != 12 {
		t.Fatalf("12 rifle kills over 4 shots on one pickup: %v", p.achieve.best)
	}
	p.collectPickup("p_sniper")
	p.creditKill(killOrigin{gun: "SNIPER", shot: 9, pickup: p.achieve.pickupSerial})
	if p.achieve.counts[scopeKey{"rifle", p.achieve.pickupSerial}] != 1 || p.achieve.best["rifle"] != 12 {
		t.Fatalf("a new pickup restarts the counter: %v", p.achieve.counts)
	}
}

// +0xa4: a streak of credited kills made while the shield timer is above 0; an
// unshielded credited kill and every primary equip zero it.
func TestShieldKillsAreAStreak(t *testing.T) {
	r := testRig(t)
	p := r.p
	kill := func() { p.creditKill(killOrigin{gun: "BUZZSAW", shot: 1, pickup: p.achieve.pickupSerial}) }
	p.collectPickup("p_buzzsaw")
	p.collectPickup("p_shield")
	kill()
	kill()
	kill()
	if p.achieve.shieldKills["buzzsaw"] != 3 {
		t.Fatalf("three shielded kills: %v", p.achieve.shieldKills)
	}
	p.achieve.shieldTimer = 0
	kill() // unshielded: the streak restarts
	p.collectPickup("p_shield")
	kill()
	kill()
	if p.achieve.shieldStreak != 2 || p.achieve.shieldKills["buzzsaw"] != 3 {
		t.Fatalf("streak %d best %d: an unshielded kill resets the streak", p.achieve.shieldStreak, p.achieve.shieldKills["buzzsaw"])
	}
	p.collectPickup("p_buzzsaw") // a new pickup zeroes the streak as well
	p.collectPickup("p_shield")
	kill()
	if p.achieve.shieldStreak != 1 {
		t.Fatalf("a primary equip zeroes the streak, got %d", p.achieve.shieldStreak)
	}
}

// Accuracy: shots rise once per firing, hits once per projectile (scene +0x45178
// via the projectile method at 1.2.5 0x00104564), so a volley can exceed 100%.
func TestAccuracyCountsHitsPerProjectile(t *testing.T) {
	r := testRig(t)
	p := r.p
	origin := killOrigin{gun: "SHOTGUN", shot: 7}
	a, b := &weapons.NativeWeaponProjectile{}, &weapons.NativeWeaponProjectile{}
	p.markProjectileHit(a, origin)
	p.markProjectileHit(a, origin)
	p.markProjectileHit(b, origin)
	if p.achieve.hits != 2 {
		t.Fatalf("two projectiles of one shot hit: %d", p.achieve.hits)
	}
	p.markProjectileHit(nil, killOrigin{gun: "SHOTGUN", shot: 8})
	p.markProjectileHit(nil, killOrigin{gun: "SHOTGUN", shot: 8})
	if p.achieve.hits != 3 {
		t.Fatalf("a projectile without identity counts once per shot: %d", p.achieve.hits)
	}
	p.markProjectileHit(a, killOrigin{gun: "GRENADE", shot: 9})
	if p.achieve.hits != 3 {
		t.Fatalf("secondary weapons are not counted: %d", p.achieve.hits)
	}
}

// The unlock jingle is the shipped Achievement_unlocked.ogg (SFX_ACHIEVEMENT_UNLOCK).
func TestUnlockSoundIsTheShippedClip(t *testing.T) {
	if achievementUnlockSound != "Achievement_unlocked" {
		t.Fatal(achievementUnlockSound)
	}
	for _, root := range achievementCaches() {
		data, err := os.ReadFile(root + "/pack.json")
		if err != nil {
			continue
		}
		var manifest struct {
			Files map[string]string `json:"files"`
		}
		if json.Unmarshal(data, &manifest) != nil {
			continue
		}
		found := false
		for key := range manifest.Files {
			if strings.HasSuffix(strings.ToLower(key), "sound/sfx/"+strings.ToLower(achievementUnlockSound)+".ogg") {
				found = true
			}
		}
		if !found && len(manifest.Files) > 0 {
			t.Fatalf("%s: no %s.ogg in the pack", root, achievementUnlockSound)
		}
	}
}
