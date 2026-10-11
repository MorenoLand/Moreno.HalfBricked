package game

import "github.com/MorenoLand/Moreno.HalfBricked/engine/formats"

// survivalStoryGate is a PORT ADDITION (-survival-gate, on by default): a survival map of a world is playable only
// once the player has reached that world in story mode (any story level of the world is unlocked). Native lets
// every STARTUNLOCKED survival map be played from the first launch. Worlds without a story chain (the DLC packs)
// are not gated.
var survivalStoryGate = true

// survivalGateLocked reports whether the gate keeps this level shut.
func (a *app) survivalGateLocked(item formats.LevelInfo) bool {
	if !survivalStoryGate || !hasLevelFlag(item, "SURVIVAL") {
		return false
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
