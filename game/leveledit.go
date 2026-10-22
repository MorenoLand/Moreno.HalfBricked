package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// PORT ADDITION (not in the original game): a level editor, opened and closed with F4 (-level-editor=false disables it).
//
// A level is a tile map of five layers (formats.Level): g = ground, d = decoration, h = tall objects drawn per row,
// hb = objects behind the player, c = collision and markers (raw value 0 and 1 are walls, 2 is the player spawn, 3 and up
// are the zombie spawn zones the wave spawners name by index+2, empty is all ones), plus props (sprites placed by
// position) and the wave table. The editor edits a copy: tiles, collision and spawn markers, props, the map size and the
// tileset; the wave table travels with the level unchanged. A level is saved anywhere as JSON (Ctrl+S), or over its own
// file in the cache (Ctrl+Shift+S, the original is kept as .orig), loaded from a file (Ctrl+O), and played at once (T).
var portLevelEditor = true

const (
	toolPaint = iota
	toolFill
	toolPick
	toolRect
	toolLine
	toolSelect
	toolPickup
)

const (
	editorPanelX    = 700.0 // the layer list and tile palette sit right of this (editor units)
	editorTopBar    = 40.0  // the toolbar
	editorTile      = 32.0  // world pixels per tile, as the game
	editorEmpty     = ^uint32(0)
	editorUndoLimit = 40
	editorTestID    = "EditorTest"
)

type editorSnapshot struct {
	width, height int
	layers        map[formats.LayerKind][]uint32
	props         []formats.Prop
	animated      []formats.AnimatedProp
	waves         []formats.Wave
	pickups       []formats.LevelPickup
	intro         []formats.IntroLine
	info          formats.LevelInfo
}

// editorProto is a prop that can be placed: a copy of one already used by a level of the same tileset.
type editorProto struct {
	prop     formats.Prop
	animated *formats.AnimatedProp
	label    string
}

type levelEditor struct {
	open, testing bool
	inList        bool
	listRow       int
	listTop       int

	level     formats.Level
	tileset   formats.TileSet
	view      *viewer.Viewer
	sourceID  string // the id of the cache level this was loaded from ("" for a new or loaded-from-file level)
	savedPath string
	dirty     bool

	layer        formats.LayerKind
	tile         uint32
	flipX        bool
	flipY        bool
	collision    uint32
	brush        int
	help         bool   // the help overlay is open
	wavePage     bool   // the wave editor is showing instead of the map
	field        string // the wave editor value being typed ("" for none)
	input        string
	waveSel      int
	spawnSel     int
	typeSel      int
	waveTop      int
	spawnTop     int
	typeTop      int
	vocab        *editorVocab
	tilesetMenu  bool
	newDialog    bool
	newTemplate  formats.Level
	newW, newH   int
	newTileset   string
	rectOn       bool // a rectangle is being dragged
	rectX, rectY int
	rectErase    bool
	tool         int // toolPaint, toolFill or toolPick: what a click on the map does
	propMode     bool
	selected     int  // index into props then animated props, -1 for none
	dragging     bool // a prop is being dragged
	panning      bool // the middle button is dragging the map
	lastX        float64
	lastY        float64
	protos       []editorProto
	protoIndex   int

	undo, redo  []editorStep
	stroke      bool
	strokeSnap  editorSnapshot
	strokeSaved bool
	strokeLabel string
	status      string
	statusUntil time.Time

	// line tool
	lineOn       bool
	lineX, lineY int
	lineErase    bool
	// selection and clipboard
	selOn        bool
	selX0, selY0 int
	selX1, selY1 int
	selDrag      bool
	clip         *editorClip
	pasting      bool
	// layers
	locked     map[formats.LayerKind]bool
	recent     []uint32
	highlight  bool
	minimap    bool
	miniImage  *ebiten.Image
	miniFrame  int
	tileColors []color.RGBA
	// popups and dialogs
	popup *edPopup
	// preferences
	keys          map[string]ebiten.Key
	prefsLoaded   bool
	minimapDrag   bool
	autoOn        bool
	changedCells  []changedCell
	autoCache     map[string]*autoStats
	stampName     string
	tileColorsFor string
	pickupKind    string
	pickupSel     int
	pickupWave    int
	pickupDrag    bool
	libEntries    []propLibEntry
	libSearch     string
	libTop        int
	libSel        string
	libW, libLift float64
	shortcutsOpen bool
	rebind        string
	problemFrame  int
	problemCache  []editorProblem
	testGod       bool
	testHere      bool
	testAtCursor  bool
	testWave      int
	testX, testY  float64
	detailField   string
	introSel      int
	propLibOpen   bool
	stampNaming   bool
	pickWait      bool
	pasteWait     bool
	pasteUntil    time.Time
	armFile       [4]float64
	armPaste      bool
	armedPaste    bool
	lastAutosave  time.Time
	detailsOpen   bool
	testOpen      bool
}

