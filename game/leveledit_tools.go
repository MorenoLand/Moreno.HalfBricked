package game

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The minimap and the stamp library.

// ---- minimap -------------------------------------------------------------------------------------------------------

func (a *app) minimapRect() (x, y, w, h float64) {
	e := a.editor
	scale := math.Min(210/float64(e.level.Width), 150/float64(e.level.Height))
	w, h = float64(e.level.Width)*scale, float64(e.level.Height)*scale
	return editorPanelX - w - 12, editorMapBottom - h - 12, w, h
}

// tileColorTable is the average colour of each tile of the atlas (what the minimap paints a cell with).
func (a *app) tileColorTable() []color.RGBA {
	e := a.editor
	atlas := e.view.Atlas
	if atlas == nil {
		return nil
	}
	if e.tileColorsFor == e.level.Tileset && e.tileColors != nil {
		return e.tileColors
	}
	defer func() { _ = recover() }()
	bounds := atlas.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	atlas.ReadPixels(pixels)
	cols := e.paletteCols(bounds.Dx())
	size := bounds.Dx() / cols
	rows := bounds.Dy() / max(1, size)
	table := make([]color.RGBA, cols*rows)
	for tile := range table {
		tx, ty := (tile%cols)*size, (tile/cols)*size
		var r, g, b, count int
		for y := ty; y < ty+size; y += 2 {
			for x := tx; x < tx+size; x += 2 {
				at := 4 * (y*bounds.Dx() + x)
				if pixels[at+3] > 40 {
					r, g, b, count = r+int(pixels[at]), g+int(pixels[at+1]), b+int(pixels[at+2]), count+1
				}
			}
		}
		if count > 0 {
			table[tile] = color.RGBA{uint8(r / count), uint8(g / count), uint8(b / count), 255}
		} else {
			table[tile] = color.RGBA{20, 20, 28, 255}
		}
	}
	e.tileColors, e.tileColorsFor = table, e.level.Tileset
	return table
}

func (a *app) rebuildMinimap() {
	e := a.editor
	table := a.tileColorTable()
	w, h := e.level.Width, e.level.Height
	if e.miniImage == nil || e.miniImage.Bounds().Dx() != w || e.miniImage.Bounds().Dy() != h {
		e.miniImage = ebiten.NewImage(w, h)
	}
	pix := make([]byte, 4*w*h)
	lookup := func(value uint32) (color.RGBA, bool) {
		if value == editorEmpty || int(value&0xffff) >= len(table) {
			return color.RGBA{}, false
		}
		return table[value&0xffff], true
	}
	blend := func(base, over color.RGBA, alpha float64) color.RGBA {
		return color.RGBA{uint8(float64(base.R)*(1-alpha) + float64(over.R)*alpha), uint8(float64(base.G)*(1-alpha) + float64(over.G)*alpha), uint8(float64(base.B)*(1-alpha) + float64(over.B)*alpha), 255}
	}
	for index := 0; index < w*h; index++ {
		clr, _ := lookup(e.level.Layers[formats.LayerG][index])
		if clr.A == 0 {
			clr = color.RGBA{30, 30, 40, 255}
		}
		if over, ok := lookup(e.level.Layers[formats.LayerD][index]); ok && e.level.Layers[formats.LayerD][index] != 0 {
			clr = blend(clr, over, .5)
		}
		if over, ok := lookup(e.level.Layers[formats.LayerHB][index]); ok && e.level.Layers[formats.LayerHB][index] != 0 {
			clr = blend(clr, over, .7)
		}
		if over, ok := lookup(e.level.Layers[formats.LayerH][index]); ok {
			clr = blend(clr, over, .8)
		}
		switch value := e.level.Layers[formats.LayerC][index]; {
		case value == 0 || value == 1:
			clr = color.RGBA{clr.R / 2, clr.G / 2, clr.B / 2, 255}
		case value == 2:
			clr = color.RGBA{80, 255, 100, 255}
		case value != editorEmpty:
			clr = blend(clr, collisionSwatch(value), .7)
		}
		pix[4*index], pix[4*index+1], pix[4*index+2], pix[4*index+3] = clr.R, clr.G, clr.B, 255
	}
	e.miniImage.WritePixels(pix)
}

