package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// survivalPlay is a playState with one level of the given waves, in survival or story, for one build.
func survivalPlay(build string, survival bool, waves []formats.Wave) *playState {
	rng := weapons.NewNativeRNG()
	p := &playState{rng: &rng, world: &viewer.Viewer{Level: formats.Level{Waves: waves, WaveBuild: build}}}
	if survival {
		p.levelInfo = formats.LevelInfo{Flags: []string{"SURVIVAL"}}
	}
	return p
}

// survivalWaves: wave 0 has a zombie spawner (30) and a pickup spawner; wave 1 has one zombie spawner (40) and
// loops back to wave 0.
func survivalWaves() []formats.Wave {
	zombie := formats.SpawnType{Name: "zombie", Chance: 1, Speed: formats.Vec2{X: 90, Y: 95}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12}
	pickup := formats.SpawnType{Name: "p_shotgun", Chance: 1}
	return []formats.Wave{
		{NextWave: 1, RunTime: 20000, EndWaveTime: 20001, EndWaveZombies: 10, Spawners: []formats.Spawner{
			{Count: 30, Index: 1, Types: []formats.SpawnType{zombie}},
			{Count: 20, Index: 2, Types: []formats.SpawnType{pickup}},
		}},
		{NextWave: 0, RunTime: 15000, EndWaveTime: 20001, EndWaveZombies: 10, Spawners: []formats.Spawner{
			{Count: 40, Index: 3, Types: []formats.SpawnType{zombie}},
		}},
	}
}

func TestSurvivalPickupSpawnerFollowsTheNativeTable(t *testing.T) {
	cases := []struct {
		build string
		name  string
		want  bool
	}{
		{content.WaveBuildV7, "p_grenade", true},
		{content.WaveBuildV7, "p_rand_all", true},
		{content.WaveBuildV7, "p_sentryUzi", false}, // not in the 1.2.1 table
		{content.WaveBuild125, "p_sentryUzi", true}, // the 1.2.5 table adds the four sentry variants
		{content.WaveBuildV7, "zombie", false},
		{content.WaveBuild125, "armed_zombie", false},
		{content.WaveBuild125, "train", false}, // code 0x1f, below the pickup range in both builds
	}
	for _, c := range cases {
		spawner := formats.Spawner{Types: []formats.SpawnType{{Name: "zombie"}, {Name: c.name}}}
		if got := survivalPickupSpawner(c.build, spawner); got != c.want {
			t.Errorf("%s %s: pickup flag %v, want %v", c.build, c.name, got, c.want)
		}
	}
}

func TestSurvivalRulesPerBuild(t *testing.T) {
	v7 := survivalRulesFor(content.WaveBuildV7)
	if v7.countStep != 10 || v7.strengthStep != 50 || v7.speedMinStep != 2 || v7.speedMaxStep != 8 || v7.turnMinStep != 1 || v7.turnMaxStep != 2 {
		t.Fatalf("1.2.1 steps = %+v", v7)
	}
	hd := survivalRulesFor(content.WaveBuild125)
	if hd.countStep != 15 || hd.strengthStep != 75 || hd.speedMinStep != 3 || hd.speedMaxStep != 12 || hd.turnMinStep != 1 || hd.turnMaxStep != 3 {
		t.Fatalf("1.2.5 steps = %+v", hd)
	}
}

