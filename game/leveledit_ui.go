package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The level editor's screen, in editor units (see leveledit_draw.go): an icon toolbar along the top, the map, a panel on
// the right (layers with show/hide eyes, the tile palette, the chosen tile) and a status bar that explains whatever the
// pointer is over.

const (
	editorToolStrip  = 46.0 // the column of tools at the left of the map
	editorStatusBar  = 26.0
	editorMapBottom  = editorH - editorStatusBar
	editorLayerTop   = 62.0 // first layer row
	editorLayerRow   = 27.0
	editorPaletteTop = 214.0 // the tile palette (or the collision list, or the prop controls)
)

var (
	editorBar      = color.RGBA{15, 18, 26, 255}
	editorPanel    = color.RGBA{21, 25, 35, 255}
	editorBtn      = color.RGBA{40, 47, 63, 255}
	editorBtnHover = color.RGBA{70, 84, 114, 255}
	editorBtnOn    = color.RGBA{44, 112, 62, 255}
	editorIcon     = color.RGBA{230, 235, 247, 255}
	editorDim      = color.RGBA{138, 148, 170, 255}
	editorText     = color.RGBA{226, 231, 242, 255}
	editorPlay     = color.RGBA{42, 142, 66, 255}
)

// edButton is one clickable control; the same list drives drawing, hover help and clicks.
type edButton struct {
	label, tip, icon string
	x, y, w, h       float64
	active           func() bool
	accent           bool
	do               func()
}

func (b edButton) contains(px, py float64) bool {
	return px >= b.x && px < b.x+b.w && py >= b.y && py < b.y+b.h
}

type editorLayerEntry struct {
	kind       formats.LayerKind
	name, hint string
	swatch     color.RGBA
	key        string
}

func editorLayers() []editorLayerEntry {
	return []editorLayerEntry{
		{formats.LayerG, "Ground", "The floor tiles. Every cell has one.", color.RGBA{120, 190, 90, 255}, "1"},
		{formats.LayerD, "Decor", "Extra detail on the floor: cracks, flowers, bones.", color.RGBA{220, 190, 90, 255}, "2"},
		{formats.LayerH, "Tall", "Trees, rocks and walls, drawn in depth order.", color.RGBA{190, 140, 220, 255}, "3"},
		{formats.LayerHB, "Behind", "Objects drawn behind the player.", color.RGBA{110, 190, 230, 255}, "4"},
		{formats.LayerC, "Collision", "Walls, the player spawn and the zombie spawn zones.", color.RGBA{230, 100, 90, 255}, "5"},
	}
}

// editorPaletteGeometry is where the pieces under the tile palette sit (editor units): the info block with the chosen tile,
// the row of recent tiles, and the highlight button.
func (a *app) editorPaletteGeometry() (infoY, recentY, highlightY float64, ok bool) {
	e := a.editor
	if e.view.Atlas == nil || e.layer == formats.LayerC || e.propMode || e.tool == toolPickup {
		return 0, 0, 0, false
	}
	bounds := e.view.Atlas.Bounds()
	panelW := editorW - editorPanelX
	infoY = editorPaletteTop + panelW*float64(bounds.Dy())/float64(bounds.Dx()) + 10
	recentY = infoY + 66
	highlightY = recentY + 34
	return infoY, recentY, highlightY, highlightY+24 < editorMapBottom
}

