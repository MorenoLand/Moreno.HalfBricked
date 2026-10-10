package game

import (
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// levelMusicTrack picks the level's track by the Music attribute of its catalog entry (Music_western for the
// president levels of world index 6, which have no entry in the per-world table) and falls back to the per-world track.
func levelMusicTrack(info formats.LevelInfo) (string, int64, bool) {
	if name := strings.ToLower(strings.TrimSpace(info.Music)); name != "" {
		for world := 0; ; world++ {
			path, loopPoint, ok := worldMusicTrack(world)
			if !ok {
				break
			}
			if strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))) == name {
				return path, loopPoint, true
			}
		}
	}
	return worldMusicTrack(info.WorldIndex)
}

// setLevelMusic starts the music of a level that is being opened.
func (a *app) setLevelMusic(info formats.LevelInfo) {
	if a == nil || a.silent || a.sound == nil {
		return
	}
	if path, loopPoint, ok := levelMusicTrack(info); ok {
		_ = a.sound.SetMusic(path, loopPoint)
	}
}
