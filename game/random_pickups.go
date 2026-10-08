package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"strings"
)

// Mystery ("?") crates. The native type table has p_rand_1..8 and p_rand_all but
// the reward dispatch was not recovered, so the pools are inferred from how the
// level data uses them: p_rand_1 only ever shares a spawner with primary weapons,
// p_rand_2 only with grenades/mines/bazookas; the remaining variants have no icon
// of their own and are treated as "any weapon".
var (
	randomPrimaryPickups   = []string{"P_SHOTGUN", "P_UZI", "P_MINIGUN", "P_SNIPER", "P_FLAMER", "P_BUZZSAW", "P_DUAL_PISTOL"}
	randomSecondaryPickups = []string{"P_GRENADE", "P_MINE", "P_BAZOOKA", "P_SENTRY"}
)

// catalogWeaponName maps pickup names to weapon catalog GunType names.
func catalogWeaponName(name string) string {
	if name == "DUAL_PISTOL" {
		return "DUALPISTOL"
	}
	return name
}

func (p *playState) randomPickupPool(name string) []string {
	var pool []string
	switch name {
	case "P_RAND_1":
		pool = randomPrimaryPickups
	case "P_RAND_2":
		pool = randomSecondaryPickups
	default:
		pool = append(append([]string(nil), randomPrimaryPickups...), randomSecondaryPickups...)
	}
	available := make([]string, 0, len(pool))
	for _, candidate := range pool {
		weaponName := strings.TrimPrefix(candidate, "P_")
		if _, _, secondary := secondaryForPickup(candidate); secondary {
			weaponName = strings.TrimPrefix(candidate, "P_")
		}
		if _, ok := p.weapons.Find(catalogWeaponName(weaponName)); ok {
			available = append(available, candidate)
		}
	}
	return available
}

// resolveRandomPickup turns a p_rand_* pickup into a concrete weapon pickup.
func (p *playState) resolveRandomPickup(name string) (string, bool) {
	if !strings.HasPrefix(name, "P_RAND_") {
		return "", false
	}
	pool := p.randomPickupPool(name)
	if len(pool) == 0 {
		return "", false
	}
	if p.rng == nil {
		rng := weapons.NewNativeRNG()
		p.rng = &rng
	}
	return pool[int(p.rng.Bounded(uint32(len(pool))))], true
}