// editorStep is one undo step: the state before a change, and what the change was.
type editorStep struct {
	label string
	snap  editorSnapshot
}

// editorClip is a copied region: every layer's cells, row by row.
type editorClip struct {
	w, h   int
	layers map[formats.LayerKind][]uint32
}

// collisionPalette is the C layer's choices.
var collisionPalette = []struct {
	label string
	value uint32
}{
	{"OPEN", editorEmpty}, {"WALL", 1}, {"PLAYER SPAWN", 2},
	{"ZOMBIE SPAWN 1", 3}, {"ZOMBIE SPAWN 2", 4}, {"ZOMBIE SPAWN 3", 5}, {"ZOMBIE SPAWN 4", 6},
	{"ZOMBIE SPAWN 5", 7}, {"ZOMBIE SPAWN 6", 8}, {"ZOMBIE SPAWN 7", 9}, {"ZOMBIE SPAWN 8", 10},
}

func collisionSwatch(value uint32) color.RGBA {
	switch value {
	case editorEmpty:
		return color.RGBA{60, 60, 60, 255}
	case 0, 1:
		return color.RGBA{200, 70, 60, 255}
	case 2:
		return color.RGBA{80, 220, 90, 255}
	}
	palette := []color.RGBA{{230, 190, 60, 255}, {70, 170, 230, 255}, {200, 90, 220, 255}, {240, 130, 50, 255}, {110, 230, 200, 255}, {230, 110, 150, 255}, {160, 200, 70, 255}, {150, 130, 240, 255}}
	return palette[int(value-3)%len(palette)]
}

// toggleEditor opens or closes the editor (F4). It is not offered inside a level or an online game.
func (a *app) toggleEditor() {
	if !portLevelEditor || a.capture != nil || a.play != nil || a.net != nil {
		return
	}
	if a.editor == nil {
		a.editor = &levelEditor{inList: true, layer: formats.LayerG, brush: 1, collision: 1, selected: -1, pickupSel: -1}
	}
	a.editor.open = !a.editor.open
	if !a.editor.open {
		a.editorSavePrefs()
	}
	a.editor.testing = false
	a.editorMusic(a.editor.open) // no menu music behind the editor
}

func (e *levelEditor) say(text string) {
	e.status, e.statusUntil = text, time.Now().Add(4*time.Second)
}

// ---- loading and creating ------------------------------------------------------------------------------------------

func copyLevel(level formats.Level) formats.Level {
	copied := level
	copied.Layers = map[formats.LayerKind][]uint32{}
	for kind, values := range level.Layers {
		copied.Layers[kind] = append([]uint32(nil), values...)
	}
	copied.Props = append([]formats.Prop(nil), level.Props...)
	copied.AnimatedProps = append([]formats.AnimatedProp(nil), level.AnimatedProps...)
	copied.Waves = copyWaves(level.Waves)
	copied.Intro = append([]formats.IntroLine(nil), level.Intro...)
	copied.Pickups = append([]formats.LevelPickup(nil), level.Pickups...)
	return copied
}

