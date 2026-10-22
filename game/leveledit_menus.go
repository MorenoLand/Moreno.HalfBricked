package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sort"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The editor's two pop-up menus: the tileset picker (a list with a preview of each tileset) and the New level dialog (a size
// and a tileset).

var tilesetNames = map[string]string{
	"1930_tileset": "1930s", "egyptbase": "Egypt", "japanbase": "Japan", "lab": "Laboratory",
	"oldwest": "Old West", "prehis_1": "Prehistoric", "scifi1": "Sci-fi",
}

// editorTilesets are the tilesets the editor offers (the HD ones; the 3GS twins share their art).
func (a *app) editorTilesets() []string {
	var names []string
	for name, tileset := range a.pack.Manifest().TileSets {
		if !strings.HasSuffix(name, "_3gs") && tileset.TileSize == 64 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func prettyTileset(name string) string {
	if pretty, ok := tilesetNames[strings.ToLower(name)]; ok {
		return pretty
	}
	return name
}

// tilesetThumb is a preview of a tileset: the top left 4 x 4 tiles of its atlas.
func (a *app) tilesetThumb(name string) *ebiten.Image {
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(name)]
	if !ok {
		return nil
	}
	atlas, err := a.Texture(tileset.Texture)
	if err != nil || atlas == nil {
		return nil
	}
	cell := tileset.TileSize
	if cell <= 0 {
		cell = 64
	}
	bounds := atlas.Bounds()
	return atlas.SubImage(image.Rect(bounds.Min.X, bounds.Min.Y, bounds.Min.X+min(bounds.Dx(), cell*4), bounds.Min.Y+min(bounds.Dy(), cell*4))).(*ebiten.Image)
}

// drawThumb draws a tileset preview into an editor-unit square.
func (a *app) drawThumb(screen *ebiten.Image, thumb *ebiten.Image, x, y, size float64) {
	a.vRect(screen, x-1, y-1, size+2, size+2, color.RGBA{0, 0, 0, 255})
	if thumb == nil {
		return
	}
	sx, sy := a.editorScale()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Scale(size*sx/float64(thumb.Bounds().Dx()), size*sy/float64(thumb.Bounds().Dy()))
	options.GeoM.Translate(x*sx, y*sy)
	screen.DrawImage(thumb, options)
}

// editorSetTileset gives the level another tileset (the tile numbers stay, so the art changes under them).
func (a *app) editorSetTileset(name string) {
	e := a.editor
	if strings.EqualFold(name, e.level.Tileset) {
		return
	}
	previous := e.level.Tileset
	e.push()
	e.level.Tileset = name
	tileset, atlas, err := a.editorTileset(e.level)
	if err != nil {
		e.level.Tileset = previous
		e.say(err.Error())
		return
	}
	e.tileset, e.view.TileSet, e.view.Atlas = tileset, tileset, atlas
	e.protos = nil
	e.say("Tileset: " + prettyTileset(name))
}

// ---- the tileset menu ----------------------------------------------------------------------------------------------

const (
	tilesetRowH  = 54.0
	tilesetMenuW = 300.0
)

// tilesetMenuRect is where the menu sits: under the toolbar button.
func (a *app) tilesetMenuRect() (x, y, w, h float64) {
	rows := float64(len(a.editorTilesets()))
	x = 300
	for _, button := range a.editorButtons() {
		if button.icon == "tileset" {
			x = button.x
		}
	}
	x = math.Min(x, editorW-tilesetMenuW-8)
	return x, editorTopBar + 2, tilesetMenuW, 34 + rows*tilesetRowH + 6
}

func (a *app) updateTilesetMenu(px, py float64) {
	e := a.editor
	x, y, w, h := a.tilesetMenuRect()
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.tilesetMenu = false
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	if px >= x && px < x+w && py >= y+30 && py < y+h {
		index := int((py - y - 30) / tilesetRowH)
		if names := a.editorTilesets(); index >= 0 && index < len(names) {
			a.editorSetTileset(names[index])
		}
	}
	// A click on a row picks it; a click anywhere else just closes the menu.
	e.tilesetMenu = false
}

func (a *app) drawTilesetMenu(screen *ebiten.Image, px, py float64) {
	e := a.editor
	x, y, w, h := a.tilesetMenuRect()
	a.vRound(screen, x-1, y-1, w+2, h+2, 9, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 8, color.RGBA{24, 29, 41, 252})
	a.vText(screen, "Tileset: the art the tile numbers point at", x+12, y+8, 14, editorDim, true)
	for index, name := range a.editorTilesets() {
		rowY := y + 30 + float64(index)*tilesetRowH
		current := strings.EqualFold(name, e.level.Tileset)
		hover := px >= x && px < x+w && py >= rowY && py < rowY+tilesetRowH
		switch {
		case current:
			a.vRound(screen, x+4, rowY, w-8, tilesetRowH-4, 6, editorBtnOn)
		case hover:
			a.vRound(screen, x+4, rowY, w-8, tilesetRowH-4, 6, editorBtnHover)
		}
		a.drawThumb(screen, a.tilesetThumb(name), x+12, rowY+3, tilesetRowH-10)
		a.vText(screen, prettyTileset(name), x+78, rowY+8, 17, editorText, true)
		sub := name
		if current {
			sub = "In use now"
		}
		a.vText(screen, sub, x+78, rowY+29, 13, editorDim, false)
	}
}

// ---- the New level dialog ------------------------------------------------------------------------------------------

var newLevelPresets = []struct {
	label         string
	width, height int
}{{"Small", 30, 20}, {"Medium", 40, 30}, {"Large", 60, 40}, {"Huge", 80, 60}}

// openNewDialog starts the dialog for a level that takes the waves (and the default tileset) of template.
func (a *app) openNewDialog(template formats.Level) {
	e := a.editor
	e.newDialog = true
	e.newTemplate = template
	e.newW, e.newH = 40, 30
	e.newTileset = template.Tileset
}

func (a *app) newDialogRect() (x, y, w, h float64) { return 150, 90, 660, 440 }

// newDialogButtons are the dialog's controls; drawing and clicking share them.
func (a *app) newDialogButtons() []edButton {
	e := a.editor
	x, y, w, h := a.newDialogRect()
	var buttons []edButton
	left := x + 24
	for index, preset := range newLevelPresets {
		p := preset
		buttons = append(buttons, edButton{label: fmt.Sprintf("%s  %d x %d", p.label, p.width, p.height), tip: "A ready-made map size", x: left, y: y + 86 + float64(index)*40, w: 250, h: 34,
			active: func() bool { return e.newW == p.width && e.newH == p.height },
			do:     func() { e.newW, e.newH = p.width, p.height }})
	}
	stepper := func(label string, row float64, get func() int, set func(int)) {
		buttons = append(buttons,
			edButton{icon: "minus", tip: label + " smaller (Shift: 10 at a time)", x: left + 130, y: y + 268 + row, w: 34, h: 30, do: func() {
				step := 1
				if shiftHeld() {
					step = 10
				}
				set(get() - step)
			}},
			edButton{icon: "plus", tip: label + " bigger (Shift: 10 at a time)", x: left + 216, y: y + 268 + row, w: 34, h: 30, do: func() {
				step := 1
				if shiftHeld() {
					step = 10
				}
				set(get() + step)
			}})
	}
	clamp := func(v int) int { return max(8, min(200, v)) }
	stepper("Width", 0, func() int { return e.newW }, func(v int) { e.newW = clamp(v) })
	stepper("Height", 40, func() int { return e.newH }, func(v int) { e.newH = clamp(v) })
	buttons = append(buttons,
		edButton{label: "Create", icon: "play", tip: "Make the level", x: x + w - 270, y: y + h - 56, w: 120, h: 38, accent: true, do: func() {
			e.newDialog = false
			a.editorNew(e.newTemplate, e.newW, e.newH, e.newTileset)
		}},
		edButton{label: "Cancel", tip: "Back to the list", x: x + w - 142, y: y + h - 56, w: 120, h: 38, do: func() { e.newDialog = false }})
	for index, name := range a.editorTilesets() {
		n := name
		buttons = append(buttons, edButton{tip: "Use the " + prettyTileset(n) + " tileset", x: x + 300 + float64(index%2)*170, y: y + 86 + float64(index/2)*70, w: 160, h: 64,
			active: func() bool { return strings.EqualFold(e.newTileset, n) }, do: func() { e.newTileset = n }, label: "tileset:" + n})
	}
	return buttons
}

func (a *app) updateNewDialog(px, py float64) {
	e := a.editor
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.newDialog = false
		return
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range a.newDialogButtons() {
		if button.contains(px, py) {
			button.do()
			return
		}
	}
}

func (a *app) drawNewDialog(screen *ebiten.Image, px, py float64) string {
	e := a.editor
	a.vRect(screen, 0, 0, editorW, editorH, color.RGBA{0, 0, 0, 170})
	x, y, w, h := a.newDialogRect()
	a.vRound(screen, x-1, y-1, w+2, h+2, 11, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 10, color.RGBA{24, 29, 41, 255})
	a.vText(screen, "New level", x+24, y+18, 26, editorText, true)
	a.vText(screen, "Size", x+24, y+62, 15, editorDim, true)
	a.vText(screen, "Tileset", x+300, y+62, 15, editorDim, true)
	a.vText(screen, fmt.Sprintf("Width   %d", e.newW), x+24, y+274, 16, editorText, true)
	a.vText(screen, fmt.Sprintf("Height  %d", e.newH), x+24, y+314, 16, editorText, true)
	zones := map[int]bool{}
	for _, wave := range e.newTemplate.Waves {
		for _, spawner := range wave.Spawners {
			zones[spawner.Index] = true
		}
	}
	note := fmt.Sprintf("It starts with the waves of %s and %d spawn zones marked along the walls.", e.newTemplate.Info.ID, len(zones))
	for index, line := range a.vWrap(note, 250, 13) {
		a.vText(screen, line, x+24, y+360+float64(index)*16, 13, editorDim, false)
	}
	tip := ""
	for _, button := range a.newDialogButtons() {
		if strings.HasPrefix(button.label, "tileset:") {
			name := strings.TrimPrefix(button.label, "tileset:")
			shade := editorBtn
			switch {
			case button.active != nil && button.active():
				shade = editorBtnOn
			case button.contains(px, py):
				shade = editorBtnHover
			}
			a.vRound(screen, button.x, button.y, button.w, button.h, 7, shade)
			a.drawThumb(screen, a.tilesetThumb(name), button.x+8, button.y+8, button.h-16)
			a.vText(screen, prettyTileset(name), button.x+button.h, button.y+button.h/2-10, 15, editorText, true)
		} else {
			a.drawEditorButton(screen, button, px, py)
		}
		if button.contains(px, py) {
			tip = button.tip
		}
	}
	return tip
}
