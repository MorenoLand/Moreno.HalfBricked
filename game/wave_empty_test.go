package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// A wave with no zombies (crates only) must not be skipped on its first frame by the all-dead rule.
func TestEmptyWaveRunsItsTimerUnderTheAllDeadRule(t *testing.T) {
	old := waveEndRule
	waveEndRule = waveEndAllDead
	defer func() { waveEndRule = old }()
	p := modelRig()
	p.world.Level.Waves = []formats.Wave{{
		RunTime: 5000, EndWaveTime: 3000, EndWaveZombies: 0,
		Spawners: []formats.Spawner{{Index: 1, Count: 1, Types: []formats.SpawnType{{Name: "p_shotgun"}}}},
	}}
	frames := 0
	for ; frames < 60*10; frames++ {
		p.waveElapsed += 1000.0 / 60.0
		if p.waveEndStep(p.world.Level.Waves[0], false, 0) {
			break
		}
	}
	if frames < 60*2 || frames > 60*4 {
		t.Fatalf("empty wave ended after %d frames, want about 3 s", frames)
	}
}
