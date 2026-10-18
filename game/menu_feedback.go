package game

// Front-end feedback sounds of the 1.2.5 build (Research/native/sound-trigger-audit-2026-10-14.md section 2.3, call
// sites resolved on 2026-10-14). Sound ids of the 1.2.5 sfx table (0x008c1e70): 0x43 menu_back, 0x44
// menu_transition_in, 0x45 menu_transition_out, 0x46 menu_level_scroll.
//
// Native sites that were resolved to a screen and are ported here (all in libmortargame.so 1.2.5):
//
//	menu_back (0x43)            FUN_000c915c (AchievementsScreen Back callback) -> page 6 left;
//	                            FUN_000e8da4 (StatsScreen Back callback)        -> page 4 left.
//	menu_transition_in (0x44)   FUN_000d7918 (LevelSelectScreen init)           -> level select shown;
//	                            FUN_000e2780 ("optionsScreen" enter)            -> options page shown;
//	                            FUN_00136e78 / FUN_001368d0 (PauseScreen show)  -> pause opened.
//	menu_transition_out (0x45)  FUN_000d6428 (LevelSelectScreen Back -> MainMenu) and FUN_000d6354 (Play, level
//	                            unlocked)                                       -> level select left;
//	                            FUN_00136e78 else branch (PauseScreen hide) and FUN_00136cc8 (AnimateOut_Quit)
//	                                                                            -> pause closed / quit;
//	                            FUN_000cbee0 (CreditsScreen exit)               -> credits.go.
//	menu_level_scroll (0x46)    FUN_000d7564 (swipie page change handler: refreshes the shown level, sound when the
//	                            screen state +4 == 1) -> the selected level card changes (levelScrollSound).
//
// The 1.2.1 build has the same families (audit: FUN_00074f94, FUN_00089434, FUN_0008c1d4, FUN_0008ec04 ...) but its
// call sites were not re-resolved, so nothing here acts for the SD cache.

// menuFeedbackState is the front-end situation one Update starts with.
type menuFeedbackState struct {
	levelSelect, options, stats, achievements bool
	play                                      *playState
	paused                                    bool
}

func (a *app) menuFeedbackSnapshot() menuFeedbackState {
	if a == nil {
		return menuFeedbackState{}
	}
	s := menuFeedbackState{
		levelSelect:  a.page == 2 && a.play == nil && !a.titleScreen,
		options:      a.page == 3 && !a.titleScreen && a.view == nil,
		stats:        a.page == 4 && a.statsScreen != nil && !a.titleScreen,
		achievements: a.page == 6 && !a.titleScreen,
		play:         a.play,
	}
	if a.play != nil {
		s.paused = a.play.paused
	}
	return s
}

// menuFeedback plays the sounds for the front-end changes of this Update (before is the state it started with).
func (a *app) menuFeedback(before menuFeedbackState) {
	if a == nil || !a.titleNative() {
		return
	}
	for _, sound := range menuFeedbackSounds(before, a.menuFeedbackSnapshot(), a.page) {
		a.playSound(sound.path, sound.volume)
	}
}

type menuFeedbackSound struct {
	path   string
	volume float64
}

// menuFeedbackSounds is the decision part of menuFeedback (page is the page the Update ended on).
func menuFeedbackSounds(before, now menuFeedbackState, page int) []menuFeedbackSound {
	const (
		in   = "audio/sound/sfx/menu_transition_in.ogg"
		out  = "audio/sound/sfx/menu_transition_out.ogg"
		back = "audio/sound/sfx/menu_back.ogg"
	)
	var sounds []menuFeedbackSound
	switch {
	case !before.levelSelect && now.levelSelect:
		sounds = append(sounds, menuFeedbackSound{in, .7})
	case before.levelSelect && !now.levelSelect && (now.play != nil || page == 0):
		sounds = append(sounds, menuFeedbackSound{out, .7})
	}
	if !before.options && now.options {
		sounds = append(sounds, menuFeedbackSound{in, .7})
	}
	// PORT ADDITION: the 1.2.5 binary plays nothing when Statistics or Achievements open (FUN_000c9244 and the
	// StatsScreen init have no play call; the board press only plays the shot, main_menu_hd.go); the slide-in gets the
	// same menu_transition_in as the other screens.
	if (!before.stats && now.stats) || (!before.achievements && now.achievements) {
		sounds = append(sounds, menuFeedbackSound{in, .7})
	}
	if (before.stats && !now.stats && !now.achievements) || (before.achievements && !now.achievements) {
		sounds = append(sounds, menuFeedbackSound{back, .8})
		// PORT ADDITION: the slide-out sound the other screens have; the binary plays menu_back alone here.
		sounds = append(sounds, menuFeedbackSound{out, .7})
	}
	if before.play != nil && before.paused {
		if now.play == nil || (now.play == before.play && !now.paused) {
			sounds = append(sounds, menuFeedbackSound{out, .7})
		}
	} else if before.play != nil && now.play == before.play && now.paused {
		sounds = append(sounds, menuFeedbackSound{in, .7})
	}
	return sounds
}

// levelScrollSound is the sound of a level card change on the level select page: 1.2.5 FUN_000d7564 plays id 0x46
// (menu_level_scroll); every other page keeps menu_move.
func (a *app) levelScrollSound() {
	if a.page == 2 && a.titleNative() {
		a.playSound("audio/sound/sfx/menu_level_scroll.ogg", .7)
		return
	}
	a.playSound("audio/sound/sfx/menu_move.ogg", .7)
}