// editorButtons is the whole clickable set of the map editor: the toolbar, the tool strip, the layer rows, the pieces under
// the palette, the bar over a selection and, in prop mode, the prop controls.
func (a *app) editorButtons() []edButton {
	e := a.editor
	var buttons []edButton
	const y, size = 5.0, 30.0
	x := 6.0
	add := func(icon, tip string, active func() bool, do func()) {
		buttons = append(buttons, edButton{icon: icon, tip: tip, x: x, y: y, w: size, h: size, active: active, do: do})
		x += size + 3
	}
	gap := func(width float64) { x += width }

	add("file", "File: open, save, share a level, add it to the game", func() bool { return e.popup != nil && e.popup.title == "File" }, func() { a.openFileMenu(buttonX(buttons, "file")) })
	gap(12)
	add("undo", "Undo (Ctrl+Z)", nil, e.doUndo)
	add("redo", "Redo (Ctrl+Y)", nil, e.doRedo)
	add("history", "History: every step, click one to go back to it", func() bool { return e.popup != nil && e.popup.title == "History" }, func() { a.openHistory(buttonX(buttons, "history")) })
	gap(12)
	add("minus", "Smaller brush (bracket key, left)", nil, func() { e.brush = max(1, e.brush-1) })
	x += 26 // the brush size is drawn between the two
	add("plus", "Bigger brush (bracket key, right)", nil, func() { e.brush = min(8, e.brush+1) })
	gap(12)
	add("flipx", "Mirror the tile left-right (X)", func() bool { return e.flipX }, func() { e.flipX = !e.flipX; e.flipClipIfPasting(true) })
	add("flipy", "Mirror the tile up-down (V)", func() bool { return e.flipY }, func() { e.flipY = !e.flipY; e.flipClipIfPasting(false) })
	gap(12)
	add("grid", "Show the tile grid (G)", func() bool { return e.view.Grid }, func() { e.view.Grid = !e.view.Grid })
	add("zoomout", "Zoom out (mouse wheel down)", nil, func() { e.zoomBy(1 / 1.25) })
	add("zoomin", "Zoom in (mouse wheel up)", nil, func() { e.zoomBy(1.25) })
	add("fit", "Fit the whole map in the view (R)", nil, e.fit)
	add("minimap", "Show a minimap of the whole level (N)", func() bool { return e.minimap }, func() { e.minimap = !e.minimap })
	add("edge", "Auto-edge (experimental): after you paint, the cells around it are matched to what the game's own levels put there", func() bool { return e.autoOn }, func() {
		e.autoOn = !e.autoOn
		if e.autoOn {
			e.say("Auto-edge is on: the cells around what you paint follow the game's own levels where they agree")
		}
	})
	add("tileset", "Change the tileset of this level: pick one from a menu (Ctrl+T)", func() bool { return e.tilesetMenu }, func() { e.tilesetMenu = !e.tilesetMenu })
	gap(12)
	add("details", "Level details: name, music, mode, world, intro dialogue", func() bool { return e.detailsOpen }, func() { a.openDetails() })
	add("waves", "Waves: edit what spawns, where and when (E)", func() bool { return e.wavePage }, func() { e.wavePage = !e.wavePage; e.field = "" })
	add("problems", "Problems: things that would stop this level working", func() bool { return e.popup != nil && e.popup.title == "Problems" }, func() { a.openProblems(buttonX(buttons, "problems")) })

	right := editorW - 6
	place := func(icon, tip string, width float64, label string, accent bool, do func()) {
		right -= width
		buttons = append(buttons, edButton{icon: icon, label: label, tip: tip, x: right, y: y, w: width, h: size, accent: accent, do: do})
		right -= 3
	}
	place("close", "Close the editor (F4)", size, "", false, func() { a.toggleEditor() })
	place("list", "Back to the list of levels (Esc)", size, "", false, a.editorBackToList)
	place("help", "Help: what everything does (F1)", size, "", false, func() { e.help = !e.help })
	right -= 8
	place("play", "Play this level now. F4 brings you back here (T)", 84, "Test", true, a.editorTest)
	place("options", "Test options: god mode, starting wave, start at the view", size, "", false, func() { e.testOpen = !e.testOpen })
	// The problem count rides on its button.
	for index := range buttons {
		if buttons[index].icon == "problems" {
			buttons[index].label = a.problemBadge()
		}
	}

	if e.wavePage {
		return buttons
	}

	// The tool strip.
	tools := []struct {
		tool int
		icon string
		tip  string
	}{
		{toolPaint, "paint", "Paint (Q): drag with the left button, the right button erases"},
		{toolLine, "line", "Line (L): drag a straight line"},
		{toolRect, "rect", "Rectangle (B): drag to fill a rectangle, right drag erases one"},
		{toolFill, "fill", "Fill (F): click to flood an area, right click erases it"},
		{toolPick, "pick", "Pick (I): click a cell to take its tile"},
		{toolSelect, "select", "Select (M): drag a region, then copy, cut, clear or flip it"},
		{toolPickup, "pickup", "Pickups (K): place a crate that is there when the level (or a wave) starts"},
	}
	for index, tool := range tools {
		t := tool.tool
		buttons = append(buttons, edButton{icon: tool.icon, tip: tool.tip, x: 5, y: editorTopBar + 8 + float64(index)*40, w: 36, h: 36,
			active: func() bool { return e.tool == t && !e.propMode && !e.pasting },
			do:     func() { a.editorSetTool(t) }})
	}
	buttons = append(buttons, edButton{icon: "props", tip: "Props (P): click places one, drag moves, right click removes", x: 5, y: editorTopBar + 8 + float64(len(tools))*40, w: 36, h: 36,
		active: func() bool { return e.propMode },
		do: func() {
			e.propMode, e.wavePage = !e.propMode, false
			if e.propMode && e.protos == nil {
				a.editorCollectProtos()
			}
		}})

	// The layer rows: an eye shows or hides a layer (Alt+click shows only that one), the row picks it, the padlock locks it.
	panelW := editorW - editorPanelX
	for index, layer := range editorLayers() {
		kind := layer.kind
		rowY := editorLayerTop + float64(index)*editorLayerRow
		buttons = append(buttons,
			edButton{icon: "eye", tip: "Show or hide the " + strings.ToLower(layer.name) + " layer (Alt+click: show only this layer)", x: editorPanelX + 6, y: rowY, w: 32, h: editorLayerRow - 3, active: func() bool { return e.view.Layers[kind] },
				do: func() { a.editorToggleLayer(kind) }},
			edButton{label: layer.name, icon: "chip" + layer.key, tip: "Edit the " + strings.ToLower(layer.name) + " layer (" + layer.key + "): " + layer.hint, x: editorPanelX + 41, y: rowY, w: panelW - 47 - 32, h: editorLayerRow - 3,
				active: func() bool { return e.layer == kind && !e.propMode },
				do: func() {
					e.layer, e.propMode, e.wavePage = kind, false, false
					e.view.Layers[kind] = true
				}},
			edButton{icon: "lock", tip: "Lock the " + strings.ToLower(layer.name) + " layer so painting cannot change it", x: editorPanelX + panelW - 38, y: rowY, w: 32, h: editorLayerRow - 3, active: func() bool { return e.locked[kind] },
				do: func() {
					if e.locked == nil {
						e.locked = map[formats.LayerKind]bool{}
					}
					e.locked[kind] = !e.locked[kind]
				}})
	}
	if infoY, recentY, highlightY, ok := a.editorPaletteGeometry(); ok {
		buttons = append(buttons, edButton{label: "tilefield", tip: "Click to type a tile number", x: editorPanelX + 62, y: infoY, w: 130, h: 24, do: func() { e.field, e.input = "tile", "" }})
		for index, value := range e.recent {
			v := value
			buttons = append(buttons, edButton{label: "recent", tip: "A tile you used recently", x: editorPanelX + 8 + float64(index)*32, y: recentY, w: 28, h: 28,
				active: func() bool { return e.tile == v&0xffff && e.flipX == (v&0x10000 != 0) && e.flipY == (v&0x20000 != 0) },
				do:     func() { e.tile, e.flipX, e.flipY = v&0xffff, v&0x10000 != 0, v&0x20000 != 0 }})
		}
		buttons = append(buttons, edButton{label: "Find this tile on the map", icon: "ring", tip: "Highlight every cell of the layer that uses the chosen tile (H)", x: editorPanelX + 6, y: highlightY, w: panelW - 12, h: 24,
			active: func() bool { return e.highlight }, do: func() { e.highlight = !e.highlight }})
	}

	// The bar over a selection or a paste.
	barX := editorToolStrip + 10
	bar := func(icon, tip string, do func()) {
		buttons = append(buttons, edButton{icon: icon, tip: tip, x: barX, y: editorTopBar + 8, w: 32, h: 32, do: do})
		barX += 35
	}
	switch {
	case e.pasting:
		bar("fliph", "Mirror what you are pasting left-right (X)", func() { e.flipClipIfPasting(true) })
		bar("flipv", "Flip what you are pasting upside down (V)", func() { e.flipClipIfPasting(false) })
		bar("close", "Cancel the paste (Esc)", func() { e.pasting = false })
	case e.selOn:
		bar("copy", "Copy the selection (Ctrl+C)", func() { e.copySelection() })
		bar("cut", "Cut the selection (Ctrl+X)", func() {
			if e.copySelection() {
				e.clearSelection("Cut")
			}
		})
		bar("paste", "Paste (Ctrl+V)", func() {
			if e.clip != nil {
				e.pasting = true
			}
		})
		bar("trash", "Clear the selected cells (Delete)", func() { e.clearSelection("Clear selection") })
		bar("fliph", "Mirror the selection left-right", func() {
			if e.copySelection() {
				e.clip.flip(true)
				x0, y0, _, _ := e.selRect()
				e.pasteClip(e.clip, x0, y0, false)
			}
		})
		bar("flipv", "Flip the selection upside down", func() {
			if e.copySelection() {
				e.clip.flip(false)
				x0, y0, _, _ := e.selRect()
				e.pasteClip(e.clip, x0, y0, false)
			}
		})
		bar("stamp", "Save the selection in the stamp library (Ctrl+Shift+C)", func() { a.editorStampSave() })
		bar("close", "Deselect (Esc)", func() { e.selOn = false })
	}
	buttons = append(buttons, edButton{icon: "stamp", tip: "Stamp library: place your saved pieces again", x: 5, y: editorTopBar + 8 + float64(len(tools)+1)*40, w: 36, h: 36,
		active: func() bool { return e.popup != nil && e.popup.title == "Stamps" }, do: func() { a.openStamps(46) }})

	if e.tool == toolPickup && !e.propMode {
		buttons = append(buttons, a.pickupButtons()...)
	}
	if e.propMode {
		top := editorPaletteTop + 44
		half := panelW/2 - 9
		buttons = append(buttons,
			edButton{label: "Previous", tip: "Previous prop (Shift+Tab)", x: editorPanelX + 6, y: top, w: half, h: 28, do: func() { e.cycleProto(-1) }},
			edButton{label: "Next", tip: "Next prop (Tab)", x: editorPanelX + panelW/2 + 3, y: top, w: half, h: 28, do: func() { e.cycleProto(1) }},
			edButton{label: "Copy", tip: "Copy the selected prop (Ctrl+D)", x: editorPanelX + 6, y: top + 32, w: half, h: 28, do: func() { e.duplicateProp(e.selected) }},
			edButton{label: "Delete", tip: "Remove the selected prop (Del)", x: editorPanelX + panelW/2 + 3, y: top + 32, w: half, h: 28, do: func() { e.deleteProp(e.selected) }},
			edButton{label: "Prop library", icon: "props", tip: "Use any texture of the game as a prop", x: editorPanelX + 6, y: top + 68, w: panelW - 12, h: 28, do: func() { a.openPropLibrary() }})
	}
	return buttons
}

// buttonX is the left edge of the toolbar button with the icon (while the list is being built).
func buttonX(buttons []edButton, icon string) float64 {
	for _, button := range buttons {
		if button.icon == icon {
			return button.x
		}
	}
	return 8
}

