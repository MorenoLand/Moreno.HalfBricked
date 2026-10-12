package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/formats"

// survivalStoryGate is a PORT ADDITION (-survival-gate, on by default): a survival map follows the story. The level
// data already says which story level opens it (unlockLevels on the ENDWORLD level of each world); native also
// flags the maps STARTUNLOCKED, which makes them playable from the first launch. With the gate on, a survival map
// that has such a story level is playable only after that level has been completed; a map no story level opens is
// playable once its world is reached (any story level of the world unlocked).
var survivalStoryGate = true

// survivalGateLocked reports whether the gate keeps this level shut.
func (a *app) survivalGateLocked(item formats.LevelInfo) bool {
	if !survivalStoryGate || !hasLevelFlag(item, "SURVIVAL") {
		return false
	}
	unlockers := 0
	for _, other := range a.levels {
		if !hasLevelFlag(other, "STORY") {
			continue
		}
		for _, id := range other.UnlockLevels {
			if id != item.ID {
				continue
			}
			unlockers++
			// Completed: recorded in the profile, or (older profiles) the level after it is open.
			if a.completed[other.ID] || (other.NextLevel != "" && a.unlocked[other.NextLevel]) {
				return false
			}
		}
	}
	if unlockers > 0 {
		return true
	}
	hasStory := false
	for _, other := range a.levels {
		if other.WorldIndex != item.WorldIndex || !hasLevelFlag(other, "STORY") {
			continue
		}
		hasStory = true
		if a.unlocked[other.ID] {
			return false
		}
	}
	return hasStory
}

// storyWaveHUD is the -story-wave-hud flag (see drawGameHUD).
var storyWaveHUD = true

// portAimAtCursor is the -aim-at-cursor flag (see projectileDirection); portAimMinDistance keeps the aim point
// outside the muzzle offset.
var portAimAtCursor = true

const portAimMinDistance = 48.0

// portRegenScale is the -regen-scale flag: a multiplier on the health regeneration rate. 1 is the native rate
// (0.25/s in 1.2.5, 0.75/s in 1.2.1); a smaller value is a port tweak for players who find the native recovery too
// quick.
var portRegenScale = 1.0
