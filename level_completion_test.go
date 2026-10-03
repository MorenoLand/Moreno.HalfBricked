package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"testing"
)

func TestFinalWaveRetiresOnce(t *testing.T) {
	p := &playState{world: &viewer.Viewer{Level: formats.Level{Waves: []formats.Wave{{RunTime: 5000, EndWaveTime: 3000}}}}, waveElapsed: 8000, health: 1}
	p.updateWaves()
	if !p.wavesFinished || p.waveIndex != 1 {
		t.Fatal("final cleanup wave did not retire")
	}
	time := p.waveElapsed
	p.updateWaves()
	if p.waveIndex != 1 || p.waveElapsed != time {
		t.Fatal("retired wave advanced again")
	}
}
func TestExitScriptCompletionGates(t *testing.T) {
	p := &playState{health: 1, wavesFinished: true}
	if !p.readyForExitScript() {
		t.Fatal("cleared level not ready")
	}
	p.zombies = []zombieState{{dying: true}}
	if p.readyForExitScript() {
		t.Fatal("pending zombie removal skipped")
	}
	p.zombies = nil
	runtime, err := scripting.New("coroutine.yield()", &playScriptHost{play: p}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	p.scriptRuntime = runtime
	if p.readyForExitScript() {
		t.Fatal("active entry script skipped")
	}
	p.scriptRuntime = nil
	p.exitScriptStarted = true
	if p.readyForExitScript() {
		t.Fatal("exit script restarted")
	}
	p.exitScriptStarted, p.health = false, 0
	if p.readyForExitScript() {
		t.Fatal("dead player completed level")
	}
}
func TestExitScriptUsesOriginalLevelMetadata(t *testing.T) {
	info := formats.LevelInfo{BaseFile: "world0_level0", SourceXML: "World0/Xml/World0_Levels.xml"}
	if got := exitScriptPath(info); got != "World0/Scripts/world0_level0_exit.script" {
		t.Fatal(got)
	}
	a := &app{mode: 1, play: &playState{health: 1, wavesFinished: true}}
	if err := a.updateLevelCompletion(); err != nil || a.play.exitScriptStarted {
		t.Fatal("survival started story outro")
	}
}
func TestStoryContinuationUsesNamedLevelNotAdjacentIndex(t *testing.T) {
	a := &app{levels: []formats.LevelInfo{{ID: "World0Level0", WorldIndex: 0}, {ID: "World0Survival0", WorldIndex: 0, Flags: []string{"SURVIVAL"}}, {ID: "World0Level1", WorldIndex: 0}, {ID: "World1Level0", WorldIndex: 1}}}
	world, level, err := a.storyLevelSelection("World0Level1")
	if err != nil || world != 0 || level != 1 {
		t.Fatalf("selection %d,%d: %v", world, level, err)
	}
	if _, _, err := a.storyLevelSelection("World0Survival0"); err == nil {
		t.Fatal("story continued into survival")
	}
	if _, _, err := a.storyLevelSelection("missing"); err == nil {
		t.Fatal("missing next level silently accepted")
	}
}
