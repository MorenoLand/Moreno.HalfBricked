package game

import (
	"math"
	"strconv"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The editor's input: one place that turns keys and the mouse into edits. Single-key shortcuts go through a table so
// they can be rebound (see leveledit_prefs.go).

type edAction struct {
	id, label string
	def       ebiten.Key
}

var editorActions = []edAction{
	{"paint", "Paint tool", ebiten.KeyQ}, {"line", "Line tool", ebiten.KeyL}, {"rect", "Rectangle tool", ebiten.KeyB},
	{"fill", "Fill tool", ebiten.KeyF}, {"pick", "Pick tool", ebiten.KeyI}, {"select", "Select tool", ebiten.KeyM},
	{"props", "Props tool", ebiten.KeyP}, {"pickups", "Pickup tool", ebiten.KeyK},
	{"flipx", "Mirror the tile", ebiten.KeyX}, {"flipy", "Flip the tile upside down", ebiten.KeyV},
	{"brushdown", "Brush smaller", ebiten.KeyBracketLeft}, {"brushup", "Brush bigger", ebiten.KeyBracketRight},
	{"grid", "Show the grid", ebiten.KeyG}, {"fit", "Fit the map in view", ebiten.KeyR}, {"collision", "Show the collision layer", ebiten.KeyC},
	{"minimap", "Show the minimap", ebiten.KeyN}, {"highlight", "Highlight the chosen tile", ebiten.KeyH},
	{"waves", "Wave editor", ebiten.KeyE}, {"test", "Test play", ebiten.KeyT}, {"help", "Help", ebiten.KeyF1},
}

// key is the key bound to an action.
func (e *levelEditor) key(id string) ebiten.Key {
	if key, ok := e.keys[id]; ok {
		return key
	}
	for _, action := range editorActions {
		if action.id == id {
			return action.def
		}
	}
	return ebiten.Key(-1)
}

func (e *levelEditor) act(id string) bool { return inpututil.IsKeyJustPressed(e.key(id)) }

// ---- selection and clipboard ---------------------------------------------------------------------------------------

func (e *levelEditor) selRect() (x0, y0, x1, y1 int) {
	return min(e.selX0, e.selX1), min(e.selY0, e.selY1), max(e.selX0, e.selX1), max(e.selY0, e.selY1)
}

func (e *levelEditor) cellOr(kind formats.LayerKind, x, y int) uint32 {
	if x < 0 || y < 0 || x >= e.level.Width || y >= e.level.Height {
		return editorEmpty
	}
	return e.level.Layers[kind][y*e.level.Width+x]
}

func (e *levelEditor) copySelection() bool {
	if !e.selOn {
		return false
	}
	x0, y0, x1, y1 := e.selRect()
	clip := &editorClip{w: x1 - x0 + 1, h: y1 - y0 + 1, layers: map[formats.LayerKind][]uint32{}}
	for _, kind := range formats.LayerKinds {
		cells := make([]uint32, clip.w*clip.h)
		for y := 0; y < clip.h; y++ {
			for x := 0; x < clip.w; x++ {
				cells[y*clip.w+x] = e.cellOr(kind, x0+x, y0+y)
			}
		}
		clip.layers[kind] = cells
	}
	e.clip = clip
	e.say("Copied " + strconv.Itoa(clip.w) + " x " + strconv.Itoa(clip.h) + " cells. Ctrl+V pastes.")
	return true
}

// clearSelection empties the selected cells on every layer that is not locked.
func (e *levelEditor) clearSelection(label string) {
	if !e.selOn {
		return
	}
	x0, y0, x1, y1 := e.selRect()
	e.strokeSaved, e.strokeSnap, e.strokeLabel = false, e.snapshot(), label
	for _, kind := range formats.LayerKinds {
		empty := editorEmpty
		if kind == formats.LayerG {
			empty = 0
		}
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				e.setCellIn(kind, x, y, empty)
			}
		}
	}
}

