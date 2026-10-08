package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Gameplay event tracking for the scoped (KILLS, KILLS_SHIELD and SPECIFIC)
// achievements. Every lethal hit goes through creditKill with the origin of the
// projectile or blast that dealt it, so single-pickup, single-shot, single-mine
// and single-sentry counters can be kept apart.

// shieldDuration is the p_shield timer: the pickup update FUN_000928f4 (type
// byte '$' = 0x24) stores DAT_00092d34 = 0x41700000 = 15.0 into the player's
// shield timer (+0x3e4), replacing any running one. See shield.go.
const shieldDuration = 15.0

// killOrigin identifies what dealt a lethal hit. gun is the primary weapon's
// catalog GunType for player shots, or ROCKET, GRENADE, COWPAT, MINE and SENTRY
// for secondaries and turrets. EXPZOMBIE, TREX and TRAIN are reserved for the
// exploding zombie blast, T-Rex shockwave and train hazards.
type killOrigin struct {
	gun                  string
	shot, pickup, sentry int
}

type scopeKey struct {
	kind string
	id   int
}

type achievementProgress struct {
	pickupSerial, shotSerial, sentrySerial int
	counts                                 map[scopeKey]int32
	best                                   map[string]int32
	shieldKills                            map[string]int32
	hitShots                               map[int]bool
	hitProjectiles                         map[*weapons.NativeWeaponProjectile]bool
	shieldStreak                           int32
	shieldSerial                           int
	shots, hits                            int32
	nonPistol, died, banked                bool
	shieldTimer                            float64
}

func (p *playState) newShot() int {
	p.achieve.shotSerial++
	return p.achieve.shotSerial
}
func (p *playState) primaryOrigin(shot int) killOrigin {
	return killOrigin{gun: p.weapon.GunType, shot: shot, pickup: p.achieve.pickupSerial}
}
func (p *playState) shielded() bool { return p.achieve.shieldTimer > 0 }

// collectShield is the p_shield pickup.
func (p *playState) collectShield() { p.achieve.shieldTimer = shieldDuration }
func (a *achievementProgress) tickShield(dt float64) {
	if a.shieldTimer > 0 {
		a.shieldTimer = max(0, a.shieldTimer-dt)
	}
}
func (a *achievementProgress) scope(kind string, id int) int32 {
	if a.counts == nil {
		a.counts = map[scopeKey]int32{}
	}
	if a.best == nil {
		a.best = map[string]int32{}
	}
	key := scopeKey{kind, id}
	a.counts[key]++
	if a.counts[key] > a.best[kind] {
		a.best[kind] = a.counts[key]
	}
	return a.counts[key]
}
func primaryGun(gun string) bool {
	switch gun {
	case "", "ROCKET", "GRENADE", "COWPAT", "MINE", "SENTRY", "EXPZOMBIE", "TREX", "TRAIN":
		return false
	}
	return true
}

// markProjectileHit is the accuracy "hit" counter. Natively every projectile
// raises scene +0x45178 once, the first time it damages a zombie (the method at
// 1.2.5 0x00104564 registered in 13 projectile classes), while the shot counter
// +0x45174 rises once per accepted firing. A shotgun volley can therefore score
// three hits for one shot. Projectiles without identity fall back to one hit per
// shot. Which of the 13 classes exist is UNRESOLVED, so only the primary
// weapons' projectiles are counted.
func (p *playState) markProjectileHit(projectile *weapons.NativeWeaponProjectile, o killOrigin) {
	if o.shot <= 0 || !primaryGun(o.gun) {
		return
	}
	if projectile != nil {
		if p.achieve.hitProjectiles == nil {
			p.achieve.hitProjectiles = map[*weapons.NativeWeaponProjectile]bool{}
		}
		if p.achieve.hitProjectiles[projectile] {
			return
		}
		p.achieve.hitProjectiles[projectile] = true
		p.achieve.hits++
		return
	}
	p.markShotHit(o)
}

// markShotHit records that a primary-weapon shot struck a zombie (one hit per
// shot, used for projectiles that carry no identity).
func (p *playState) markShotHit(o killOrigin) {
	if o.shot <= 0 || !primaryGun(o.gun) || p.achieve.hitShots[o.shot] {
		return
	}
	if p.achieve.hitShots == nil {
		p.achieve.hitShots = map[int]bool{}
	}
	p.achieve.hitShots[o.shot] = true
	p.achieve.hits++
}

var weaponBestStat = map[string]string{"shotgun": "Kills with single Shotgun", "smg": "Kills with single Uzi", "flamethrower": "Kills with single Flamer", "minigun": "Kills with single Minigun", "rifle": "Kills with single Rifle", "buzzsaw": "Kills with single Buzzsaw", "grenade": "Kills with single Grenade", "mine": "Kills with single Mine"}

