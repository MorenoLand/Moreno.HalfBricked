package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
)

// Native 1.2.5 title screen state machine (Research/native/title-screen-125-2026-10-14.md; class vtable 0x008c6a70,
// update FUN_000e7310, text 0xe6d08). Every counter below runs per update call, not per second.
//
//   - state 3 (cocking), jump-table target 0xe758c: c starts at 8 (0xe751c); each update c -= 1, the grip is
//     A + (B-A) * (|c|-2)/6 when |c| >= 2 and A otherwise (A = SPLASHSCREENS_HAND_ANIM_POS_VAR, B = the grip's title
//     position); c == 2 plays sound id 0x47 (0xe769c); c < -8 (literal 0xe776c) plays id 0x48 (0xe75f4), sets c = 35
//     (0x23) and enters state 4. Ticks 1-5 move B -> A, 6-10 hold A, 11-16 return, tick 17 ends.
//   - state 4 (slide), 0xe76c4: c -= 1; while c > 0 all four pieces sit at M + (T-M) * c/35 with M = T + (-285, +70)
//     (static block +0x28 against +0x08); c <= 0 enters state 5, whose next update opens the main menu (0xe77e8).
//   - blink, 0xe6d08: the counter cycles -15..16 (+1 per call, 16 -> -15) and the text is drawn while it is >= 0.
//
// Sound ids in the 1.2.5 sfx table (0x008c1e70, 8-byte entries): 0x47 = menu_shotgun_cock_1, 0x48 =
// menu_shotgun_cock_2. The 1.2.1 build keeps the older timing: its title (FUN_00087ad8) was not re-derived.
const (
	titleCockUpdates   = 17                                       // updates of state 3, the last one plays 0x48 and starts the slide
	titleCockSoundTick = 6                                        // c == 2, id 0x47
	titleSlideUpdates  = 35                                       // state 4 counter start
	titleMenuTick      = titleCockUpdates + titleSlideUpdates + 1 // state 5 update that opens the main menu
	titleSlideX        = -285.0
	titleSlideY        = 70.0
	titleBlinkPeriod   = 32
	titleBlinkShown    = 16 // counter >= 0 -> (n mod 32) <= 16
	titleHandAnimVar   = "SPLASHSCREENS_HAND_ANIM_POS_VAR"
)

// titleNative reports whether the 1.2.5 title schedule applies (HD cache; a pack without a build marker is 1.2.5).
func (a *app) titleNative() bool {
	return a.pack == nil || a.pack.Manifest().WaveBuild != content.WaveBuildV7
}

// titleTick is the number of update calls since the click that started the cocking.
func (a *app) titleTick() int { return int(math.Round(a.titleSoundElapsed * 60)) }

// titleCockingOffset125 is the displacement of title piece `index` (0 body, 1 shotgun, 2 grip, 3 hand) from its
// title position after `tick` updates of the cocking. handX/handY is A - B (the grip displacement at the hold).
func titleCockingOffset125(index, tick int, handX, handY float64) (float64, float64) {
	switch {
	case tick <= 0:
		return 0, 0
	case tick <= titleCockUpdates-1:
		if index != 2 {
			return 0, 0
		}
		c := 8 - tick
		if c < 0 {
			c = -c
		}
		t := 0.0
		if c >= 2 {
			t = float64(c-2) / 6
		}
		return handX * (1 - t), handY * (1 - t)
	case tick == titleCockUpdates:
		return 0, 0 // grip back on B, the other three never left
	}
	// state 4: c = 35 - k for the k-th slide update; the update that reaches c == 0 changes state only.
	k := tick - titleCockUpdates
	if k > titleSlideUpdates-1 {
		k = titleSlideUpdates - 1
	}
	f := float64(k) / titleSlideUpdates
	return titleSlideX * f, titleSlideY * f
}

// titleBlinkVisible is the "Touch to Start" test for the n-th draw call.
func titleBlinkVisible(n int) bool {
	m := n % titleBlinkPeriod
	if m < 0 {
		m += titleBlinkPeriod
	}
	return m <= titleBlinkShown
}

// titleHandDelta is A - B for the grip: the hand animation position against the grip's title position.
func (a *app) titleHandDelta() (float64, float64) {
	if a.variables == nil {
		return 0, 0
	}
	hand, ok := a.variables.Vec2Value(titleHandAnimVar)
	if !ok {
		return 0, 0
	}
	grip := titleBarryPieces[2]
	return hand.X - grip.x, hand.Y - grip.y
}