// editorToggleLayer shows or hides a layer; with Alt it shows only that layer (and again shows them all).
func (a *app) editorToggleLayer(kind formats.LayerKind) {
	e := a.editor
	if ebiten.IsKeyPressed(ebiten.KeyAltLeft) || ebiten.IsKeyPressed(ebiten.KeyAltRight) {
		only := true
		for _, other := range formats.LayerKinds {
			if other != kind && e.view.Layers[other] {
				only = false
			}
		}
		for _, other := range formats.LayerKinds {
			e.view.Layers[other] = only || other == kind
		}
		if !only {
			e.view.Layers[kind] = true
		}
		return
	}
	e.view.Layers[kind] = !e.view.Layers[kind]
}

func (e *levelEditor) cycleProto(step int) {
	if len(e.protos) > 0 {
		e.protoIndex = (e.protoIndex + step + len(e.protos)) % len(e.protos)
	}
}

func (a *app) editorBackToList() {
	e := a.editor
	if e.dirty {
		a.askConfirm("DISCARD CHANGES?", func() error { e.inList, e.dirty = true, false; return nil })
		return
	}
	e.inList = true
}

// editorClickButtons runs the button under the pointer and reports whether there was one.
func (a *app) editorClickButtons(px, py float64) bool {
	for _, button := range a.editorButtons() {
		if button.contains(px, py) {
			button.do()
			return true
		}
	}
	return false
}

// editorMusic keeps the menu music off while the editor is open (and on, as the options say, while a test level plays).
func (a *app) editorMusic(editing bool) {
	if a.sound != nil {
		a.sound.MusicEnabled(!editing && !a.silent && a.options.music)
	}
}

// ---- icons ---------------------------------------------------------------------------------------------------------

