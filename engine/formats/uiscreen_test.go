package formats

import (
	"math"
	"os"
	"testing"
)

func TestParseEndScreenLayout(t *testing.T) {
	data, err := os.ReadFile("../../bin/data-cache/source/Common0/UserInterface/screens/EndScreen.uiscreen")
	if err != nil {
		t.Skip("EndScreen.uiscreen not cached")
	}
	screen, err := ParseUIScreen(data)
	if err != nil {
		t.Fatal(err)
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < .6 }
	for _, tc := range []struct {
		name string
		x, y float64
	}{{"StatsBox", 392, 70}, {"MenuBox", 392, 161.5}, {"ReplayBox", 392, 234.5}} {
		component := screen.Find(tc.name)
		if component == nil {
			t.Fatalf("%s missing", tc.name)
		}
		if x, y := component.Anchor(); !near(x, tc.x) || !near(y, tc.y) {
			t.Fatalf("%s anchor %.1f,%.1f want %.1f,%.1f", tc.name, x, y, tc.x, tc.y)
		}
	}
	// ScoreBox is top-left anchored; its Screen child is centred on (154,70).
	score := screen.Find("ScoreBox")
	if score == nil || len(score.Children) == 0 {
		t.Fatal("ScoreBox missing")
	}
	var window *UIComponent
	for _, child := range score.Children {
		if child.Name == "Screen" {
			window = child
		}
	}
	if x, y := window.Anchor(); !near(x, 154) || !near(y, 70) {
		t.Fatalf("score screen centre %.1f,%.1f want 154,70", x, y)
	}
	light := screen.Find("Light1")
	if light == nil || light.CurrentValue() != 1 {
		t.Fatalf("Light1 decor value wrong: %+v", light)
	}
	if w, ok := light.Width(); !ok || w != 18 {
		t.Fatalf("Light1 width %v %v want 18", w, ok)
	}
	if x, y, w, h := screen.Find("BottomShine").TexCoords(); x != 0 || y != .5 || w != 1 || h != .5 {
		t.Fatalf("BottomShine texcoords %v %v %v %v", x, y, w, h)
	}
	if got := screen.Find("Connector3").Texture(); got != "Textures/HUD_Connector.tex" {
		t.Fatalf("connector texture %q", got)
	}
}

// MainScreen.uiscreen mirrors hands through a negative texCoordSize component; the
// native quad builder (0x003240b0) writes UV pos, pos+(size.x,0), pos+size,
// pos+(0,size.y), so the sign is the whole flip.
func TestMainScreenHandFlipsFollowTexCoordSizeSign(t *testing.T) {
	data, err := os.ReadFile("../../bin/data-cache/source/Common0/UserInterface/screens/MainScreen.uiscreen")
	if err != nil {
		t.Skip("MainScreen.uiscreen not cached")
	}
	screen, err := ParseUIScreen(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		group                 string
		leftFlip, rightFlip   bool
		leftRotation, rightRo float64
	}{
		{"AchievementsZombie", true, false, 90, -93},
		{"StatsZombie", false, true, 0, -3},
		{"MoreGamesZombie", false, false, -369, -200},
		{"OptionsZombie", true, false, 89, -90},
		{"PlayZombie", true, false, 11, -163},
		{"QuitZombie", true, false, 11, -163},
	} {
		left, right := screen.FindIn(tc.group, "LeftHand"), screen.FindIn(tc.group, "RightHand")
		if left == nil || right == nil {
			t.Fatalf("%s hands missing", tc.group)
		}
		if flipX, flipY := left.TexCoordFlip(); flipX != tc.leftFlip || flipY {
			t.Fatalf("%s left flip %v,%v", tc.group, flipX, flipY)
		}
		if flipX, flipY := right.TexCoordFlip(); flipX != tc.rightFlip || flipY {
			t.Fatalf("%s right flip %v,%v", tc.group, flipX, flipY)
		}
		if left.Rotation() != tc.leftRotation || right.Rotation() != tc.rightRo {
			t.Fatalf("%s rotations %v %v", tc.group, left.Rotation(), right.Rotation())
		}
	}
}