// flipClip mirrors the clipboard left-right or up-down; the mirror flags of its tiles flip with it.
func (c *editorClip) flip(horizontal bool) {
	for kind, cells := range c.layers {
		flipped := make([]uint32, len(cells))
		for y := 0; y < c.h; y++ {
			for x := 0; x < c.w; x++ {
				from, to := y*c.w+x, y*c.w+x
				if horizontal {
					to = y*c.w + (c.w - 1 - x)
				} else {
					to = (c.h-1-y)*c.w + x
				}
				value := cells[from]
				if kind != formats.LayerC && value != editorEmpty && !(value == 0 && kind != formats.LayerG) {
					if horizontal {
						value ^= 0x10000
					} else {
						value ^= 0x20000
					}
				}
				flipped[to] = value
			}
		}
		c.layers[kind] = flipped
	}
}

// pasteClip puts the clipboard down with its top left cell at (tx, ty). Empty cells do not overwrite (the ground always
// does); onlyLayer limits it to the layer being edited.
func (e *levelEditor) pasteClip(clip *editorClip, tx, ty int, onlyLayer bool) {
	if clip == nil {
		return
	}
	e.strokeSaved, e.strokeSnap, e.strokeLabel = false, e.snapshot(), "Paste"
	for _, kind := range formats.LayerKinds {
		if onlyLayer && kind != e.layer {
			continue
		}
		for y := 0; y < clip.h; y++ {
			for x := 0; x < clip.w; x++ {
				value := clip.layers[kind][y*clip.w+x]
				if kind != formats.LayerG && (value == editorEmpty || (value == 0 && kind != formats.LayerC)) {
					continue
				}
				e.setCellIn(kind, tx+x, ty+y, value)
			}
		}
	}
}

func (e *levelEditor) noteRecent(value uint32) {
	if e.layer == formats.LayerC {
		return
	}
	for index, existing := range e.recent {
		if existing == value {
			e.recent = append(e.recent[:index], e.recent[index+1:]...)
			break
		}
	}
	e.recent = append([]uint32{value}, e.recent...)
	if len(e.recent) > 8 {
		e.recent = e.recent[:8]
	}
}

// lineCells is the cells on the straight line from (x0, y0) to (x1, y1).
func lineCells(x0, y0, x1, y1 int) [][2]int {
	var cells [][2]int
	dx, dy := absInt(x1-x0), -absInt(y1-y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		cells = append(cells, [2]int{x0, y0})
		if x0 == x1 && y0 == y1 {
			return cells
		}
		doubled := 2 * err
		if doubled >= dy {
			err += dy
			x0 += sx
		}
		if doubled <= dx {
			err += dx
			y0 += sy
		}
	}
}

// paintBrush sets the cells under the brush centred on (x, y).
func (e *levelEditor) paintBrush(x, y int, value uint32) {
	from := -(e.brush - 1) / 2
	for dy := 0; dy < e.brush; dy++ {
		for dx := 0; dx < e.brush; dx++ {
			e.setCell(x+from+dx, y+from+dy, value)
		}
	}
}

// ---- the main update -----------------------------------------------------------------------------------------------

func (a *app) editorSetTool(tool int) {
	e := a.editor
	e.tool, e.propMode, e.wavePage = tool, false, false
	e.lineOn, e.rectOn, e.pasting = false, false, false
}

