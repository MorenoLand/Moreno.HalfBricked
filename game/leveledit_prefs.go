package game

import (
	"encoding/json"
	"fmt"
	"image/color"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/game/platform"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// What the editor remembers between sessions: the tool, layer, brush, recent tiles, the minimap and the shortcut keys.

const prefsName = "editor/prefs.json"

type editorPrefs struct {
	Keys    map[string]int `json:"keys,omitempty"`
	Tool    int            `json:"tool"`
	Layer   string         `json:"layer,omitempty"`
	Brush   int            `json:"brush"`
	Recent  []uint32       `json:"recent,omitempty"`
	Minimap bool           `json:"minimap"`
	Grid    bool           `json:"grid"`
}

// editorEnsurePrefs applies the saved preferences the first time a level is being edited.
func (a *app) editorEnsurePrefs() {
	e := a.editor
	if e.prefsLoaded || e.view == nil {
		return
	}
	e.prefsLoaded = true
	data, ok := platform.ConfigRead(prefsName)
	if !ok {
		return
	}
	var prefs editorPrefs
	if json.Unmarshal(data, &prefs) != nil {
		return
	}
	e.keys = map[string]ebiten.Key{}
	for id, key := range prefs.Keys {
		e.keys[id] = ebiten.Key(key)
	}
	if prefs.Tool >= toolPaint && prefs.Tool <= toolPickup {
		e.tool = prefs.Tool
	}
	for _, layer := range editorLayers() {
		if string(layer.kind) == prefs.Layer {
			e.layer = layer.kind
		}
	}
	if prefs.Brush >= 1 && prefs.Brush <= 8 {
		e.brush = prefs.Brush
	}
	e.recent = prefs.Recent
	e.minimap = prefs.Minimap
	e.view.Grid = prefs.Grid
}

func (a *app) editorSavePrefs() {
	e := a.editor
	if e == nil || e.view == nil {
		return
	}
	prefs := editorPrefs{Keys: map[string]int{}, Tool: e.tool, Layer: string(e.layer), Brush: e.brush, Recent: e.recent, Minimap: e.minimap, Grid: e.view.Grid}
	for id, key := range e.keys {
		prefs.Keys[id] = int(key)
	}
	if data, err := json.Marshal(prefs); err == nil {
		platform.ConfigWrite(prefsName, data)
	}
}

// ---- the shortcuts dialog ------------------------------------------------------------------------------------------

func (a *app) shortcutsRect() (x, y, w, h float64) { return 200, 50, 560, 548 }

func (a *app) shortcutsButtons() []edButton {
	e := a.editor
	x, y, w, h := a.shortcutsRect()
	var buttons []edButton
	for index, action := range editorActions {
		id := action.id
		col, row := index/10, index%10
		buttons = append(buttons, edButton{label: "", tip: "Click, then press the key you want for " + action.label, x: x + 20 + float64(col)*270, y: y + 74 + float64(row)*38, w: 256, h: 32,
			active: func() bool { return e.rebind == id }, do: func() { e.rebind = id }, icon: "key:" + id})
	}
	buttons = append(buttons,
		edButton{label: "Reset all", tip: "Back to the standard keys", x: x + 20, y: y + h - 60, w: 150, h: 38, do: func() { e.keys = nil; e.rebind = ""; a.editorSavePrefs() }},
		edButton{label: "Done", icon: "close", tip: "Close (Esc)", x: x + w - 150, y: y + h - 60, w: 126, h: 38, accent: true, do: func() { e.shortcutsOpen, e.rebind = false, ""; a.editorSavePrefs() }})
	return buttons
}

func keyName(key ebiten.Key) string {
	name := key.String()
	switch name {
	case "BracketLeft":
		return "[ key"
	case "BracketRight":
		return "] key"
	}
	return name
}

func (a *app) updateShortcuts(px, py float64) {
	e := a.editor
	if e.rebind != "" {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			e.rebind = ""
			return
		}
		for key := ebiten.Key(0); key <= ebiten.KeyMax; key++ {
			if inpututil.IsKeyJustPressed(key) {
				switch key {
				case ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyAltLeft, ebiten.KeyAltRight:
					continue
				}
				if e.keys == nil {
					e.keys = map[string]ebiten.Key{}
				}
				for _, other := range editorActions {
					if other.id != e.rebind && e.key(other.id) == key {
						e.say(fmt.Sprintf("%s used that key; it is now free", other.label))
						e.keys[other.id] = ebiten.Key(-1)
					}
				}
				e.keys[e.rebind] = key
				e.rebind = ""
				a.editorSavePrefs()
				return
			}
		}
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.shortcutsOpen = false
		a.editorSavePrefs()
		return
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for _, button := range a.shortcutsButtons() {
			if button.contains(px, py) {
				button.do()
				return
			}
		}
	}
}

func (a *app) drawShortcuts(screen *ebiten.Image, px, py float64) {
	e := a.editor
	x, y, w, h := a.shortcutsRect()
	a.vRect(screen, 0, 0, editorW, editorH, color.RGBA{0, 0, 0, 175})
	a.vRound(screen, x-1, y-1, w+2, h+2, 11, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 10, color.RGBA{24, 29, 41, 255})
	a.vText(screen, "Keyboard shortcuts", x+20, y+14, 22, editorText, true)
	a.vText(screen, "Click an action, then press the key you want for it. Esc cancels.", x+20, y+46, 13, editorDim, false)
	for _, button := range a.shortcutsButtons() {
		if len(button.icon) > 4 && button.icon[:4] == "key:" {
			id := button.icon[4:]
			shade := editorBtn
			switch {
			case e.rebind == id:
				shade = editorBtnOn
			case button.contains(px, py):
				shade = editorBtnHover
			}
			a.vRound(screen, button.x, button.y, button.w, button.h, 5, shade)
			for _, action := range editorActions {
				if action.id == id {
					a.vText(screen, action.label, button.x+10, button.y+6, 14, editorText, false)
				}
			}
			label := keyName(e.key(id))
			if e.key(id) < 0 {
				label = "(none)"
			}
			if e.rebind == id {
				label = "press a key…"
			}
			a.vText(screen, label, button.x+button.w-10-a.vTextWidth(label, 14, true), button.y+6, 14, editorPlay, true)
			continue
		}
		a.drawEditorButton(screen, button, px, py)
	}
	_ = formats.LayerG
}
