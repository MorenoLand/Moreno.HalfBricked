package game

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Level playthrough tests. `go test ./game -run Playthrough` plays every level of both caches
// with a capped frame budget; AOZ_FULL_PLAYTHROUGH=1 lifts the caps (and AOZ_PLAYTHROUGH_REPORT=path
// writes the per-level table).

func ptFrameCap(story bool) int {
	if os.Getenv("AOZ_FULL_PLAYTHROUGH") != "" {
		return 60 * 60 * 40
	}
	if story {
		return 60 * 60 * 20
	}
	return 60 * 60 * 3
}

// ptStoryOrder lists story levels along the NextLevel chain starting from BEGINSTORY levels,
// followed by any story level not reached (so every level is played).
func ptStoryOrder(levels []formats.LevelInfo) []formats.LevelInfo {
	byID := map[string]formats.LevelInfo{}
	for _, l := range levels {
		byID[strings.ToLower(l.ID)] = l
	}
	var order []formats.LevelInfo
	seen := map[string]bool{}
	for _, l := range levels {
		if !hasLevelFlag(l, "BEGINSTORY") {
			continue
		}
		for cur, ok := l, true; ok && !seen[strings.ToLower(cur.ID)]; {
			seen[strings.ToLower(cur.ID)] = true
			order = append(order, cur)
			cur, ok = byID[strings.ToLower(cur.NextLevel)]
		}
	}
	for _, l := range levels {
		if !hasLevelFlag(l, "SURVIVAL") && !seen[strings.ToLower(l.ID)] {
			seen[strings.ToLower(l.ID)] = true
			order = append(order, l)
		}
	}
	return order
}

func ptFormat(o *ptOutcome) string {
	status := "PASS"
	if !o.Pass {
		status = "FAIL"
	}
	if o.God && o.Pass {
		status = "PASS(god)"
	}
	return fmt.Sprintf("%-9s %-9s %-22s %-8s frames=%-6d t=%5.0fs waves=%d/%d kills=%-4d bosses=%v results=%s next=%s entry@%d exit@%d | %s | %s", status, o.Cache, o.ID, o.Mode, o.Frames, o.Seconds, o.Waves, o.WavesLen, o.Kills, o.Bosses, o.ResultsOpened, o.NextLoaded, o.EntryDoneFrame, o.ExitStarted, strings.Join(o.Issues, "; "), strings.Join(o.Notes, "; "))
}

func TestPlaythroughStaticAudit(t *testing.T) {
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		for _, info := range a.levels {
			out := &ptOutcome{Cache: cache.name, ID: info.ID, Pass: true}
			ptStaticAudit(a, info, out)
			for _, issue := range out.Issues {
				t.Errorf("%s %s: %s", cache.name, info.ID, issue)
			}
		}
	}
}

func TestPlaythroughScriptCallbacksAreRegistered(t *testing.T) {
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		paths := map[string]bool{}
		for key := range a.pack.Manifest().Files {
			if strings.HasSuffix(strings.ToLower(key), ".script") {
				paths[key] = true
			}
		}
		if len(paths) == 0 {
			t.Fatalf("%s: no scripts in manifest", cache.name)
		}
		for path := range paths {
			source, err := a.pack.ScriptSource(path)
			if err != nil {
				t.Errorf("%s %s: %v", cache.name, path, err)
				continue
			}
			// exit scripts run on the entry script's VM (Successor) and may use its helpers
			var shared []string
			if strings.HasSuffix(strings.ToLower(path), "_exit.script") {
				if entry, err := a.pack.ScriptSource(strings.TrimSuffix(path, "_exit.script") + "_entry.script"); err == nil {
					shared = append(shared, entry)
				}
			}
			if _, unknown := ptScriptCalls(source, shared...); len(unknown) > 0 {
				if strings.HasSuffix(strings.ToLower(path), "world4_level0_entry.script") && strings.Join(unknown, ",") == "Update" {
					// Update() appears only inside WalkThePlayerTo, a helper this script defines but never calls.
					continue
				}
				t.Errorf("%s %s calls unregistered functions %v", cache.name, path, unknown)
			}
		}
	}
}

var ptPlaySFXRE = regexp.MustCompile(`(?m)^[^-
]*PlaySFX\(\s*"([^"]+)"`)

// Every PlaySFX name used by a script, and every level's music track, must resolve to a file of the pack.
func TestPlaythroughSoundsResolve(t *testing.T) {
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		open := func(path string) bool {
			reader, err := a.pack.Open(path)
			if err != nil {
				return false
			}
			reader.Close()
			return true
		}
		for _, info := range a.levels {
			if path, _, ok := levelMusicTrack(info); !ok || !open(path) {
				t.Errorf("%s %s: music %q (%s) does not resolve", cache.name, info.ID, info.Music, path)
			}
		}
		for key := range a.pack.Manifest().Files {
			if !strings.HasSuffix(strings.ToLower(key), ".script") {
				continue
			}
			source, err := a.pack.ScriptSource(key)
			if err != nil {
				t.Errorf("%s %s: %v", cache.name, key, err)
				continue
			}
			for _, match := range ptPlaySFXRE.FindAllStringSubmatch(source, -1) {
				if path := a.scriptSoundPath(match[1]); path == "" || !open(path) {
					t.Errorf("%s %s: PlaySFX(%q) resolves to %q which is not in the pack", cache.name, key, match[1], path)
				}
			}
		}
	}
}

