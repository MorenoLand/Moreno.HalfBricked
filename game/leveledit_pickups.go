package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// The pickup tool: crates a level puts down at fixed places, when it starts or when a given wave begins. (Waves can also
// drop crates at their spawn zones; this is for a crate you want exactly here.)

var pickupChoices = []struct{ id, label string }{
	{"p_health", "Extra life"}, {"p_shield", "Shield"}, {"p_dual_pistol", "Dual pistols"}, {"p_shotgun", "Shotgun"},
	{"p_uzi", "Uzi"}, {"p_minigun", "Minigun"}, {"p_sniper", "Sniper"}, {"p_flamer", "Flamethrower"},
	{"p_buzzsaw", "Buzzsaw"}, {"p_grenade", "Grenade"}, {"p_mine", "Mine"}, {"p_cow_pat", "Cow pat"},
	{"p_bazooka", "Bazooka"}, {"p_sentry", "Sentry"}, {"p_rand_all", "Random"}, {"p_rand_1", "Random 1"},
}

func pickupLabel(id string) string {
	for _, c := range pickupChoices {
		if c.id == id {
			return c.label
		}
	}
	return id
}

func (e *levelEditor) pickupNear(worldX, worldY float64) int {
	best, bestDistance := -1, 26.0
	for index, pickup := range e.level.Pickups {
		if distance := math.Hypot(pickup.X-worldX, pickup.Y-worldY); distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best
}

// pickupButtons are the panel's controls while the pickup tool is on: the kinds, and the wave.
func (a *app) pickupButtons() []edButton {
	e := a.editor
	var buttons []edButton
	if e.pickupKind == "" {
		e.pickupKind = "p_shotgun"
	}
	panelW := editorW - editorPanelX
	for index, choice := range pickupChoices {
		id := choice.id
		buttons = append(buttons, edButton{label: choice.label, tip: "Place " + choice.label + " crates", x: editorPanelX + 6 + float64(index%2)*(panelW/2-2), y: editorPaletteTop + float64(index/2)*30, w: panelW/2 - 10, h: 27,
			active: func() bool {
				if e.pickupSel >= 0 && e.pickupSel < len(e.level.Pickups) {
					return e.level.Pickups[e.pickupSel].Name == id
				}
				return e.pickupKind == id
			},
			do: func() {
				e.pickupKind = id
				if e.pickupSel >= 0 && e.pickupSel < len(e.level.Pickups) {
					e.pushAs("Change pickup")
					e.level.Pickups[e.pickupSel].Name = id
				}
			}})
	}
	top := editorPaletteTop + float64(len(pickupChoices)/2)*30 + 20
	change := func(delta int) {
		if e.pickupSel >= 0 && e.pickupSel < len(e.level.Pickups) {
			e.pushAs("Change pickup wave")
			e.level.Pickups[e.pickupSel].Wave = max(0, e.level.Pickups[e.pickupSel].Wave+delta)
		} else {
			e.pickupWave = max(0, e.pickupWave+delta)
		}
	}
	buttons = append(buttons,
		edButton{icon: "minus", tip: "Appears earlier (0 is when the level starts)", x: editorPanelX + 150, y: top, w: 32, h: 28, do: func() { change(-1) }},
		edButton{icon: "plus", tip: "Appears later: when this wave begins", x: editorPanelX + panelW - 38, y: top, w: 32, h: 28, do: func() { change(1) }},
		edButton{label: "Remove", icon: "trash", tip: "Remove the selected crate (right click on the map does too)", x: editorPanelX + 6, y: top + 40, w: panelW - 12, h: 28, do: func() { e.deletePickup(e.pickupSel) }})
	return buttons
}

func (e *levelEditor) deletePickup(index int) {
	if index < 0 || index >= len(e.level.Pickups) {
		return
	}
	e.pushAs("Remove pickup")
	e.level.Pickups = append(e.level.Pickups[:index:index], e.level.Pickups[index+1:]...)
	e.pickupSel = -1
}

func (e *levelEditor) currentPickupWave() int {
	if e.pickupSel >= 0 && e.pickupSel < len(e.level.Pickups) {
		return e.level.Pickups[e.pickupSel].Wave
	}
	return e.pickupWave
}

func (a *app) drawPickupPanel(screen *ebiten.Image, px, py float64) {
	e := a.editor
	panelW := editorW - editorPanelX
	a.vText(screen, "Pickups", editorPanelX+8, editorPaletteTop-20, 14, editorDim, true)
	top := editorPaletteTop + float64(len(pickupChoices)/2)*30 + 20
	wave := e.currentPickupWave()
	label := "Appears when the level starts"
	if wave > 1 {
		label = fmt.Sprintf("Appears when wave %d begins", wave)
	}
	a.vText(screen, fmt.Sprintf("Wave  %d", wave), editorPanelX+8, top+4, 16, editorText, true)
	for index, line := range a.vWrap(label, panelW-16, 13) {
		a.vText(screen, line, editorPanelX+8, top+82+float64(index)*16, 13, editorDim, false)
	}
	for index, line := range a.vWrap("Click the map to place a crate, drag one to move it, right click removes it.", panelW-16, 13) {
		a.vText(screen, line, editorPanelX+8, top+116+float64(index)*16, 13, editorDim, false)
	}
}

func (a *app) updatePickupTool(inMap, justLeft, left, justRight bool, worldX, worldY float64) {
	e := a.editor
	if !inMap {
		return
	}
	snap := func(v float64) float64 { return (math.Floor(v/editorTile) + .5) * editorTile }
	switch {
	case justLeft:
		if near := e.pickupNear(worldX, worldY); near >= 0 {
			e.pickupSel, e.pickupDrag = near, true
			e.pushAs("Move pickup")
		} else {
			if e.pickupKind == "" {
				e.pickupKind = "p_shotgun"
			}
			e.pushAs("Place pickup")
			e.level.Pickups = append(e.level.Pickups, formats.LevelPickup{Name: e.pickupKind, X: snap(worldX), Y: snap(worldY), Wave: e.pickupWave})
			e.pickupSel = len(e.level.Pickups) - 1
		}
	case left && e.pickupDrag && e.pickupSel >= 0 && e.pickupSel < len(e.level.Pickups):
		e.level.Pickups[e.pickupSel].X, e.level.Pickups[e.pickupSel].Y = snap(worldX), snap(worldY)
	case justRight:
		e.deletePickup(e.pickupNear(worldX, worldY))
	}
	if !left {
		e.pickupDrag = false
	}
}

// drawPickupsOnMap draws the level's crates on the map.
func (a *app) drawPickupsOnMap(screen *ebiten.Image, toScreen func(x, y float64) (float64, float64)) {
	e := a.editor
	if len(e.level.Pickups) == 0 {
		return
	}
	crate, _ := a.Texture("Common0/Textures/crate_SD")
	sx, sy := a.editorScale()
	size := 40 * e.view.Zoom * 2
	for index, pickup := range e.level.Pickups {
		x, y := toScreen(pickup.X, pickup.Y)
		if crate != nil {
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(size*sx/float64(crate.Bounds().Dx()), size*sy/float64(crate.Bounds().Dy()))
			options.GeoM.Translate((x-size/2)*sx, (y-size/2)*sy)
			if e.tool != toolPickup {
				options.ColorScale.ScaleAlpha(.8)
			}
			screen.DrawImage(crate, options)
		}
		if index == e.pickupSel && e.tool == toolPickup {
			a.vOutline(screen, x-size/2, y-size/2, size, size, 2, color.RGBA{80, 255, 120, 255})
		}
		text := pickupLabel(pickup.Name)
		if pickup.Wave > 1 {
			text += fmt.Sprintf(" (wave %d)", pickup.Wave)
		}
		a.vRect(screen, x-a.vTextWidth(text, 11, true)/2-3, y+size/2, a.vTextWidth(text, 11, true)+6, 15, color.RGBA{0, 0, 0, 170})
		a.vText(screen, text, x-a.vTextWidth(text, 11, true)/2, y+size/2+1, 11, color.RGBA{255, 255, 255, 255}, true)
	}
}
