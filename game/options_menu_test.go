package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"image"
	"strings"
	"testing"
)

func TestOptionsToggleAndBack(t *testing.T) {
	menu := newOptionsMenu(true, true)
	musicCalls := 0
	if menu.activate(optionsSound, nil) || menu.sound {
		t.Fatal("sound toggle failed")
	}
	menu.activate(optionsMusic, func(enabled bool) {
		musicCalls++
		if enabled {
			t.Fatal("music remained enabled")
		}
	})
	if menu.music || musicCalls != 1 {
		t.Fatal("music callback failed")
	}
	if !menu.activate(optionsBack, nil) {
		t.Fatal("back did not exit")
	}
	if menu.activate(optionsNone, nil) {
		t.Fatal("empty action exited")
	}
}
func TestOptionsMainMenuDispatchAndBack(t *testing.T) {
	game := &app{page: 0, menuSelection: 0, options: newOptionsMenu(true, true)}
	if err := game.activate(); err != nil {
		t.Fatal(err)
	}
	if game.page != 3 {
		t.Fatalf("OPTIONS page = %d, want 3", game.page)
	}
	if game.options.activate(optionsBack, nil) {
		game.page = 0
	}
	if game.page != 0 {
		t.Fatalf("BACK page = %d, want 0", game.page)
	}
	if !game.options.sound || !game.options.music {
		t.Fatal("navigation changed audio settings")
	}
}
func TestOptionsNativeSoundCells(t *testing.T) {
	for _, test := range []struct {
		music, enabled bool
		want           image.Rectangle
	}{{true, true, image.Rect(0, 0, 64, 64)}, {true, false, image.Rect(64, 0, 128, 64)}, {false, true, image.Rect(0, 64, 64, 128)}, {false, false, image.Rect(64, 64, 128, 128)}} {
		if got := optionsSoundRect(test.music, test.enabled); got != test.want {
			t.Fatalf("cell = %v, want %v", got, test.want)
		}
	}
}
func TestOptionsXMLHitRegions(t *testing.T) {
	variables, err := formats.ParseVariables(strings.NewReader(`<Variables><Vec2 name="OPTIONS_SOUND_ICON_POS_VAR" value="342,40"/><Vec2 name="OPTIONS_SOUND_ICON_SIZE_VAR" value="42,42"/><Vec2 name="OPTIONS_MUSIC_ICON_POS_VAR" value="396,40"/><Vec2 name="OPTIONS_MUSIC_ICON_SIZE_VAR" value="42,42"/><Vec2 name="OPTIONS_BACK_ICON_POS_VAR" value="417,258"/><Vec2 name="OPTIONS_BACK_ICON_SIZE_VAR" value="58,40"/></Variables>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		x, y float64
		want optionsAction
	}{{342, 40, optionsSound}, {396, 40, optionsMusic}, {417, 258, optionsBack}, {0, 0, optionsNone}} {
		if got := optionsHit(variables, test.x, test.y); got != test.want {
			t.Fatalf("hit = %v, want %v", got, test.want)
		}
	}
}
func TestOptionsNativeControlHitRegions(t *testing.T) {
	variables := formats.FrontendVariables{}
	variables["OPTIONS_TEXT_BUTTONS_SIZE_VAR"] = formats.FrontendVariable{Kind: "Vec2", Vec2: formats.Vec2{X: 96, Y: 24}}
	for index, label := range optionsControlLabels {
		position := formats.Vec2{X: 74, Y: float64(40 + index*28)}
		variables["OPTIONS_"+label.name+"_TEXT_POS_VAR"] = formats.FrontendVariable{Kind: "Vec2", Vec2: position}
		if got := optionsHit(variables, position.X, position.Y); got != label.action {
			t.Fatalf("%s hit %d", label.name, got)
		}
	}
	variables["OPTIONS_SIZE_CENTER_POS_VAR"] = formats.FrontendVariable{Kind: "Vec2", Vec2: formats.Vec2{X: 372, Y: 150}}
	variables["OPTIONS_SIZE_CENTER_SIZE_VAR"] = formats.FrontendVariable{Kind: "Vec2", Vec2: formats.Vec2{X: 160, Y: 160}}
	if optionsHit(variables, 372, 150) != optionsPad {
		t.Fatal("pad drag hit missing")
	}
}
