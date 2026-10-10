package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Survival escalation. The wave builder (FUN_000c0b70 in v7, FUN_00123008 in 1.2.5) calls the escalation
// (FUN_000bea10 in v7, FUN_0012187c in 1.2.5) on every wave advance, and only in survival (v7: the frame flag
// +0x514e8 is 1; 1.2.5: mode->IsSurvival()). Addresses, literal pools and the UNRESOLVED items are in
// Research/native/survival-escalation-2026-10-09.md.
//
// One advance into wave W (a valid index), in native order:
//  1. The running copy of W is made from the table entry before anything changes (the builder's list copy).
//  2. Table pass (kept in the table, so it shows on the next pass through W): every spawner without a pickup
//     whose count is below 150 gets count += countStep + rnd(10) (v7 10, 1.2.5 15). Its interval is window/count,
//     which the port derives from the count when it spawns (waveSpawnerInterval).
//  3. Copy pass (the running copy only): every zombie type of a spawner without a pickup draws r = rnd(100):
//     r >= 70: turnSpeed range += (turnMin, turnMax); 30 <= r < 70: speed range += (speedMin, speedMax);
//     10 <= r < 30: strength += strength, then size = (strength-100)/30 + 29 .. + 31 (signed division);
//     r < 10: nothing.
//  4. The counters step once (survivalCounters.step), after the draws.
//
// The first wave of a level is copied from the table unescalated (FUN_000c3be0 v7 / FUN_00126608 1.2.5), and the
// counters start at zero for each level load.

// turnRange is a zombie type's native turnSpeed pair (min, max). formats.SpawnType keeps one turnSpeed value, so an
// escalated pair is kept beside the running copy (survivalWave.turns) and used when that type spawns.
type turnRange struct{ lo, hi int }

// survivalRules are the per-build steps of the escalation (v7 literals at 0x000bea10 .. 0x000bebf8; 1.2.5 at
// 0x0012187c .. 0x00121a34).
type survivalRules struct {
	countStep, strengthStep, speedMinStep, speedMaxStep, turnMinStep, turnMaxStep int
}

var (
	survivalRulesV7  = survivalRules{countStep: 10, strengthStep: 50, speedMinStep: 2, speedMaxStep: 8, turnMinStep: 1, turnMaxStep: 2}
	survivalRules125 = survivalRules{countStep: 15, strengthStep: 75, speedMinStep: 3, speedMaxStep: 12, turnMinStep: 1, turnMaxStep: 3}
)

// survivalRulesFor returns the steps of a build (1.2.5 unless the cache is v7).
func survivalRulesFor(build string) survivalRules {
	if build == content.WaveBuildV7 {
		return survivalRulesV7
	}
	return survivalRules125
}

// survivalCounters are the per-level globals of the escalation. Their native homes are the wave manager fields
// (v7 +0xdc08, +0xdc0c, +0xdc10, +0xdc14, +0xdc18; 1.2.5 +0x4c, +0x50, +0x54, +0x58, +0x5c).
type survivalCounters struct {
	strength, speedMin, speedMax, turnMin, turnMax int
}

// step is the counter update at the end of the builder. Each test reads the value before this advance, so the
// speed and turn tests read speedMax and turnMax before they change.
func (c *survivalCounters) step(rules survivalRules) {
	if c.strength < 200 {
		c.strength += rules.strengthStep
	}
	if c.speedMax < 50 {
		c.speedMin += rules.speedMinStep
	}
	if c.speedMax <= 64 {
		c.speedMax += rules.speedMaxStep
	}
	if c.turnMax <= 17 {
		c.turnMin += rules.turnMinStep
	}
	if c.turnMax <= 24 {
		c.turnMax += rules.turnMaxStep
	}
}

// survivalPickupV7 are the type names the v7 name table gives a pickup code (0x23..0x3b): p_health .. p_rand_all.
var survivalPickupV7 = []string{
	"p_health", "p_shield", "p_hover", "p_dual_pistol", "p_shotgun", "p_uzi", "p_minigun", "p_sniper",
	"p_flamer", "p_buzzsaw", "p_grenade", "p_mine", "p_cow_pat", "p_bazooka", "p_sentry",
	"p_rand_1", "p_rand_2", "p_rand_3", "p_rand_4", "p_rand_5", "p_rand_6", "p_rand_7", "p_rand_8", "p_rand_all",
}