// drawEditorIcon draws a 24 x 24 icon centred on (cx, cy) from smooth round-capped strokes and shapes (no art files), so it
// stays sharp at any window size. bg is the colour behind it, for the parts that cut into a shape.
func (a *app) drawEditorIcon(screen *ebiten.Image, icon string, cx, cy float64, c, bg color.RGBA) {
	sx, sy := a.editorScale()
	px := func(v float64) float32 { return float32((cx + v) * sx) }
	py := func(v float64) float32 { return float32((cy + v) * sy) }
	build := func(points [][2]float64, closed bool) *vector.Path {
		var path vector.Path
		for index, p := range points {
			if index == 0 {
				path.MoveTo(px(p[0]), py(p[1]))
			} else {
				path.LineTo(px(p[0]), py(p[1]))
			}
		}
		if closed {
			path.Close()
		}
		return &path
	}
	stroke := func(path *vector.Path, width float64, clr color.RGBA) {
		options := &vector.DrawPathOptions{AntiAlias: true}
		options.ColorScale.ScaleWithColor(clr)
		vector.StrokePath(screen, path, &vector.StrokeOptions{Width: float32(width * sx), LineCap: vector.LineCapRound, LineJoin: vector.LineJoinRound}, options)
	}
	fill := func(path *vector.Path, clr color.RGBA) {
		options := &vector.DrawPathOptions{AntiAlias: true}
		options.ColorScale.ScaleWithColor(clr)
		vector.FillPath(screen, path, &vector.FillOptions{}, options)
	}
	line := func(width float64, points ...[2]float64) { stroke(build(points, false), width, c) }
	poly := func(clr color.RGBA, points ...[2]float64) { fill(build(points, true), clr) }
	rect := func(x, y, w, h float64, clr color.RGBA) {
		vector.FillRect(screen, px(x), py(y), float32(w*sx), float32(h*sy), clr, true)
	}
	disc := func(x, y, r float64, clr color.RGBA) {
		vector.FillCircle(screen, px(x), py(y), float32(r*sx), clr, true)
	}
	ring := func(x, y, r, w float64) {
		vector.StrokeCircle(screen, px(x), py(y), float32(r*sx), float32(w*sx), c, true)
	}
	curve := func(width float64, start [2]float64, segments ...[4]float64) {
		var path vector.Path
		path.MoveTo(px(start[0]), py(start[1]))
		for _, s := range segments {
			path.QuadTo(px(s[0]), py(s[1]), px(s[2]), py(s[3]))
		}
		stroke(&path, width, c)
	}
	switch icon {
	case "open":
		poly(c, [2]float64{-10, -7}, [2]float64{-3, -7}, [2]float64{-1, -4}, [2]float64{10, -4}, [2]float64{10, 8}, [2]float64{-10, 8})
		line(1.6, [2]float64{-6, 1}, [2]float64{6, 1})
	case "save":
		poly(c, [2]float64{-9, -9}, [2]float64{6, -9}, [2]float64{9, -6}, [2]float64{9, 9}, [2]float64{-9, 9})
		rect(-5, -9, 9, 5, bg)
		rect(-5, 2, 10, 7, bg)
		rect(-2, 4, 4, 5, c)
	case "stack":
		// a database drum: the cache
		ellipse := func(cy, rx, ry float64, clr color.RGBA) {
			points := make([][2]float64, 0, 28)
			for i := 0; i < 28; i++ {
				angle := float64(i) * 2 * math.Pi / 28
				points = append(points, [2]float64{rx * math.Cos(angle), cy + ry*math.Sin(angle)})
			}
			poly(clr, points...)
		}
		ellipse(6, 9, 4, c)
		rect(-9, -6, 18, 12, c)
		ellipse(-6, 9, 4, c)
		for _, cy := range []float64{-1, 3.5} {
			var path vector.Path
			path.MoveTo(px(-9), py(cy))
			path.QuadTo(px(0), py(cy+6), px(9), py(cy))
			stroke(&path, 1.6, bg)
		}
	case "undo", "redo":
		d := 1.0
		if icon == "redo" {
			d = -1
		}
		curve(2.4, [2]float64{d * 7, 8}, [4]float64{d * 7, -5, d * 0, -5}, [4]float64{d * -3, -5, d * -3, -5})
		poly(c, [2]float64{d * -10, -5}, [2]float64{d * -3, -10}, [2]float64{d * -3, 0})
	case "paint":
		poly(c, [2]float64{-9, 9}, [2]float64{-8, 3}, [2]float64{3, -8}, [2]float64{8, -3}, [2]float64{-3, 8})
		poly(c, [2]float64{4, -9}, [2]float64{6, -11}, [2]float64{11, -6}, [2]float64{9, -4})
		line(1.4, [2]float64{-6, 6}, [2]float64{-4, 4})
	case "fill":
		poly(c, [2]float64{-1, -9}, [2]float64{8, 0}, [2]float64{0, 8}, [2]float64{-9, -1})
		curve(1.8, [2]float64{-5, -5}, [4]float64{-6, -11, 0, -11}, [4]float64{3, -11, 4, -8})
		disc(9.5, 7, 2.4, c)
	case "pick":
		// an eyedropper: tip, glass tube, rubber bulb
		line(1.8, [2]float64{-9, 9}, [2]float64{-5.5, 5.5})
		line(5, [2]float64{-5, 5}, [2]float64{2.5, -2.5})
		line(8, [2]float64{4, -4}, [2]float64{8, -8})
		line(1.6, [2]float64{-2, 3}, [2]float64{2, -1})
	case "props":
		poly(c, [2]float64{0, -10}, [2]float64{-6, -2}, [2]float64{6, -2})
		poly(c, [2]float64{0, -5}, [2]float64{-8, 5}, [2]float64{8, 5})
		rect(-1.6, 5, 3.2, 5, c)
	case "minus":
		line(3, [2]float64{-6, 0}, [2]float64{6, 0})
	case "plus":
		line(3, [2]float64{-6, 0}, [2]float64{6, 0})
		line(3, [2]float64{0, -6}, [2]float64{0, 6})
	case "flipx":
		poly(c, [2]float64{-9, 0}, [2]float64{-2.5, -6.5}, [2]float64{-2.5, 6.5})
		stroke(build([][2]float64{{2.5, -6.5}, {9, 0}, {2.5, 6.5}}, true), 1.6, c)
		line(1.2, [2]float64{0, -10}, [2]float64{0, 10})
	case "flipy":
		poly(c, [2]float64{0, -9}, [2]float64{-6.5, -2.5}, [2]float64{6.5, -2.5})
		stroke(build([][2]float64{{-6.5, 2.5}, {0, 9}, {6.5, 2.5}}, true), 1.6, c)
		line(1.2, [2]float64{-10, 0}, [2]float64{10, 0})
	case "grid":
		stroke(build([][2]float64{{-9, -9}, {9, -9}, {9, 9}, {-9, 9}}, true), 1.8, c)
		line(1.6, [2]float64{-3, -9}, [2]float64{-3, 9})
		line(1.6, [2]float64{3, -9}, [2]float64{3, 9})
		line(1.6, [2]float64{-9, -3}, [2]float64{9, -3})
		line(1.6, [2]float64{-9, 3}, [2]float64{9, 3})
	case "fit":
		for _, corner := range [][2]float64{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			line(2.2, [2]float64{corner[0] * 4, corner[1] * 9}, [2]float64{corner[0] * 9, corner[1] * 9}, [2]float64{corner[0] * 9, corner[1] * 4})
		}
	case "tileset":
		for index, clr := range []color.RGBA{{236, 192, 62, 255}, {72, 172, 232, 255}, {204, 92, 224, 255}, {112, 232, 124, 255}} {
			x, y := -9+float64(index%2)*10, -9+float64(index/2)*10
			poly(clr, [2]float64{x, y}, [2]float64{x + 8, y}, [2]float64{x + 8, y + 8}, [2]float64{x, y + 8})
		}
	case "play":
		poly(c, [2]float64{-6, -9}, [2]float64{9, 0}, [2]float64{-6, 9})
	case "list":
		line(2.4, [2]float64{-8, -6}, [2]float64{8, -6})
		line(2.4, [2]float64{-8, 0}, [2]float64{8, 0})
		line(2.4, [2]float64{-8, 6}, [2]float64{8, 6})
	case "close":
		line(2.6, [2]float64{-7, -7}, [2]float64{7, 7})
		line(2.6, [2]float64{-7, 7}, [2]float64{7, -7})
	case "help":
		a.vText(screen, "?", cx-5, cy-11, 22, c, true)
	case "eye":
		curve(1.8, [2]float64{-10, 0}, [4]float64{0, -11, 10, 0}, [4]float64{0, 11, -10, 0})
		disc(0, 0, 3.4, c)
	case "waves":
		// a zombie skull: this is where the enemies come from
		disc(0, -2, 8.6, c)
		rect(-5, 3, 10, 6.5, c)
		disc(-3.4, -2.2, 2.5, bg)
		disc(3.4, -2.2, 2.5, bg)
		poly(bg, [2]float64{0, 1}, [2]float64{-1.6, 3.6}, [2]float64{1.6, 3.6})
		rect(-2.2, 5.6, 1, 3.9, bg)
		rect(1.2, 5.6, 1, 3.9, bg)
	case "rect":
		stroke(build([][2]float64{{-8, -6}, {8, -6}, {8, 6}, {-8, 6}}, true), 1.8, c)
		for _, corner := range [][2]float64{{-8, -6}, {8, -6}, {8, 6}, {-8, 6}} {
			rect(corner[0]-2.3, corner[1]-2.3, 4.6, 4.6, c)
		}
	case "zoomin", "zoomout":
		ring(-2, -2, 7, 2)
		line(3, [2]float64{3, 3}, [2]float64{9, 9})
		line(1.8, [2]float64{-5, -2}, [2]float64{1, -2})
		if icon == "zoomin" {
			line(1.8, [2]float64{-2, -5}, [2]float64{-2, 1})
		}
	case "file":
		poly(c, [2]float64{-7, -9}, [2]float64{3, -9}, [2]float64{8, -4}, [2]float64{8, 9}, [2]float64{-7, 9})
		poly(bg, [2]float64{3, -9}, [2]float64{3, -4}, [2]float64{8, -4})
		rect(-4, 0, 9, 1.6, bg)
		rect(-4, 4, 9, 1.6, bg)
	case "history":
		ring(0, 0, 8.5, 2)
		line(2, [2]float64{0, -5}, [2]float64{0, 0}, [2]float64{4, 3})
	case "details":
		line(2, [2]float64{-9, -5}, [2]float64{9, -5})
		line(2, [2]float64{-9, 0}, [2]float64{9, 0})
		line(2, [2]float64{-9, 5}, [2]float64{9, 5})
		disc(-3, -5, 2.8, c)
		disc(4, 0, 2.8, c)
		disc(-1, 5, 2.8, c)
	case "problems":
		stroke(build([][2]float64{{0, -9}, {9.5, 8}, {-9.5, 8}}, true), 2, c)
		line(2.2, [2]float64{0, -3}, [2]float64{0, 2})
		disc(0, 5.3, 1.3, c)
	case "minimap":
		stroke(build([][2]float64{{-9, -7}, {9, -7}, {9, 7}, {-9, 7}}, true), 1.8, c)
		stroke(build([][2]float64{{-4, -3}, {3, -3}, {3, 3}, {-4, 3}}, true), 1.4, c)
	case "options":
		disc(0, 0, 6.5, c)
		for i := 0; i < 8; i++ {
			angle := float64(i) * math.Pi / 4
			cx, cy := 8.2*math.Cos(angle), 8.2*math.Sin(angle)
			poly(c, [2]float64{cx - 1.8, cy - 1.8}, [2]float64{cx + 1.8, cy - 1.8}, [2]float64{cx + 1.8, cy + 1.8}, [2]float64{cx - 1.8, cy + 1.8})
		}
		disc(0, 0, 2.6, bg)
	case "line":
		line(2.2, [2]float64{-8, 8}, [2]float64{8, -8})
		disc(-8, 8, 2.8, c)
		disc(8, -8, 2.8, c)
	case "select":
		for _, seg := range [][4]float64{{-8, -7, -4, -7}, {0, -7, 4, -7}, {8, -7, 8, -3}, {8, 1, 8, 5}, {8, 9, 4, 9}, {0, 9, -4, 9}, {-8, 9, -8, 5}, {-8, 1, -8, -3}} {
			line(2, [2]float64{seg[0], seg[1]}, [2]float64{seg[2], seg[3]})
		}
	case "pickup":
		stroke(build([][2]float64{{-8, -4}, {8, -4}, {8, 9}, {-8, 9}}, true), 2, c)
		line(2, [2]float64{-8, 2}, [2]float64{8, 2})
		line(2, [2]float64{0, -4}, [2]float64{0, 9})
		curve(1.8, [2]float64{0, -4}, [4]float64{-3, -10, -6, -6}, [4]float64{-4, -3, 0, -4})
		curve(1.8, [2]float64{0, -4}, [4]float64{3, -10, 6, -6}, [4]float64{4, -3, 0, -4})
	case "lock":
		rect(-6, -1, 12, 10, c)
		curve(2, [2]float64{-3.5, -1}, [4]float64{-3.5, -8, 0, -8}, [4]float64{3.5, -8, 3.5, -1})
		disc(0, 4, 1.5, bg)
	case "fliph":
		poly(c, [2]float64{-9, 0}, [2]float64{-2.5, -6.5}, [2]float64{-2.5, 6.5})
		stroke(build([][2]float64{{2.5, -6.5}, {9, 0}, {2.5, 6.5}}, true), 1.6, c)
		line(1.2, [2]float64{0, -10}, [2]float64{0, 10})
	case "flipv":
		poly(c, [2]float64{0, -9}, [2]float64{-6.5, -2.5}, [2]float64{6.5, -2.5})
		stroke(build([][2]float64{{-6.5, 2.5}, {0, 9}, {6.5, 2.5}}, true), 1.6, c)
		line(1.2, [2]float64{-10, 0}, [2]float64{10, 0})
	case "copy":
		stroke(build([][2]float64{{-2, -9}, {8, -9}, {8, 3}, {-2, 3}}, true), 1.8, c)
		poly(c, [2]float64{-8, -3}, [2]float64{2, -3}, [2]float64{2, 9}, [2]float64{-8, 9})
	case "cut":
		line(2.2, [2]float64{-6, -8}, [2]float64{5, 5})
		line(2.2, [2]float64{6, -8}, [2]float64{-5, 5})
		ring(-5, 7.5, 2.6, 1.8)
		ring(5, 7.5, 2.6, 1.8)
	case "paste":
		stroke(build([][2]float64{{-7, -6}, {7, -6}, {7, 9}, {-7, 9}}, true), 1.8, c)
		poly(c, [2]float64{-4, -9}, [2]float64{4, -9}, [2]float64{4, -4}, [2]float64{-4, -4})
		line(1.6, [2]float64{-3, 1}, [2]float64{3, 1})
		line(1.6, [2]float64{-3, 5}, [2]float64{3, 5})
	case "trash":
		line(2, [2]float64{-8, -5}, [2]float64{8, -5})
		line(2, [2]float64{-3, -8}, [2]float64{3, -8})
		poly(c, [2]float64{-6, -3}, [2]float64{6, -3}, [2]float64{5, 9}, [2]float64{-5, 9})
		rect(-1.5, 0, 1, 7, bg)
		rect(1, 0, 1, 7, bg)
	case "stamp":
		disc(0, -5.5, 3.6, c)
		rect(-1.6, -3, 3.2, 6, c)
		rect(-8, 3, 16, 3, c)
		rect(-6, 7, 12, 2, c)
	case "edge":
		line(2.4, [2]float64{-9, 9}, [2]float64{3, -3})
		poly(c, [2]float64{6, -10}, [2]float64{8, -6}, [2]float64{12, -4}, [2]float64{8, -2}, [2]float64{6, 2}, [2]float64{4, -2}, [2]float64{0, -4}, [2]float64{4, -6})
	case "ring":
		ring(0, 0, 8, 2)
	}
}

