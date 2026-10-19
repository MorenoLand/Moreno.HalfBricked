package game

import (
	"fmt"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// PORT ADDITION (not in the original binary), switchable with -pickup-flight=false: like the "+1" that flies to the life
// counter (random_box_mark.go), the icon of a special weapon or secondary (grenade, mine, bazooka, sentry, cow pat) rises
// from the crate and flies to its button above the stick, where the new weapon shows.
var portPickupFlight = true

const (
	itemFlightRise  = 0.2  // seconds the icon pops up at the crate
	itemFlightTotal = 0.7  // seconds until it has arrived and faded
	itemFlightFade  = 0.12 // closing share of the flight that fades out
)

type itemFlight struct {
	x, y      float64 // logical screen position at pickup time
	age       float64
	secondary bool
	cell      int
	// The button as it was when the crate was taken: it keeps showing this (or nothing, when the slot was empty) until
	// the icon arrives, so the slot changes when the flight lands instead of when it starts.
	prevShown bool
	prevCell  int
	prevCount string
}

// flightLanded is when the flying icon has reached its button (the fade at the end hands over to the real one).
func flightLanded(flight itemFlight) bool { return flight.age >= itemFlightTotal-itemFlightFade }

// pendingFlight is the oldest flight of the given kind still on its way to the button, if any.
func (p *playState) pendingFlight(secondary bool) *itemFlight {
	for index := range p.itemFlights {
		if flight := &p.itemFlights[index]; flight.secondary == secondary && !flightLanded(*flight) {
			return flight
		}
	}
	return nil
}

// addItemFlight starts the icon flight for a picked up weapon crate at the world point (x, y). Pickups that do not fill
// a weapon button (health, shield, the pistol) start nothing.
func (p *playState) addItemFlight(name string, x, y float64) {
	if !portPickupFlight || p.world == nil {
		return
	}
	name = strings.ToUpper(strings.TrimSpace(name))
	if resolved, ok := p.resolveRandomPickup(name); ok {
		name = strings.ToUpper(resolved)
	}
	flight := itemFlight{}
	if slot, _, ok := secondaryForPickup(name); ok {
		flight.secondary, flight.cell = true, secondaryIconCell(slot)
	} else if weapon, ok := p.weapons.Find(catalogWeaponName(strings.TrimPrefix(name, "P_"))); ok && weapon.GunType != "PISTOL" {
		cell, found := primaryWeaponIconCell(weapon)
		if !found {
			return
		}
		flight.cell = cell
	} else {
		return
	}
	if flight.secondary {
		if p.grenades > 0 {
			flight.prevShown, flight.prevCell, flight.prevCount = true, secondaryIconCell(p.secondaryType), fmt.Sprintf("x%d", p.grenades)
		}
	} else if p.weapon.Ammo > 0 && p.weapon.GunType != "PISTOL" {
		if cell, ok := primaryWeaponIconCell(p.weapon); ok {
			flight.prevShown, flight.prevCell, flight.prevCount = true, cell, fmt.Sprintf("x%d", p.weapon.Ammo)
		}
	}
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	flight.x = (x-p.world.CameraX)*zoom + p.world.ViewportX
	flight.y = (y-p.world.CameraY)*zoom + p.world.ViewportY - 30*zoom
	p.itemFlights = append(p.itemFlights, flight)
}

func (p *playState) updateItemFlights(dt float64) {
	live := p.itemFlights[:0]
	for _, flight := range p.itemFlights {
		flight.age += dt
		if flight.age < itemFlightTotal {
			live = append(live, flight)
		}
	}
	p.itemFlights = live
}

// itemFlightPose is the screen point, scale and alpha of a flight at its age, ending at the button centre (endX, endY).
func itemFlightPose(flight itemFlight, endX, endY float64) (x, y, scale, alpha float64) {
	if flight.age < itemFlightRise {
		t := smoothStep(flight.age / itemFlightRise)
		return flight.x, flight.y - 14*t, 1 + .4*t, 1
	}
	t := smoothStep((flight.age - itemFlightRise) / (itemFlightTotal - itemFlightRise))
	startX, startY := flight.x, flight.y-14
	x, y = startX+(endX-startX)*t, startY+(endY-startY)*t
	scale = 1.4 - .4*t
	alpha = 1
	if left := itemFlightTotal - flight.age; left < itemFlightFade {
		alpha = math.Max(0, left/itemFlightFade)
	}
	return x, y, scale, alpha
}

// drawItemFlights draws the icon flights over the HUD.
func (a *app) drawItemFlights(screen *ebiten.Image) {
	if a.play == nil || len(a.play.itemFlights) == 0 {
		return
	}
	for _, flight := range a.play.itemFlights {
		sheetName := "Common0/Textures/Weapons_Primary_SD"
		endX, endY, _, _ := a.play.primaryButtonGeometry()
		if flight.secondary {
			sheetName = "Common0/Textures/Weapons_Secondary_SD"
			endX, endY, _, _ = a.play.secondaryButtonGeometry()
		}
		sheet, err := a.Texture(sheetName)
		if err != nil {
			continue
		}
		const cells = 8
		rect := sheet.Bounds()
		rect.Min.X, rect.Max.X = rect.Min.X+rect.Dx()*flight.cell/cells, rect.Min.X+rect.Dx()*(flight.cell+1)/cells
		icon := sheet.SubImage(rect).(*ebiten.Image)
		x, y, scale, alpha := itemFlightPose(flight, endX, endY)
		density := a.pack.TextureSourceScale(sheetName)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-float64(icon.Bounds().Dx())/2, -float64(icon.Bounds().Dy())/2)
		options.GeoM.Scale(.5*scale/density, .5*scale/density) // the same size the button readout draws it at
		options.GeoM.Translate(x, y)
		options.ColorScale.ScaleAlpha(float32(alpha))
		a.drawImage(screen, icon, options)
	}
}