// survivalPickup125 adds the four sentry variants that the 1.2.5 table holds (its pickup range is 0x23..0x40).
var survivalPickup125 = append(append([]string(nil), survivalPickupV7...),
	"p_sentryShotgun", "p_sentryUzi", "p_sentryFlamer", "p_sentryBazooka")

// survivalPickupSpawner is the spawner flag the parser sets (+0x3c): one of its types is a pickup code.
func survivalPickupSpawner(build string, spawner formats.Spawner) bool {
	names := survivalPickupV7
	if build != content.WaveBuildV7 {
		names = survivalPickup125
	}
	for _, t := range spawner.Types {
		for _, name := range names {
			if t.Name == name {
				return true
			}
		}
	}
	return false
}

// survivalCopy duplicates a spawner list as the builder does (operator=), with each type in native units: an absent
// speed is -1..-1, an absent size 15..16 and an absent turnSpeed -1..-1 (the parser defaults, FUN_000c3534).
// The table's Types slices are never written, so the copy can change its own.
func survivalCopy(spawners []formats.Spawner) ([]formats.Spawner, [][]turnRange) {
	out := make([]formats.Spawner, len(spawners))
	turns := make([][]turnRange, len(spawners))
	for i, s := range spawners {
		s.Types = append([]formats.SpawnType(nil), s.Types...)
		turns[i] = make([]turnRange, len(s.Types))
		for j := range s.Types {
			t := &s.Types[j]
			if t.Speed.X == 0 && t.Speed.Y == 0 {
				t.Speed = formats.Vec2{X: -1, Y: -1}
			}
			if t.Size.X == 0 && t.Size.Y == 0 {
				t.Size = formats.Vec2{X: 15, Y: 16}
			}
			turns[i][j] = turnRange{lo: -1, hi: -1}
			if t.TurnSpeed != 0 {
				turns[i][j] = turnRange{lo: int(t.TurnSpeed), hi: int(t.TurnSpeed)}
			}
		}
		out[i] = s
	}
	return out, turns
}

// survivalTablePass is step 2, on the table entry (the first loop of FUN_000bea10 / FUN_0012187c).
func survivalTablePass(rng *weapons.NativeRNG, rules survivalRules, build string, spawners []formats.Spawner) {
	for i := range spawners {
		if survivalPickupSpawner(build, spawners[i]) || spawners[i].Count >= 150 {
			continue
		}
		spawners[i].Count += rules.countStep + int(rng.Bounded(10))
	}
}

// survivalCopyPass is step 3, on the running copy (the second loop).
func survivalCopyPass(rng *weapons.NativeRNG, counters survivalCounters, build string, spawners []formats.Spawner, turns [][]turnRange) {
	for i := range spawners {
		if survivalPickupSpawner(build, spawners[i]) {
			continue
		}
		for j := range spawners[i].Types {
			survivalRoll(int(rng.Bounded(100)), counters, &spawners[i].Types[j], &turns[i][j])
		}
	}
}

// survivalRoll applies one draw r (0..99) to one zombie type of the running copy.
func survivalRoll(r int, counters survivalCounters, t *formats.SpawnType, turn *turnRange) {
	switch {
	case r >= 70:
		turn.lo += counters.turnMin
		turn.hi += counters.turnMax
	case r >= 30:
		t.Speed = formats.Vec2{X: t.Speed.X + float64(counters.speedMin), Y: t.Speed.Y + float64(counters.speedMax)}
	case r >= 10:
		strength := int(t.Strength) + counters.strength
		t.Strength = float64(strength)
		q := (strength - 100) / 30 // C signed division: truncates toward zero, as Go's
		t.Size = formats.Vec2{X: float64(q + 29), Y: float64(q + 31)}
	}
}

// survivalWave is the running wave after a survival advance: the escalated copy and the turn ranges of its types.
type survivalWave struct {
	index int
	wave  formats.Wave
	turns [][]turnRange
}

// survivalState is one level's escalation state. table holds the level's waves with the table pass applied.
type survivalState struct {
	build    string
	rules    survivalRules
	source   *formats.Wave
	table    []formats.Wave
	counters survivalCounters
	active   *survivalWave
}

// newSurvivalState copies a level's waves into the table. source identifies the level the table belongs to.
func newSurvivalState(build string, waves []formats.Wave) *survivalState {
	table := make([]formats.Wave, len(waves))
	for i, w := range waves {
		w.Spawners = append([]formats.Spawner(nil), w.Spawners...)
		table[i] = w
	}
	s := &survivalState{build: build, rules: survivalRulesFor(build), table: table}
	if len(waves) > 0 {
		s.source = &waves[0]
	}
	return s
}

