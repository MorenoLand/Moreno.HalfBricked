package game

import (
	"math"
	"os"
	"strings"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/scripting"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
)

func TestScriptCameoYMatchesNativeFormula(t *testing.T) {
	// 1.2.5 FUN_0012a8dc(1) for the port's 480x320 surface: 320 - 65 - 32 + 1 = 224.
	if got := scriptCameoY(480, 320); math.Abs(float64(got)-224) > 1e-3 {
		t.Fatalf("GetCameoY(480x320) = %v, want 224", got)
	}
	// 16:9 surface: 320 - 65 - (62/3) * W/H.
	want := 255 - 62.0/3.0*1920.0/1080.0
	if got := scriptCameoY(1920, 1080); math.Abs(float64(got)-want) > 1e-3 {
		t.Fatalf("GetCameoY(1920x1080) = %v, want %v", got, want)
	}
}

func TestGetCameoYAndShowSkipThroughScript(t *testing.T) {
	p := &playState{scriptTextures: map[int]*scriptTexture{}}
	h := &playScriptHost{app: &app{silent: true}, play: p}
	runtime, err := scripting.New("LoadTexture(0, \"Cameos/barrycameo\")\nSetTexturePos(0, -32, GetCameoY())\nShowSkip(true)", h, scriptCallbacks)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	if err := runtime.Step(); err != nil {
		t.Fatal(err)
	}
	texture := p.scriptTextures[0]
	if texture == nil || texture.x != -32 || math.Abs(texture.y-224) > 1e-3 {
		t.Fatalf("cameo texture = %+v, want x -32 y 224", texture)
	}
	if !p.scriptShowSkip {
		t.Fatal("ShowSkip(true) did not store the flag")
	}
}

func TestCreditsRollMatchesNativeTable(t *testing.T) {
	if len(creditsRoll) != 116 {
		t.Fatalf("roll has %d entries, want 116", len(creditsRoll))
	}
	if creditsRoll[0].Text != "Mobile Adaptation" || creditsRoll[0].Style != 0 || creditsRoll[1].Text != "Design" || creditsRoll[1].Style != 3 || creditsRoll[2].Text != "Michael Dobele" || creditsRoll[2].Style != 1 {
		t.Fatalf("roll head: %+v", creditsRoll[:3])
	}
	last := creditsRoll[len(creditsRoll)-1]
	if last.Style != 2 || last.Text != "" || creditsRoll[len(creditsRoll)-3].Text != "Xavier Yun Ho" {
		t.Fatalf("roll tail: %+v", creditsRoll[len(creditsRoll)-4:])
	}
	if got := creditsVersionLine(); got != "Game Version: 1.2.5 (54)" {
		t.Fatal(got)
	}
}

func TestCreditsTrackStepWraps(t *testing.T) {
	if got := creditsStepTrack(0, -1); got != 6 {
		t.Fatalf("prev from 0 = %d, want 6", got)
	}
	if got := creditsStepTrack(6, 1); got != 0 {
		t.Fatalf("next from 6 = %d, want 0", got)
	}
	if got := creditsStepTrack(3, 1); got != 4 {
		t.Fatal(got)
	}
}

func TestCreditsControllerGraphAndFocus(t *testing.T) {
	controller := parseUIController("Back=CreditsScreen.BackBox.Box.Back;\nPrevTrack=CreditsScreen.BackBox.Box.PrevTrack;\nNextTrack=CreditsScreen.BackBox.Box.NextTrack;\nCredits=CreditsScreen.Screen.Backing.ComponentCredits;\n\nBack Left:NextTrack IsDefault IsBack;\nNextTrack Left:PrevTrack Right:Back;\nPrevTrack Left:Credits Right:NextTrack;\nCredits Right:PrevTrack;\n")
	if controller.def != "Back" || controller.back != "Back" {
		t.Fatalf("default %q back %q", controller.def, controller.back)
	}
	focus := "Back"
	for _, step := range []struct{ dir, want string }{{"Left", "NextTrack"}, {"Left", "PrevTrack"}, {"Left", "Credits"}, {"Left", "Credits"}, {"Right", "PrevTrack"}, {"Right", "NextTrack"}, {"Right", "Back"}, {"Right", "Back"}} {
		focus = creditsNavigate(controller, focus, step.dir)
		if focus != step.want {
			t.Fatalf("after %s focus %s, want %s", step.dir, focus, step.want)
		}
	}
}

func TestCreditsAnimationInputGate(t *testing.T) {
	c := newCreditsState(creditsExit{}, "")
	if c.focus != "Back" || c.inputEnabled() || c.slide() != 0 {
		t.Fatal("credits must start sliding in with input disabled and Back focused")
	}
	for frame := 0; frame < 29; frame++ {
		if c.step(1.0 / 60.0) {
			t.Fatal("finished while sliding in")
		}
	}
	if c.inputEnabled() {
		t.Fatal("input enabled before AnimateIn ended")
	}
	c.step(1.0 / 60.0)
	c.step(1.0 / 60.0)
	if !c.inputEnabled() || c.slide() != 1 {
		t.Fatalf("after AnimateIn: input %v slide %v", c.inputEnabled(), c.slide())
	}
	c.phase, c.phaseTime = creditsOut, 0
	if c.inputEnabled() {
		t.Fatal("input enabled during AnimateOut")
	}
	done := false
	for frame := 0; frame < 40 && !done; frame++ {
		done = c.step(1.0 / 60.0)
	}
	if !done {
		t.Fatal("AnimateOut never finished")
	}
}

