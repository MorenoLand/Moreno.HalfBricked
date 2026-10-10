package game

import (
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Mystery ("?") crates, p_rand_1..8 and p_rand_all. Recovered from the v7 binary
// (Research/native/parity-audit-2026-10-09.md): the pickup initialiser at
// 0x00092da0 rewrites the type when the pickup is created:
//
//   - type 0x35 (p_rand_3) becomes 0x25, p_hover (FUN_0008c148 is a stub that
//     returns 1, so it always stays a hover pickup: the speed boost);
//   - every other p_rand_N (N = 1, 2, 4..8) asks FUN_000b188c(weaponManager, N)
//     and p_rand_all asks it with -1. That function walks the 13 weapon slots in
//     enum order (PISTOL, SHOTGUN, UZI, MINIGUN, SNIPER, FLAMER, BUZZSAW,
//     DUALPISTOL, GRENADE, MINE, BAZOOKA, SENTRY, COWPATBOMB), keeps the loaded
//     ones whose record field +0x34 equals N (any value > 0 for -1; the parser
//     0x000b1c70 reads +0x34 from the weapon XML <Value>), and picks one with the
//     shared RNG (no draw when there is exactly one). No candidate returns 0x11,
//     which the initialiser turns into 1 = the shotgun. The pickup type becomes
//     0x25 + slot, i.e. an ordinary weapon pickup.
//
// So the "random" crates are pack gates: in the shipped data only BUZZSAW (<Value>1),
// SENTRY (2), COWPATBOMB (4) and DUALPISTOL (5) have a non-zero value, p_rand_6..8
// match nothing (shotgun) and p_rand_all rolls among those four. The earlier port
// inferred pools from the level data instead.
var nativeWeaponSlots = [...]struct{ gun, pickup string }{
	{"PISTOL", ""}, {"SHOTGUN", "P_SHOTGUN"}, {"UZI", "P_UZI"}, {"MINIGUN", "P_MINIGUN"}, {"SNIPER", "P_SNIPER"},
	{"FLAMER", "P_FLAMER"}, {"BUZZSAW", "P_BUZZSAW"}, {"DUALPISTOL", "P_DUAL_PISTOL"}, {"GRENADE", "P_GRENADE"},
	{"MINE", "P_MINE"}, {"BAZOOKA", "P_BAZOOKA"}, {"SENTRY", "P_SENTRY"}, {"COWPATBOMB", "P_COW_PAT"},
}

// catalogWeaponName maps pickup names to weapon catalog GunType names.
func catalogWeaponName(name string) string {
	if name == "DUAL_PISTOL" {
		return "DUALPISTOL"
	}
	return name
}

// randomPickupGroup is the FUN_000b188c group argument of a p_rand_* name.
func randomPickupGroup(name string) (group int, ok bool) {
	name = strings.ToUpper(strings.TrimSpace(name))
	if name == "P_RAND_ALL" {
		return -1, true
	}
	if len(name) == len("P_RAND_1") && strings.HasPrefix(name, "P_RAND_") && name[7] >= '1' && name[7] <= '8' {
		return int(name[7] - '0'), true
	}
	return 0, false
}

// randomPickupCandidates is the loop of FUN_000b188c: the pickup names of the
// loaded weapons whose <Value> matches the group, in slot order.
func (p *playState) randomPickupCandidates(group int) []string {
	var out []string
	for _, slot := range nativeWeaponSlots {
		weapon, ok := p.weapons.Find(slot.gun)
		if !ok || slot.pickup == "" {
			continue
		}
		if int(weapon.Value) == group || (group < 0 && weapon.Value > 0) {
			out = append(out, slot.pickup)
		}
	}
	return out
}

// resolveRandomPickup turns a p_rand_* pickup into the concrete pickup the native
// initialiser creates (a weapon crate, or P_HOVER for p_rand_3).
func (p *playState) resolveRandomPickup(name string) (string, bool) {
	group, ok := randomPickupGroup(name)
	if !ok {
		return "", false
	}
	if group == 3 {
		return "P_HOVER", true
	}
	pool := p.randomPickupCandidates(group)
	switch len(pool) {
	case 0:
		return "P_SHOTGUN", true
	case 1:
		return pool[0], true
	}
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
	return pool[int(p.rng.Bounded(uint32(len(pool))))], true
}