// drawMinimap draws the whole level small, with the part in view outlined.
func (a *app) drawMinimap(screen *ebiten.Image, px, py float64) {
	e := a.editor
	if !e.minimap || e.view == nil {
		return
	}
	e.miniFrame++
	if e.miniImage == nil || e.miniFrame%20 == 1 {
		a.rebuildMinimap()
	}
	x, y, w, h := a.minimapRect()
	a.vRect(screen, x-3, y-3, w+6, h+6, color.RGBA{0, 0, 0, 200})
	sx, sy := a.editorScale()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(w*sx/float64(e.miniImage.Bounds().Dx()), h*sy/float64(e.miniImage.Bounds().Dy()))
	options.GeoM.Translate(x*sx, y*sy)
	screen.DrawImage(e.miniImage, options)
	v := e.view
	viewW := (editorPanelX - editorToolStrip) / 2 / v.Zoom / editorTile
	viewH := (editorMapBottom - editorTopBar) / 2 / v.Zoom / editorTile
	cellW, cellH := w/float64(e.level.Width), h/float64(e.level.Height)
	rx, ry := x+v.CameraX/editorTile*cellW, y+v.CameraY/editorTile*cellH
	rw, rh := math.Min(viewW*cellW, w), math.Min(viewH*cellH, h)
	a.vOutline(screen, math.Max(x, rx), math.Max(y, ry), math.Min(rw, x+w-math.Max(x, rx)), math.Min(rh, y+h-math.Max(y, ry)), 2, color.RGBA{255, 255, 255, 255})
}

func (a *app) editorMinimapHit(px, py float64) bool {
	e := a.editor
	if !e.minimap || e.view == nil {
		return false
	}
	x, y, w, h := a.minimapRect()
	return px >= x-3 && px < x+w+3 && py >= y-3 && py < y+h+3
}

// editorMinimapClick moves the view so the clicked place is in the middle.
func (a *app) editorMinimapClick(px, py float64) bool {
	e := a.editor
	if !a.editorMinimapHit(px, py) {
		return false
	}
	x, y, w, h := a.minimapRect()
	wx := math.Max(0, math.Min(1, (px-x)/w)) * float64(e.level.Width) * editorTile
	wy := math.Max(0, math.Min(1, (py-y)/h)) * float64(e.level.Height) * editorTile
	v := e.view
	v.CameraX = wx - (editorPanelX-editorToolStrip)/4/v.Zoom
	v.CameraY = wy - (editorMapBottom-editorTopBar)/4/v.Zoom
	return true
}

// ---- stamps --------------------------------------------------------------------------------------------------------

type stampFile struct {
	Name    string                         `json:"name"`
	Tileset string                         `json:"tileset"`
	W       int                            `json:"w"`
	H       int                            `json:"h"`
	Layers  map[formats.LayerKind][]uint32 `json:"layers"`
}

var stampUnsafe = regexp.MustCompile(`[^A-Za-z0-9 _-]+`)

func stampKey(name string) string {
	return "stamps/" + strings.ReplaceAll(strings.TrimSpace(stampUnsafe.ReplaceAllString(name, "")), " ", "_") + ".json"
}

func (a *app) stampList() []stampFile {
	var stamps []stampFile
	for _, name := range platform.ConfigList("stamps") {
		data, ok := platform.ConfigRead(name)
		if !ok {
			continue
		}
		var stamp stampFile
		if json.Unmarshal(data, &stamp) == nil && stamp.W > 0 && stamp.H > 0 {
			stamps = append(stamps, stamp)
		}
	}
	sort.Slice(stamps, func(i, j int) bool { return strings.ToLower(stamps[i].Name) < strings.ToLower(stamps[j].Name) })
	return stamps
}

