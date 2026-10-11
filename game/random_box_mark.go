package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

// PORT ADDITIONS (not in the original binary), both switchable with -mystery-mark=false:
//
//   - A p_rand_* crate is rolled into a weapon the moment it spawns, so the original shows the weapon crate with its
//     icon. Here it is drawn as a plain crate with a "?" in the game font instead, so a random box reads as one.
//   - Collecting an extra life (P_HEALTH, "Lives + 1" in the original) pops a "+1" where the crate was; it rises, then
//     flies to the life counter and fades out as it arrives.
var portMysteryMark = true

const (
	lifePopupRise  = 0.3  // seconds the "+1" pops and rises at the crate
	lifePopupTotal = 1.0  // seconds until it has arrived and faded
	lifePopupFade  = 0.15 // closing share of the flight that fades out
	// The "x%d" life counter is drawn at (410, 20); the flight ends on its left edge, next to the heart icon.
	lifeCounterX, lifeCounterY = 404.0, 20.0
)

type lifePopup struct {
	x, y float64 // logical screen position at pickup time
	age  float64
}

// addLifePopup starts a "+1" at the world point (x, y).
func (p *playState) addLifePopup(x, y float64) {
	if !portMysteryMark || p.world == nil {
		return
	}
	zoom := p.world.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	p.lifePopups = append(p.lifePopups, lifePopup{x: (x-p.world.CameraX)*zoom + p.world.ViewportX, y: (y-p.world.CameraY)*zoom + p.world.ViewportY - 30*zoom})
}

func (p *playState) updateLifePopups(dt float64) {
	live := p.lifePopups[:0]
	for _, popup := range p.lifePopups {
		popup.age += dt
		if popup.age < lifePopupTotal {
			live = append(live, popup)
		}
	}
	p.lifePopups = live
}

// lifePopupPose is the screen point, scale and alpha of a "+1" at age seconds.
func lifePopupPose(popup lifePopup) (x, y, scale, alpha float64) {
	if popup.age < lifePopupRise {
		t := smoothStep(popup.age / lifePopupRise)
		return popup.x, popup.y - 14*t, .5 + .35*t, 1
	}
	t := smoothStep((popup.age - lifePopupRise) / (lifePopupTotal - lifePopupRise))
	startX, startY := popup.x, popup.y-14
	x, y = startX+(lifeCounterX-startX)*t, startY+(lifeCounterY-startY)*t
	scale = .85 - .3*t
	alpha = 1
	if left := lifePopupTotal - popup.age; left < lifePopupFade {
		alpha = math.Max(0, left/lifePopupFade)
	}
	return x, y, scale, alpha
}

// drawLifePopups draws the "+1" flights over the HUD.
func (a *app) drawLifePopups(screen *ebiten.Image) {
	if a.play == nil {
		return
	}
	for _, popup := range a.play.lifePopups {
		x, y, scale, alpha := lifePopupPose(popup)
		width := a.fontTextWidth("+1", scale)
		if a.font != nil {
			a.font.DrawScaledTintedAlpha(screen, "+1", (x-width/2)*a.frontendScaleX, (y-11*scale)*a.frontendScaleY, scale*a.frontendScaleX, scale*a.frontendScaleY, 1, 1, .6, float32(alpha))
		}
	}
}

// drawMysteryMark draws the "?" of a random crate and reports whether the crate is one.
func (a *app) drawMysteryMark(screen *ebiten.Image, entity *scriptEntity, x, y, zoom float64) bool {
	if !portMysteryMark || !entity.mystery {
		return false
	}
	scale := 1.0
	width := a.fontTextWidth("?", scale)
	a.text(screen, "?", x-width/2*zoom, y-50*0.375*zoom-16*zoom, scale*zoom)
	return true
}
