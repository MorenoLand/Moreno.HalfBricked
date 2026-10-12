package game

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Browser build: starting a level used to fetch its JSON, tileset atlas, entry script and music one after the
// other on the game loop (the "click Start and the frame rate falls from 165 to 4" freeze). While the level
// select is open the selected level's files are downloaded in the background instead, in two stages: the level
// JSON first, and once it has arrived (it names the tileset) everything else it needs.
type levelPrefetchState struct {
	id    string
	stage int
}

func (a *app) prefetchSelectedLevel() {
	if runtime.GOOS != "js" || a.silent || a.pack == nil {
		return
	}
	levels := a.filteredLevels()
	if a.level < 0 || a.level >= len(levels) {
		return
	}
	info := levels[a.level]
	st := &a.levelPrefetch
	if st.id != info.ID {
		*st = levelPrefetchState{id: info.ID}
	}
	switch st.stage {
	case 0:
		a.pack.Prefetch(a.levelEarlyPaths(info))
		st.stage = 1
	case 1:
		if !a.pack.LevelLoaded(info.ID) && !a.pack.PrefetchReady(content.LevelPath(info.ID)) {
			return
		}
		level, err := a.pack.Load(info.ID)
		if err != nil {
			st.stage = 2
			return
		}
		a.pack.Prefetch(a.levelAssetPaths(level, info))
		st.stage = 2
	}
}

// levelEarlyPaths are the files known from the catalogue alone: the level JSON (first), its scripts and music.
func (a *app) levelEarlyPaths(info formats.LevelInfo) []string {
	paths := []string{content.LevelPath(info.ID)}
	if name, ok := a.pack.ScriptPath(entryScriptPath(info)); ok {
		paths = append(paths, name)
	}
	if name, ok := a.pack.ScriptPath(survivalIntroScript); ok {
		paths = append(paths, name)
	}
	if path, _, ok := levelMusicTrack(info); ok {
		paths = append(paths, path)
	}
	return paths
}

// levelAssetPaths lists what a level shows beyond its JSON: the tileset atlas, the common and world textures.
func (a *app) levelAssetPaths(level formats.Level, info formats.LevelInfo) []string {
	var paths []string
	if tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]; ok {
		if path, ok := a.pack.TexturePath(tileset.Texture); ok {
			paths = append(paths, path)
		}
	}
	world := ""
	if parts := strings.Split(filepath.ToSlash(info.SourceXML), "/"); len(parts) > 0 {
		world = strings.ToLower(parts[0])
	}
	var textures []string
	for _, path := range a.pack.TexturePaths() {
		lower := strings.ToLower(filepath.ToSlash(path))
		if strings.Contains(lower, "/common0/") || (world != "" && strings.Contains(lower, "/"+world+"/")) {
			textures = append(textures, path)
		}
	}
	sort.Strings(textures)
	return append(paths, textures...)
}
