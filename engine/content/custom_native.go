//go:build !js

package content

import "path/filepath"

// CustomDir is the folder of level files the game loads at start, next to its "data" folder: levels made in the level
// editor and copied there (the editor's "Add to the game"). Each *.json file is one level.
func CustomDir() string { return filepath.Join(filepath.Dir(DefaultCacheDir()), "custom") }
