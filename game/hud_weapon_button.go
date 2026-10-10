package game

import (
	"fmt"
	"image"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// Round-count text beside the weapon icons on the control buttons.
const (
	controlCountTextScale  = .34
	controlCountTextHeight = 9.0
)

func (a *app) drawControlButtonPlate(screen *ebiten.Image, x, y, width, height float64) {
	texture, err := a.Texture("Common0/Textures/SecondaryButton_SD")
	if err != nil {
		return
	}
	density := a.pack.TextureSourceScale("Common0/Textures/SecondaryButton_SD")
	cropW, cropH := 64*density, 32*density
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-cropW/2, -cropH/2)
	options.GeoM.Scale(width/cropW, height/cropH)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, texture.SubImage(image.Rect(0, 0, int(cropW), int(cropH))).(*ebiten.Image), options)
}

// primaryWeaponIconCell is the Weapons_Primary sheet cell for the equipped gun.
func primaryWeaponIconCell(weapon formats.Weapon) (int, bool) {
	name := "p_" + strings.ToLower(weapon.GunType)
	if weapon.GunType == "DUALPISTOL" {
		name = "p_dual_pistol" // the pickup name has an underscore the XML gun type lacks
	}
	return pickupPrimaryCell(name)
}

// drawPrimaryWeaponButton shows the special weapon and its remaining rounds
// above the left stick, mirroring the grenade button above the right one. The
// pistol has unlimited rounds, so nothing is shown for it.
func (a *app) drawPrimaryWeaponButton(screen *ebiten.Image) {
	if a.play == nil || a.play.weapon.Ammo <= 0 || a.play.weapon.GunType == "PISTOL" {
		return
	}
	cell, ok := primaryWeaponIconCell(a.play.weapon)
	if !ok {
		return
	}
	x, y, width, height := a.play.primaryButtonGeometry()
	a.drawControlButtonPlate(screen, x, y, width, height)
	a.drawButtonReadout(screen, x, y, "Common0/Textures/Weapons_Primary_SD", cell, fmt.Sprintf("x%d", a.play.weapon.Ammo))
}

// drawButtonReadout draws a silhouette icon and a count side by side, centred
// on a control button's plate. Both the primary weapon button (left stick) and
// the secondary button (right stick) use it.
func (a *app) drawButtonReadout(screen *ebiten.Image, x, y float64, sheetName string, cell int, count string) {
	sheet, err := a.Texture(sheetName)
	if err != nil {
		return
	}
	const cells = 8
	rect := sheet.Bounds()
	rect.Min.X, rect.Max.X = rect.Min.X+rect.Dx()*cell/cells, rect.Min.X+rect.Dx()*(cell+1)/cells
	icon := sheet.SubImage(rect).(*ebiten.Image)
	offset, found := a.variables.Vec2Value("HUD_SECOND_ICON_OFFSET_VAR")
	if !found {
		offset = formats.Vec2{X: -12, Y: 0}
	}
	density := a.pack.TextureSourceScale(sheetName)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(icon.Bounds().Dx())/2, -float64(icon.Bounds().Dy())/2)
	// Sheet cells are twice the grenade icon's size; keep the icon inside the plate. Plate, icon and count are all in
	// logical units, so they scale together with the window (drawImage and the text apply the frontend scale).
	iconScale := .5
	options.GeoM.Scale(iconScale/density, iconScale/density)
	iconWidth := float64(icon.Bounds().Dx()) * iconScale / density
	_, iconY := secondaryIconPosition(x, y, offset, a.mobile)
	// Centre the icon and the count together so three-digit counts stay on the plate.
	countWidth := a.fontTextWidth(count, controlCountTextScale)
	iconX := x - (3+countWidth)/2
	options.GeoM.Translate(iconX, iconY)
	a.drawImage(screen, icon, options)
	a.text(screen, count, iconX+iconWidth/2+3, iconY-controlCountTextHeight/2, controlCountTextScale)
}