func (a *app) updateEditor() error {
	e := a.editor
	if e.inList {
		return a.updateEditorList()
	}
	a.editorEnsurePrefs()
	a.editorPoll()
	a.editorApplyArms()
	if e.autoOn && len(e.changedCells) > 0 && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		a.autoEdge()
	}
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	pressed := inpututil.IsKeyJustPressed
	v := e.view
	px, py := a.editorPointer()
	a.editorAutosaveTick()

	if a.editorModals(px, py, ctrl, shift) {
		return nil
	}
	if e.wavePage {
		return a.updateWaveMode(px, py, ctrl, shift)
	}
	if e.field == "tile" {
		a.editorTypeTile()
		return nil
	}
	if pressed(ebiten.KeyEscape) {
		switch {
		case e.pasting:
			e.pasting = false
		case e.selOn:
			e.selOn = false
		case e.dirty:
			a.askConfirm("DISCARD CHANGES?", func() error { e.inList, e.dirty = true, false; return nil })
		default:
			e.inList = true
		}
		return nil
	}
	if ctrl {
		a.editorCtrlKeys(shift)
		return nil
	}

	// Layers and view.
	layerKeys := map[ebiten.Key]formats.LayerKind{ebiten.Key1: formats.LayerG, ebiten.Key2: formats.LayerD, ebiten.Key3: formats.LayerH, ebiten.Key4: formats.LayerHB, ebiten.Key5: formats.LayerC}
	for key, kind := range layerKeys {
		if pressed(key) {
			e.layer, e.propMode = kind, false
			v.Layers[kind] = true
		}
	}
	for _, step := range []struct {
		id string
		do func()
	}{
		{"collision", func() { v.Layers[formats.LayerC] = !v.Layers[formats.LayerC] }},
		{"help", func() { e.help = !e.help }},
		{"grid", func() { v.Grid = !v.Grid }},
		{"fit", e.fit},
		{"minimap", func() { e.minimap = !e.minimap }},
		{"highlight", func() { e.highlight = !e.highlight }},
		{"waves", func() { e.wavePage, e.field = true, "" }},
		{"test", func() {
			if shift && px >= editorToolStrip && px < editorPanelX {
				e.testAtCursor = true
				e.testX, e.testY = e.mouseWorld(px, py)
			}
			a.editorTest()
		}},
		{"flipx", func() { e.flipX = !e.flipX; e.flipClipIfPasting(true) }},
		{"flipy", func() { e.flipY = !e.flipY; e.flipClipIfPasting(false) }},
		{"brushdown", func() { e.brush = max(1, e.brush-1) }},
		{"brushup", func() { e.brush = min(8, e.brush+1) }},
		{"paint", func() { a.editorSetTool(toolPaint) }},
		{"line", func() { a.editorSetTool(toolLine) }},
		{"rect", func() { a.editorSetTool(toolRect) }},
		{"fill", func() { a.editorSetTool(toolFill) }},
		{"pick", func() { a.editorSetTool(toolPick) }},
		{"select", func() { a.editorSetTool(toolSelect) }},
		{"pickups", func() { a.editorSetTool(toolPickup) }},
		{"props", func() {
			e.propMode, e.wavePage = !e.propMode, false
			if e.propMode && e.protos == nil {
				a.editorCollectProtos()
			}
		}},
	} {
		if e.act(step.id) {
			step.do()
			if e.wavePage {
				return nil
			}
		}
	}
	if pressed(ebiten.KeyF5) {
		a.editorTest()
		return nil
	}
	if e.propMode && pressed(ebiten.KeyTab) && len(e.protos) > 0 {
		if shift {
			e.protoIndex = (e.protoIndex + len(e.protos) - 1) % len(e.protos)
		} else {
			e.protoIndex = (e.protoIndex + 1) % len(e.protos)
		}
	}
	if e.propMode && (pressed(ebiten.KeyDelete) || pressed(ebiten.KeyBackspace)) {
		e.deleteProp(e.selected)
	}
	if e.tool == toolSelect && !e.propMode && e.selOn && (pressed(ebiten.KeyDelete) || pressed(ebiten.KeyBackspace)) {
		e.clearSelection("Clear selection")
	}

	// Camera: WASD or arrows, middle drag, wheel zoom around the cursor.
	pan := 10 / v.Zoom
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		v.CameraX -= pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		v.CameraX += pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		v.CameraY -= pan
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		v.CameraY += pan
	}
	overStrip := px < editorToolStrip
	if _, wheel := ebiten.Wheel(); wheel != 0 && px < editorPanelX && !overStrip {
		worldX, worldY := e.mouseWorld(px, py)
		v.Zoom = math.Max(.1, math.Min(4, v.Zoom*(1+wheel*.12)))
		v.CameraX = worldX - (px/2-v.ViewportX)/v.Zoom
		v.CameraY = worldY - (py/2-v.ViewportY)/v.Zoom
	}
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		if e.panning {
			v.CameraX -= (px - e.lastX) / 2 / v.Zoom
			v.CameraY -= (py - e.lastY) / 2 / v.Zoom
		}
		e.panning = true
	} else {
		e.panning = false
	}
	e.lastX, e.lastY = px, py

	inMap := px >= editorToolStrip && px < editorPanelX && py >= editorTopBar && py < editorMapBottom
	worldX, worldY := e.mouseWorld(px, py)
	tileX, tileY := int(math.Floor(worldX/editorTile)), int(math.Floor(worldY/editorTile))
	left, right := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	justLeft := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	justRight := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)

	if pressed(ebiten.KeyI) && inMap {
		a.editorPickAt(tileX, tileY)
	}
	if e.act("fill") && inMap && !e.propMode {
		// the key fills at once under the pointer, whatever the tool
		value := e.paintValue()
		if shift {
			value = e.eraseValue()
		}
		e.fill(tileX, tileY, value)
	}

	if justLeft {
		if a.editorClickButtons(px, py) {
			return nil
		}
		if px >= editorPanelX && py < editorMapBottom {
			a.editorPalettePick(px, py)
			return nil
		}
		if a.editorMinimapClick(px, py) {
			return nil
		}
	}
	if (justLeft || e.minimapDrag) && e.minimap && a.editorMinimapHit(px, py) {
		a.editorMinimapClick(px, py)
		e.minimapDrag = left
		return nil
	}
	if !left {
		e.minimapDrag = false
	}
	if !inMap && !e.rectOn && !e.lineOn && !e.selDrag {
		e.stroke = false
		return nil
	}

	switch {
	case e.pasting:
		if inMap && justLeft {
			e.pasteClip(e.clip, tileX, tileY, shift)
			if !ebiten.IsKeyPressed(ebiten.KeyAltLeft) {
				e.pasting = false
			}
		}
		if justRight {
			e.pasting = false
		}
		return nil
	case e.propMode:
		e.updateProps(inMap, justLeft, left, justRight, worldX, worldY)
		return nil
	case e.tool == toolPickup:
		a.updatePickupTool(inMap, justLeft, left, justRight, worldX, worldY)
		return nil
	case e.tool == toolSelect:
		if inMap && justLeft {
			e.selDrag, e.selOn = true, false
			e.selX0, e.selY0, e.selX1, e.selY1 = tileX, tileY, tileX, tileY
		}
		if e.selDrag {
			e.selX1, e.selY1 = max(0, min(e.level.Width-1, tileX)), max(0, min(e.level.Height-1, tileY))
			if !left {
				e.selDrag, e.selOn = false, true
			}
		}
		return nil
	case e.tool == toolRect:
		if inMap && !e.rectOn && (justLeft || justRight) {
			e.rectOn, e.rectX, e.rectY, e.rectErase = true, tileX, tileY, !justLeft
		}
		if e.rectOn && !(left || right) {
			value := e.paintValue()
			if e.rectErase {
				value = e.eraseValue()
			}
			e.strokeSaved, e.strokeSnap, e.strokeLabel = false, e.snapshot(), "Rectangle"
			for y := min(e.rectY, tileY); y <= max(e.rectY, tileY); y++ {
				for x := min(e.rectX, tileX); x <= max(e.rectX, tileX); x++ {
					e.setCell(x, y, value)
				}
			}
			e.noteRecent(e.paintValue())
			e.rectOn = false
		}
		return nil
	case e.tool == toolLine:
		if inMap && !e.lineOn && (justLeft || justRight) {
			e.lineOn, e.lineX, e.lineY, e.lineErase = true, tileX, tileY, !justLeft
		}
		if e.lineOn && !(left || right) {
			value := e.paintValue()
			if e.lineErase {
				value = e.eraseValue()
			}
			e.strokeSaved, e.strokeSnap, e.strokeLabel = false, e.snapshot(), "Line"
			for _, cell := range lineCells(e.lineX, e.lineY, tileX, tileY) {
				e.paintBrush(cell[0], cell[1], value)
			}
			e.noteRecent(e.paintValue())
			e.lineOn = false
		}
		return nil
	case e.tool == toolFill:
		if inMap && justLeft {
			e.fill(tileX, tileY, e.paintValue())
			e.noteRecent(e.paintValue())
		}
		if inMap && justRight {
			e.fill(tileX, tileY, e.eraseValue())
		}
		return nil
	case e.tool == toolPick:
		if inMap && justLeft {
			a.editorPickAt(tileX, tileY)
			e.tool = toolPaint
		}
		return nil
	}
	// The paint tool.
	if !(left || right) || !inMap {
		e.stroke = false
		return nil
	}
	if !e.stroke {
		e.stroke, e.strokeSaved, e.strokeSnap, e.strokeLabel = true, false, e.snapshot(), "Paint"
		if !right {
			e.noteRecent(e.paintValue())
		}
	}
	value := e.paintValue()
	if right || ebiten.IsKeyPressed(ebiten.KeyAltLeft) {
		value = e.eraseValue()
	}
	e.paintBrush(tileX, tileY, value)
	return nil
}

