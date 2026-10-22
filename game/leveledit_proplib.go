package game

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The prop library: any texture of the game can be placed as a prop. A prop is a picture drawn on the map at a position; its
// size is given in tiles. The list starts with the textures the game's levels already use as props.

type propLibEntry struct {
	name string
	used bool // some level of the game uses it as a prop
}

func (a *app) propLibraryList() []propLibEntry {
	e := a.editor
	if e.libEntries != nil {
		return e.libEntries
	}
	used := map[string]bool{}
	for _, info := range a.levels {
		if info.ID == editorTestID {
			continue
		}
		level, err := a.pack.Load(info.ID)
		if err != nil {
			continue
		}
		for _, prop := range level.Props {
			used[strings.ToLower(prop.Texture)] = true
		}
		for _, prop := range level.AnimatedProps {
			used[strings.ToLower(prop.Texture)] = true
		}
	}
	seen := map[string]bool{}
	var entries []propLibEntry
	for name := range a.pack.TexturePaths() {
		base := strings.ToLower(name)
		if strings.Contains(base, "/") {
			continue
		}
		base = strings.TrimSuffix(strings.TrimSuffix(base, "_hd"), "_sd") // the HD and SD art of one thing is one entry
		if !seen[base] {
			seen[base] = true
			entries = append(entries, propLibEntry{name: base, used: used[base]})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].used != entries[j].used {
			return entries[i].used
		}
		return entries[i].name < entries[j].name
	})
	e.libEntries = entries
	return entries
}

func (a *app) openPropLibrary() {
	e := a.editor
	e.propLibOpen = true
	e.libSearch, e.libTop = "", 0
	if e.libW == 0 {
		e.libW = 2
	}
}

func (a *app) propLibRect() (x, y, w, h float64) { return 60, 52, 840, 540 }

func (a *app) filteredLibrary() []propLibEntry {
	e := a.editor
	var out []propLibEntry
	needle := strings.ToLower(strings.TrimSpace(e.libSearch))
	for _, entry := range a.propLibraryList() {
		if needle == "" || strings.Contains(entry.name, needle) {
			out = append(out, entry)
		}
	}
	return out
}

func (a *app) propLibButtons() []edButton {
	e := a.editor
	x, y, w, h := a.propLibRect()
	rx := x + 440
	step := func(delta float64) float64 {
		if shiftHeld() {
			return delta * 4
		}
		return delta
	}
	return []edButton{
		{icon: "minus", tip: "Narrower (Shift: faster)", x: rx + 150, y: y + 330, w: 32, h: 28, do: func() { e.libW = max(.25, e.libW-step(.25)) }},
		{icon: "plus", tip: "Wider (Shift: faster)", x: rx + 330, y: y + 330, w: 32, h: 28, do: func() { e.libW = min(16, e.libW+step(.25)) }},
		{icon: "minus", tip: "Lower: the prop is drawn higher on the map", x: rx + 150, y: y + 370, w: 32, h: 28, do: func() { e.libLift = max(0, e.libLift-step(.25)) }},
		{icon: "plus", tip: "Higher: lift the prop above its spot (floating things)", x: rx + 330, y: y + 370, w: 32, h: 28, do: func() { e.libLift = min(8, e.libLift+step(.25)) }},
		{label: "Use this prop", icon: "props", tip: "Make it the prop you place with the props tool", x: rx, y: y + h - 62, w: 200, h: 38, accent: true, do: func() {
			if e.libSel == "" {
				return
			}
			a.useLibraryProp()
		}},
		{label: "Close", icon: "close", tip: "Close (Esc)", x: x + w - 140, y: y + h - 62, w: 116, h: 38, do: func() { e.propLibOpen = false }},
	}
}

// useLibraryProp makes the chosen texture the first prop of the props tool.
func (a *app) useLibraryProp() {
	e := a.editor
	texture, err := a.Texture(e.libSel)
	height := e.libW
	if err == nil && texture.Bounds().Dx() > 0 {
		height = e.libW * float64(texture.Bounds().Dy()) / float64(texture.Bounds().Dx())
	}
	prop := formats.Prop{Texture: e.libSel, Height: e.libLift * 32, ScaleX: e.libW, ScaleY: height}
	e.protos = append([]editorProto{{prop: prop, label: e.libSel + " (library)"}}, e.protos...)
	e.protoIndex = 0
	e.propMode, e.wavePage, e.propLibOpen = true, false, false
	e.say("Click the map to place " + e.libSel)
}

