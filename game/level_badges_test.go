package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"math"
	"testing"
)

func TestCatalogBadgesUseSavedLaunchState(t *testing.T) {
	a := &app{unlocked: map[string]bool{"played": true, "unplayed": true}, newDismissed: map[string]bool{"played": true}}
	for _, id := range []string{"played", "unplayed", "locked"} {
		state := a.catalogLevelBadgeState(formats.LevelInfo{ID: id})
		if state.NewDismissed != (id == "played") || state.Locked != (id == "locked") {
			t.Fatalf("%s: %+v", id, state)
		}
	}
}

func TestLevelBadgePredicates(t *testing.T) {
	for _, item := range []struct {
		state   levelBadgeState
		count   int
		texture string
	}{
		{levelBadgeState{Locked: true}, 1, "Common0/Textures/blank_SD"},
		{levelBadgeState{}, 1, "Common0/Textures/NewIcon_SD"},
		{levelBadgeState{NewDismissed: true}, 0, ""},
		{levelBadgeState{Locked: true, Flags: 0x200}, 0, ""},
		{levelBadgeState{NewDismissed: true, Flags: 0x400}, 1, "Common0/Textures/TutorialIcon_SD"},
		{levelBadgeState{Flags: 0x400}, 2, "Common0/Textures/NewIcon_SD"},
	} {
		badges := levelBadges(item.state, 90, 100, 128, 128, 100, 50, 0)
		if len(badges) != item.count {
			t.Fatalf("%+v: got %d", item.state, len(badges))
		}
		if len(badges) > 0 && badges[0].Texture != item.texture {
			t.Fatalf("%+v: %s", item.state, badges[0].Texture)
		}
	}
}

func TestLevelBadgeGeometry(t *testing.T) {
	lock := levelBadges(levelBadgeState{Locked: true}, 90, 100, 128, 128, 0, 0, 0)[0]
	if lock.X != 90 || lock.Y != 100 || lock.Width != 128./1.5 || lock.Height != 128 || lock.Source.Dx() != 64 {
		t.Fatalf("lock: %+v", lock)
	}
	badges := levelBadges(levelBadgeState{Flags: 0x400}, 90, 100, 128, 128, 100, 50, 0)
	if math.Abs(badges[0].Height-28.8) > 1e-9 || badges[0].Rotation != float64(float32(.55)) || badges[1].Rotation != float64(float32(-.55)) {
		t.Fatalf("badges: %+v", badges)
	}
	peak := levelBadges(levelBadgeState{}, 90, 100, 128, 128, 0, 0, math.Pi/2)[0]
	if peak.Height <= badges[0].Height {
		t.Fatal("pulse did not grow")
	}
}

func TestNativeCatalogLevelFlags(t *testing.T) {
	for _, item := range []struct {
		flags []string
		mask  uint32
	}{
		{[]string{"STORY", "STARTUNLOCKED", "BEGINSTORY"}, 0x61},
		{[]string{"SURVIVAL", "STARTUNLOCKED"}, 0x22},
		{[]string{"STORY", "ENDWORLD", "RATEONFINISH"}, 0x0d},
		{[]string{"ENDSTORY"}, 0x11},
		{[]string{"SHOWCREDITS"}, 0x80},
		{[]string{"TUTORIAL", "UNKNOWN"}, 0},
	} {
		if got := nativeCatalogLevelFlags(item.flags); got != item.mask {
			t.Fatalf("%v: %#x, want %#x", item.flags, got, item.mask)
		}
		if nativeCatalogLevelFlags(item.flags)&0x600 != 0 {
			t.Fatal("XML parser fabricated runtime flags")
		}
	}
}
