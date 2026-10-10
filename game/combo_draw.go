package game

import (
	"image"
	"math"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// wireCombo is one tracker popup in the online snapshot (screen space).
type wireCombo struct {
	Text       string
	X, Y, Size float64
	R, G, B, A uint8
}

// comboShownTrackers lists what is drawn: exactly one combo count, the live counter, which stands for both held slots
// (see shownPopup). Released trackers draw only their tier or bonus words ("Good", "+2", "Epic Fail"); their count text
// ("x35") would be a second counter and is not drawn. Their multiplier bonus still goes to the score multiplier.
func (p *playState) comboShownTrackers() []*comboTracker {
	var list []*comboTracker
	// The counter is shown by hits and kills, not by holding a weapon: nothing is drawn while the count is 0.
	if live := p.combo.liveCounter(); live != nil && p.combo.sharedHits() > 0 {
		list = append(list, live)
	}
	for _, t := range p.combo.popups {
		if !strings.HasPrefix(t.text, "x") {
			list = append(list, t)
		}
	}
	return list
}

// shownPopup is the popup of a shown tracker. The live counter reads the shared count of both held slots.
func (c *comboSystem) shownPopup(t *comboTracker) (comboPopup, bool) {
	popup, ok := t.popup()
	if ok && t == c.liveCounter() {
		popup.Text = "x" + strconv.Itoa(c.sharedHits())
	}
	return popup, ok
}

// comboWire exports the host's popups and HUD flash for guests.
func (p *playState) comboWire() ([]wireCombo, float64, [3]float64) {
	var out []wireCombo
	for _, t := range p.comboShownTrackers() {
		if popup, ok := p.combo.shownPopup(t); ok {
			out = append(out, wireCombo{Text: popup.Text, X: popup.X, Y: popup.Y, Size: popup.Size, R: popup.R, G: popup.G, B: popup.B, A: popup.A})
		}
	}
	return out, p.combo.hudFlash, p.combo.hudColor
}

// applyComboWire stores the host's popups; guests do not run trackers.
func (p *playState) applyComboWire(popups []wireCombo, flash float64, color [3]float64) {
	p.combo.mirror = append(p.combo.mirror[:0], popups...)
	p.combo.hudFlash, p.combo.hudColor = flash, color
	p.combo.updateHUD(1.0/60.0, p.multiplier)
	p.combo.hudFlash = flash
}

// comboPopups lists what to draw this frame: the host's own trackers, or the
// popups mirrored from the host.
func (p *playState) comboPopups() []comboPopup {
	if len(p.combo.mirror) > 0 {
		list := make([]comboPopup, 0, len(p.combo.mirror))
		for _, w := range p.combo.mirror {
			list = append(list, comboPopup{Text: w.Text, X: w.X, Y: w.Y, Size: w.Size, R: w.R, G: w.G, B: w.B, A: w.A})
		}
		return list
	}
	var list []comboPopup
	for _, t := range p.comboShownTrackers() {
		if popup, ok := p.combo.shownPopup(t); ok {
			list = append(list, popup)
		}
	}
	return list
}

// drawComboHUD draws the "x<N>" multiplier next to the score (glyph sheet
// MultiplyFont: digits 0-9 then "x") and the combo popups.
func (a *app) drawComboHUD(screen *ebiten.Image, x, y float64) {
	if a.play == nil {
		return
	}
	a.drawComboMultiplier(screen, x, y)
	if len(a.play.combo.mirror) > 0 {
		for _, popup := range a.play.comboPopups() {
			a.drawComboText(screen, popup, nil)
		}
		return
	}
	// FUN_000993d0 only draws while the HUD is visible.
	if !a.play.hudVisible {
		return
	}
	for _, t := range a.play.comboShownTrackers() {
		if popup, ok := a.play.combo.shownPopup(t); ok {
			a.drawComboText(screen, popup, t)
		}
	}
}

func (a *app) drawComboMultiplier(screen *ebiten.Image, x, y float64) {
	texture, err := a.Texture("Common0/Textures/MultiplyFont_SD")
	if err != nil {
		return
	}
	resolution := a.pack.TextureSourceScale("Common0/Textures/MultiplyFont_SD")
	c := &a.play.combo
	multiplier := max(a.play.multiplier, 0)
	// Native: glyph height = HUD +0x94 * +0x98 (FUN_000d0a64), where +0x94 eases
	// toward multiplier/5 + 28. The port's x1 size is 8x16, the native x1 value is 28.2.
	scale := c.hudScale
	if scale <= 0 {
		scale = comboHudTarget(multiplier)
	}
	pulse := c.hudPulse
	if pulse <= 0 {
		pulse = 1
	}
	grow := scale * pulse / comboHudTarget(1)
	tint := c.hudTint()
	text := "x" + strconv.Itoa(multiplier)
	cursor := x
	for _, r := range text {
		glyph := 10
		if r != 'x' {
			glyph = int(r - '0')
		}
		source := texture.SubImage(resolutionRect(image.Rect(glyph*16, 0, glyph*16+16, 32), resolution)).(*ebiten.Image)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5/resolution*grow, .5/resolution*grow)
		options.GeoM.Translate(cursor, y)
		options.ColorScale.Scale(float32(tint[0]/255), float32(tint[1]/255), float32(tint[2]/255), 1)
		a.drawImage(screen, source, options)
		cursor += 8 * grow
	}
	_ = cursor
}