func (a *app) editorTileset(level formats.Level) (formats.TileSet, *ebiten.Image, error) {
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		return formats.TileSet{}, nil, fmt.Errorf("tileset %q not found", level.Tileset)
	}
	atlas, err := a.Texture(tileset.Texture)
	return tileset, atlas, err
}

// setLevel makes level the one being edited.
func (a *app) editorSetLevel(level formats.Level, sourceID string) error {
	e := a.editor
	tileset, atlas, err := a.editorTileset(level)
	if err != nil {
		return err
	}
	e.level, e.tileset, e.sourceID = level, tileset, sourceID
	e.view = viewer.New(e.level, tileset, atlas, a)
	e.view.Debug = false
	e.view.Layers[formats.LayerC] = false
	e.undo, e.redo, e.dirty, e.savedPath, e.selected, e.propMode, e.pickupSel = nil, nil, false, "", -1, false, -1
	e.protos, e.protoIndex = nil, 0
	e.tile = 0
	e.fit()
	e.inList = false
	return nil
}

func (e *levelEditor) fit() {
	width, height := float64(e.level.Width)*editorTile, float64(e.level.Height)*editorTile
	// The viewer works in the game's logical units, half of the editor's.
	zoom := math.Min((editorPanelX-editorToolStrip)/2/width, (editorMapBottom-editorTopBar)/2/height)
	e.view.Zoom = math.Max(.1, math.Min(2, zoom))
	e.view.CameraX, e.view.CameraY = 0, 0
	e.view.ViewportX, e.view.ViewportY = editorToolStrip/2, editorTopBar/2
}

func (a *app) editorLoad(id string) {
	e := a.editor
	level, err := a.pack.Load(id)
	if err != nil {
		e.say("Could not load " + id + ": " + err.Error())
		return
	}
	if err := a.editorSetLevel(copyLevel(level), id); err != nil {
		e.say(err.Error())
		return
	}
	e.say("Editing " + id)
}

// editorNew makes an empty level that takes its tileset, world and wave table from the level the list is on.
func (a *app) editorNew(template formats.Level, width, height int, tileset string) {
	e := a.editor
	if tileset == "" {
		tileset = template.Tileset
	}
	level := formats.Level{Width: width, Height: height, Tileset: tileset, Layers: map[formats.LayerKind][]uint32{}, Waves: copyWaves(template.Waves), WaveBuild: template.WaveBuild}
	for _, kind := range formats.LayerKinds {
		values := make([]uint32, width*height)
		for index := range values {
			if kind != formats.LayerG {
				values[index] = editorEmpty
			}
		}
		level.Layers[kind] = values
	}
	collision := level.Layers[formats.LayerC]
	for x := 0; x < width; x++ {
		collision[x] = 1
		collision[(height-1)*width+x] = 1
	}
	for y := 0; y < height; y++ {
		collision[y*width] = 1
		collision[y*width+width-1] = 1
	}
	collision[(height/2)*width+width/2] = 2
	// Mark each spawn zone the waves use along the inside of the walls, so the level can be played at once.
	var zones []int
	seen := map[int]bool{}
	for _, wave := range level.Waves {
		for _, spawner := range wave.Spawners {
			if spawner.Index >= 1 && spawner.Index <= 13 && !seen[spawner.Index] {
				seen[spawner.Index] = true
				zones = append(zones, spawner.Index)
			}
		}
	}
	sort.Ints(zones)
	var ring [][2]int
	for x := 1; x < width-1; x++ {
		ring = append(ring, [2]int{x, 1})
	}
	for y := 2; y < height-1; y++ {
		ring = append(ring, [2]int{width - 2, y})
	}
	for x := width - 3; x >= 1; x-- {
		ring = append(ring, [2]int{x, height - 2})
	}
	for y := height - 3; y >= 2; y-- {
		ring = append(ring, [2]int{1, y})
	}
	for index, zone := range zones {
		start := index * len(ring) / max(1, len(zones))
		for step := 0; step < 4 && len(ring) > 0; step++ {
			cell := ring[(start+step*2)%len(ring)]
			collision[cell[1]*width+cell[0]] = uint32(zone + 2)
		}
	}
	stamp := time.Now().Format("0102_1504")
	level.Info = formats.LevelInfo{ID: "Custom_" + stamp, DisplayName: "Custom " + stamp, BaseFile: "custom_" + strings.ToLower(stamp), WorldIndex: template.Info.WorldIndex, Flags: append([]string(nil), template.Info.Flags...), Description: "MADE IN THE LEVEL EDITOR", Music: template.Info.Music, PostcardImage: template.Info.PostcardImage}
	if err := a.editorSetLevel(level, ""); err != nil {
		e.say(err.Error())
		return
	}
	e.dirty = true
	e.say("New level " + level.Info.ID)
}