func TestPlaythroughAllLevels(t *testing.T) {
	var report []string
	failed := 0
	seeds := 1
	if value, err := strconv.Atoi(os.Getenv("AOZ_PLAYTHROUGH_SEEDS")); err == nil && value > 1 {
		seeds = value // extra runs with a different RNG stream and bot phase: spawn points, boss positions, pickups
	}
	for _, cache := range ptCaches() {
		for seed := 0; seed < seeds; seed++ {
			cacheName := cache.name
			if seed > 0 {
				cacheName = fmt.Sprintf("%s#%d", cache.name, seed)
			}
			a := newHeadlessApp(t, cache.root)
			for draw := 0; draw < seed*997; draw++ {
				a.rng.Bounded(1 << 30)
			}
			var order []formats.LevelInfo
			order = append(order, ptStoryOrder(a.levels)...)
			for _, l := range a.levels {
				if hasLevelFlag(l, "SURVIVAL") {
					order = append(order, l)
				}
			}
			storyDone := false
			for _, info := range order {
				if !ptSelected(cache.name, info.ID) {
					continue
				}
				survival := hasLevelFlag(info, "SURVIVAL")
				if survival && !storyDone {
					storyDone = true
					ptCheckEverythingUnlocked(t, a, cacheName)
				}
				out := ptRun(t, a, cacheName, info, ptOptions{maxFrames: ptFrameCap(!survival), seed: seed}, true)
				if !out.Pass && !survival && strings.Contains(strings.Join(out.Issues, ";"), "died") {
					first := out
					out = ptRun(t, a, cacheName, info, ptOptions{maxFrames: ptFrameCap(true), god: true, seed: seed}, false)
					out.Notes = append(out.Notes, "bot without god mode: "+strings.Join(first.Issues, "; "))
				}
				if os.Getenv("AOZ_PLAYTHROUGH_NO_DEATH") == "" {
					if issues := ptDeathFlow(a, info); len(issues) > 0 {
						out.Pass = false
						out.Issues = append(out.Issues, issues...)
					}
				}
				line := ptFormat(out)
				report = append(report, line)
				t.Log(line)
				if !out.Pass {
					failed++
					t.Errorf("%s %s: %s", cacheName, info.ID, strings.Join(out.Issues, "; "))
				}
			}
		}
	}
	if path := os.Getenv("AOZ_PLAYTHROUGH_REPORT"); path != "" {
		_ = os.WriteFile(path, []byte(strings.Join(report, "\n")+"\n"), 0644)
	}
	_ = failed
}

// ptSelected applies AOZ_PLAYTHROUGH_ONLY, a comma separated list of case-insensitive substrings matched against
// "<cache>/<level id>" (for example "sd-1.2.1/world2level2,president").
func ptSelected(cache, id string) bool {
	filter := strings.TrimSpace(os.Getenv("AOZ_PLAYTHROUGH_ONLY"))
	if filter == "" {
		return true
	}
	name := strings.ToLower(cache + "/" + id)
	for _, part := range strings.Split(filter, ",") {
		if part = strings.ToLower(strings.TrimSpace(part)); part != "" && strings.Contains(name, part) {
			return true
		}
	}
	return false
}

// Every survival level starts with the survival intro script (native: scripts/survival_tute.script from the survival
// game state's slot 16, 1.2.5 FUN_000995a8) and the waves only begin once it has finished.
func TestSurvivalLevelsOpenWithTheIntroScript(t *testing.T) {
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		for _, info := range a.levels {
			if !hasLevelFlag(info, "SURVIVAL") {
				continue
			}
			a.resultsScreen, a.play = nil, nil
			if err := a.selectCaptureLevel(info.ID); err != nil {
				t.Fatal(err)
			}
			if err := a.openPlay(); err != nil {
				t.Fatalf("%s %s: %v", cache.name, info.ID, err)
			}
			p := a.play
			if p.scriptRuntime == nil || p.scriptRuntime.Done() {
				t.Fatalf("%s %s: no survival intro script is running", cache.name, info.ID)
			}
			bot := &ptBot{a: a}
			a.inputHook = bot.input
			spawnedDuringIntro := false
			frame := 0
			for ; frame < 1200 && !p.scriptRuntime.Done(); frame++ {
				if err := a.Update(); err != nil {
					t.Fatalf("%s %s frame %d: %v", cache.name, info.ID, frame, err)
				}
				if len(p.zombies) > 0 && !p.scriptRuntime.Done() {
					spawnedDuringIntro = true
				}
			}
			a.inputHook = nil
			if !p.scriptRuntime.Done() {
				t.Fatalf("%s %s: the survival intro never finished", cache.name, info.ID)
			}
			if spawnedDuringIntro {
				t.Errorf("%s %s: zombies spawned while the intro was still running", cache.name, info.ID)
			}
		}
	}
}