// drawComboText draws a popup centred on its position with the HUD font,
// tinted by the tracker colour and alpha.
func (a *app) drawComboText(screen *ebiten.Image, popup comboPopup, t *comboTracker) {
	if a.font == nil || a.font.LineHeight <= 0 || popup.A == 0 {
		return
	}
	scale := popup.Size / float64(a.font.LineHeight)
	width := a.fontTextWidth(popup.Text, scale)
	// Native clamp: inside the screen with a 2 pixel margin (DAT_000997bc).
	half := width / 2
	cx := math.Min(float64(logicalWidth)-comboMargin-half, math.Max(comboMargin+half, popup.X))
	glyph := comboHudMinGlyph
	if t != nil && t.hudGlyph > 0 {
		glyph = t.hudGlyph
	}
	// Below the HUD multiplier, above the bottom margin (the bottom bound wins
	// first, then the top bound, as in FUN_000993d0).
	cy := math.Max(comboTopBound(glyph, popup.Size), math.Min(float64(logicalHeight)-comboMargin-popup.Size/2, popup.Y))
	if t != nil && t.state == comboActive {
		// The clamped position is where a released tracker starts its flight.
		t.shownX, t.shownY, t.placed = cx, cy, true
	}
	x := cx - half
	y := cy - popup.Size/2
	alpha := float32(popup.A) / 255
	// A dark outline keeps the number readable over grass and sand.
	for _, offset := range [][2]float64{{-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
		a.drawComboGlyphs(screen, popup.Text, x+offset[0], y+offset[1], scale, 0, 0, 0, alpha*.7)
	}
	a.drawComboGlyphs(screen, popup.Text, x, y, scale, float32(popup.R)/255, float32(popup.G)/255, float32(popup.B)/255, alpha)
}

func (a *app) drawComboGlyphs(screen *ebiten.Image, text string, x, y, scale float64, red, green, blue, alpha float32) {
	for _, r := range text {
		glyph, ok := a.font.Glyphs[r]
		if !ok {
			continue
		}
		if glyph.Width > 0 && glyph.Height > 0 {
			source := a.font.Atlas.SubImage(image.Rect(glyph.X, glyph.Y, glyph.X+glyph.Width, glyph.Y+glyph.Height)).(*ebiten.Image)
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(scale, scale)
			options.GeoM.Translate(x+float64(glyph.XOffset)*scale, y+float64(glyph.YOffset)*scale)
			options.ColorScale.Scale(red*alpha, green*alpha, blue*alpha, alpha)
			a.drawImage(screen, source, options)
		}
		x += float64(glyph.XAdvance) * scale
	}
}