func TestCreditsRollScrollsUpAndLoops(t *testing.T) {
	c := newCreditsState(creditsExit{}, "")
	c.phase = creditsIdle
	c.step(1)
	if math.Abs(c.scroll-creditsScrollSpeed) > 1e-9 {
		t.Fatalf("scroll after 1 s = %v, want %v", c.scroll, creditsScrollSpeed)
	}
	c.scroll = creditsRollLength() - 1
	c.step(1)
	if c.scroll < 0 || c.scroll >= creditsRollLength() || math.Abs(c.scroll-(creditsScrollSpeed-1)) > 1e-9 {
		t.Fatalf("scroll did not loop: %v", c.scroll)
	}
}

func creditsTestApp(t *testing.T, root string, build string, levels ...formats.LevelInfo) *app {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Skip("asset cache unavailable")
	}
	pack, err := content.NewPack(content.NewSource(root))
	if err != nil {
		t.Fatal(err)
	}
	a := &app{pack: pack, silent: true, levels: levels, unlocked: map[string]bool{}}
	a.play = &playState{world: &viewer.Viewer{Level: formats.Level{WaveBuild: build}}, score: 1234, combatKillCount: 7}
	return a
}

func TestHDFinalLevelContinueEntersCreditsThenMenu(t *testing.T) {
	a := creditsTestApp(t, "bin/data-cache", content.WaveBuild125)
	if !a.creditsAvailable() {
		t.Skip("HD CreditsScreen.uiscreen unavailable")
	}
	info := formats.LevelInfo{ID: "World5Level2", Flags: []string{"STORY", "ENDWORLD", "ENDSTORY"}}
	if err := a.continueStoryLevel(info); err != nil {
		t.Fatal(err)
	}
	if a.page != creditsPage || a.credits == nil || a.credits.exit.hasNext || a.play == nil {
		t.Fatalf("page %d credits %v: final level must enter Credits", a.page, a.credits)
	}
	if err := a.finishCredits(); err != nil {
		t.Fatal(err)
	}
	if a.page != 0 || a.play != nil || a.credits != nil {
		t.Fatalf("Credits exit: page %d play %v", a.page, a.play)
	}
}

func TestHDShowCreditsLevelEntersCreditsBeforeNextLevel(t *testing.T) {
	next := formats.LevelInfo{ID: "World5Level0", WorldIndex: 5, Flags: []string{"STORY"}}
	a := creditsTestApp(t, "bin/data-cache", content.WaveBuild125, next)
	if !a.creditsAvailable() {
		t.Skip("HD CreditsScreen.uiscreen unavailable")
	}
	info := formats.LevelInfo{ID: "World4Level2", NextLevel: "World5Level0", Flags: []string{"STORY", "ENDWORLD", "SHOWCREDITS"}}
	if err := a.continueStoryLevel(info); err != nil {
		t.Fatal(err)
	}
	if a.page != creditsPage || a.credits == nil || !a.credits.exit.hasNext {
		t.Fatalf("SHOWCREDITS level did not enter Credits (page %d)", a.page)
	}
	if !a.unlocked["World5Level0"] {
		t.Fatal("completion rewards lost")
	}
}

func TestHDNextLevelNotFoundEntersCredits(t *testing.T) {
	a := creditsTestApp(t, "bin/data-cache", content.WaveBuild125)
	if !a.creditsAvailable() {
		t.Skip("HD CreditsScreen.uiscreen unavailable")
	}
	if err := a.continueStoryLevel(formats.LevelInfo{ID: "x", NextLevel: "missing", Flags: []string{"STORY"}}); err != nil {
		t.Fatal(err)
	}
	if a.page != creditsPage || a.credits == nil || a.credits.exit.hasNext {
		t.Fatal("missing next level must enter Credits then menu")
	}
}

func TestSDFinalLevelKeepsDirectMenuReturn(t *testing.T) {
	a := creditsTestApp(t, "bin/web/data/data-cache", content.WaveBuildV7)
	if a.creditsAvailable() {
		t.Fatal("SD build must not use the HD Credits screen")
	}
	if err := a.continueStoryLevel(formats.LevelInfo{ID: "World5Level2", Flags: []string{"STORY", "ENDWORLD", "ENDSTORY"}}); err != nil {
		t.Fatal(err)
	}
	if a.page != 0 || a.play != nil || a.credits != nil {
		t.Fatalf("SD final level: page %d", a.page)
	}
}

func TestCreditsTrackNamesNotEmpty(t *testing.T) {
	for _, name := range creditsTrackNames {
		if strings.TrimSpace(name) == "" {
			t.Fatal("empty track name")
		}
	}
}
