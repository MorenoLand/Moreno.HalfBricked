package main

import (
	"fmt"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"math"
)

type optionsSettings struct {
	Sound         bool    `json:"sound"`
	Music         bool    `json:"music"`
	Normal        bool    `json:"normal"`
	Visible       bool    `json:"visible"`
	LeftFloating  bool    `json:"leftFloating"`
	RightFloating bool    `json:"rightFloating"`
	PadRadius     float32 `json:"padRadius"`
}
type optionsStickBinding struct {
	Movement, Floating, Visible bool
	Radius                      float32
}

func nativeOptionsFixedOrigin(width, height, trackRadius float32, right bool) (float32, float32) {
	inset := trackRadius
	if inset < 100 {
		inset = 100
	}
	x := inset
	if right {
		x = width - inset
	}
	return x, height - inset
}
func nativeOptionsStickInput(dx, dy, trackRadius, dt float32, floating bool) (x, y, followX, followY float32) {
	distance := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	limit := trackRadius * float32(1.3)
	if distance <= limit {
		if distance < 8 {
			return
		}
		return dx / limit, dy / limit, 0, 0
	}
	x, y = dx/distance, dy/distance
	if floating {
		followX, followY = x*dt*60, y*dt*60
	}
	return
}

func (menu optionsMenu) Settings() optionsSettings {
	c := menu.controls
	return optionsSettings{Sound: menu.sound, Music: menu.music, Normal: c.Normal, Visible: c.Visible, LeftFloating: c.LeftFloating, RightFloating: c.RightFloating, PadRadius: c.PadRadius}
}
func (menu *optionsMenu) RestoreSettings(settings optionsSettings) error {
	if math.IsNaN(float64(settings.PadRadius)) || math.IsInf(float64(settings.PadRadius), 0) || settings.PadRadius < 32 || settings.PadRadius > 80 {
		return fmt.Errorf("invalid control pad radius %v", settings.PadRadius)
	}
	menu.sound, menu.music = settings.Sound, settings.Music
	menu.controls = optionsControls{Normal: settings.Normal, Visible: settings.Visible, LeftFloating: settings.LeftFloating, RightFloating: settings.RightFloating, PadRadius: settings.PadRadius}
	menu.padDragging = false
	return nil
}
func (menu optionsMenu) StickBindings() [2]optionsStickBinding {
	c := menu.controls
	return [2]optionsStickBinding{{Movement: c.Normal, Floating: c.LeftFloating, Visible: c.Visible, Radius: c.PadRadius}, {Movement: !c.Normal, Floating: c.RightFloating, Visible: c.Visible, Radius: c.PadRadius}}
}

var optionsControlLabels = []struct {
	name, text string
	action     optionsAction
}{
	{"NORMAL", "Normal", optionsNormal}, {"INVERT", "Invert", optionsInvert},
	{"VISIBLE", "Visible", optionsVisible}, {"HIDDEN", "Hidden", optionsHidden},
	{"L_FIXED", "Fixed", optionsLeftFixed}, {"R_FIXED", "Fixed", optionsRightFixed},
	{"L_FLOATING", "Floating", optionsLeftFloating}, {"R_FLOATING", "Floating", optionsRightFloating},
}

type optionsControls struct {
	Normal, Visible, LeftFloating, RightFloating bool
	PadRadius                                    float32
}