func (p *playState) recordWeaponBest(kind string, value int32) {
	name, ok := weaponBestStat[kind]
	if !ok || p.statistics == nil {
		return
	}
	s := p.statistics
	var field *int32
	switch kind {
	case "shotgun":
		field = &s.ShotgunKills
	case "smg":
		field = &s.UziKills
	case "flamethrower":
		field = &s.FlamerKills
	case "minigun":
		field = &s.MinigunKills
	case "rifle":
		field = &s.RifleKills
	case "buzzsaw":
		field = &s.BuzzsawKills
	case "grenade":
		field = &s.GrenadeKills
	case "mine":
		field = &s.MineKills
	}
	if value > *field {
		*field = value
	}
	if s.Available == nil {
		s.Available = map[string]bool{}
	}
	s.Available[name] = true
}

// creditKill is the single place a lethal hit by the player's side is counted.
func (p *playState) creditKill(o killOrigin) {
	p.combatKillCount++
	a := &p.achieve
	scoped := func(kind string, id int) {
		n := a.scope(kind, id)
		p.recordWeaponBest(kind, n)
		// SUPERSIZED COMBO is not derived from kills: the native "combo" value is
		// the integer combo counter of a weapon tracker (combo.go, comboCredit).
	}
	// Primary-weapon kills follow FUN_000f1990 (1.2.5): the kill counts only while
	// the weapon that fired is the one equipped now (weapon id compare, not a
	// pickup serial), never for the pistol, and the counters (+0xa0 kills, +0xa4
	// shield streak) are zeroed by every primary equip (FUN_000f1bcc). The count is
	// submitted under the weapon's name, so the rifle counts per pickup too.
	primaryKind := ""
	if primaryGun(o.gun) && o.gun == p.weapon.GunType {
		switch o.gun {
		case "SHOTGUN":
			primaryKind = "shotgun"
		case "UZI":
			primaryKind = "smg"
		case "FLAMER":
			primaryKind = "flamethrower"
		case "MINIGUN":
			primaryKind = "minigun"
		case "BUZZSAW":
			primaryKind = "buzzsaw"
		case "DUALPISTOL":
			primaryKind = "dualpistol"
		case "SNIPER":
			primaryKind = "rifle"
		}
	}
	if primaryKind != "" {
		scoped(primaryKind, a.pickupSerial)
		// +0xa4: a streak of kills made while the shield timer (+0x40) is above 0;
		// a credited kill without the shield zeroes it.
		if a.shieldSerial != a.pickupSerial {
			a.shieldStreak, a.shieldSerial = 0, a.pickupSerial
		}
		if p.shielded() {
			a.shieldStreak++
			if a.shieldKills == nil {
				a.shieldKills = map[string]int32{}
			}
			if a.shieldStreak > a.shieldKills[primaryKind] {
				a.shieldKills[primaryKind] = a.shieldStreak
			}
			if primaryKind == "buzzsaw" && p.statistics != nil {
				p.statistics.BuzzsawShieldKills++
			}
		} else {
			a.shieldStreak = 0
		}
	}
	switch o.gun {
	case "ROCKET":
		scoped("bazooka", o.shot)
	case "GRENADE":
		scoped("grenade", o.shot)
	case "COWPAT":
		scoped("cowpat", o.shot)
	case "MINE":
		scoped("mine", o.shot)
	case "SENTRY":
		scoped("sentry", o.sentry)
		if p.statistics != nil {
			p.statistics.SentryGunKills++
			if p.statistics.Available == nil {
				p.statistics.Available = map[string]bool{}
			}
			p.statistics.Available["Sentry Gun Kills"] = true
		}
	case "EXPZOMBIE":
		scoped("exp_zombie", o.shot)
	case "TREX":
		scoped("t_rex", o.shot)
	case "TRAIN":
		// The native train submits its kill count once per pass, when the pass ends
		// (FUN_000fd7a4 -> FUN_00142e18), not per kill: trainHazard.finishPass
		// writes the "train" scope, so there is nothing to count here.
	}
}

// achievementKillSpecifics are the KILLS scopes the port can currently produce.
// t_rex is the rex shockwave (rex_shockwave.go); train needs a hazard the port does not simulate yet.
var achievementKillSpecifics = map[string]bool{"t_rex": true, "shotgun": true, "smg": true, "flamethrower": true, "minigun": true, "rifle": true, "bazooka": true, "mine": true, "grenade": true, "buzzsaw": true, "dualpistol": true, "cowpat": true, "sentry": true, "exp_zombie": true}

func achievementEventTracked(entry formats.Achievement) bool {
	switch {
	case entry.Type == "KILLS" && entry.Check == "ge":
		return achievementKillSpecifics[entry.SpecificType]
	case entry.Type == "KILLS_SHIELD" && entry.Check == "ge":
		return entry.SpecificType == "buzzsaw"
	case entry.Type == "SPECIFIC" && entry.Check == "ge":
		return entry.SpecificType == "combo" || entry.SpecificType == "accuracy"
	case entry.Type == "SPECIFIC" && entry.Check == "e":
		switch entry.SpecificType {
		case "pistol_only", "continue", "death", "maddog":
			return true
		}
	}
	return false
}