// advance runs the escalation for an advance into wave index next (the builder's valid-index branch).
func (s *survivalState) advance(rng *weapons.NativeRNG, next int) {
	if rng == nil || next < 0 || next >= len(s.table) {
		return
	}
	entry := &s.table[next]
	spawners, turns := survivalCopy(entry.Spawners)
	survivalTablePass(rng, s.rules, s.build, entry.Spawners)
	survivalCopyPass(rng, s.counters, s.build, spawners, turns)
	s.counters.step(s.rules)
	wave := *entry
	wave.Spawners = spawners
	s.active = &survivalWave{index: next, wave: wave, turns: turns}
}

// survivalAdvance runs the escalation for an advance into wave index next. Only survival levels escalate.
func (p *playState) survivalAdvance(next int) {
	if p.world == nil || len(p.world.Level.Waves) == 0 || !p.isSurvival() || p.rng == nil {
		return
	}
	waves := p.world.Level.Waves
	if p.survival == nil || p.survival.source != &waves[0] {
		p.survival = newSurvivalState(p.waveBuild(), waves)
	}
	p.survival.advance(p.rng, next)
}

// runningSurvivalWave is the escalated copy of the current wave, when a survival advance made one for this level.
func (p *playState) runningSurvivalWave() *survivalWave {
	s := p.survival
	if s == nil || s.active == nil || s.active.index != p.waveIndex || p.world == nil || len(p.world.Level.Waves) == 0 {
		return nil
	}
	if s.source != &p.world.Level.Waves[0] {
		return nil
	}
	return s.active
}

// waveNow is the wave the update runs: the escalated copy after a survival advance, otherwise the level's wave.
func (p *playState) waveNow() formats.Wave {
	if a := p.runningSurvivalWave(); a != nil {
		return a.wave
	}
	return p.world.Level.Waves[p.waveIndex]
}

// waveTurns are the native turn ranges of spawner index of the running wave, or nil for the level's own waves.
func (p *playState) waveTurns(spawner int) []turnRange {
	a := p.runningSurvivalWave()
	if a == nil || spawner < 0 || spawner >= len(a.turns) {
		return nil
	}
	return a.turns[spawner]
}

// survivalBossPresent is the 1.2.5 survival test of FUN_0009962c (FUN_0009788c(obj,10,16) > 0 gives 20): a boss
// zombie is on the field. The native test counts the per-type entity lists 10..15; the port counts the zombies
// that are not dying (the list exit time is UNRESOLVED).
func (p *playState) survivalBossPresent() bool {
	for _, z := range p.zombies {
		if z.dying || z.health <= 0 {
			continue
		}
		if e := p.scriptEntities[z.scriptID]; e != nil && isBossType(e.entityType) {
			return true
		}
	}
	return false
}

// rollSurvivalSpawnRecord is rollZombieSpawnRecord with the native turnSpeed range. FUN_000bec48 rolls size, speed,
// turnSpeed (no draw when the range is a point), deviateAmount, alertRadius and deviateCycleSpeed in that order.
func rollSurvivalSpawnRecord(entry formats.SpawnType, turn turnRange, rng *weapons.NativeRNG) zombieSpawnRecord {
	record := zombieSpawnRecord{Kind: nativeZombieTypes[entry.Name], Gun: 13, Strength: -1}
	if gun, ok := zombieGunIDs[entry.Weapon]; ok {
		record.Gun = gun
	}
	if entry.Strength >= 0 {
		record.Strength = int(entry.Strength)
	}
	lo, hi := zombieRange(entry.Size, 15, 16)
	record.HalfSize = zombieRoll(lo, hi, rng)
	lo, hi = zombieRange(entry.Speed, -1, -1)
	record.Speed = zombieRoll(lo, hi, rng)
	record.TurnSpeed = zombieRoll(turn.lo, turn.hi, rng)
	record.Amount = int(entry.DeviateAmount)
	if entry.DeviateAmount == 0 {
		record.Amount = zombieRoll(15, 20, rng)
	}
	record.AlertRadius = int(entry.AlertRadius)
	if entry.AlertRadius == 0 {
		record.AlertRadius = zombieRoll(200, 300, rng)
	}
	record.CycleSpeed = int(entry.DeviateCycleSpeed)
	if entry.DeviateCycleSpeed == 0 {
		record.CycleSpeed = zombieRoll(40, 50, rng)
	}
	return record
}