// The counter sequences follow the native tests (v7 FUN_000bea10 0x000bea84 onward; 1.2.5 FUN_0012187c 0x001219d4 onward).
func TestSurvivalCountersStepPerBuild(t *testing.T) {
	type state [5]int // strength, speedMin, speedMax, turnMin, turnMax
	run := func(build string, steps int) []state {
		rules := survivalRulesFor(build)
		var c survivalCounters
		var out []state
		for i := 0; i < steps; i++ {
			c.step(rules)
			out = append(out, state{c.strength, c.speedMin, c.speedMax, c.turnMin, c.turnMax})
		}
		return out
	}
	v7 := run(content.WaveBuildV7, 10)
	wantV7 := []state{
		{50, 2, 8, 1, 2}, {100, 4, 16, 2, 4}, {150, 6, 24, 3, 6}, {200, 8, 32, 4, 8}, {200, 10, 40, 5, 10},
		{200, 12, 48, 6, 12}, {200, 14, 56, 7, 14}, {200, 14, 64, 8, 16}, {200, 14, 72, 9, 18}, {200, 14, 72, 9, 20},
	}
	for i := range wantV7 {
		if v7[i] != wantV7[i] {
			t.Errorf("1.2.1 step %d = %v, want %v", i+1, v7[i], wantV7[i])
		}
	}
	hd := run(content.WaveBuild125, 9)
	wantHD := []state{
		{75, 3, 12, 1, 3}, {150, 6, 24, 2, 6}, {225, 9, 36, 3, 9}, {225, 12, 48, 4, 12}, {225, 15, 60, 5, 15},
		{225, 15, 72, 6, 18}, {225, 15, 72, 6, 21}, {225, 15, 72, 6, 24}, {225, 15, 72, 6, 27},
	}
	for i := range wantHD {
		if hd[i] != wantHD[i] {
			t.Errorf("1.2.5 step %d = %v, want %v", i+1, hd[i], wantHD[i])
		}
	}
}

// survivalRoll picks the branch by r: >= 70 turn, 30..69 speed, 10..29 strength and size, below 10 nothing.
func TestSurvivalRollBranchesByDraw(t *testing.T) {
	counters := survivalCounters{strength: 50, speedMin: 2, speedMax: 8, turnMin: 1, turnMax: 2}
	fresh := func() (formats.SpawnType, turnRange) {
		return formats.SpawnType{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}}, turnRange{12, 12}
	}
	type result struct {
		speed, size formats.Vec2
		strength    float64
		turn        turnRange
	}
	check := func(r int) result {
		item, turn := fresh()
		survivalRoll(r, counters, &item, &turn)
		return result{item.Speed, item.Size, item.Strength, turn}
	}
	base := result{formats.Vec2{X: 90, Y: 95}, formats.Vec2{X: 29, Y: 31}, 100, turnRange{12, 12}}
	if got := check(9); got != base {
		t.Errorf("r=9 changed the type: %+v", got)
	}
	turn := check(70)
	if turn.turn != (turnRange{13, 14}) || turn.speed != base.speed {
		t.Errorf("r=70 = %+v, want turn 13..14 and speed unchanged", turn)
	}
	if check(69).speed != (formats.Vec2{X: 92, Y: 103}) || check(30).turn != base.turn {
		t.Errorf("speed branch wrong: r=69 %+v r=30 %+v", check(69).speed, check(30).turn)
	}
	strength := check(29)
	if strength.strength != 150 || strength.size != (formats.Vec2{X: 30, Y: 32}) {
		t.Errorf("r=29 strength %v size %v, want 150 and 30..32", strength.strength, strength.size)
	}
	if check(10).strength != 150 || check(9).strength != 100 {
		t.Errorf("strength branch edges wrong")
	}
	// Strength 0 + 50 = 50: (50-100)/30 truncates to -1 (C signed division), so the size is 28..30.
	low := formats.SpawnType{Name: "zombie", Strength: 0, Size: formats.Vec2{X: 29, Y: 31}}
	lowTurn := turnRange{12, 12}
	survivalRoll(20, counters, &low, &lowTurn)
	if low.Strength != 50 || low.Size != (formats.Vec2{X: 28, Y: 30}) {
		t.Errorf("negative quotient: strength %v size %v, want 50 and 28..30", low.Strength, low.Size)
	}
}