// editorStampSave starts saving the selection in the stamp library: it asks for a name first.
func (a *app) editorStampSave() {
	e := a.editor
	if !e.selOn {
		e.say("Select a region first (the select tool), then save it as a stamp")
		return
	}
	e.copySelection()
	e.stampNaming = true
	e.stampName = fmt.Sprintf("Stamp %d", len(a.stampList())+1)
}

func (a *app) saveStamp(name string) {
	e := a.editor
	if e.clip == nil {
		return
	}
	stamp := stampFile{Name: name, Tileset: e.level.Tileset, W: e.clip.w, H: e.clip.h, Layers: e.clip.layers}
	data, err := json.Marshal(stamp)
	if err != nil || !platform.ConfigWrite(stampKey(name), data) {
		e.say("The stamp could not be saved")
		return
	}
	e.say(fmt.Sprintf("Saved the stamp \"%s\" (%d x %d). Find it in the stamp library.", name, stamp.W, stamp.H))
}

func (a *app) updateStampNaming() {
	e := a.editor
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && r != 127 && utf8.RuneCountInString(e.stampName) < 30 {
			e.stampName += string(r)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && e.stampName != "" {
		runes := []rune(e.stampName)
		e.stampName = string(runes[:len(runes)-1])
	}
	switch {
	case inpututil.IsKeyJustPressed(ebiten.KeyEscape):
		e.stampNaming = false
	case inpututil.IsKeyJustPressed(ebiten.KeyEnter), inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter):
		if strings.TrimSpace(stampUnsafe.ReplaceAllString(e.stampName, "")) != "" {
			a.saveStamp(strings.TrimSpace(e.stampName))
		}
		e.stampNaming = false
	}
}

func (a *app) drawStampNaming(screen *ebiten.Image) {
	e := a.editor
	a.vRect(screen, 0, 0, editorW, editorH, color.RGBA{0, 0, 0, 170})
	x, y, w, h := 280.0, 230.0, 400.0, 150.0
	a.vRound(screen, x-1, y-1, w+2, h+2, 11, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 10, color.RGBA{24, 29, 41, 255})
	a.vText(screen, "Name this stamp", x+20, y+16, 20, editorText, true)
	a.vRound(screen, x+20, y+60, w-40, 32, 5, color.RGBA{20, 26, 40, 255})
	a.vOutline(screen, x+20, y+60, w-40, 32, 1.5, editorPlay)
	a.vText(screen, e.stampName+"|", x+30, y+66, 17, editorText, false)
	a.vText(screen, "Enter saves it, Esc cancels. Stamps are kept for every level.", x+20, y+108, 13, editorDim, false)
}

// openStamps lists the saved stamps; a click picks one up to paste, Shift+click deletes it.
func (a *app) openStamps(x float64) {
	e := a.editor
	p := a.openPopup("Stamps", x, 380, nil)
	p.build = func() []edMenuItem {
		items := []edMenuItem{{label: "Click to place a stamp, Shift+click to delete it", header: true}}
		stamps := a.stampList()
		if len(stamps) == 0 {
			return append(items, edMenuItem{label: "No stamps yet: select a region, then Ctrl+Shift+C", disabled: true})
		}
		for _, stamp := range stamps {
			st := stamp
			items = append(items, edMenuItem{label: st.Name, sub: fmt.Sprintf("%d x %d", st.W, st.H), keepOpen: true, tip: "Place this stamp (Shift+click deletes it)", do: func() {
				if shiftHeld() {
					platform.ConfigDelete(stampKey(st.Name))
					return
				}
				e.clip = &editorClip{w: st.W, h: st.H, layers: st.Layers}
				e.pasting, e.propMode, e.popup = true, false, nil
				a.editorSetToolKeep()
				e.say("Click to place \"" + st.Name + "\". X and V flip it, Esc cancels.")
			}})
		}
		return items
	}
}

// editorSetToolKeep leaves paste mode on while making sure no other tool action is half done.
func (a *app) editorSetToolKeep() {
	e := a.editor
	e.lineOn, e.rectOn, e.selDrag = false, false, false
	e.wavePage = false
}
