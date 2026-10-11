package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// The default end rule (port option, -wave-end=all-dead): a wave ends only when every spawner has delivered
// and no living zombie is left, whatever its timer says.
func TestAllDeadWaveEndWaitsForEveryZombie(t *testing.T) {
	old := waveEndRule
	waveEndRule = waveEndAllDead
	t.Cleanup(func() { waveEndRule = old })

	p := &playState{}
	zombies := []formats.Spawner{{Index: 1, Count: 3, Types: []formats.SpawnType{{Name: "zombie"}}}}
	timed := formats.Wave{RunTime: 1000, EndWaveTime: 2000, EndWaveZombies: 0, Spawners: zombies}
	// Everything is dead long before the 2 s timer: the wave still waits it out (the gap between waves).
	frames := 0
	for ; frames < 600; frames++ {
		p.waveElapsed += 1000.0 / 60.0
		if p.waveEndStep(timed, false, 0) {
			break
		}
	}
	if frames < 100 || frames > 140 {
		t.Fatalf("cleared wave ended after %d frames, want about its 2 s timer", frames)
	}
	q := &playState{}
	for i := 0; i < 400; i++ {
		q.waveElapsed += 1000.0 / 60.0
		if q.waveEndStep(timed, false, 2) {
			t.Fatal("two zombies alive: the wave must not end at its timer")
		}
	}
	q.waveElapsed += 1000.0 / 60.0
	if q.waveEndStep(timed, false, 2) {
		t.Fatal("still two alive")
	}
	if !q.waveEndStep(timed, false, 0) {
		t.Fatal("timer run out, spawners done, nothing alive: the wave ends")
	}
	r := &playState{}
	for i := 0; i < 400; i++ {
		r.waveElapsed += 1000.0 / 60.0
		if r.waveEndStep(timed, true, 0) {
			t.Fatal("a spawner still has zombies to deliver: no end")
		}
	}
	limited := formats.Wave{EndWaveTime: 0, EndWaveZombies: 10, Spawners: zombies}
	if p.waveEndStep(limited, false, 1) {
		t.Fatal("one zombie alive is not below the native limit, and the all-dead rule needs none")
	}
}

// The native rule stays available behind -wave-end=native.
func TestNativeWaveEndRuleIsSelectable(t *testing.T) {
	old := waveEndRule
	waveEndRule = waveEndNative
	t.Cleanup(func() { waveEndRule = old })

	p := &playState{}
	timed := formats.Wave{EndWaveTime: 2000}
	p.waveElapsed = 0
	ended := false
	for frame := 0; frame < 300 && !ended; frame++ {
		p.waveElapsed += 1000.0 / 60.0
		ended = p.waveEndStep(timed, false, 2)
	}
	if !ended {
		t.Fatal("the native timer must end a timed wave with two zombies alive")
	}
}

// Script zombies and bosses are not counted by the all-dead rule, so a boss fight cannot hold a wave open.
func TestAllDeadWaveIgnoresScriptAndBossZombies(t *testing.T) {
	old := waveEndRule
	waveEndRule = waveEndAllDead
	t.Cleanup(func() { waveEndRule = old })

	p := &playState{scriptEntities: map[int]*scriptEntity{9: {id: 9, kind: "zombie", entityType: "boss_rex"}}}
	p.zombies = []zombieState{{scriptID: 9, health: 100}, {scriptControlled: true, health: 100}}
	if got := p.waveAliveForEnd(); got != 0 {
		t.Fatalf("boss and scripted zombies counted: %d", got)
	}
	p.zombies = append(p.zombies, zombieState{health: 100})
	if got := p.waveAliveForEnd(); got != 1 {
		t.Fatalf("a wave zombie alive counted as %d, want 1", got)
	}
}