// ---- drawing -------------------------------------------------------------------------------------------------------

func (a *app) drawEditorButton(screen *ebiten.Image, button edButton, px, py float64) {
	shade := editorBtn
	on := button.active != nil && button.active()
	hover := button.contains(px, py)
	switch {
	case button.accent && hover:
		shade = color.RGBA{62, 172, 88, 255}
	case button.accent:
		shade = editorPlay
	case on && hover:
		shade = color.RGBA{62, 142, 80, 255}
	case on:
		shade = editorBtnOn
	case hover:
		shade = editorBtnHover
	}
	chip := strings.HasPrefix(button.icon, "chip")
	if chip && !on && !hover {
		shade = editorPanel
	}
	a.vRound(screen, button.x, button.y, button.w, button.h, 5, shade)
	if button.icon == "problems" && button.label != "" {
		badge := button.label
		defer func() { // the count of problems rides on the corner of its button
			a.vRound(screen, button.x+button.w-14, button.y-3, 20, 17, 8, color.RGBA{220, 70, 60, 255})
			a.vText(screen, badge, button.x+button.w-4-a.vTextWidth(badge, 12, true)/2, button.y-1, 12, color.RGBA{255, 255, 255, 255}, true)
		}()
		button.label = ""
	}
	iconColor := editorIcon
	if button.icon == "eye" && !on {
		iconColor = color.RGBA{88, 96, 116, 255}
	}
	switch {
	case chip:
		for _, layer := range editorLayers() {
			if layer.name == button.label {
				a.vRound(screen, button.x+8, button.y+button.h/2-6, 12, 12, 3, layer.swatch)
			}
		}
		clr := editorDim
		if on || hover {
			clr = editorText
		}
		a.vText(screen, button.label, button.x+28, button.y+button.h/2-8, 15, clr, on)
	case button.label != "" && button.icon != "":
		a.drawEditorIcon(screen, button.icon, button.x+20, button.y+button.h/2, iconColor, shade)
		a.vText(screen, button.label, button.x+38, button.y+button.h/2-9, 17, iconColor, true)
	case button.icon != "":
		a.drawEditorIcon(screen, button.icon, button.x+button.w/2, button.y+button.h/2, iconColor, shade)
	case button.label != "":
		size := 15.0
		a.vText(screen, button.label, button.x+(button.w-a.vTextWidth(button.label, size, false))/2, button.y+button.h/2-size*.62, size, iconColor, false)
	}
}

// editorCellText is what the layers hold at a cell, for the status bar.
func (e *levelEditor) editorCellText(x, y int) string {
	if x < 0 || y < 0 || x >= e.level.Width || y >= e.level.Height {
		return "Outside the map"
	}
	index := y*e.level.Width + x
	show := func(value uint32) string {
		if value == editorEmpty {
			return "-"
		}
		return fmt.Sprint(value & 0xffff)
	}
	collision := "open"
	if value := e.level.Layers[formats.LayerC][index]; value != editorEmpty {
		collision = fmt.Sprint(value)
		for _, item := range collisionPalette {
			if item.value == value {
				collision = strings.ToLower(item.label)
			}
		}
	}
	return fmt.Sprintf("Cell %d, %d      Ground %s      Decor %s      Tall %s      Behind %s      Collision %s", x, y,
		show(e.level.Layers[formats.LayerG][index]), show(e.level.Layers[formats.LayerD][index]), show(e.level.Layers[formats.LayerH][index]), show(e.level.Layers[formats.LayerHB][index]), collision)
}

func (a *app) drawEditor(screen *ebiten.Image) {
	e := a.editor
	if e.inList {
		a.drawEditorList(screen)
		return
	}
	px, py := a.editorPointer()
	buttons := a.editorButtons()
	tip := ""
	for _, button := range buttons {
		if button.contains(px, py) {
			tip = button.tip
		}
	}
	if e.wavePage {
		screen.Fill(editorBar)
		if waveTip := a.drawWaveEditor(screen, px, py); waveTip != "" && tip == "" {
			tip = waveTip
		}
	} else {
		a.drawEditorMap(screen, px, py)
	}

	// Toolbar and status bar (the panel is drawn with the map).
	a.vRect(screen, 0, 0, editorW, editorTopBar, editorBar)
	a.vRect(screen, 0, editorTopBar-1, editorW, 1, color.RGBA{255, 255, 255, 22})
	a.vRect(screen, 0, editorMapBottom, editorW, editorStatusBar, editorBar)
	for _, button := range buttons {
		if button.y < editorTopBar {
			a.drawEditorButton(screen, button, px, py)
		}
	}
	for _, button := range buttons {
		if button.icon == "minus" && button.y < editorTopBar { // the brush size sits between the toolbar's two brush buttons
			label := fmt.Sprint(e.brush)
			a.vText(screen, label, button.x+button.w+3+(26-a.vTextWidth(label, 17, true))/2, button.y+5, 17, editorText, true)
		}
	}
	status := fmt.Sprintf("%s      %d x %d%s      F1: help", e.level.Info.ID, e.level.Width, e.level.Height, map[bool]string{true: "      (unsaved)", false: ""}[e.dirty])
	overMap := !e.wavePage && px < editorPanelX && py >= editorTopBar && py < editorMapBottom
	worldX, worldY := e.mouseWorld(px, py)
	switch {
	case tip != "":
		status = tip
	case time.Now().Before(e.statusUntil):
		status = e.status
	case overMap:
		status = e.editorCellText(int(math.Floor(worldX/editorTile)), int(math.Floor(worldY/editorTile)))
	}
	a.vText(screen, status, 10, editorMapBottom+5, 15, editorText, false)
	if e.tilesetMenu {
		a.drawTilesetMenu(screen, px, py)
	}
	if e.popup != nil {
		if popupTip := a.drawPopup(screen, px, py); popupTip != "" {
			a.vRect(screen, 0, editorMapBottom, editorW, editorStatusBar, editorBar)
			a.vText(screen, popupTip, 10, editorMapBottom+5, 15, editorText, false)
		}
	}
	a.drawEditorDialogs(screen, px, py)
	if e.help {
		a.drawEditorHelp(screen)
	}
}

