package game

// Statistic writers added by the completion pass (G08). Native evidence (libmortargame.so v7):
//
//   - TimesPlayed (+0x100): GameSave::Load 0x000cce30 write 0x000cce74, called once from the state enter 0x00087468
//     (the call clears its own first-load flag 0x005cea74): once per launch, after the profile has been read.
//   - StoryLevelsPlayed (+0x110) / SurvivalLevelsPlayed (+0x114): enter callback 0x0007c3e8, mode != 1 / mode == 1.
//   - ZombiesIgnited (+0x108): 0x000a0304 (the zombie damage routine) when a lethal hit selects death state 2
//     (param_1[0xc5] = 2), the same decision the port's classifyZombieDeath makes (zombie_gib.go).
//   - BestTRexTime (+0xe4) FUN_000b9d2c, BestGangsterTime (+0xe8) FUN_000b6c58, BestEgyptTime (+0xec) 0x000b50a0:
//     the boss death hooks store the boss clock (+0x328, seconds, advanced while FUN_000d9ab4 reports no running
//     script) as the best time when none is set (-1) or it does not exceed the stored one, only outside survival
//     (scene +0x514e8 == 0), and always add one to BossesKilled (+0x12c).
//   - Not written: BestBonusTime (+0xf8) and FruitSliced have no writer in the v7 exports and no mini-game exists in
//     the shipped data; Mad Dog's death FUN_000b82d4 writes no statistic.

// noteLaunchStats is GameSave::Load's TimesPlayed increment.
func (a *app) noteLaunchStats() {
	a.statistics.TimesPlayed++
	a.markStat("Number of times played")
}

// noteLevelEntryStats is the enter callback 0x0007c3e8.
func (a *app) noteLevelEntryStats() {
	if a.mode == 1 {
		a.statistics.SurvivalLevelsPlayed++
		a.markStat("Survival Levels Played")
		return
	}
	a.statistics.StoryLevelsPlayed++
	a.markStat("Story Levels Played")
}

func (a *app) markStat(label string) {
	if a.statistics.Available == nil {
		a.statistics.Available = map[string]bool{}
	}
	a.statistics.Available[label] = true
}

func (p *playState) markStat(label string) {
	if p.statistics != nil {
		if p.statistics.Available == nil {
			p.statistics.Available = map[string]bool{}
		}
		p.statistics.Available[label] = true
	}
}

// noteZombieIgnited is the +0x108 increment of a death-state-2 hit.
func (p *playState) noteZombieIgnited() {
	if p.statistics != nil {
		p.statistics.ZombiesIgnited++
		p.markStat("Zombies Ignited")
	}
}

// bossStatsKind names which boss clock/statistic a zombie's death hook writes ("" = none).
func (p *playState) bossStatsKind(z *zombieState) string {
	switch {
	case z.native.car.active:
		return "gangster"
	case z.native.samurai.active, z.native.robot.active:
		return "" // their own hooks (samurai.go, robot_boss.go)
	}
	entity := p.scriptEntities[z.scriptID]
	if entity == nil {
		return ""
	}
	switch entity.entityType {
	case "boss_rex":
		return "trex"
	case "boss_egyptian":
		return "egypt"
	}
	return ""
}

// stepBossClock advances +0x328 while no script runs.
func (p *playState) stepBossClock(z *zombieState, dt float64) {
	if z.dying || z.health <= 0 || p.samuraiScriptRunning() || p.bossStatsKind(z) == "" {
		return
	}
	z.native.bossTime += dt
}

// recordBestBossTime stores seconds (truncated) when unset (-1) or not larger than the stored value.
func (p *playState) recordBestBossTime(slot *int32, label string, seconds float64) {
	t := float32(seconds)
	if *slot == -1 || !(float32(uint32(*slot)) < t) {
		if t > 0 {
			*slot = int32(uint32(t))
		} else {
			*slot = 0
		}
		p.markStat(label)
	}
}

// noteBossDeathStats runs the stat part of the rex / Don Zombie car / mummy death hooks once per boss.
func (p *playState) noteBossDeathStats(z *zombieState) {
	if z.native.bossStatsDone || p.statistics == nil {
		return
	}
	kind := p.bossStatsKind(z)
	if kind == "" {
		return
	}
	z.native.bossStatsDone = true
	var slot *int32
	var label string
	switch kind {
	case "trex":
		slot, label = &p.statistics.BestTRexTime, "Best T-Rex time"
	case "gangster":
		slot, label = &p.statistics.BestGangsterTime, "Best Gangster Boss Time"
	case "egypt":
		slot, label = &p.statistics.BestEgyptTime, "Best Egypt Boss Time"
	}
	if !p.isSurvival() {
		p.recordBestBossTime(slot, label, z.native.bossTime)
	}
	p.noteBossKilled()
}