func TestSurvivalTablePassDrawsOnlyEligibleSpawners(t *testing.T) {
	for _, build := range []string{content.WaveBuildV7, content.WaveBuild125} {
		rules := survivalRulesFor(build)
		rng := weapons.NewNativeRNG()
		zombie := formats.SpawnType{Name: "zombie", Chance: 1}
		pickup := formats.SpawnType{Name: "p_uzi", Chance: 1}
		for trial := 0; trial < 200; trial++ {
			spawners := []formats.Spawner{
				{Count: 30, Types: []formats.SpawnType{zombie}},
				{Count: 30, Types: []formats.SpawnType{pickup}},
				{Count: 150, Types: []formats.SpawnType{zombie}},
				{Count: 149, Types: []formats.SpawnType{zombie}},
			}
			survivalTablePass(&rng, rules, build, spawners)
			lo := rules.countStep
			if got := spawners[0].Count - 30; got < lo || got > lo+9 {
				t.Fatalf("%s: zombie count went 30 -> %d", build, spawners[0].Count)
			}
			if spawners[1].Count != 30 {
				t.Fatalf("%s: pickup spawner changed to %d", build, spawners[1].Count)
			}
			if spawners[2].Count != 150 {
				t.Fatalf("%s: count 150 changed to %d", build, spawners[2].Count)
			}
			if got := spawners[3].Count - 149; got < lo || got > lo+9 {
				t.Fatalf("%s: count 149 went to %d", build, spawners[3].Count)
			}
		}
	}
}

func TestSurvivalCopyNormalizesAbsentRanges(t *testing.T) {
	spawners := []formats.Spawner{{Count: 5, Types: []formats.SpawnType{
		{Name: "zombie"},
		{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12},
	}}}
	copied, turns := survivalCopy(spawners)
	if s := copied[0].Types[0]; s.Speed != (formats.Vec2{X: -1, Y: -1}) || s.Size != (formats.Vec2{X: 15, Y: 16}) {
		t.Errorf("absent ranges: speed %v size %v", s.Speed, s.Size)
	}
	if turns[0][0] != (turnRange{-1, -1}) || turns[0][1] != (turnRange{12, 12}) {
		t.Errorf("turn ranges %v", turns[0])
	}
	// The source list keeps its own zero pairs; the copy has its own Types slice.
	if spawners[0].Types[0].Speed != (formats.Vec2{}) {
		t.Errorf("copy changed the source list")
	}
}

// A survival advance: the running copy keeps the counts from before the table pass, the table keeps the new count,
// the first wave is not escalated, and the counters step once.
func TestSurvivalAdvanceCopiesBeforeTheTablePass(t *testing.T) {
	for _, build := range []string{content.WaveBuildV7, content.WaveBuild125} {
		p := survivalPlay(build, true, survivalWaves())
		if p.waveNow().Spawners[0].Count != 30 || p.waveTurns(0) != nil {
			t.Fatalf("%s: the level's first wave should be the unescalated table entry", build)
		}
		p.survivalAdvance(1)
		p.waveIndex = 1
		running := p.waveNow()
		if running.Spawners[0].Count != 40 {
			t.Errorf("%s: running copy count %d, want 40 (the count before this advance)", build, running.Spawners[0].Count)
		}
		table := p.survival.table[1].Spawners[0].Count
		rules := survivalRulesFor(build)
		if table < 40+rules.countStep || table > 40+rules.countStep+9 {
			t.Errorf("%s: table count %d after the advance", build, table)
		}
		if p.survival.counters.strength != rules.strengthStep {
			t.Errorf("%s: counters strength %d after one advance", build, p.survival.counters.strength)
		}
		if p.waveTurns(0) == nil {
			t.Errorf("%s: no turn ranges for the running wave", build)
		}
	}
}

// The table entry escalates again on each pass through the same index, and the counters keep stepping.
func TestSurvivalTableEscalatesAcrossPasses(t *testing.T) {
	p := survivalPlay(content.WaveBuildV7, true, survivalWaves())
	p.survivalAdvance(1)
	p.waveIndex = 1
	first := p.survival.table[1].Spawners[0].Count
	p.survivalAdvance(1)
	if got := p.waveNow().Spawners[0].Count; got != first {
		t.Errorf("second pass copy count %d, want the table count after the first pass %d", got, first)
	}
	if p.survival.table[1].Spawners[0].Count < first+10 {
		t.Errorf("second pass did not escalate the table again")
	}
	if p.survival.counters.strength != 100 {
		t.Errorf("counters strength %d after two advances, want 100", p.survival.counters.strength)
	}
}

