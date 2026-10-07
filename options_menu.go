package main

import (
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"image"
)

type optionsMenu struct {
	sound, music         bool
	controls             optionsControls
	compact, padDragging bool
	defaultVisible       bool
	padDragDistance      float32
}
type optionsAction uint8

const (
	optionsNone optionsAction = iota
	optionsSound
	optionsMusic
	optionsBack
	optionsDefault
	optionsNormal
	optionsInvert
	optionsVisible
	optionsHidden
	optionsLeftFixed
	optionsRightFixed
	optionsLeftFloating
	optionsRightFloating
	optionsPad
)

func newOptionsMenu(sound, music bool) optionsMenu {
	return optionsMenu{sound: sound, music: music, controls: nativeOptionsDefaults(false), defaultVisible: true}
}
func (menu *optionsMenu) useDeviceDefaults(mobile bool) {
	menu.defaultVisible, menu.controls.Visible = mobile, mobile
}
func optionsHit(variables formats.FrontendVariables, x, y float64) optionsAction {
	for _, control := range []struct {
		action optionsAction
		name   string
	}{{optionsSound, "SOUND"}, {optionsMusic, "MUSIC"}, {optionsDefault, "DEFAULT"}, {optionsBack, "BACK"}} {
		position, positionOK := variables.Vec2Value("OPTIONS_" + control.name + "_ICON_POS_VAR")
		size, sizeOK := variables.Vec2Value("OPTIONS_" + control.name + "_ICON_SIZE_VAR")
		if positionOK && sizeOK && x >= position.X-size.X/2 && x <= position.X+size.X/2 && y >= position.Y-size.Y/2 && y <= position.Y+size.Y/2 {
			return control.action
		}
	}
	for _, control := range optionsControlLabels {
		position, ok := variables.Vec2Value("OPTIONS_" + control.name + "_TEXT_POS_VAR")
		size, sizeOK := variables.Vec2Value("OPTIONS_TEXT_BUTTONS_SIZE_VAR")
		if ok && sizeOK && x >= position.X-size.X/2 && x <= position.X+size.X/2 && y >= position.Y-size.Y/2 && y <= position.Y+size.Y/2 {
			return control.action
		}
	}
	position, ok := variables.Vec2Value("OPTIONS_SIZE_CENTER_POS_VAR")
	size, sizeOK := variables.Vec2Value("OPTIONS_SIZE_CENTER_SIZE_VAR")
	if ok && sizeOK && x >= position.X-size.X/2 && x <= position.X+size.X/2 && y >= position.Y-size.Y/2 && y <= position.Y+size.Y/2 {
		return optionsPad
	}
	return optionsNone
}
func (menu *optionsMenu) activate(action optionsAction, musicEnabled func(bool)) bool {
	switch action {
	case optionsSound:
		menu.sound = !menu.sound
	case optionsMusic:
		menu.music = !menu.music
		if musicEnabled != nil {
			musicEnabled(menu.music)
		}
	case optionsBack:
		return true
	case optionsDefault:
		menu.sound, menu.music, menu.controls = true, true, nativeOptionsDefaults(menu.compact)
		menu.controls.Visible = menu.defaultVisible
		menu.padDragging = false
		if musicEnabled != nil {
			musicEnabled(true)
		}
	default:
		menu.controls.apply(action)
	}
	return false
}
func optionsSoundRect(music, enabled bool) image.Rectangle {
	x, y := 0, 64
	if music {
		y = 0
	}
	if !enabled {
		x = 64
	}
	return image.Rect(x, y, x+64, y+64)
}
func (a *app) drawOptionsMenu(screen *ebiten.Image, menu *optionsMenu) {
	if texture, err := a.Texture("Common0/Textures/TutorialBoxes"); err == nil {
		for _, name := range []string{"CONTROL", "SOUND", "EXIT"} {
			position, positionOK := a.variables.Vec2Value("OPTIONS_" + name + "_BOX_OUTER_POS_VAR")
			size, sizeOK := a.variables.Vec2Value("OPTIONS_" + name + "_BOX_OUTER_SIZE_VAR")
			if positionOK && sizeOK {
				a.drawOptionsBox(screen, texture, position, size)
			}
		}
		position, positionOK := a.variables.Vec2Value("OPTIONS_CONTROL_BOX_INNER_POS_VAR")
		size, sizeOK := a.variables.Vec2Value("OPTIONS_CONTROL_BOX_INNER_SIZE_VAR")
		if positionOK && sizeOK {
			a.drawOptionsBoxStyle(screen, texture, position, size, 1)
		}
	}
	a.drawOptionsControls(screen, menu)
	for _, control := range []struct {
		name           string
		music, enabled bool
	}{{"SOUND", false, menu.sound}, {"MUSIC", true, menu.music}} {
		position, positionOK := a.variables.Vec2Value("OPTIONS_" + control.name + "_ICON_POS_VAR")
		size, sizeOK := a.variables.Vec2Value("OPTIONS_" + control.name + "_ICON_SIZE_VAR")
		texture, err := a.Texture("Common0/Textures/Sound")
		if err != nil || !positionOK || !sizeOK {
			continue
		}
		rect := optionsSoundRect(control.music, control.enabled)
		if !rect.In(texture.Bounds()) {
			continue
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Translate(-32, -32)
		op.GeoM.Scale(size.X/64, size.Y/64)
		op.GeoM.Translate(position.X, position.Y)
		a.drawImage(screen, texture.SubImage(rect).(*ebiten.Image), op)
	}
	position, positionOK := a.variables.Vec2Value("OPTIONS_BACK_ICON_POS_VAR")
	size, sizeOK := a.variables.Vec2Value("OPTIONS_BACK_ICON_SIZE_VAR")
	if positionOK && sizeOK {
		if texture, err := a.Texture("Common0/Textures/Button_Text_SD"); err == nil {
			rect, ok := buttonTextRect(9)
			rect = resolutionRect(rect, a.pack.TextureSourceScale("Common0/Textures/Button_Text_SD"))
			if ok && rect.In(texture.Bounds()) {
				op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
				op.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
				op.GeoM.Scale(size.X/float64(rect.Dx()), size.Y/float64(rect.Dy()))
				op.GeoM.Translate(position.X, position.Y)
				a.drawImage(screen, texture.SubImage(rect).(*ebiten.Image), op)
			}
		}
	}
}
func (a *app) drawOptionsBox(screen, texture *ebiten.Image, position, size formats.Vec2) {
	a.drawOptionsBoxStyle(screen, texture, position, size, 0)
}
func (a *app) drawOptionsBoxStyle(screen, texture *ebiten.Image, position, size formats.Vec2, style int) {
	quarter := texture.Bounds().Dy() / 4
	sourceX := (style & 0x3ff) * 64
	x, y := size.X/2-8, size.Y/2-8
	w, h := size.X/2-16, (size.Y-32)/2
	for _, quad := range []struct {
		sx, sy, sw, sh int
		x, y, w, h     float64
	}{
		{33, quarter, 16, 16, -x, y, 16, 16}, {48, quarter, 14, 16, -w / 2, y, w, 16}, {48, quarter, 14, 16, w / 2, y, -w, 16}, {33, quarter, 16, 16, x, y, -16, 16},
		{1, 1, 16, 16, -x, -y, 16, 16}, {16, 1, 14, 16, -w / 2, -y, w, 16}, {16, 1, 14, 16, w / 2, -y, -w, 16}, {1, 1, 16, 16, x, -y, -16, 16},
		{1, quarter, 16, 14, -x, -h / 2, 16, h}, {1, quarter, 16, 14, x, -h / 2, -16, h},
		{33, 2, 16, 15, -x, h / 2, 16, h}, {33, 2, 16, 15, x, h / 2, -16, h},
		{16, quarter, 14, 14, 0, 0, size.X - 32, size.Y - 32},
	} {
		rect := image.Rect(sourceX+quad.sx, quad.sy, sourceX+quad.sx+quad.sw, quad.sy+quad.sh)
		if !rect.In(texture.Bounds()) {
			continue
		}
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Translate(-float64(quad.sw)/2, -float64(quad.sh)/2)
		op.GeoM.Scale(quad.w/float64(quad.sw), quad.h/float64(quad.sh))
		op.GeoM.Translate(position.X+quad.x, position.Y+quad.y)
		a.drawImage(screen, texture.SubImage(rect).(*ebiten.Image), op)
	}
}
