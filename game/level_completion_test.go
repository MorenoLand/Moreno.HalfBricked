package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"os"
	"testing"
)

func TestCachedStoryProgressionDestinations(t *testing.T) {
	root := "bin/web/data/data-cache"
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("original asset cache unavailable")
	}
	pack, err := content.NewPack(content.NewSource(root))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{levels: pack.List()}
	known := map[string]bool{}
	for _, info := range a.levels {
		known[info.ID] = true
	}
	checked := 0
	for _, info := range a.levels {
		if hasLevelFlag(info, "SURVIVAL") {
			continue
		}
		if info.NextLevel != "" {
			if _, _, err := a.storyLevelSelection(info.NextLevel); err != nil {
				t.Fatalf("%s: %v", info.ID, err)
			}
			checked++
		}
		for _, id := range info.UnlockLevels {
			if !known[id] {
				t.Fatalf("%s unlock target missing: %s", info.ID, id)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no original story transitions checked")
	}
	t.Logf("checked %d original story continuation links", checked)
}

func TestFinalWaveRetiresOnce(t *testing.T) {
	useNativeWaveEnd(t)
	p := &playState{world: &viewer.Viewer{Level: formats.Level{Waves: []formats.Wave{{RunTime: 5000, EndWaveTime: 3000}}}}, waveElapsed: 0, health: 1}
	// Native FUN_000bf120: the end_wave_time timer starts at 3000 - 10 ms and loses
	// (int)(dt * 1000) = 17 ms per 60 Hz frame; with no spawner left the wave
	// retires the frame it reaches zero (ceil(2990 / 17) = 176 frames).
	frames := 0
	for !p.wavesFinished && frames < 400 {
		p.updateWaves()
		frames++
	}
	if !p.wavesFinished || p.waveIndex != 1 || frames != 176 {
		t.Fatalf("final cleanup wave did not retire correctly (finished %v index %d after %d frames)", p.wavesFinished, p.waveIndex, frames)
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
func TestStoryCompletionUnlocksBeforeContinue(t *testing.T) {
	a := &app{}
	info := formats.LevelInfo{NextLevel: "World1Level0", UnlockLevels: []string{"World0Survival0", "World0Survival1", ""}}
	if err := a.recordStoryCompletion(info); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"World1Level0", "World0Survival0", "World0Survival1"} {
		if !a.unlocked[id] {
			t.Fatalf("completion failed to unlock %s", id)
		}
	}
	if a.unlocked[""] {
		t.Fatal("empty level unlocked")
	}
	if err := a.recordStoryCompletion(formats.LevelInfo{UnlockLevels: []string{"World5Survival0"}}); err != nil || !a.unlocked["World5Survival0"] {
		t.Fatal("terminal completion lost rewards")
	}
}
func TestInitialUnlocksRequireStoryPredecessorCompletion(t *testing.T) {
	levels := []formats.LevelInfo{{ID: "first", NextLevel: "second", Flags: []string{"STORY", "STARTUNLOCKED"}}, {ID: "second", NextLevel: "third", Flags: []string{"STORY", "STARTUNLOCKED"}}, {ID: "third", Flags: []string{"STORY", "STARTUNLOCKED"}}, {ID: "independent", Flags: []string{"STORY", "STARTUNLOCKED"}}, {ID: "survival", Flags: []string{"SURVIVAL", "STARTUNLOCKED"}}}
	a := &app{unlocked: initialUnlocks(levels)}
	if !a.unlocked["first"] || !a.unlocked["independent"] || !a.unlocked["survival"] || a.unlocked["second"] || a.unlocked["third"] {
		t.Fatalf("initial unlocks: %v", a.unlocked)
	}
	if err := a.recordStoryCompletion(levels[0]); err != nil {
		t.Fatal(err)
	}
	if !a.unlocked["second"] || a.unlocked["third"] {
		t.Fatalf("completion unlocks: %v", a.unlocked)
	}
}
func TestCached125StoryUnlocksFollowCompletionLinks(t *testing.T) {
	pack, err := content.NewPack(content.NewSource("bin/data-cache"))
	if err != nil {
		t.Skipf("1.2.5 asset cache unavailable: %v", err)
	}
	levels := pack.List()
	initial := initialUnlocks(levels)
	checked := 0
	for _, level := range levels {
		if !hasLevelFlag(level, "STORY") || level.NextLevel == "" {
			continue
		}
		if initial[level.NextLevel] {
			t.Fatalf("%s unlocked before completing %s", level.NextLevel, level.ID)
		}
		a := &app{unlocked: initialUnlocks(levels)}
		if err := a.recordStoryCompletion(level); err != nil {
			t.Fatal(err)
		}
		if !a.unlocked[level.NextLevel] {
			t.Fatalf("completing %s did not unlock %s", level.ID, level.NextLevel)
		}
		checked++
	}
	if checked == 0 {
		t.Fatal("no story completion links checked")
	}
}

func TestWorld0Level0ExitScriptUsesEntryWait(t *testing.T) {
	h := script125CachedHost(t, "world0_level0")
	p := h.play
	source, err := h.app.pack.ScriptSource("World0/Scripts/world0_level0_exit.script")
	if err != nil {
		t.Fatal(err)
	}
	// The entry script defines Wait; run just that definition in the shared state.
	entry, err := scripting.New("function Wait(amount)\nWaitInit(amount)\nwhile IsWaitComplete() == 0 do Idle() end\nend", h, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	if err := entry.Step(); err != nil {
		t.Fatal(err)
	}
	p.scriptRuntime, err = entry.Successor(source, h, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	for frame := 0; frame < 600 && !p.scriptRuntime.Done(); frame++ {
		if err := p.updateScript(); err != nil {
			t.Fatalf("frame=%d callback=%s: %v", frame, p.scriptLastCallback, err)
		}
	}
	if p.scriptRuntime.Err() != nil {
		t.Fatal(p.scriptRuntime.Err())
	}
}