// drawAtlasTile draws one tile of the atlas (value carries the mirror flags) into an editor-unit square.
func (a *app) drawAtlasTile(screen *ebiten.Image, value uint32, x, y, size, alpha float64) {
	e := a.editor
	atlas := e.view.Atlas
	if atlas == nil {
		return
	}
	cols := e.paletteCols(atlas.Bounds().Dx())
	source := atlas.Bounds().Dx() / cols
	tx, ty := int(value&0xffff)%cols, int(value&0xffff)/cols
	if (ty+1)*source > atlas.Bounds().Dy() {
		return
	}
	sub := atlas.SubImage(image.Rect(tx*source, ty*source, (tx+1)*source, (ty+1)*source)).(*ebiten.Image)
	sx, sy := a.editorScale()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	scale := size * sx / float64(source)
	flipX, flipY := 1.0, 1.0
	offsetX, offsetY := 0.0, 0.0
	if value&0x10000 != 0 {
		flipX, offsetX = -1, size*sx
	}
	if value&0x20000 != 0 {
		flipY, offsetY = -1, size*sy
	}
	options.GeoM.Scale(scale*flipX, scale*flipY)
	options.GeoM.Translate(x*sx+offsetX, y*sy+offsetY)
	if alpha < 1 {
		options.ColorScale.ScaleAlpha(float32(alpha))
	}
	screen.DrawImage(sub, options)
}

// drawClipGhost shows a clipboard under the pointer, half see-through.
func (a *app) drawClipGhost(screen *ebiten.Image, clip *editorClip, tileX, tileY int, toScreen func(x, y float64) (float64, float64)) {
	e := a.editor
	size := editorTile * e.view.Zoom * 2
	for _, kind := range formats.LayerKinds {
		if !e.view.Layers[kind] {
			continue
		}
		for y := 0; y < clip.h; y++ {
			for x := 0; x < clip.w; x++ {
				value := clip.layers[kind][y*clip.w+x]
				if kind != formats.LayerG && (value == editorEmpty || (value == 0 && kind != formats.LayerC)) {
					continue
				}
				x0, y0 := toScreen(float64(tileX+x)*editorTile, float64(tileY+y)*editorTile)
				if kind == formats.LayerC {
					swatch := collisionSwatch(value)
					a.vRect(screen, x0, y0, size, size, color.RGBA{swatch.R, swatch.G, swatch.B, 110})
					continue
				}
				a.drawAtlasTile(screen, value, x0, y0, size, .65)
			}
		}
	}
	x0, y0 := toScreen(float64(tileX)*editorTile, float64(tileY)*editorTile)
	a.vOutline(screen, x0, y0, float64(clip.w)*size, float64(clip.h)*size, 2, color.RGBA{120, 220, 255, 255})
}

// drawAnts draws a dashed outline that crawls: the selection.
func (a *app) drawAnts(screen *ebiten.Image, x, y, w, h float64) {
	phase := float64(time.Now().UnixMilli()/90%12) - 12
	dash := func(x0, y0, dx, dy, length float64) {
		for at := phase; at < length; at += 12 {
			start, end := math.Max(0, at), math.Min(length, at+6)
			if end > start {
				if dx != 0 {
					a.vRect(screen, x0+start*dx, y0, (end-start)*math.Abs(dx), 2, color.RGBA{255, 255, 255, 255})
				} else {
					a.vRect(screen, x0, y0+start*dy, 2, (end-start)*math.Abs(dy), color.RGBA{255, 255, 255, 255})
				}
			}
		}
	}
	a.vRect(screen, x, y, w, h, color.RGBA{90, 160, 255, 40})
	a.vOutline(screen, x, y, w, h, 2, color.RGBA{0, 0, 0, 200})
	dash(x, y, 1, 0, w)
	dash(x, y+h-2, 1, 0, w)
	dash(x, y, 0, 1, h)
	dash(x+w-2, y, 0, 1, h)
}