// Continue on the last story level (no NextLevel: World5Level2, president_story_2) must not fail the frame.
func TestContinueOnTheFinalStoryLevelLeavesForTheMainMenu(t *testing.T) {
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		for _, info := range a.levels {
			if info.NextLevel != "" || hasLevelFlag(info, "SURVIVAL") || !hasLevelFlag(info, "ENDSTORY") {
				continue
			}
			a.resultsScreen, a.play = nil, nil
			if err := a.selectCaptureLevel(info.ID); err != nil {
				t.Fatal(err)
			}
			if err := a.openPlay(); err != nil {
				t.Fatal(err)
			}
			a.page = 5
			if err := a.continueStoryLevel(a.play.levelInfo); err != nil {
				t.Fatalf("%s %s: Continue failed: %v", cache.name, info.ID, err)
			}
			if a.play != nil || a.page != 0 {
				t.Errorf("%s %s: Continue left page %d, play %v; want the main menu", cache.name, info.ID, a.page, a.play != nil)
			}
			for _, id := range info.UnlockLevels {
				if !a.unlocked[id] {
					t.Errorf("%s %s: %s not unlocked by completing the level", cache.name, info.ID, id)
				}
			}
		}
	}
}

// The simulation (and so the whole playthrough) must be a pure function of the data: two runs of the same level
// with the same bot give the same state on every frame. Go map order once leaked into pickup collection and the
// bot's pickup choice and made the playthrough results change from run to run.
func TestPlaythroughIsDeterministic(t *testing.T) {
	trace := func(root, id string) []string {
		a := newHeadlessApp(t, root)
		if err := a.selectCaptureLevel(id); err != nil {
			t.Fatal(err)
		}
		if err := a.openPlay(); err != nil {
			t.Fatal(err)
		}
		bot := &ptBot{a: a}
		a.inputHook = bot.input
		a.play.cheats.infiniteAmmo = true
		var states []string
		for frame := 0; frame < 4000; frame++ {
			p := a.play
			if p.dialogueIndex >= 0 && p.dialogueIndex < len(p.dialogue) && frame%25 == 0 {
				p.dialogueIndex++
			}
			if err := a.Update(); err != nil {
				t.Fatal(err)
			}
			states = append(states, fmt.Sprintf("%.4f,%.4f z%d k%d e%d rng%v", p.x, p.y, len(p.zombies), p.levelKills, len(p.scriptEntities), *p.rng))
		}
		return states
	}
	for _, cache := range ptCaches() {
		for _, id := range []string{"World0Level1", "World3Level1", "World2Survival0"} {
			first, second := trace(cache.root, id), trace(cache.root, id)
			for frame := range first {
				if first[frame] != second[frame] {
					t.Fatalf("%s %s diverges at frame %d: %s vs %s", cache.name, id, frame, first[frame], second[frame])
				}
			}
		}
	}
}

// Completing the whole story (the NextLevel chain plus every UnlockLevels list) must unlock every playable level.
func ptCheckEverythingUnlocked(t *testing.T, a *app, cache string) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("AOZ_PLAYTHROUGH_ONLY")) != "" {
		return
	}
	for _, info := range a.levels {
		if len(info.Flags) > 0 && !a.unlocked[info.ID] {
			t.Errorf("%s %s is still locked after the whole story was completed", cache, info.ID)
		}
	}
}

// AOZ_PLAYTHROUGH_FUZZ=<n> replays every story level's exit script n times from random boss / player end positions and
// every wave boss intro n times from random boss / player start positions.
func TestPlaythroughExitScriptsSurviveAnyEndPosition(t *testing.T) {
	samples, _ := strconv.Atoi(os.Getenv("AOZ_PLAYTHROUGH_FUZZ"))
	if samples <= 0 {
		t.Skip("set AOZ_PLAYTHROUGH_FUZZ=<samples per level> to run")
	}
	for _, cache := range ptCaches() {
		a := newHeadlessApp(t, cache.root)
		for _, info := range a.levels {
			if hasLevelFlag(info, "SURVIVAL") || len(info.Flags) == 0 || !ptSelected(cache.name, info.ID) {
				continue
			}
			for sample := 0; sample < samples; sample++ {
				if problem := ptExitFuzz(a, info, sample); problem != "" {
					t.Errorf("%s %s sample %d: %s", cache.name, info.ID, sample, problem)
				}
				if problem := ptBossIntroFuzz(a, info, sample); problem != "" {
					t.Errorf("%s %s sample %d: %s", cache.name, info.ID, sample, problem)
				}
			}
		}
	}
}
