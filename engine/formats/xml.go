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
	DisplayName   string `xml:"displayName,attr"`
	LevelName     string `xml:"levelName,attr"`
	BaseFile      string `xml:"baseFileName,attr"`
	WorldIndex    string `xml:"worldIndex,attr"`
	Flags         string `xml:"levelFlags,attr"`
	Description   string `xml:"description,attr"`
	PostcardImage string `xml:"postcardImage,attr"`
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
	Waves   waveList `xml:"waves"`
}
type propList struct {
	Props         []propXML         `xml:"prop"`
	AnimatedProps []animatedPropXML `xml:"animated_prop"`
}
type propXML struct {
	Texture  string  `xml:"texture,attr"`
	Position string  `xml:"position,attr"`
	Height   float64 `xml:"height,attr"`
	Scale    string  `xml:"scale,attr"`
	UV1      string  `xml:"uv1,attr"`
	UV2      string  `xml:"uv2,attr"`
}
type animatedPropXML struct {
	Texture   string `xml:"texture,attr"`
	Position  string `xml:"position,attr"`
	Height    string `xml:"height,attr"`
	Scale     string `xml:"scale,attr"`
	XFrames   string `xml:"x_frames,attr"`
	YFrames   string `xml:"y_frames,attr"`
	FrameTime string `xml:"frame_time,attr"`
}
type waveList struct {
	Waves []waveXML `xml:"wave"`
}
type waveXML struct {
	NextWave       string       `xml:"next_wave,attr"`
	RunTime        string       `xml:"run_time,attr"`
	EndWaveTime    string       `xml:"end_wave_time,attr"`
	EndWaveZombies string       `xml:"end_wave_zombies,attr"`
	Spawners       []spawnerXML `xml:"spawner"`
}
type spawnerXML struct {
	DelayTime string         `xml:"delay_time,attr"`
	Count     string         `xml:"count,attr"`
	Index     string         `xml:"index,attr"`
	Types     []spawnTypeXML `xml:"type"`
}
type spawnTypeXML struct {
	Name      string `xml:"name,attr"`
	Chance    string `xml:"chance,attr"`
	Speed     string `xml:"speed,attr"`
	Strength  string `xml:"strength,attr"`
	Size      string `xml:"size,attr"`
	TurnSpeed string `xml:"turnSpeed,attr"`
	Texture   string `xml:"texture,attr"`
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
			candidate := LevelInfo{ID: item.LevelName, DisplayName: item.DisplayName, BaseFile: item.BaseFile, WorldIndex: world, Flags: flags, Description: item.Description, PostcardImage: item.PostcardImage, SourceXML: filepath.ToSlash(rel)}
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
	props, animatedProps, err := parseProps(d.Props)
	if err != nil {
		return Level{}, fmt.Errorf("%s props: %w", info.ID, err)
	}
	waves, err := parseWaves(d.Waves)
	if err != nil {
		return Level{}, fmt.Errorf("%s waves: %w", info.ID, err)
	}
	level := Level{Info: info, Width: d.Width, Height: d.Height, Tileset: d.Tileset, Layers: layers, Props: props, AnimatedProps: animatedProps, Waves: waves}
	return level, level.Validate()
}

func parseProps(document propList) ([]Prop, []AnimatedProp, error) {
	props := make([]Prop, 0, len(document.Props))
	for _, p := range document.Props {
		x, y := pair(p.Position)
		sx, sy := pair(p.Scale)
		u1x, u1y := pair(p.UV1)
		u2x, u2y := pair(p.UV2)
		props = append(props, Prop{Texture: p.Texture, X: x, Y: y, Height: p.Height, ScaleX: sx, ScaleY: sy, UV1X: u1x, UV1Y: u1y, UV2X: u2x, UV2Y: u2y})
	}
	animated := make([]AnimatedProp, 0, len(document.AnimatedProps))
	for index, p := range document.AnimatedProps {
		x, y := pair(p.Position)
		sx, sy := pair(p.Scale)
		height, err := parseXMLFloat(p.Height, "animated_prop height", index)
		if err != nil {
			return nil, nil, err
		}
		xFrames, err := parseXMLInt(p.XFrames, "animated_prop x_frames", index)
		if err != nil {
			return nil, nil, err
		}
		yFrames, err := parseXMLInt(p.YFrames, "animated_prop y_frames", index)
		if err != nil {
			return nil, nil, err
		}
		frameTime, err := parseXMLFloat(p.FrameTime, "animated_prop frame_time", index)
		if err != nil {
			return nil, nil, err
		}
		animated = append(animated, AnimatedProp{Texture: p.Texture, X: x, Y: y, Height: height, ScaleX: sx, ScaleY: sy, XFrames: xFrames, YFrames: yFrames, FrameTime: frameTime})
	}
	return props, animated, nil
}