// playAchievementMet evaluates the kill-scoped achievements from live play.
func (a *app) playAchievementMet(p *playState, entry formats.Achievement) bool {
	if p == nil || entry.Total <= 0 {
		return false
	}
	switch {
	case entry.Type == "KILLS" && entry.Check == "ge":
		return int(p.achieve.best[entry.SpecificType]) >= entry.Total
	case entry.Type == "KILLS_SHIELD" && entry.Check == "ge":
		// The submitted value is the live shield streak (FUN_000f1990 +0xa4); the
		// best streak of this play stands for "was >= total at some kill".
		return int(p.achieve.shieldKills[entry.SpecificType]) >= entry.Total
	case entry.Type == "SPECIFIC" && entry.Check == "ge" && entry.SpecificType == "combo":
		return int(p.achieve.best["combo"]) >= entry.Total
	}
	return false
}

// achievementSession is progress that spans levels within one run of the game.
type achievementSession struct {
	storyActive, storyBroken bool
	chapter                  map[int][2]int32
	chapterAccuracy          int
	deathFree                map[string]bool
}

// noteAchievementLevelEntry runs when a level opens; prev is the play it replaces.
func (a *app) noteAchievementLevelEntry(prev *playState, info formats.LevelInfo) {
	if a.mode != 0 || !hasLevelFlag(info, "STORY") {
		return
	}
	s := &a.achieveSession
	retry := prev != nil && prev.levelInfo.ID == info.ID
	switch {
	case hasLevelFlag(info, "BEGINSTORY") && !retry:
		s.storyActive, s.storyBroken = true, false
	case retry || prev == nil:
		s.storyBroken = true
	}
}

// bankLevelAchievementProgress folds a finished story level into the chapter
// and death-free ledgers exactly once per play.
func (a *app) bankLevelAchievementProgress(info *formats.LevelInfo) {
	p := a.play
	if p == nil || info == nil || a.mode != 0 || p.achieve.banked {
		return
	}
	p.achieve.banked = true
	s := &a.achieveSession
	if !p.achieve.died {
		if s.deathFree == nil {
			s.deathFree = map[string]bool{}
		}
		s.deathFree[info.ID] = true
	}
	if s.chapter == nil {
		s.chapter = map[int][2]int32{}
	}
	totals := s.chapter[info.WorldIndex]
	totals[0] += p.achieve.shots
	totals[1] += p.achieve.hits
	s.chapter[info.WorldIndex] = totals
	if hasLevelFlag(*info, "ENDWORLD") {
		s.chapterAccuracy = -1
		if totals[0] > 0 {
			s.chapterAccuracy = int(totals[1] * 100 / totals[0])
		}
		delete(s.chapter, info.WorldIndex)
	}
}

func (a *app) allStoryLevelsDeathFree() bool {
	count := 0
	for _, level := range a.levels {
		if !hasLevelFlag(level, "STORY") || hasLevelFlag(level, "SURVIVAL") || level.WorldIndex > 4 {
			continue
		}
		count++
		if !a.achieveSession.deathFree[level.ID] {
			return false
		}
	}
	return count > 0
}

// completionAchievementMet evaluates the SPECIFIC conditions that resolve when a
// story level is completed.
func (a *app) completionAchievementMet(entry formats.Achievement, info *formats.LevelInfo) bool {
	if info == nil || a.mode != 0 || a.play == nil || entry.Type != "SPECIFIC" {
		return false
	}
	s := &a.achieveSession
	switch {
	case entry.Check == "e" && entry.SpecificType == "pistol_only":
		return !a.play.achieve.nonPistol && a.play.achieve.shots > 0
	case entry.Check == "e" && entry.SpecificType == "continue":
		return hasLevelFlag(*info, "ENDWORLD") && hasLevelFlag(*info, "SHOWCREDITS") && s.storyActive && !s.storyBroken
	case entry.Check == "e" && entry.SpecificType == "death":
		return a.allStoryLevelsDeathFree()
	case entry.Check == "ge" && entry.SpecificType == "accuracy":
		return hasLevelFlag(*info, "ENDWORLD") && s.chapterAccuracy >= entry.Total
	}
	return false
}

// awardMadDog is UnlockWesternBossAchievement(choice): 0 shot first, 1 honour code.
func (a *app) awardMadDog(choice int) {
	if a == nil {
		return
	}
	for _, entry := range a.achievements {
		if entry.Type == "SPECIFIC" && entry.SpecificType == "maddog" && entry.Total == choice && !a.achievementUnlocks[entry.ID] {
			a.unlockAchievement(entry)
			_ = a.savePlayerProfile()
		}
	}
}
