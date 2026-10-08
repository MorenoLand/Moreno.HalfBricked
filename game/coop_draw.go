package game

import (
	"fmt"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// coopTint is the colour that tells players apart (Barry is white for player 1).
// Native has per-player colours; these values are the port's own.
func coopTint(index int) [3]float32 {
	switch index {
	case 1:
		return [3]float32{.62, .82, 1.25}
	case 2:
		return [3]float32{1.25, .66, .66}
	case 3:
		return [3]float32{.7, 1.2, .7}
	}
	return [3]float32{}
}

// Co-op HUD corners, from the native multiplayer layout (FUN_00134190): hearts,
// weapon readout and name tag per player, scaled 0.7.
var (
	coopHeartsAt = [maxCoopPlayers][2]float64{{15, 15}, {435, 15}, {15, 160}, {460, 160}}
	coopWeaponAt = [maxCoopPlayers][2]float64{{20, 35}, {370, 35}, {20, 278}, {400, 278}}
	coopTagAt    = [maxCoopPlayers][2]float64{{50, 15}, {398, 62}, {50, 300}, {400, 300}}
)

const coopHudScale = .7

// A joining player's portal opens first, then they climb out of it.
const (
	coopPortalDelay = .25
	coopEmergeTime  = .45
)

// drawCoopPlayer draws one of players 2-4 in the world, tinted and placed like player 1.
func (a *app) drawCoopPlayer(target *ebiten.Image, c *coopPlayer) {
	p := a.play
	if !c.joined || c.dead || c.body.health <= 0 {
		return
	}
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	// The player climbs out of the portal that opened where they join or return.
	emerge := math.Max(0, math.Min(1, (c.spawnAge-coopPortalDelay)/coopEmergeTime))
	if emerge <= 0 {
		return
	}
	bodyZoom := zoom * emerge
	p.withPlayer(c, func() {
		screenX := (p.x-p.world.CameraX)*zoom + p.world.ViewportX
		screenY := (p.y-p.world.CameraY)*zoom + p.world.ViewportY
		frame := int(p.time*8) % 4
		if p.moving {
			frame = int(p.time*10) % 4
		}
		a.barryTint = hurtTint(p.hurt, p.time, coopTint(c.index))
		a.drawBarryShadow(target, screenX, screenY-24*zoom, bodyZoom)
		a.drawBarry(target, screenX, screenY, bodyZoom, frame, p.angle, p.flipX)
		if p.flash > 0 {
			a.drawBarryFlash(target, screenX, screenY, bodyZoom, p.angle, p.flipX)
		}
		a.barryTint = [3]float32{}
	})
}

// drawWeaponReadout draws the weapon icon with its remaining rounds.
func (a *app) drawWeaponReadout(screen *ebiten.Image, weapon formats.Weapon, x, y, scale float64) {
	cell, ok := primaryWeaponIconCell(weapon)
	if !ok {
		cell = 0
	}
	sheet, err := a.Texture("Common0/Textures/Weapons_Primary_SD")
	if err != nil {
		return
	}
	const cells = 8
	rect := sheet.Bounds()
	rect.Min.X, rect.Max.X = rect.Min.X+rect.Dx()*cell/cells, rect.Min.X+rect.Dx()*(cell+1)/cells
	icon := sheet.SubImage(rect).(*ebiten.Image)
	density := a.pack.TextureSourceScale("Common0/Textures/Weapons_Primary_SD")
	size := float64(icon.Bounds().Dx()) / density * .5 * scale
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(icon.Bounds().Dx())/2, -float64(icon.Bounds().Dy())/2)
	options.GeoM.Scale(.5/density*scale, .5/density*scale)
	options.GeoM.Translate(x+size/2, y)
	a.drawImage(screen, icon, options)
	if weapon.GunType != "PISTOL" && weapon.Ammo > 0 {
		a.text(screen, fmt.Sprintf("x%d", weapon.Ammo), x+size+3, y-controlCountTextHeight/2, controlCountTextScale)
	}
}

// drawCoopHUD draws each player's name tag, weapon readout and hearts.
func (a *app) drawCoopHUD(screen *ebiten.Image) {
	p := a.play
	bodies := []bodyState{p.body()}
	for _, c := range p.coop.players {
		bodies = append(bodies, c.body)
	}
	for index, body := range bodies {
		health := body.health
		if index > 0 && !p.coop.players[index-1].joined {
			continue
		}
		hearts, weapon, tag := coopHeartsAt[index], coopWeaponAt[index], coopTagAt[index]
		a.drawHealthHeartsAt(screen, health, hearts[0]+healthHeartSize/2*coopHudScale, hearts[1]+96, coopHudScale)
		a.drawWeaponReadout(screen, body.weapon, weapon[0], weapon[1], coopHudScale)
		a.drawSecondaryReadout(screen, body, weapon[0], weapon[1]+14, coopHudScale)
		a.text(screen, fmt.Sprintf("P%d", index+1), tag[0], tag[1]-6, .4)
	}
}

// drawSecondaryReadout shows a player's grenade, mine, rocket or sentry with its count.
func (a *app) drawSecondaryReadout(screen *ebiten.Image, body bodyState, x, y, scale float64) {
	if body.grenades <= 0 {
		return
	}
	sheet, err := a.Texture("Common0/Textures/Weapons_Secondary_SD")
	if err != nil {
		return
	}
	const cells = 8
	cell := secondaryIconCell(body.secondaryType)
	rect := sheet.Bounds()
	rect.Min.X, rect.Max.X = rect.Min.X+rect.Dx()*cell/cells, rect.Min.X+rect.Dx()*(cell+1)/cells
	icon := sheet.SubImage(rect).(*ebiten.Image)
	density := a.pack.TextureSourceScale("Common0/Textures/Weapons_Secondary_SD")
	size := float64(icon.Bounds().Dx()) / density * .5 * scale
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(icon.Bounds().Dx())/2, -float64(icon.Bounds().Dy())/2)
	options.GeoM.Scale(.5/density*scale, .5/density*scale)
	options.GeoM.Translate(x+size/2, y)
	a.drawImage(screen, icon, options)
	a.text(screen, fmt.Sprintf("x%d", body.grenades), x+size+3, y-controlCountTextHeight/2, controlCountTextScale)
}