// ---- saving and testing --------------------------------------------------------------------------------------------

func (a *app) editorSaveAs() {
	e := a.editor
	if err := e.level.Validate(); err != nil {
		e.say(err.Error())
		return
	}
	data, err := json.Marshal(e.level)
	if err != nil {
		e.say(err.Error())
		return
	}
	path, err := platform.SaveFileAs("Save the level", e.level.Info.ID+".json", "Level (*.json)", "*.json", data)
	if err != nil {
		e.say("Save failed: " + err.Error())
		return
	}
	if path == "" {
		return
	}
	e.savedPath, e.dirty = path, false
	e.say("Saved " + path)
}

func (a *app) editorSaveToCache() {
	e := a.editor
	if e.sourceID == "" {
		e.say("This level is not in the cache: use Save as (Ctrl+S)")
		return
	}
	a.askConfirm("SAVE OVER CACHE LEVEL?", func() error {
		path, err := a.pack.SaveLevel(e.level)
		if err != nil {
			e.say("Cache save failed: " + err.Error())
			return nil
		}
		e.dirty = false
		e.say("Saved " + path + " (the original is kept as .orig)")
		return nil
	})
}

// editorTest starts the level being edited as a game; quitting to the menu comes back here.
func (a *app) editorTest() {
	e := a.editor
	if err := e.level.Validate(); err != nil {
		e.say(err.Error())
		return
	}
	if len(e.level.Waves) == 0 {
		e.waveChanged()
		e.level.Waves = []formats.Wave{defaultWave()} // a level without waves would never spawn anything
		e.say("Added a first wave so the level has zombies")
	}
	e.autosave()
	level := copyLevel(e.level)
	level.Info.ID = editorTestID
	level.Info.BaseFile = "editor_test"
	level.Info.DisplayName = "EDITOR: " + e.level.Info.DisplayName
	level.Info.NextLevel, level.Info.UnlockLevels = "", nil
	a.pack.RegisterLevel(level)
	replaced := false
	for index, info := range a.levels {
		if info.ID == editorTestID {
			a.levels[index], replaced = level.Info, true
		}
	}
	if !replaced {
		a.levels = append(a.levels, level.Info)
	}
	a.mode = 0
	for _, flag := range level.Info.Flags {
		if flag == "SURVIVAL" {
			a.mode = 1
		}
	}
	a.world = -1
	for index, world := range a.worlds() {
		if world == level.Info.WorldIndex {
			a.world = index
		}
	}
	if a.world < 0 {
		e.say("The level's world is not available")
		return
	}
	a.level = -1
	for index, info := range a.filteredLevels() {
		if info.ID == editorTestID {
			a.level = index
		}
	}
	if a.level < 0 {
		e.say("Could not find the test level")
		return
	}
	a.titleScreen, a.page = false, 2
	e.open, e.testing = false, true
	a.editorMusic(false)
	err := a.openPlay()
	if err == nil {
		a.applyTestOptions()
	}
	if err != nil {
		a.editorMusic(true)
		e.open, e.testing = true, false
		a.page = 0
		e.say("Could not start: " + err.Error())
	}
}