func nativeOptionsDefaults(compact bool) optionsControls {
	radius := float32(56)
	if compact {
		radius = 42
	}
	return optionsControls{Normal: true, Visible: true, LeftFloating: true, RightFloating: true, PadRadius: radius}
}
func (c *optionsControls) apply(action optionsAction) {
	switch action {
	case optionsNormal:
		c.Normal = true
	case optionsInvert:
		c.Normal = false
	case optionsVisible:
		c.Visible = true
	case optionsHidden:
		c.Visible = false
	case optionsLeftFixed:
		c.LeftFloating = false
	case optionsRightFixed:
		c.RightFloating = false
	case optionsLeftFloating:
		c.LeftFloating = true
	case optionsRightFloating:
		c.RightFloating = true
	}
}
func (c optionsControls) selected(action optionsAction) bool {
	switch action {
	case optionsNormal:
		return c.Normal
	case optionsInvert:
		return !c.Normal
	case optionsVisible:
		return c.Visible
	case optionsHidden:
		return !c.Visible
	case optionsLeftFixed:
		return !c.LeftFloating
	case optionsRightFixed:
		return !c.RightFloating
	case optionsLeftFloating:
		return c.LeftFloating
	case optionsRightFloating:
		return c.RightFloating
	}
	return false
}
func (menu *optionsMenu) dragPad(x, y, centerX, centerY float64, pressed, held bool) {
	if !held {
		menu.padDragging = false
		return
	}
	distance := float32(math.Sqrt((x-centerX)*(x-centerX) + (y-centerY)*(y-centerY)))
	if pressed {
		menu.padDragging = true
		menu.padDragDistance = distance
	}
	if !menu.padDragging {
		return
	}
	radius := (distance - menu.padDragDistance) + menu.controls.PadRadius
	if radius < 32 {
		radius = 32
	}
	if radius > 80 {
		radius = 80
	}
	menu.controls.PadRadius, menu.padDragDistance = radius, distance
}
func (a *app) drawOptionsControls(screen *ebiten.Image, menu *optionsMenu) {
	for _, label := range optionsControlLabels {
		position, ok := a.variables.Vec2Value("OPTIONS_" + label.name + "_TEXT_POS_VAR")
		if ok {
			a.drawOptionsLabel(screen, label.text, position, "OPTIONS_TEXT_SCALE_NORMAL_VAR", menu.controls.selected(label.action), false)
		}
	}
	if position, ok := a.variables.Vec2Value("OPTIONS_CONTROL_TEXT_POS_VAR"); ok {
		a.drawOptionsLabel(screen, "Controls", position, "OPTIONS_TEXT_SCALE_LARGE_VAR", false, true)
	}
	if position, ok := a.variables.Vec2Value("OPTIONS_DEFAULT_ICON_POS_VAR"); ok {
		a.drawOptionsLabel(screen, "Default", position, "OPTIONS_TEXT_SCALE_NORMAL_VAR", false, true)
	}
	for _, stick := range []struct {
		name     string
		movement bool
	}{{"L", menu.controls.Normal}, {"R", !menu.controls.Normal}} {
		position, ok := a.variables.Vec2Value("OPTIONS_" + stick.name + "_STICK_POS_VAR")
		size, sizeOK := a.variables.Vec2Value("OPTIONS_STICK_SIZE_VAR")
		name := "Common0/Textures/Analog_Nub_Gun_SD"
		if stick.movement {
			name = "Common0/Textures/Analog_Nub_Move_SD"
		}
		if ok && sizeOK {
			a.drawOptionsControlTexture(screen, name, position, size)
		}
	}
	if position, ok := a.variables.Vec2Value("OPTIONS_SIZE_CENTER_POS_VAR"); ok {
		size := float64(menu.controls.PadRadius) * 2
		a.drawOptionsControlTexture(screen, "Common0/Textures/Analog_Back_SD", position, formats.Vec2{X: size, Y: size})
		if nubSize, ok := a.variables.Vec2Value("OPTIONS_STICK_SIZE_VAR"); ok {
			a.drawOptionsControlTexture(screen, "Common0/Textures/Analog_Nub_SD", position, nubSize)
		}
	}
}
func (a *app) drawOptionsControlTexture(screen *ebiten.Image, name string, position, size formats.Vec2) {
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	op.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
	op.GeoM.Scale(size.X/float64(bounds.Dx()), size.Y/float64(bounds.Dy()))
	op.GeoM.Translate(position.X, position.Y)
	a.drawImage(screen, texture, op)
}
func (a *app) drawOptionsLabel(screen *ebiten.Image, text string, position formats.Vec2, sizeVariable string, selected, heading bool) {
	if a.font == nil || a.font.Atlas == nil || a.font.LineHeight <= 0 {
		return
	}
	size, ok := a.variables.FloatValue(sizeVariable)
	if !ok {
		return
	}
	scale := size / float64(a.font.LineHeight)
	x, y := position.X-a.fontTextWidth(text, scale)/2, position.Y-size/2
	r, g, b := float32(127.0/255), float32(127.0/255), float32(127.0/255)
	if selected {
		r, g, b = 100.0/255, 228.0/255, 100.0/255
	}
	if heading {
		r, g, b = 210.0/255, 246.0/255, 9.0/255
	}
	for _, ch := range text {
		glyph, ok := a.font.Glyphs[ch]
		if !ok {
			continue
		}
		if glyph.Width > 0 && glyph.Height > 0 {
			rect := image.Rect(glyph.X, glyph.Y, glyph.X+glyph.Width, glyph.Y+glyph.Height)
			op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(x+float64(glyph.XOffset)*scale, y+float64(glyph.YOffset)*scale)
			op.ColorScale.Scale(r, g, b, 1)
			a.drawImage(screen, a.font.Atlas.SubImage(rect).(*ebiten.Image), op)
		}
		x += float64(glyph.XAdvance) * scale
	}
}