func (e *levelEditor) flipClipIfPasting(horizontal bool) {
	if e.pasting && e.clip != nil {
		e.clip.flip(horizontal)
	}
}

// editorPickAt takes the tile (or collision value) of a cell.
func (a *app) editorPickAt(x, y int) {
	e := a.editor
	value, ok := e.cellAt(x, y)
	if !ok {
		return
	}
	if e.layer == formats.LayerC {
		e.collision = value
	} else if value != editorEmpty {
		e.tile, e.flipX, e.flipY = value&0xffff, value&0x10000 != 0, value&0x20000 != 0
	}
}

// editorCtrlKeys runs the Ctrl shortcuts.
func (a *app) editorCtrlKeys(shift bool) {
	e := a.editor
	pressed := inpututil.IsKeyJustPressed
	switch {
	case pressed(ebiten.KeyS) && shift:
		a.editorSaveToCache()
	case pressed(ebiten.KeyS):
		a.editorSaveAs()
	case pressed(ebiten.KeyO):
		a.editorOpenFile()
	case pressed(ebiten.KeyZ) && shift, pressed(ebiten.KeyY):
		e.doRedo()
	case pressed(ebiten.KeyZ):
		e.doUndo()
	case pressed(ebiten.KeyD):
		e.duplicateProp(e.selected)
	case pressed(ebiten.KeyT):
		e.tilesetMenu = !e.tilesetMenu
	case pressed(ebiten.KeyA):
		a.editorSetTool(toolSelect)
		e.selOn, e.selX0, e.selY0, e.selX1, e.selY1 = true, 0, 0, e.level.Width-1, e.level.Height-1
	case pressed(ebiten.KeyC) && shift:
		a.editorStampSave()
	case pressed(ebiten.KeyC):
		e.copySelection()
	case pressed(ebiten.KeyX):
		if e.copySelection() {
			e.clearSelection("Cut")
		}
	case pressed(ebiten.KeyV):
		if e.clip == nil {
			e.say("Nothing copied yet: select a region and press Ctrl+C")
		} else {
			e.pasting, e.propMode = true, false
			e.say("Click to paste. X and V flip it, Shift pastes on this layer only, Esc cancels.")
		}
	}
	step := 1
	if shift {
		step = 8
	}
	switch {
	case pressed(ebiten.KeyArrowRight):
		e.resize(e.level.Width+step, e.level.Height)
	case pressed(ebiten.KeyArrowLeft):
		e.resize(e.level.Width-step, e.level.Height)
	case pressed(ebiten.KeyArrowDown):
		e.resize(e.level.Width, e.level.Height+step)
	case pressed(ebiten.KeyArrowUp):
		e.resize(e.level.Width, e.level.Height-step)
	}
}

