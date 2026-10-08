package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"math"
	"testing"
)

func TestLevelZombieCountMatchesNativeSpawnerFilter(t *testing.T) {
	waves := []formats.Wave{{Spawners: []formats.Spawner{{Count: 10, Types: []formats.SpawnType{{Name: "zombie"}, {Name: "armed_zombie"}}}, {Count: 3, Types: []formats.SpawnType{{Name: "p_grenade"}}}, {Count: 4, Types: []formats.SpawnType{{Name: "zombie"}, {Name: "p_health"}}}, {Count: 5, Types: []formats.SpawnType{{Name: "prospector_zombie"}}}, {Count: 1, Types: []formats.SpawnType{{Name: "boss_rex"}}}, {Count: 20, Types: []formats.SpawnType{{Name: "zombie"}}}}}}
	if got := levelZombieCount(waves, false); got != 15 {
		t.Fatalf("story total = %d, want 15 before the boss", got)
	}
	if got := levelZombieCount(waves, true); got != 0 {
		t.Fatalf("survival total = %d, want 0", got)
	}
}
func TestLevelProgressUsesKillsAndNativeOpacity(t *testing.T) {
	p := &playState{levelZombieTotal: 20, health: .01, maxHealth: 1, hudVisible: true}
	p.updateProgressOpacity(.2)
	remaining, alpha := p.levelProgress()
	if remaining != 1 || alpha != 178 {
		t.Fatalf("initial progress = %f/%d, want 1/178", remaining, alpha)
	}
	p.levelKills = 5
	p.updateProgressOpacity(.2)
	remaining, alpha = p.levelProgress()
	if remaining != .75 || alpha != 255 {
		t.Fatalf("quarter complete = %f/%d, want .75/255", remaining, alpha)
	}
	p.hudVisible = false
	p.updateProgressOpacity(.25)
	remaining, alpha = p.levelProgress()
	if remaining != .75 || alpha != 127 {
		t.Fatalf("fade out = %f/%d, want .75/127", remaining, alpha)
	}
	p.levelKills = 25
	remaining, _ = p.levelProgress()
	if remaining != 0 {
		t.Fatalf("over-complete progress = %f, want 0", remaining)
	}
}
func TestFinalZombieRemovalCountsEachDeathOnce(t *testing.T) {
	collision := make([]uint32, 64)
	for i := range collision {
		collision[i] = math.MaxUint32
	}
	p := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 8, Height: 8, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}, Zoom: 1}, tileSize: 32, x: 32, y: 32, health: 1, maxHealth: 1, zombies: []zombieState{{health: 0, dying: true, deathAge: zombieDeathDelay}, {health: 0, spawnAway: true}, {x: 160, y: 160, health: 100}}}
	p.Update(0, 0, false, false, false)
	if p.levelKills != 2 || len(p.zombies) != 1 || len(p.bloodPops) != 1 {
		t.Fatalf("death finalization = kills %d, zombies %d, pops %d; want 2/1/1", p.levelKills, len(p.zombies), len(p.bloodPops))
	}
	p.Update(0, 0, false, false, false)
	if p.levelKills != 2 {
		t.Fatalf("second update counted deaths again: %d", p.levelKills)
	}
}
func TestZombieScoreAwardUsesInitialPointsAndHUDGate(t *testing.T) {
	for _, test := range []struct {
		points, multiplier, start, want int
		visible                         bool
	}{{100, 1, 0, 5, true}, {100, 3, 0, 15, true}, {19, 4, 0, 0, true}, {39, 2, 0, 2, true}, {40, 2, 0, 4, true}, {100, 3, 0, 0, false}, {100, math.MaxInt32, 0, math.MaxInt32 - 4, true}, {100, 1, math.MaxInt32, math.MinInt32 + 4, true}} {
		collision := make([]uint32, 64)
		for i := range collision {
			collision[i] = math.MaxUint32
		}
		p := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 8, Height: 8, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}, Zoom: 1}, tileSize: 32, x: 32, y: 32, health: 1, maxHealth: 1, hudVisible: test.visible, multiplier: test.multiplier, score: test.start, zombies: []zombieState{{health: -400, rawPoints: test.points, dying: true, deathAge: zombieDeathDelay}}}
		p.Update(0, 0, false, false, false)
		p.Update(0, 0, false, false, false)
		if p.score != test.want || p.levelKills != 1 {
			t.Fatalf("points %d, multiplier %d, visible %t: score/kills %d/%d, want %d/1", test.points, test.multiplier, test.visible, p.score, p.levelKills, test.want)
		}
	}
}
func TestScriptZombieStoresNativeInitialScorePoints(t *testing.T) {
	p := &playState{scriptNextEntity: 1, scriptEntities: map[int]*scriptEntity{}}
	if _, err := (&playScriptHost{play: p}).spawnZombie([]scripting.Value{100, 100, 32, 0}); err != nil {
		t.Fatal(err)
	}
	if p.zombies[0].health != 100 || p.zombies[0].rawPoints != 100 {
		t.Fatalf("scripted zombie = %#v, want health/rawPoints 100/100", p.zombies[0])
	}
	p.zombies[0].health -= 500
	if p.zombies[0].rawPoints != 100 {
		t.Fatal("damage changed the native initial score value")
	}
}
func TestWaveZombieStrengthUsesNativeHealthAndScoreDefaults(t *testing.T) {
	for _, test := range []struct {
		strength, health float64
		points           int
	}{{100, 100, 100}, {0, 0, 0}, {-1, 100, 0}, {39, 39, 39}} {
		collision := make([]uint32, 64)
		collision[9] = 3
		rng := weapons.NewNativeRNG()
		p := &playState{world: &viewer.Viewer{Level: formats.Level{Width: 8, Height: 8, Layers: map[formats.LayerKind][]uint32{formats.LayerC: collision}}}, tileSize: 32, rng: &rng}
		p.spawnZombie(formats.Spawner{Index: 1, Types: []formats.SpawnType{{Name: "zombie", Chance: 1, Strength: test.strength}}}, 0)
		if len(p.zombies) != 1 || p.zombies[0].health != test.health || p.zombies[0].rawPoints != test.points {
			t.Fatalf("strength %f produced %#v, want health %f points %d", test.strength, p.zombies, test.health, test.points)
		}
	}
}

func TestSurvivalBarShowsTheCurrentWaveRemaining(t *testing.T) {
	p := &playState{levelInfo: formats.LevelInfo{Flags: []string{"SURVIVAL"}}, progressOpacity: 1, waveSpawned: []int{2}}
	p.world = &viewer.Viewer{Level: formats.Level{Waves: []formats.Wave{{Spawners: []formats.Spawner{{Index: 1, Count: 4, Types: []formats.SpawnType{{Name: "zombie"}}}}}}}}
	p.zombies = []zombieState{{health: 10}}
	remaining, alpha := p.levelProgress()
	if alpha == 0 || remaining != .75 {
		t.Fatalf("2 unspawned + 1 alive of 4 should leave .75, got %v alpha %d", remaining, alpha)
	}
	p.levelInfo.Flags = nil
	if _, alpha := p.levelProgress(); alpha != 0 {
		t.Fatal("story levels keep the native total-based meter")
	}
}