// editorReturnFromTest reopens the editor after a test play ended in the menu (quitToMenu).
func (a *app) editorReturnFromTest() {
	if a.editor != nil && a.editor.testing {
		a.editor.testing, a.editor.open = false, true
		a.editorMusic(true)
	}
}

// ---- undo ----------------------------------------------------------------------------------------------------------

func (e *levelEditor) snapshot() editorSnapshot {
	copied := copyLevel(e.level)
	info := e.level.Info
	info.Flags = append([]string(nil), info.Flags...)
	return editorSnapshot{width: e.level.Width, height: e.level.Height, layers: copied.Layers, props: copied.Props, animated: copied.AnimatedProps, waves: copied.Waves, pickups: copied.Pickups, intro: copied.Intro, info: info}
}

func (e *levelEditor) restore(s editorSnapshot) {
	e.level.Width, e.level.Height, e.level.Layers, e.level.Props, e.level.AnimatedProps, e.level.Waves = s.width, s.height, s.layers, s.props, s.animated, s.waves
	e.level.Pickups, e.level.Intro, e.level.Info = s.pickups, s.intro, s.info
	e.view.Level = e.level
	e.selected = -1
}

// push records the state before a change (once per stroke).
func (e *levelEditor) push() { e.pushAs("Edit") }

// pushAs is push with a name for the history list.
func (e *levelEditor) pushAs(label string) {
	e.undo = append(e.undo, editorStep{label: label, snap: e.snapshot()})
	if len(e.undo) > editorUndoLimit {
		e.undo = e.undo[1:]
	}
	e.redo = nil
	e.dirty = true
}

func (e *levelEditor) doUndo() {
	if len(e.undo) == 0 {
		return
	}
	step := e.undo[len(e.undo)-1]
	e.redo = append(e.redo, editorStep{label: step.label, snap: e.snapshot()})
	e.restore(step.snap)
	e.undo = e.undo[:len(e.undo)-1]
	e.dirty = true
}

func (e *levelEditor) doRedo() {
	if len(e.redo) == 0 {
		return
	}
	step := e.redo[len(e.redo)-1]
	e.undo = append(e.undo, editorStep{label: step.label, snap: e.snapshot()})
	e.restore(step.snap)
	e.redo = e.redo[:len(e.redo)-1]
	e.dirty = true
}

// ---- cells ---------------------------------------------------------------------------------------------------------

func (e *levelEditor) eraseValue() uint32 {
	if e.layer == formats.LayerG {
		return 0
	}
	return editorEmpty
}

func (e *levelEditor) paintValue() uint32 {
	if e.layer == formats.LayerC {
		return e.collision
	}
	value := e.tile & 0xffff
	if e.flipX {
		value |= 0x10000
	}
	if e.flipY {
		value |= 0x20000
	}
	return value
}

func (e *levelEditor) setCell(x, y int, value uint32) { e.setCellIn(e.layer, x, y, value) }

// setCellIn sets one cell of a layer (a locked layer ignores it). The first change of a stroke records the undo step.
func (e *levelEditor) setCellIn(kind formats.LayerKind, x, y int, value uint32) {
	if x < 0 || y < 0 || x >= e.level.Width || y >= e.level.Height || e.locked[kind] {
		return
	}
	cells := e.level.Layers[kind]
	index := y*e.level.Width + x
	if cells[index] == value {
		return
	}
	if !e.strokeSaved {
		label := e.strokeLabel
		if label == "" {
			label = "Paint"
		}
		e.undo = append(e.undo, editorStep{label: label, snap: e.strokeSnap})
		if len(e.undo) > editorUndoLimit {
			e.undo = e.undo[1:]
		}
		e.redo, e.strokeSaved, e.dirty = nil, true, true
	}
	if kind == formats.LayerC && value == 2 {
		for i, v := range cells {
			if v == 2 { // one player spawn
				cells[i] = editorEmpty
			}
		}
	}
	cells[index] = value
	if e.autoOn && kind != formats.LayerC {
		e.changedCells = append(e.changedCells, changedCell{kind, x, y})
	}
}