// editorTypeTile reads digits into the tile-number field.
func (a *app) editorTypeTile() {
	e := a.editor
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= '0' && r <= '9' {
			e.input += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && len(e.input) > 0 {
		e.input = e.input[:len(e.input)-1]
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		e.field = ""
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft):
		if value, err := strconv.Atoi(e.input); err == nil && e.view.Atlas != nil {
			cols := e.paletteCols(e.view.Atlas.Bounds().Dx())
			rows := e.view.Atlas.Bounds().Dy() / max(1, e.view.Atlas.Bounds().Dx()/cols)
			if value >= 0 && value < cols*rows {
				e.tile = uint32(value)
			} else {
				e.say("This tileset has tiles 0 to " + strconv.Itoa(cols*rows-1))
			}
		}
		e.field = ""
	}
}

// editorPalettePick picks the tile (or collision value) under a click in the panel.
func (a *app) editorPalettePick(px, py float64) {
	e := a.editor
	x, y := px-editorPanelX, py-editorPaletteTop
	if e.propMode || y < 0 {
		return
	}
	if e.layer == formats.LayerC {
		for index, item := range collisionPalette {
			if y >= float64(index)*24 && y < float64(index)*24+24 {
				e.collision = item.value
			}
		}
		return
	}
	if e.view.Atlas == nil {
		return
	}
	cols := e.paletteCols(e.view.Atlas.Bounds().Dx())
	cell := (editorW - editorPanelX) / float64(cols)
	tx, ty := int(x/cell), int(y/cell)
	rows := e.view.Atlas.Bounds().Dy() / max(1, e.view.Atlas.Bounds().Dx()/cols)
	if tx >= 0 && tx < cols && ty >= 0 && ty < rows {
		e.tile = uint32(ty*cols + tx)
	}
}