func parseWaves(document waveList) ([]Wave, error) {
	result := make([]Wave, 0, len(document.Waves))
	for waveIndex, item := range document.Waves {
		nextWave, err := parseXMLInt(item.NextWave, "next_wave", waveIndex)
		if err != nil {
			return nil, err
		}
		runTime, err := parseXMLFloat(item.RunTime, "run_time", waveIndex)
		if err != nil {
			return nil, err
		}
		endWaveTime, err := parseXMLFloat(item.EndWaveTime, "end_wave_time", waveIndex)
		if err != nil {
			return nil, err
		}
		endWaveZombies, err := parseXMLInt(item.EndWaveZombies, "end_wave_zombies", waveIndex)
		if err != nil {
			return nil, err
		}
		wave := Wave{NextWave: nextWave, RunTime: runTime, EndWaveTime: endWaveTime, EndWaveZombies: endWaveZombies, Spawners: make([]Spawner, 0, len(item.Spawners))}
		for spawnerIndex, source := range item.Spawners {
			delay, err := parseXMLFloat(source.DelayTime, "delay_time", spawnerIndex)
			if err != nil {
				return nil, err
			}
			count, err := parseXMLInt(source.Count, "count", spawnerIndex)
			if err != nil {
				return nil, err
			}
			index, err := parseXMLInt(source.Index, "index", spawnerIndex)
			if err != nil {
				return nil, err
			}
			spawner := Spawner{DelayTime: delay, Count: count, Index: index, Types: make([]SpawnType, 0, len(source.Types))}
			for typeIndex, entry := range source.Types {
				chance, err := parseXMLFloatDefault(entry.Chance, "chance", typeIndex, 1)
				if err != nil {
					return nil, err
				}
				strength, err := parseXMLFloat(entry.Strength, "strength", typeIndex)
				if err != nil {
					return nil, err
				}
				turnSpeed, err := parseXMLFloat(entry.TurnSpeed, "turnSpeed", typeIndex)
				if err != nil {
					return nil, err
				}
				speed, err := parseXMLVec2(entry.Speed, "speed", typeIndex)
				if err != nil {
					return nil, err
				}
				size, err := parseXMLVec2(entry.Size, "size", typeIndex)
				if err != nil {
					return nil, err
				}
				spawner.Types = append(spawner.Types, SpawnType{Name: entry.Name, Chance: chance, Speed: speed, Strength: strength, Size: size, TurnSpeed: turnSpeed, Texture: entry.Texture})
			}
			wave.Spawners = append(wave.Spawners, spawner)
		}
		result = append(result, wave)
	}
	return result, nil
}

func parseXMLInt(value, field string, index int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s %d %q: %w", field, index, value, err)
	}
	return parsed, nil
}

func parseXMLFloat(value, field string, index int) (float64, error) {
	return parseXMLFloatDefault(value, field, index, 0)
}

func parseXMLFloatDefault(value, field string, index int, fallback float64) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return 0, fmt.Errorf("%s %d %q: %w", field, index, value, err)
	}
	return parsed, nil
}

func parseXMLVec2(value, field string, index int) (Vec2, error) {
	if strings.TrimSpace(value) == "" {
		return Vec2{}, nil
	}
	parts := strings.Split(value, ",")
	if len(parts) == 1 {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		if err != nil {
			return Vec2{}, fmt.Errorf("%s %d %q: %w", field, index, value, err)
		}
		return Vec2{X: parsed, Y: parsed}, nil
	}
	if len(parts) != 2 {
		return Vec2{}, fmt.Errorf("%s %d %q: want one or two values", field, index, value)
	}
	x, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil {
		return Vec2{}, fmt.Errorf("%s %d x %q: %w", field, index, value, err)
	}
	y, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err != nil {
		return Vec2{}, fmt.Errorf("%s %d y %q: %w", field, index, value, err)
	}
	return Vec2{X: x, Y: y}, nil
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
