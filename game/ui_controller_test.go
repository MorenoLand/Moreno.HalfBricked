package game

import (
	"os"
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func shippedController(t *testing.T, name string) *uiController {
	t.Helper()
	data, err := os.ReadFile("bin/data-cache/source/Common0/UserInterface/Controller/" + name + ".txt")
	if err != nil {
		t.Skip("shipped controller file not available:", err)
	}
	return parseUIController(string(data))
}

func TestControllerParserReadsDefaultBackAndLinks(t *testing.T) {
	c := shippedController(t, "MainScreen")
	if c.def != "PlayButton" || c.back != "QuitButton" {
		t.Fatalf("default/back = %q/%q, want PlayButton/QuitButton", c.def, c.back)
	}
	if target, ok := c.neighbour("PlayButton", "Left"); !ok || target != "QuitButton" {
		t.Fatalf("Play Left = %q,%v", target, ok)
	}
	if c.aliases["QuitButton"] != "@QuitZombie.Button" {
		t.Fatalf("alias = %q", c.aliases["QuitButton"])
	}
	q := shippedController(t, "QuitPrompt")
	if q.def != "Back" || q.back != "Back" {
		t.Fatalf("QuitPrompt default/back = %q/%q, want Back/Back", q.def, q.back)
	}
}

// The native left/right order is Options, Quit, Play, Stats (not the port's draw
// order Options, Play, Quit, Stats).
func TestMainMenuFocusFollowsMainScreenGraph(t *testing.T) {
	c := shippedController(t, "MainScreen")
	index := map[int]int{}
	for i, b := range mainMenuButtons {
		index[b.action] = i
	}
	options, play, quit, stats := index[2], index[0], index[4], index[3]
	for _, step := range []struct {
		from int
		dir  string
		want int
	}{
		{options, "Right", quit}, {quit, "Right", play}, {play, "Right", stats},
		{stats, "Left", play}, {play, "Left", quit}, {quit, "Left", options},
		{options, "Left", -1}, {stats, "Right", -1},
		{play, "Up", -1}, // Achievements has no button in the port
		{play, "Down", -1},
	} {
		if got := mainMenuTarget(c, step.from, step.dir); got != step.want {
			t.Fatalf("%d %s = %d, want %d", step.from, step.dir, got, step.want)
		}
	}
	if mainMenuTarget(nil, play, "Left") != -1 {
		t.Fatal("missing controller must not move focus")
	}
}

func TestNativeQuitPromptTextComesFromScreen(t *testing.T) {
	data, err := os.ReadFile("bin/data-cache/source/Common0/UserInterface/screens/QuitPrompt.uiscreen")
	if err != nil {
		t.Skip(err)
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		t.Fatal(err)
	}
	title, yes, no := nativeQuitPromptText(screen)
	if title != "Quit Age of Zombies?" || yes != "Quit" || no != "Back" {
		t.Fatalf("quit prompt = %q %q %q", title, yes, no)
	}
	if title, _, _ = nativeQuitPromptText(nil); title != "Quit Age of Zombies?" {
		t.Fatal("fallback title")
	}
}

func TestAchievementProgressTextMatchesScreenPlaceholder(t *testing.T) {
	data, err := os.ReadFile("bin/data-cache/source/Common0/UserInterface/screens/AchievementsScreen.uiscreen")
	if err != nil {
		t.Skip(err)
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		t.Fatal(err)
	}
	if c := screen.Find("ProgressText"); c == nil || c.String("text") != achievementProgressText(3, 12) {
		t.Fatalf("ProgressText = %v, want %q", c, achievementProgressText(3, 12))
	}
}