// updateProps is the props tool: click places, drag moves, right click removes.
func (e *levelEditor) updateProps(inMap, justLeft, left, justRight bool, worldX, worldY float64) {
	if !inMap {
		return
	}
	switch {
	case justLeft:
		if near := e.propNear(worldX, worldY); near >= 0 {
			e.selected, e.dragging = near, true
			e.pushAs("Move prop")
		} else {
			e.placeProto(worldX, worldY)
		}
	case left && e.selected >= 0 && e.dragging:
		e.movePropTo(e.selected, math.Round(worldX/2)*2, math.Round(worldY/2)*2)
	case justRight:
		if near := e.propNear(worldX, worldY); near >= 0 {
			e.deleteProp(near)
		}
	}
	if !left {
		e.dragging = false
	}
}

// editorModals runs whichever pop-up or dialog is open and reports whether it took the frame.
func (a *app) editorModals(px, py float64, ctrl, shift bool) bool {
	e := a.editor
	pressed := inpututil.IsKeyJustPressed
	switch {
	case e.help:
		// The help overlay is up: any click, Esc or F1 closes it and nothing else happens.
		if pressed(ebiten.KeyEscape) || pressed(e.key("help")) || inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			e.help = false
		}
		return true
	case e.popup != nil:
		a.updatePopup(px, py)
		return true
	case e.tilesetMenu:
		a.updateTilesetMenu(px, py)
		return true
	}
	return a.editorDialogs(px, py)
}