func (a *app) updatePropLibrary(px, py float64) {
	e := a.editor
	list := a.filteredLibrary()
	// typing edits the search
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 32 && r != 127 && utf8.RuneCountInString(e.libSearch) < 24 {
			e.libSearch += string(r)
			e.libTop = 0
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) && e.libSearch != "" {
		runes := []rune(e.libSearch)
		e.libSearch = string(runes[:len(runes)-1])
		e.libTop = 0
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.propLibOpen = false
		return
	}
	const rows = 19
	if _, wheel := ebiten.Wheel(); wheel != 0 {
		e.libTop = max(0, min(max(0, len(list)-rows), e.libTop-int(wheel*3)))
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	for _, button := range a.propLibButtons() {
		if button.contains(px, py) {
			button.do()
			return
		}
	}
	x, y, _, _ := a.propLibRect()
	if px >= x+16 && px < x+416 && py >= y+84 {
		if row := int((py - y - 84) / 22); row >= 0 && row < rows && e.libTop+row < len(list) {
			e.libSel = list[e.libTop+row].name
			if texture, err := a.Texture(e.libSel); err == nil && texture.Bounds().Dx() > 0 {
				e.libW = max(.5, float64(texture.Bounds().Dx())/a.pack.TextureSourceScale(e.libSel)/32)
			}
		}
	}
}

func (a *app) drawPropLibrary(screen *ebiten.Image, px, py float64) {
	e := a.editor
	x, y, w, h := a.propLibRect()
	a.vRect(screen, 0, 0, editorW, editorH, color.RGBA{0, 0, 0, 175})
	a.vRound(screen, x-1, y-1, w+2, h+2, 11, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 10, color.RGBA{24, 29, 41, 255})
	a.vText(screen, "Prop library", x+20, y+14, 22, editorText, true)
	for index, line := range a.vWrap("Type to search. Textures the game's levels already use as props come first.", 400, 13) {
		a.vText(screen, line, x+20, y+44+float64(index)*14, 13, editorDim, false)
	}
	a.vRound(screen, x+16, y+66, 400, 18, 4, color.RGBA{20, 26, 40, 255})
	a.vText(screen, "Search: "+e.libSearch+"|", x+22, y+66, 13, editorText, false)
	list := a.filteredLibrary()
	const rows = 19
	for row := 0; row < rows && e.libTop+row < len(list); row++ {
		entry := list[e.libTop+row]
		rowY := y + 84 + float64(row)*22
		hover := px >= x+16 && px < x+416 && py >= rowY && py < rowY+22
		switch {
		case entry.name == e.libSel:
			a.vRound(screen, x+16, rowY, 400, 21, 4, editorBtnOn)
		case hover:
			a.vRound(screen, x+16, rowY, 400, 21, 4, editorBtnHover)
		}
		a.vText(screen, entry.name, x+24, rowY+3, 14, editorText, false)
		if entry.used {
			a.vText(screen, "used in levels", x+330, rowY+4, 11, editorDim, false)
		}
	}
	a.vText(screen, fmt.Sprintf("%d textures", len(list)), x+20, y+h-28, 12, editorDim, false)
	rx := x + 440
	a.vRound(screen, rx, y+20, 360, 290, 8, color.RGBA{14, 17, 24, 255})
	if e.libSel != "" {
		if texture, err := a.Texture(e.libSel); err == nil {
			b := texture.Bounds()
			scale := min(340/float64(b.Dx()), 270/float64(b.Dy()))
			sx, sy := a.editorScale()
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(scale*sx, scale*sy)
			options.GeoM.Translate((rx+10+(340-scale*float64(b.Dx()))/2)*sx, (y+30+(270-scale*float64(b.Dy()))/2)*sy)
			screen.DrawImage(texture, options)
		}
		a.vText(screen, e.libSel, rx+10, y+296, 12, editorDim, false)
	} else {
		a.vText(screen, "Pick a texture on the left", rx+90, y+150, 15, editorDim, false)
	}
	a.vText(screen, "Width in tiles", rx, y+336, 15, editorText, false)
	a.vText(screen, fmt.Sprintf("%.2f", e.libW), rx+190, y+336, 16, editorText, true)
	a.vText(screen, "Lift (tiles)", rx, y+376, 15, editorText, false)
	a.vText(screen, fmt.Sprintf("%.2f", e.libLift), rx+190, y+376, 16, editorText, true)
	for _, button := range a.propLibButtons() {
		a.drawEditorButton(screen, button, px, py)
	}
}