func TestSurvivalAdvanceOnlyInSurvival(t *testing.T) {
	p := survivalPlay(content.WaveBuildV7, false, survivalWaves())
	p.survivalAdvance(1)
	if p.survival != nil {
		t.Fatalf("a story level escalated")
	}
	p.waveIndex = 1
	if got := p.waveNow().Spawners[0].Count; got != 40 {
		t.Errorf("story wave count %d, want 40 (unchanged)", got)
	}
}

// A pickup spawner is never escalated, but its zombie types still take the native turn range.
func TestSurvivalPickupSpawnerIsNotEscalated(t *testing.T) {
	waves := survivalWaves()
	waves[0].Spawners[1].Types = []formats.SpawnType{{Name: "p_shotgun", Chance: 1, Speed: formats.Vec2{X: 90, Y: 95}, TurnSpeed: 12}}
	p := survivalPlay(content.WaveBuild125, true, waves)
	p.survivalAdvance(0)
	p.waveIndex = 0
	spawner := p.waveNow().Spawners[1]
	if spawner.Count != 20 || spawner.Types[0].Speed != (formats.Vec2{X: 90, Y: 95}) {
		t.Errorf("pickup spawner escalated: count %d speed %v", spawner.Count, spawner.Types[0].Speed)
	}
	if turn := p.waveTurns(1)[0]; turn != (turnRange{12, 12}) {
		t.Errorf("pickup spawner turn range %v, want the unescalated 12..12", turn)
	}
}

func TestSurvivalRecordMatchesTheOldRollForAPointTurn(t *testing.T) {
	entry := formats.SpawnType{Name: "zombie", Speed: formats.Vec2{X: 90, Y: 95}, Strength: 100, Size: formats.Vec2{X: 29, Y: 31}, TurnSpeed: 12}
	a := weapons.NewNativeRNG()
	b := weapons.NewNativeRNG()
	got := rollSurvivalSpawnRecord(entry, turnRange{12, 12}, &a)
	want := rollZombieSpawnRecord(entry, &b)
	if got != want {
		t.Errorf("record %+v, want %+v", got, want)
	}
	if a != b {
		t.Errorf("a point turn drew random numbers the old roll did not")
	}
	// A range draws once, after speed and before the deviation values.
	c := weapons.NewNativeRNG()
	ranged := rollSurvivalSpawnRecord(entry, turnRange{10, 14}, &c)
	if ranged.TurnSpeed < 10 || ranged.TurnSpeed > 13 {
		t.Errorf("turn %d outside 10..13", ranged.TurnSpeed)
	}
	if ranged.Speed != want.Speed || ranged.HalfSize != want.HalfSize {
		t.Errorf("size or speed changed by the turn range")
	}
}

func TestSurvivalGateLimitPerBuild(t *testing.T) {
	boss := func(p *playState, dying bool) {
		p.zombies = []zombieState{{scriptID: 7, health: 100, dying: dying}}
		p.scriptEntities = map[int]*scriptEntity{7: {id: 7, entityType: "boss_rex"}}
	}
	cases := []struct {
		name     string
		build    string
		survival bool
		boss     bool
		dying    bool
		want     int
	}{
		{"1.2.5 story", content.WaveBuild125, false, false, false, 95},
		{"1.2.5 story with boss", content.WaveBuild125, false, true, false, 95},
		{"1.2.5 survival", content.WaveBuild125, true, false, false, 95},
		{"1.2.5 survival boss alive", content.WaveBuild125, true, true, false, 20},
		{"1.2.5 survival boss dying", content.WaveBuild125, true, true, true, 95},
		{"1.2.1 story", content.WaveBuildV7, false, false, false, 65},
		{"1.2.1 survival boss alive", content.WaveBuildV7, true, true, false, 65},
	}
	for _, c := range cases {
		p := survivalPlay(c.build, c.survival, nil)
		if c.boss {
			boss(p, c.dying)
		}
		if got := p.waveGateLimit(); got != c.want {
			t.Errorf("%s: gate limit %d, want %d", c.name, got, c.want)
		}
	}
}
