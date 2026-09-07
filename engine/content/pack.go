package content

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

type PackManifest struct {
	SchemaVersion int                        `json:"schemaVersion"`
	SourceVersion string                     `json:"sourceVersion"`
	Levels        []formats.LevelInfo        `json:"levels"`
	TileSets      map[string]formats.TileSet `json:"tileSets"`
	Textures      map[string]string          `json:"textures"`
	Files         map[string]string          `json:"files"`
}

type AssetSource interface {
	Open(path string) (io.ReadCloser, error)
	Manifest() (PackManifest, error)
}
type LevelRepository interface {
	List() []formats.LevelInfo
	Load(id string) (formats.Level, error)
}

type Pack struct {
	source   AssetSource
	manifest PackManifest
	levels   map[string]formats.Level
}

func NewPack(source AssetSource) (*Pack, error) {
	manifest, err := source.Manifest()
	if err != nil {
		return nil, err
	}
	if manifest.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported cache schema %d", manifest.SchemaVersion)
	}
	return &Pack{source: source, manifest: manifest, levels: make(map[string]formats.Level)}, nil
}
func (p *Pack) Manifest() PackManifest { return p.manifest }
func (p *Pack) List() []formats.LevelInfo {
	return append([]formats.LevelInfo(nil), p.manifest.Levels...)
}
func (p *Pack) Load(id string) (formats.Level, error) {
	if level, ok := p.levels[id]; ok {
		return level, nil
	}
	r, err := p.source.Open(filepath.ToSlash(filepath.Join("levels", id+".json")))
	if err != nil {
		return formats.Level{}, err
	}
	defer r.Close()
	var level formats.Level
	if err := json.NewDecoder(r).Decode(&level); err != nil {
		return formats.Level{}, err
	}
	if err := level.Validate(); err != nil {
		return formats.Level{}, err
	}
	p.levels[id] = level
	return level, nil
}
func (p *Pack) TexturePath(name string) (string, bool) {
	path, ok := p.manifest.Textures[strings.ToLower(strings.TrimSuffix(filepath.Base(name), filepath.Ext(name)))]
	return path, ok
}
func (p *Pack) Open(path string) (io.ReadCloser, error) { return p.source.Open(path) }
