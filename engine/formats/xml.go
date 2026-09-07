package formats

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type levelsDocument struct {
	Levels []levelXML `xml:"Level"`
}
type levelXML struct {
	DisplayName string `xml:"displayName,attr"`
	LevelName   string `xml:"levelName,attr"`
	BaseFile    string `xml:"baseFileName,attr"`
	WorldIndex  string `xml:"worldIndex,attr"`
	Flags       string `xml:"levelFlags,attr"`
	Description string `xml:"description,attr"`
}
type levelDocument struct {
	Level levelXMLData `xml:"level"`
}
type levelXMLData struct {
	Name    string   `xml:"name,attr"`
	Width   int      `xml:"width,attr"`
	Height  int      `xml:"height,attr"`
	Tileset string   `xml:"tileset,attr"`
	G       string   `xml:"g_tiles,attr"`
	D       string   `xml:"d_tiles,attr"`
	H       string   `xml:"h_tiles,attr"`
	HB      string   `xml:"hb_tiles,attr"`
	C       string   `xml:"c_tiles,attr"`
	Props   propList `xml:"props"`
}
type propList struct {
	Props []propXML `xml:"prop"`
}
type propXML struct {
	Texture  string  `xml:"texture,attr"`
	Position string  `xml:"position,attr"`
	Height   float64 `xml:"height,attr"`
	Scale    string  `xml:"scale,attr"`
	UV1      string  `xml:"uv1,attr"`
	UV2      string  `xml:"uv2,attr"`
}
type tileDocument struct {
	TileSets []tileXML `xml:"TileSet"`
}
type tileXML struct {
	Name      string  `xml:"name,attr"`
	Texture   string  `xml:"texture,attr"`
	TileSize  int     `xml:"tileSize,attr"`
	TileShift int     `xml:"tileShift,attr"`
	UVOffset  float64 `xml:"uvOffset,attr"`
}

func ParseLevelCatalog(root string) ([]LevelInfo, error) {
	var files []string
	err := filepath.WalkDir(filepath.Join(root, "assets"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), "_levels.xml") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	entries := make(map[string]LevelInfo)
	for _, path := range files {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		var doc levelsDocument
		err = xml.NewDecoder(f).Decode(&doc)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		rel, _ := filepath.Rel(filepath.Join(root, "assets"), path)
		for _, item := range doc.Levels {
			world, _ := strconv.Atoi(item.WorldIndex)
			flags := splitFlags(item.Flags)
			candidate := LevelInfo{ID: item.LevelName, DisplayName: item.DisplayName, BaseFile: item.BaseFile, WorldIndex: world, Flags: flags, Description: item.Description, SourceXML: filepath.ToSlash(rel)}
			current, exists := entries[item.LevelName]
			if !exists || (!hasLevelFile(root, current) && hasLevelFile(root, candidate)) {
				entries[item.LevelName] = candidate
			}
		}
	}
	result := make([]LevelInfo, 0, len(entries))
	for _, item := range entries {
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].WorldIndex != result[j].WorldIndex {
			return result[i].WorldIndex < result[j].WorldIndex
		}
		return result[i].ID < result[j].ID
	})
	return result, nil
}

func hasLevelFile(root string, info LevelInfo) bool {
	packageRoot := filepath.Dir(filepath.Dir(filepath.Join(root, "assets", filepath.FromSlash(info.SourceXML))))
	_, err := os.Stat(filepath.Join(packageRoot, "Levels", info.BaseFile+".xml"))
	return err == nil
}

func ParseLevel(root string, info LevelInfo) (Level, error) {
	packageRoot := filepath.Dir(filepath.Dir(filepath.Join(root, "assets", filepath.FromSlash(info.SourceXML))))
	path := filepath.Join(packageRoot, "Levels", info.BaseFile+".xml")
	if _, statErr := os.Stat(path); statErr != nil {
		fallback, findErr := findBaseFile(root, info.BaseFile, ".xml")
		if findErr != nil {
			return Level{}, findErr
		}
		path = fallback
	}
	f, err := os.Open(path)
	if err != nil {
		return Level{}, err
	}
	var d levelXMLData
	err = xml.NewDecoder(f).Decode(&d)
	f.Close()
	if err != nil {
		return Level{}, fmt.Errorf("%s: %w", path, err)
	}
	if d.Width <= 0 || d.Height <= 0 {
		return Level{}, fmt.Errorf("%s: invalid dimensions", path)
	}
	files := map[LayerKind]string{LayerG: d.G, LayerD: d.D, LayerH: d.H, LayerHB: d.HB, LayerC: d.C}
	layers := make(map[LayerKind][]uint32, len(files))
	for kind, name := range files {
		mapPath, err := findBaseFile(filepath.Dir(path), strings.TrimSuffix(name, filepath.Ext(name)), filepath.Ext(name))
		if err != nil {
			return Level{}, fmt.Errorf("%s layer %s: %w", info.ID, kind, err)
		}
		mf, err := os.Open(mapPath)
		if err != nil {
			return Level{}, err
		}
		layers[kind], err = ReadMap(mf, d.Width, d.Height)
		mf.Close()
		if err != nil {
			return Level{}, fmt.Errorf("%s: %w", mapPath, err)
		}
	}
	props := make([]Prop, 0, len(d.Props.Props))
	for _, p := range d.Props.Props {
		x, y := pair(p.Position)
		sx, sy := pair(p.Scale)
		u1x, u1y := pair(p.UV1)
		u2x, u2y := pair(p.UV2)
		props = append(props, Prop{Texture: p.Texture, X: x, Y: y, Height: p.Height, ScaleX: sx, ScaleY: sy, UV1X: u1x, UV1Y: u1y, UV2X: u2x, UV2Y: u2y})
	}
	level := Level{Info: info, Width: d.Width, Height: d.Height, Tileset: d.Tileset, Layers: layers, Props: props}
	return level, level.Validate()
}

func ParseTileSets(root string) (map[string]TileSet, error) {
	path, err := findNamedFile(filepath.Join(root, "assets"), "Common0_TileSets_SD.xml")
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var doc tileDocument
	if err := xml.NewDecoder(f).Decode(&doc); err != nil {
		return nil, err
	}
	result := make(map[string]TileSet, len(doc.TileSets))
	for _, item := range doc.TileSets {
		result[strings.ToLower(item.Name)] = TileSet{Name: item.Name, Texture: item.Texture, TileSize: item.TileSize, TileShift: item.TileShift, UVOffset: item.UVOffset}
	}
	return result, nil
}

func findBaseFile(root, base, ext string) (string, error) {
	var match string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.EqualFold(entry.Name(), base+ext) {
			match = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil && err != filepath.SkipDir {
		return "", err
	}
	if match == "" {
		return "", fmt.Errorf("missing %s%s below %s", base, ext, root)
	}
	return match, nil
}

func findNamedFile(root, name string) (string, error) {
	return findBaseFile(root, strings.TrimSuffix(name, filepath.Ext(name)), filepath.Ext(name))
}

func pair(value string) (float64, float64) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return 0, 0
	}
	a, _ := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	b, _ := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	return a, b
}
func splitFlags(value string) []string {
	value = strings.ReplaceAll(value, "|", ";")
	var result []string
	for _, item := range strings.Split(value, ";") {
		if item = strings.TrimSpace(item); item != "" {
			result = append(result, item)
		}
	}
	return result
}

var _ io.Reader