// drawEditorMap draws the map, what is being drawn on it, the tool strip and the right panel.
func (a *app) drawEditorMap(screen *ebiten.Image, px, py float64) {
	e := a.editor
	v := e.view
	v.SetRenderScale(a.frontendScaleX, a.frontendScaleY)
	v.Draw(screen)
	worldX, worldY := e.mouseWorld(px, py)
	tileX, tileY := int(math.Floor(worldX/editorTile)), int(math.Floor(worldY/editorTile))
	overMap := px >= editorToolStrip && px < editorPanelX && py >= editorTopBar && py < editorMapBottom
	toScreen := func(x, y float64) (float64, float64) { // editor units
		return ((x-v.CameraX)*v.Zoom + v.ViewportX) * 2, ((y-v.CameraY)*v.Zoom + v.ViewportY) * 2
	}
	cell := editorTile * v.Zoom * 2
	if e.highlight && e.layer != formats.LayerC {
		cells := e.level.Layers[e.layer]
		for index, value := range cells {
			if value != editorEmpty && value&0xffff == e.tile {
				x0, y0 := toScreen(float64(index%e.level.Width)*editorTile, float64(index/e.level.Width)*editorTile)
				if x0 > -cell && y0 > -cell && x0 < editorW && y0 < editorH {
					a.vRect(screen, x0, y0, cell, cell, color.RGBA{255, 235, 60, 110})
					a.vOutline(screen, x0, y0, cell, cell, 2, color.RGBA{255, 235, 60, 255})
				}
			}
		}
	}
	if overMap && !e.propMode && e.tool == toolPaint && !e.pasting && !ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		a.drawGhost(screen, tileX, tileY, toScreen)
	}
	if overMap && !e.propMode && !e.pasting && e.tool != toolSelect && e.tool != toolPickup {
		outline := color.RGBA{255, 255, 255, 70}
		switch e.tool {
		case toolFill:
			outline = color.RGBA{90, 160, 255, 90}
		case toolPick:
			outline = color.RGBA{255, 230, 80, 100}
		}
		x0, y0 := toScreen(float64(tileX+(-(e.brush-1)/2))*editorTile, float64(tileY+(-(e.brush-1)/2))*editorTile)
		a.vRect(screen, x0, y0, float64(e.brush)*cell, float64(e.brush)*cell, outline)
	}
	if e.rectOn && !e.propMode {
		x0, y0 := toScreen(float64(min(e.rectX, tileX))*editorTile, float64(min(e.rectY, tileY))*editorTile)
		width, height := float64(absInt(tileX-e.rectX)+1)*cell, float64(absInt(tileY-e.rectY)+1)*cell
		clr := color.RGBA{255, 230, 80, 255}
		if e.rectErase {
			clr = color.RGBA{255, 110, 100, 255}
		}
		a.vRect(screen, x0, y0, width, height, color.RGBA{clr.R, clr.G, clr.B, 50})
		a.vOutline(screen, x0, y0, width, height, 2, clr)
	}
	if e.lineOn && !e.propMode {
		clr := color.RGBA{255, 230, 80, 90}
		if e.lineErase {
			clr = color.RGBA{255, 110, 100, 90}
		}
		for _, c := range lineCells(e.lineX, e.lineY, tileX, tileY) {
			x0, y0 := toScreen(float64(c[0])*editorTile, float64(c[1])*editorTile)
			a.vRect(screen, x0, y0, cell, cell, clr)
		}
	}
	if e.selOn || e.selDrag {
		x0, y0, x1, y1 := e.selRect()
		sx0, sy0 := toScreen(float64(x0)*editorTile, float64(y0)*editorTile)
		a.drawAnts(screen, sx0, sy0, float64(x1-x0+1)*cell, float64(y1-y0+1)*cell)
	}
	if e.pasting && e.clip != nil && overMap {
		a.drawClipGhost(screen, e.clip, tileX, tileY, toScreen)
	}
	a.drawPickupsOnMap(screen, toScreen)
	if e.propMode {
		for index := 0; index < e.propCount(); index++ {
			x, y := e.propPosition(index)
			sx, sy := toScreen(x, y)
			shade := color.RGBA{255, 220, 80, 180}
			if index == e.selected {
				shade = color.RGBA{80, 255, 120, 250}
				a.vRect(screen, sx-6, sy-6, 12, 12, color.RGBA{0, 0, 0, 170})
			}
			a.vRect(screen, sx-4, sy-4, 8, 8, shade)
		}
	}
	a.drawMinimap(screen, px, py)

	// The tool strip and the right panel.
	panelW := editorW - editorPanelX
	a.vRect(screen, 0, editorTopBar, editorToolStrip, editorMapBottom-editorTopBar, editorPanel)
	a.vRect(screen, editorPanelX, editorTopBar, panelW, editorMapBottom-editorTopBar, editorPanel)
	for _, button := range a.editorButtons() {
		if button.y >= editorTopBar && button.label != "tilefield" && button.label != "recent" {
			a.drawEditorButton(screen, button, px, py)
		}
	}
	a.vText(screen, "Layers", editorPanelX+8, editorTopBar+6, 14, editorDim, true)
	y := editorPaletteTop
	switch {
	case e.propMode:
		a.vText(screen, "Props", editorPanelX+8, y-20, 14, editorDim, true)
		label := "No props for this tileset"
		if len(e.protos) > 0 {
			label = fmt.Sprintf("%d of %d: %s", e.protoIndex%len(e.protos)+1, len(e.protos), e.protos[e.protoIndex%len(e.protos)].label)
		}
		for index, line := range a.vWrap(label, panelW-20, 15) {
			if index < 2 {
				a.vText(screen, line, editorPanelX+8, y+float64(index)*18, 15, editorText, false)
			}
		}
		y += 44 + 108
		if e.selected >= 0 && e.selected < e.propCount() {
			x, py2 := e.propPosition(e.selected)
			a.vText(screen, fmt.Sprintf("Selected at %d, %d", int(x), int(py2)), editorPanelX+8, y, 14, editorText, false)
		} else {
			for _, line := range a.vWrap("Click the map to place the prop shown above, or click an existing prop to move it.", panelW-20, 14) {
				a.vText(screen, line, editorPanelX+8, y, 14, editorDim, false)
				y += 17
			}
		}
	case e.tool == toolPickup && !e.propMode:
		a.drawPickupPanel(screen, px, py)
	case e.layer == formats.LayerC:
		a.vText(screen, "Collision and spawns", editorPanelX+8, y-20, 14, editorDim, true)
		for index, item := range collisionPalette {
			row := y + float64(index)*24
			if item.value == e.collision {
				a.vRound(screen, editorPanelX+4, row, panelW-8, 23, 4, editorBtnOn)
			}
			a.vRound(screen, editorPanelX+12, row+5, 13, 13, 3, collisionSwatch(item.value))
			a.vText(screen, item.label, editorPanelX+34, row+3, 15, editorText, item.value == e.collision)
		}
		y += float64(len(collisionPalette))*24 + 8
	case v.Atlas != nil:
		a.vText(screen, "Tiles: click one to paint with it", editorPanelX+8, y-20, 14, editorDim, true)
		cols := e.paletteCols(v.Atlas.Bounds().Dx())
		cellSize := panelW / float64(cols)
		scale := panelW / float64(v.Atlas.Bounds().Dx())
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		sx, sy := a.editorScale()
		options.GeoM.Scale(scale*sx, scale*sy)
		options.GeoM.Translate(editorPanelX*sx, y*sy)
		screen.DrawImage(v.Atlas, options)
		cx := editorPanelX + float64(int(e.tile)%cols)*cellSize
		cy := y + float64(int(e.tile)/cols)*cellSize
		a.vOutline(screen, cx, cy, cellSize, cellSize, 2, color.RGBA{255, 255, 80, 255})
		if infoY, recentY, _, ok := a.editorPaletteGeometry(); ok {
			const previewSize = 46.0
			a.vRect(screen, editorPanelX+6, infoY, previewSize+4, previewSize+4, color.RGBA{0, 0, 0, 255})
			a.drawAtlasTile(screen, e.paintValue(), editorPanelX+8, infoY+2, previewSize, 1)
			title := fmt.Sprintf("Tile %d", e.tile)
			if e.field == "tile" {
				title = "Tile " + e.input + "|"
			}
			a.vText(screen, title, editorPanelX+66, infoY+3, 17, editorText, true)
			flips := ""
			if e.flipX {
				flips += ", mirrored"
			}
			if e.flipY {
				flips += ", upside down"
			}
			a.vText(screen, fmt.Sprintf("Brush %d x %d%s", e.brush, e.brush, flips), editorPanelX+66, infoY+27, 14, editorDim, false)
			a.vText(screen, "Recent", editorPanelX+8, recentY-14, 12, editorDim, true)
			for index, value := range e.recent {
				cellX := editorPanelX + 8 + float64(index)*32
				on := e.tile == value&0xffff && e.flipX == (value&0x10000 != 0) && e.flipY == (value&0x20000 != 0)
				bg := color.RGBA{0, 0, 0, 255}
				if on {
					bg = color.RGBA{255, 255, 80, 255}
				}
				a.vRect(screen, cellX-1, recentY-1, 30, 30, bg)
				a.drawAtlasTile(screen, value, cellX, recentY, 28, 1)
			}
		}
	}
	for _, button := range a.editorButtons() {
		if button.label == "Find this tile on the map" {
			_ = button
		}
	}
}

func (a *app) drawEditorHelp(screen *ebiten.Image) {
	a.vRect(screen, 0, editorTopBar, editorW, editorMapBottom-editorTopBar, color.RGBA{0, 0, 0, 185})
	a.vRound(screen, 40, editorTopBar+16, editorW-80, editorMapBottom-editorTopBar-32, 10, color.RGBA{24, 29, 41, 252})
	rows := [][2]string{
		{"Mouse", ""},
		{"Left button", "Paint with the chosen tile (drag to paint many)"},
		{"Right button", "Erase"},
		{"Middle button drag", "Move the map          Wheel: zoom"},
		{"Keyboard", ""},
		{"W A S D or arrows", "Move the map"},
		{"1  2  3  4  5", "Choose a layer: ground, decor, tall, behind, collision"},
		{"I   F   P   E", "Pick a tile, fill an area, edit props, edit enemy waves"},
		{"X   V", "Mirror the tile, flip it upside down"},
		{"Bracket keys", "The two keys next to P: brush smaller, brush bigger"},
		{"Ctrl+Z  Ctrl+Y", "Undo and redo"},
		{"Ctrl+Arrows", "Make the map bigger or smaller (add Shift for 8 at a time)"},
		{"Ctrl+S  Ctrl+O", "Save as a file, open a file"},
		{"T", "Test play the level. F4 brings you back to the editor"},
		{"Esc", "Back to the list of levels"},
		{"In the cache", ""},
		{"Ctrl+Shift+S", "Save over this level in the game's own files (the original is kept)"},
	}
	y := editorTopBar + 34
	for _, row := range rows {
		if row[1] == "" {
			y += 6
			a.vText(screen, row[0], 70, y, 19, editorPlay, true)
			y += 30
			continue
		}
		a.vText(screen, row[0], 70, y, 16, editorText, true)
		a.vText(screen, row[1], 300, y, 16, editorDim, false)
		y += 26
	}
	a.vText(screen, "Click anywhere or press F1 to close this", 70, editorMapBottom-52, 15, editorText, false)
}