func (e *levelEditor) cellAt(x, y int) (uint32, bool) {
	if x < 0 || y < 0 || x >= e.level.Width || y >= e.level.Height {
		return 0, false
	}
	return e.level.Layers[e.layer][y*e.level.Width+x], true
}

func (e *levelEditor) fill(x, y int, value uint32) {
	target, ok := e.cellAt(x, y)
	if !ok || target == value {
		return
	}
	e.pushAs("Fill")
	cells := e.level.Layers[e.layer]
	queue := [][2]int{{x, y}}
	for len(queue) > 0 {
		cell := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		cx, cy := cell[0], cell[1]
		if cx < 0 || cy < 0 || cx >= e.level.Width || cy >= e.level.Height || cells[cy*e.level.Width+cx] != target {
			continue
		}
		cells[cy*e.level.Width+cx] = value
		queue = append(queue, [2]int{cx + 1, cy}, [2]int{cx - 1, cy}, [2]int{cx, cy + 1}, [2]int{cx, cy - 1})
	}
}

func (e *levelEditor) resize(width, height int) {
	if width < 8 || height < 8 || width > 400 || height > 400 {
		return
	}
	e.pushAs("Resize map")
	resized := map[formats.LayerKind][]uint32{}
	for _, kind := range formats.LayerKinds {
		values := make([]uint32, width*height)
		if kind != formats.LayerG {
			for index := range values {
				values[index] = editorEmpty
			}
		}
		for y := 0; y < min(height, e.level.Height); y++ {
			for x := 0; x < min(width, e.level.Width); x++ {
				values[y*width+x] = e.level.Layers[kind][y*e.level.Width+x]
			}
		}
		resized[kind] = values
	}
	e.level.Width, e.level.Height, e.level.Layers = width, height, resized
	e.view.Level = e.level
	e.say(fmt.Sprintf("Map %d x %d", width, height))
}

// ---- props ---------------------------------------------------------------------------------------------------------

func (e *levelEditor) propCount() int { return len(e.level.Props) + len(e.level.AnimatedProps) }

func (e *levelEditor) propPosition(index int) (float64, float64) {
	if index < len(e.level.Props) {
		return e.level.Props[index].X, e.level.Props[index].Y
	}
	prop := e.level.AnimatedProps[index-len(e.level.Props)]
	return prop.X, prop.Y
}

func (e *levelEditor) movePropTo(index int, x, y float64) {
	if index < len(e.level.Props) {
		e.level.Props[index].X, e.level.Props[index].Y = x, y
		return
	}
	prop := &e.level.AnimatedProps[index-len(e.level.Props)]
	prop.X, prop.Y = x, y
}

