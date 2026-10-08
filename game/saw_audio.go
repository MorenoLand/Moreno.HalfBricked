package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"

// Buzzsaw sound lifecycle (weapon class 6, Research/native/
// buzzsaw-dualpistol-2026-10-08.md):
//
//   - attach (0x000a9eb8) plays SFX_Start (chainsaw_start) once;
//   - every frame (0x000a9f0c) the managed sound starts SFX_Shoot (chainsaw_idle)
//     as soon as the Start clip is no longer playing, and restarts it whenever it
//     finishes (FUN_000ca828 is "is playing"), for as long as the weapon is held,
//     whether or not the trigger is down;
//   - detach (0x000aa4a4) plays SFX_End (chainsaw_stop) and stops the idle sound.
type sawAudioPhase int

const (
	sawAudioOff sawAudioPhase = iota
	sawAudioStarting
	sawAudioIdle
)

type sawAudioState struct {
	phase sawAudioPhase
}

// sawAudioSwitch handles the weapon changing: leaving the buzzsaw plays the End
// clip, taking one plays the Start clip.
func (p *playState) sawAudioSwitch(left, entered bool) {
	if left && p.sawAudio.phase != sawAudioOff {
		stop := "SFX_BUZZSAW_IDLE"
		if p.sawAudio.phase == sawAudioStarting {
			stop = "SFX_BUZZSAW_START"
		}
		p.spinEvents.Add([]weapons.WeaponAudioEvent{{Sound: stop, Stop: true}})
		p.sfxQueue = append(p.sfxQueue, "SFX_BUZZSAW_DEATH")
		p.sawAudio.phase = sawAudioOff
	}
	if entered {
		p.spinEvents.Add([]weapons.WeaponAudioEvent{{Sound: "SFX_BUZZSAW_START", LoopPointSamples: -1}})
		p.sawAudio.phase = sawAudioStarting
	}
}

// sawAudioTick is the per-frame managed sound check. spinEndFinished is set by the
// app before the frame: true when no tracked weapon clip is playing.
func (p *playState) sawAudioTick() {
	switch p.sawAudio.phase {
	case sawAudioStarting, sawAudioIdle:
		if !p.spinEndFinished {
			return
		}
		p.spinEvents.Add([]weapons.WeaponAudioEvent{{Sound: "SFX_BUZZSAW_IDLE", Loop: true, LoopPointSamples: weapons.NativeWeaponSoundLoopSamples("SFX_BUZZSAW_IDLE")}})
		p.sawAudio.phase = sawAudioIdle
	}
}
