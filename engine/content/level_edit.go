package content

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

// Support for the in-game level editor (game/leveledit.go).

// RegisterLevel puts a level in memory under its id (replacing one with the same id) and lists it with the pack's
// levels, so the game can start it like any other. Nothing is written to disk.
func (p *Pack) RegisterLevel(level formats.Level) {
	if p.levels == nil {
		p.levels = map[string]formats.Level{}
	}
	level.WaveBuild = p.manifest.WaveBuild
	p.levels[level.Info.ID] = level
	for index, info := range p.manifest.Levels {
		if strings.EqualFold(info.ID, level.Info.ID) {
			p.manifest.Levels[index] = level.Info
			return
		}
	}
	p.manifest.Levels = append(p.manifest.Levels, level.Info)
}

// SourceRoot is the cache folder the pack reads from, "" when the pack does not read from a folder.
func (p *Pack) SourceRoot() string {
	if source, ok := p.source.(fileSource); ok {
		return source.root
	}
	return ""
}

// SaveLevel writes the level over its file in the cache (levels/<id>.json). The first time, the original is kept next to
// it as <id>.json.orig. It returns the path written.
func (p *Pack) SaveLevel(level formats.Level) (string, error) {
	root := p.SourceRoot()
	if root == "" {
		return "", fmt.Errorf("this pack is not a folder, so there is no cache file to write")
	}
	if err := level.Validate(); err != nil {
		return "", err
	}
	path := filepath.Join(root, filepath.FromSlash(LevelPath(level.Info.ID)))
	if original, err := os.ReadFile(path); err == nil {
		if _, statErr := os.Stat(path + ".orig"); os.IsNotExist(statErr) {
			if err := os.WriteFile(path+".orig", original, 0o644); err != nil {
				return "", err
			}
		}
	}
	data, err := json.Marshal(level)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	p.RegisterLevel(level)
	return path, nil
}

// UnregisterLevel takes a level registered with RegisterLevel out of the pack again.
func (p *Pack) UnregisterLevel(id string) {
	delete(p.levels, id)
	kept := p.manifest.Levels[:0]
	for _, info := range p.manifest.Levels {
		if !strings.EqualFold(info.ID, id) {
			kept = append(kept, info)
		}
	}
	p.manifest.Levels = kept
}
