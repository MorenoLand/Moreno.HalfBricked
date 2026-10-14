package game

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/hajimehoshi/ebiten/v2/audio"
)

// v7 (1.2.1, SD cache) hoverboard sound, FUN_00094258 (called from the player update FUN_00096818, hover timer
// +0x3e8 = playerVitals.speedBoost):
//
//   - timer >= 1.0: start SFX_HOVERBOARD (id 0x41, looping) when it is not playing;
//   - 0 < timer < 1.0: stop it, then play SFX_HOVERBOARD_STOP (id 0x42) once (latch +0x27e, cleared by the pickup);
//   - timer == 0: stop both.
//
// The 1.2.5 twin FUN_000f185c has the same shape, but its _INIT_84 does not hash the two names, so every lookup
// returns -1 and the build is silent: this file only acts for content.WaveBuildV7.

// stepHoverAudio is FUN_00094258's branch choice for the timer after this frame's decay.
func (v *playerVitals) stepHoverAudio() {
	switch {
	case v.speedBoost >= 1:
		v.hoverLoop = true
	case v.speedBoost == 0:
		v.hoverLoop = false
	default:
		v.hoverLoop = false
		if !v.hoverStopLatch {
			v.hoverStopLatch, v.hoverStopPending = true, true
		}
	}
}

// hoverLoopPlayback is the single hoverboard loop handle.
var hoverLoopPlayback struct {
	owner  *playState
	player *audio.Player
}

// syncHoverAudio starts / stops the looping hoverboard clip and plays the stop clip once; it also silences the loop
// when the level that owned it is gone.
func (a *app) syncHoverAudio() {
	var wanted *playState
	if a.play != nil && a.play.waveBuild() == content.WaveBuildV7 && a.play.vitals.hoverLoop {
		wanted = a.play
	}
	if hoverLoopPlayback.player != nil && (wanted == nil || hoverLoopPlayback.owner != wanted) {
		_ = hoverLoopPlayback.player.Close()
		hoverLoopPlayback.player, hoverLoopPlayback.owner = nil, nil
	}
	if a.play != nil && a.play.vitals.hoverStopPending {
		a.play.vitals.hoverStopPending = false
		if a.play.waveBuild() == content.WaveBuildV7 {
			a.playSound(a.scriptSoundPath("SFX_HOVERBOARD_STOP"), .8)
		}
	}
	if wanted == nil || hoverLoopPlayback.player != nil || a.silent || !a.options.sound || a.sound == nil {
		return
	}
	// Loop point 0: the whole clip repeats (the table's loop byte, like the flamethrower).
	player, err := a.sound.PlayTracked(a.scriptSoundPath("SFX_HOVERBOARD"), .8, 0)
	if err == nil {
		hoverLoopPlayback.player, hoverLoopPlayback.owner = player, wanted
	}
}