// ---- list ----------------------------------------------------------------------------------------------------------

// editorLevels are the levels the list offers: the game's own, without the temporary test copy.
func (a *app) editorLevels() []formats.LevelInfo {
	levels := make([]formats.LevelInfo, 0, len(a.levels))
	for _, info := range a.levels {
		if info.ID != editorTestID {
			levels = append(levels, info)
		}
	}
	return levels
}

const (
	editorListRowHeight = 22.0
	editorListFirstRow  = 96.0
	editorListVisible   = 21
)

// editorListButtons are the buttons under the list.
func (a *app) editorListButtons() []edButton {
	e := a.editor
	levels := a.editorLevels()
	const y, h = 580.0, 36.0
	return []edButton{
		{label: "Edit", icon: "paint", tip: "Edit the selected level (Enter, or click a level)", x: 16, y: y, w: 130, h: h, accent: true, do: func() {
			if len(levels) > 0 {
				a.editorLoad(levels[e.listRow].ID)
			}
		}},
		{label: "New level", icon: "plus", tip: "A new empty level that takes the tileset and waves of the selected one (N)", x: 154, y: y, w: 160, h: h, do: func() {
			if len(levels) > 0 {
				if template, err := a.pack.Load(levels[e.listRow].ID); err == nil {
					a.openNewDialog(template)
				}
			}
		}},
		{label: "Open file", icon: "open", tip: "Open a level file you saved before (L)", x: 322, y: y, w: 160, h: h, do: a.editorOpenFile},
		{label: "Close", icon: "close", tip: "Close the editor (Esc or F4)", x: 490, y: y, w: 130, h: h, do: func() { a.toggleEditor() }},
	}
}

func (a *app) updateEditorList() error {
	e := a.editor
	a.editorPoll()
	a.editorApplyArms()
	levels := a.editorLevels()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) && !e.newDialog {
		a.toggleEditor()
		return nil
	}
	if len(levels) > 0 && !e.newDialog {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
			e.listRow = (e.listRow + 1) % len(levels)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
			e.listRow = (e.listRow + len(levels) - 1) % len(levels)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageDown) {
			e.listRow = min(len(levels)-1, e.listRow+10)
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyPageUp) {
			e.listRow = max(0, e.listRow-10)
		}
		if _, wheel := ebiten.Wheel(); wheel != 0 {
			e.listTop = max(0, min(max(0, len(levels)-editorListVisible), e.listTop-wheelStep(wheel, 3)))
		}
	}
	if e.listRow < e.listTop {
		e.listTop = e.listRow
	}
	if e.listRow >= e.listTop+editorListVisible {
		e.listTop = e.listRow - editorListVisible + 1
	}
	px, py := a.editorPointer()
	if e.newDialog {
		a.updateNewDialog(px, py)
		return nil
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for _, button := range a.editorListButtons() {
			if button.contains(px, py) {
				button.do()
				return nil
			}
		}
		if py >= editorListFirstRow {
			// One click on a level opens it.
			if row := e.listTop + int((py-editorListFirstRow)/editorListRowHeight); row < len(levels) && row < e.listTop+editorListVisible {
				e.listRow = row
				a.editorLoad(levels[row].ID)
				return nil
			}
		}
	}
	if len(levels) > 0 && (inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)) {
		a.editorLoad(levels[e.listRow].ID)
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyN) {
		for _, button := range a.editorListButtons() {
			if button.label == "New level" {
				button.do()
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyL) {
		a.editorOpenFile()
	}
	return nil
}

func (a *app) drawEditorList(screen *ebiten.Image) {
	e := a.editor
	screen.Fill(editorBar)
	px, py := a.editorPointer()
	a.vText(screen, "Level editor", 16, 12, 36, editorText, true)
	a.vText(screen, "Click a level to edit it, or start a new one with the buttons below.", 16, 62, 17, editorDim, false)
	levels := a.editorLevels()
	for row := e.listTop; row < len(levels) && row < e.listTop+editorListVisible; row++ {
		info := levels[row]
		y := editorListFirstRow + float64(row-e.listTop)*editorListRowHeight
		hover := px >= 12 && px < editorW-12 && py >= y && py < y+editorListRowHeight
		switch {
		case hover:
			a.vRound(screen, 12, y, editorW-24, editorListRowHeight-1, 4, editorBtnHover)
		case row == e.listRow:
			a.vRound(screen, 12, y, editorW-24, editorListRowHeight-1, 4, editorBtnOn)
		}
		kind, chip := "Story", color.RGBA{110, 190, 230, 255}
		for _, flag := range info.Flags {
			if flag == "SURVIVAL" {
				kind, chip = "Survival", color.RGBA{230, 150, 80, 255}
			}
		}
		a.vRound(screen, 20, y+6, 10, 10, 3, chip)
		a.vText(screen, info.ID, 40, y+2, 15, editorText, true)
		a.vText(screen, info.DisplayName, 250, y+2, 15, editorText, false)
		a.vText(screen, fmt.Sprintf("World %d    %s", info.WorldIndex, kind), 640, y+2, 15, editorDim, false)
	}
	if len(levels) > editorListVisible {
		a.vText(screen, "More below: mouse wheel or Page Down", 620, 66, 14, editorDim, false)
	}
	tip := ""
	for _, button := range a.editorListButtons() {
		a.drawEditorButton(screen, button, px, py)
		if button.contains(px, py) {
			tip = button.tip
		}
	}
	if time.Now().Before(e.statusUntil) {
		tip = e.status
	}
	a.vText(screen, tip, 16, 548, 15, editorText, false)
	if e.newDialog {
		a.drawNewDialog(screen, px, py)
	}
}

// drawGhost shows what a click would paint: the chosen tile, half see-through, under the brush.
func (a *app) drawGhost(screen *ebiten.Image, tileX, tileY int, toScreen func(x, y float64) (float64, float64)) {
	e := a.editor
	v := e.view
	sx, sy := a.editorScale()
	size := editorTile * v.Zoom * 2
	from := -(e.brush - 1) / 2
	for dy := 0; dy < e.brush; dy++ {
		for dx := 0; dx < e.brush; dx++ {
			cellX, cellY := tileX+from+dx, tileY+from+dy
			if cellX < 0 || cellY < 0 || cellX >= e.level.Width || cellY >= e.level.Height {
				continue
			}
			x0, y0 := toScreen(float64(cellX)*editorTile, float64(cellY)*editorTile)
			if e.layer == formats.LayerC {
				swatch := collisionSwatch(e.collision)
				a.vRect(screen, x0, y0, size, size, color.RGBA{swatch.R, swatch.G, swatch.B, 140})
				continue
			}
			if v.Atlas == nil {
				continue
			}
			cols := e.paletteCols(v.Atlas.Bounds().Dx())
			source := v.Atlas.Bounds().Dx() / cols
			tx, ty := int(e.tile)%cols, int(e.tile)/cols
			sub := v.Atlas.SubImage(image.Rect(tx*source, ty*source, (tx+1)*source, (ty+1)*source)).(*ebiten.Image)
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			scale := size * sx / float64(source)
			flipX, flipY := 1.0, 1.0
			offsetX, offsetY := 0.0, 0.0
			if e.flipX {
				flipX, offsetX = -1, size*sx
			}
			if e.flipY {
				flipY, offsetY = -1, size*sy
			}
			options.GeoM.Scale(scale*flipX, scale*flipY)
			options.GeoM.Translate(x0*sx+offsetX, y0*sy+offsetY)
			options.ColorScale.ScaleAlpha(.7)
			screen.DrawImage(sub, options)
		}
	}
}