func (e *levelEditor) propNear(worldX, worldY float64) int {
	best, bestDistance := -1, 28.0
	for index := 0; index < e.propCount(); index++ {
		x, y := e.propPosition(index)
		if distance := math.Hypot(x-worldX, y-worldY); distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best
}

func (e *levelEditor) deleteProp(index int) {
	if index < 0 || index >= e.propCount() {
		return
	}
	e.pushAs("Remove prop")
	if index < len(e.level.Props) {
		e.level.Props = append(e.level.Props[:index:index], e.level.Props[index+1:]...)
	} else {
		index -= len(e.level.Props)
		e.level.AnimatedProps = append(e.level.AnimatedProps[:index:index], e.level.AnimatedProps[index+1:]...)
	}
	e.view.Level = e.level
	e.selected = -1
}

func (e *levelEditor) duplicateProp(index int) {
	if index < 0 || index >= e.propCount() {
		return
	}
	e.pushAs("Copy prop")
	if index < len(e.level.Props) {
		prop := e.level.Props[index]
		prop.X, prop.Y = prop.X+16, prop.Y+16
		e.level.Props = append(e.level.Props, prop)
		e.selected = len(e.level.Props) - 1
	} else {
		prop := e.level.AnimatedProps[index-len(e.level.Props)]
		prop.X, prop.Y = prop.X+16, prop.Y+16
		e.level.AnimatedProps = append(e.level.AnimatedProps, prop)
		e.selected = e.propCount() - 1
	}
	e.view.Level = e.level
}

// collectProtos gathers the props the levels of this tileset use, so they can be placed again.
func (a *app) editorCollectProtos() {
	e := a.editor
	seen := map[string]bool{}
	var protos []editorProto
	add := func(level formats.Level) {
		for _, prop := range level.Props {
			key := fmt.Sprintf("%s|%v|%v|%v|%v|%v|%v|%v", prop.Texture, prop.Height, prop.ScaleX, prop.ScaleY, prop.UV1X, prop.UV1Y, prop.UV2X, prop.UV2Y)
			if !seen[key] {
				seen[key] = true
				protos = append(protos, editorProto{prop: prop, label: prop.Texture})
			}
		}
		for _, prop := range level.AnimatedProps {
			key := fmt.Sprintf("A|%s|%v|%v|%v|%v|%v", prop.Texture, prop.Height, prop.ScaleX, prop.ScaleY, prop.XFrames, prop.YFrames)
			if !seen[key] {
				seen[key] = true
				copied := prop
				protos = append(protos, editorProto{animated: &copied, label: prop.Texture + " (ANIMATED)"})
			}
		}
	}
	add(e.level)
	for _, info := range a.levels {
		if len(protos) > 300 || info.ID == editorTestID {
			continue
		}
		level, err := a.pack.Load(info.ID)
		if err == nil && strings.EqualFold(level.Tileset, e.level.Tileset) {
			add(level)
		}
	}
	e.protos, e.protoIndex = protos, 0
}

func (e *levelEditor) placeProto(x, y float64) {
	if len(e.protos) == 0 {
		return
	}
	proto := e.protos[e.protoIndex%len(e.protos)]
	e.pushAs("Place prop")
	x, y = math.Round(x/4)*4, math.Round(y/4)*4
	if proto.animated != nil {
		prop := *proto.animated
		prop.X, prop.Y = x, y
		e.level.AnimatedProps = append(e.level.AnimatedProps, prop)
		e.selected = e.propCount() - 1
	} else {
		prop := proto.prop
		prop.X, prop.Y = x, y
		e.level.Props = append(e.level.Props, prop)
		e.selected = len(e.level.Props) - 1
	}
	e.view.Level = e.level
}

// ---- update --------------------------------------------------------------------------------------------------------

// mouseWorld is the world point under the pointer (px, py in editor units).
func (e *levelEditor) mouseWorld(px, py float64) (float64, float64) {
	v := e.view
	return (px/2-v.ViewportX)/v.Zoom + v.CameraX, (py/2-v.ViewportY)/v.Zoom + v.CameraY
}

func (e *levelEditor) paletteCols(atlasW int) int {
	cell := e.tileset.TileSize
	if cell <= 0 {
		cell = 32
	}
	return max(1, atlasW/cell)
}

func (a *app) editorCapture(state string) error {
	a.titleScreen, a.page = false, 0
	a.editor = &levelEditor{open: true, inList: true, layer: formats.LayerG, brush: 1, collision: 1, selected: -1, pickupSel: -1}
	if state == "editor-list" {
		return nil
	}
	if state == "editor-new" {
		if template, err := a.pack.Load("World0Level1"); err == nil {
			a.openNewDialog(template)
		}
		return nil
	}
	a.editorLoad("World0Level1")
	if a.editor.inList {
		return fmt.Errorf("editor level did not load: %s", a.editor.status)
	}
	if state == "editor-details" {
		a.editor.level.Intro = []formats.IntroLine{{Cameo: "barrycameo", Text: "Here they come! Stay close to the wall."}, {Cameo: "professorbrainscameo", Text: "Brains... I mean, good luck."}}
		a.openDetails()
	}
	if state == "editor-problems" {
		a.editor.level.Layers[formats.LayerC][33*a.editor.level.Width/2] = editorEmpty
		a.editor.level.Waves[0].Spawners[0].Index = 11
		a.openProblems(300)
	}
	if state == "editor-testopts" {
		a.editor.testOpen = true
	}
	switch state {
	case "editor-minimap":
		a.editor.minimap = true
	case "editor-pickups":
		a.editor.tool = toolPickup
		a.editor.level.Pickups = []formats.LevelPickup{{Name: "p_shotgun", X: 600, Y: 400}, {Name: "p_health", X: 760, Y: 420, Wave: 3}}
		a.editor.pickupSel = 0
	case "editor-select":
		a.editor.tool = toolSelect
		a.editor.selOn, a.editor.selX0, a.editor.selY0, a.editor.selX1, a.editor.selY1 = true, 10, 6, 18, 12
	case "editor-proplib":
		a.openPropLibrary()
	case "editor-shortcuts":
		a.editor.shortcutsOpen = true
	case "editor-filemenu":
		a.openFileMenu(6)
	case "editor-history":
		a.editor.pushAs("Paint")
		a.editor.pushAs("Fill")
		a.editor.pushAs("Rectangle")
		a.openHistory(100)
	}
	if state == "editor-tilesets" {
		a.editor.tilesetMenu = true
	}
	if state == "editor-waves" {
		a.editor.wavePage = true
	}
	if state == "editor-help" {
		a.editor.help = true
	}
	if state == "editor-test" {
		a.editorTest()
		return nil
	}
	if state == "editor-props" {
		a.editor.propMode = true
		a.editorCollectProtos()
	}
	if state == "editor-collision" {
		a.editor.layer = formats.LayerC
		a.editor.view.Layers[formats.LayerC] = true
	}
	return nil
}

// editorTestStep runs every frame of a test play. F4 ends the test, and so does any other way out of the level (quit,
// results, the main menu): the editor comes back with everything as it was.
func (a *app) editorTestStep() bool {
	e := a.editor
	back := inpututil.IsKeyJustPressed(ebiten.KeyF4)
	if back && a.play != nil {
		a.stopWeaponPlayback()
		a.play.closeScript()
		a.play = nil
	}
	if back || (a.play == nil && a.page != 5) {
		a.play, a.page = nil, 0
		a.resultsScreen = nil
		a.confirm = nil
		a.native.closeKey = ""
		e.testing, e.open = false, true
		a.editorMusic(true)
		e.say("Back in the editor")
		return true
	}
	return false
}

// drawEditorTestHint tells the player how to get back while a level from the editor is running.
func (a *app) drawEditorTestHint(screen *ebiten.Image) {
	if a.editor != nil && a.editor.testing && a.play != nil && !a.play.paused {
		a.text(screen, "F4: BACK TO THE EDITOR", 4, 308, .24)
	}
}

// zoomBy zooms the map around the centre of its area (the toolbar's zoom buttons).
func (e *levelEditor) zoomBy(factor float64) {
	v := e.view
	cx, cy := (editorToolStrip+editorPanelX)/4, (editorTopBar+editorMapBottom)/4 // the middle of the map area, in the viewer's units
	worldX, worldY := (cx-v.ViewportX)/v.Zoom+v.CameraX, (cy-v.ViewportY)/v.Zoom+v.CameraY
	v.Zoom = math.Max(.1, math.Min(4, v.Zoom*factor))
	v.CameraX = worldX - (cx-v.ViewportX)/v.Zoom
	v.CameraY = worldY - (cy-v.ViewportY)/v.Zoom
}

// changedCell is a cell a stroke touched, for the auto-edge pass that follows it.
type changedCell struct {
	kind formats.LayerKind
	x, y int
}
