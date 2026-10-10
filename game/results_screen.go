package game

import (
	"fmt"
	"image"
	"math"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// The 1.2.5 EndScreen is a compiled UIScreen. Its decorations (connectors, wires,
// screen shines and the light/toggle/dial panels) are drawn from the parsed
// component tree; the older version has no such file and keeps the XML layout.
const resultsScreenPath = "Common0/UserInterface/screens/EndScreen.uiscreen"

func (a *app) resultsUIScreen() *formats.UIScreen {
	if a.resultsUILoaded {
		return a.resultsUI
	}
	a.resultsUILoaded = true
	path, ok := a.pack.SourcePath(resultsScreenPath)
	if !ok {
		return nil
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil
	}
	screen, err := formats.ParseUIScreen(data)
	if err != nil {
		return nil
	}
	a.resultsUI = screen
	return screen
}

type uiDecorSpec struct {
	texture string
	frames  int
}

// decorSpecs maps ComponentAnimatedHudDecor currentValue to its native texture.
var decorSpecs = map[int]uiDecorSpec{
	0: {"HUD_LightGrid", 4}, 1: {"HUD_RedLight", 2}, 2: {"HUD_SliderPanel", 1}, 3: {"HUD_ToggleSwitchOn", 1},
	4: {"HUD_ToggleSwitchOff", 1}, 5: {"HUD_Vent", 1}, 6: {"HUD_HorzLine", 1}, 7: {"HUD_Dial", 1}, 8: {"HUD_HorzLine2", 1},
}

func uiTextureName(path string) string {
	base := strings.TrimSuffix(filepath.Base(strings.ReplaceAll(path, "\\", "/")), filepath.Ext(path))
	return "Common0/Textures/" + base
}

func decorFrame(id, frames int, elapsed float64) int {
	if frames <= 1 {
		return 0
	}
	step := uint32(elapsed / .3)
	hash := uint32(id)*2654435761 ^ step*2246822519
	hash ^= hash >> 15
	return int(hash % uint32(frames))
}

func (a *app) drawUIImage(screen *ebiten.Image, source *ebiten.Image, centerX, centerY, width, height, degrees float64) {
	bounds := source.Bounds()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
	options.GeoM.Scale(width/float64(bounds.Dx()), height/float64(bounds.Dy()))
	options.GeoM.Rotate(degrees * math.Pi / 180)
	options.GeoM.Translate(centerX, centerY)
	a.drawImage(screen, source, options)
}

func uiCenter(component *formats.UIComponent, offset formats.Vec2, width, height float64) (float64, float64) {
	x, y := component.Anchor()
	x, y = x+offset.X, y+offset.Y
	if !component.OriginFromCenter() {
		x, y = x+width/2, y+height/2
	}
	return x, y
}

func (a *app) drawUITexture(screen *ebiten.Image, component *formats.UIComponent, offset formats.Vec2) {
	path := component.Texture()
	if path == "" {
		return
	}
	texture, err := a.Texture(uiTextureName(path))
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	tx, ty, tw, th := component.TexCoords()
	rect := image.Rect(int(math.Round(tx*float64(bounds.Dx()))), int(math.Round(ty*float64(bounds.Dy()))), int(math.Round((tx+tw)*float64(bounds.Dx()))), int(math.Round((ty+th)*float64(bounds.Dy()))))
	rect = rect.Intersect(bounds)
	if rect.Empty() {
		return
	}
	width, hasWidth := component.Width()
	height, hasHeight := component.Height()
	switch {
	case !hasWidth && !hasHeight:
		width, height = float64(rect.Dx()), float64(rect.Dy())
	case !hasWidth:
		width = height * float64(rect.Dx()) / float64(rect.Dy())
	case !hasHeight:
		height = width * float64(rect.Dy()) / float64(rect.Dx())
	}
	x, y := uiCenter(component, offset, width, height)
	a.drawUIImage(screen, texture.SubImage(rect).(*ebiten.Image), x, y, width, height, component.Rotation())
}

func (a *app) drawUIDecor(screen *ebiten.Image, component *formats.UIComponent, offset formats.Vec2, elapsed float64) {
	spec, ok := decorSpecs[component.CurrentValue()]
	if !ok {
		return
	}
	texture, err := a.Texture("Common0/Textures/" + spec.texture)
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	frameWidth := bounds.Dx() / spec.frames
	frame := decorFrame(component.ID, spec.frames, elapsed)
	rect := image.Rect(bounds.Min.X+frame*frameWidth, bounds.Min.Y, bounds.Min.X+(frame+1)*frameWidth, bounds.Max.Y)
	width, hasWidth := component.Width()
	height, hasHeight := component.Height()
	switch {
	case !hasWidth && !hasHeight:
		width, height = float64(rect.Dx()), float64(rect.Dy())
	case !hasWidth:
		width = height * float64(rect.Dx()) / float64(rect.Dy())
	case !hasHeight:
		height = width * float64(rect.Dy()) / float64(rect.Dx())
	}
	x, y := uiCenter(component, offset, width, height)
	a.drawUIImage(screen, texture.SubImage(rect).(*ebiten.Image), x, y, width, height, component.Rotation())
}

// drawResultsBox draws one of the XML-defined panels (SCORE, STATS, MENU, REPLAY).
func (a *app) drawResultsBox(screen *ebiten.Image, texture *ebiten.Image, name string, offset formats.Vec2) {
	for style, part := range []string{"OUTER", "INNER"} {
		position, posOK := a.variables.Vec2Value("ENDSCREEN_" + name + "_BOX_" + part + "_POS_VAR")
		size, sizeOK := a.variables.Vec2Value("ENDSCREEN_" + name + "_BOX_" + part + "_SIZE_VAR")
		if !posOK || !sizeOK {
			continue
		}
		position.X, position.Y = position.X+offset.X, position.Y+offset.Y
		if name == "MENU" || name == "REPLAY" {
			style = 2
		}
		a.drawOptionsBoxStyle(screen, texture, position, size, style)
	}
}

func (a *app) drawResultsUIGroup(screen *ebiten.Image, boxes *ebiten.Image, group *formats.UIComponent, box string, offset formats.Vec2, elapsed float64) {
	var walk func(parent *formats.UIComponent)
	walk = func(parent *formats.UIComponent) {
		for _, child := range parent.Children {
			if !child.Enabled() {
				continue
			}
			switch child.Class {
			case "ComponentWindow":
				if boxes != nil {
					a.drawResultsBox(screen, boxes, box, offset)
				}
				walk(child)
			case "ComponentTexture":
				a.drawUITexture(screen, child, offset)
				walk(child)
			case "ComponentAnimatedHudDecor":
				a.drawUIDecor(screen, child, offset, elapsed)
			case "ComponentVisual":
				walk(child)
			}
		}
	}
	walk(group)
}

func (a *app) drawUIText(screen *ebiten.Image, text string, centerX, centerY, width float64, align string, size float64, offset formats.Vec2) {
	if a.font == nil || a.font.LineHeight == 0 || text == "" {
		return
	}
	scale := size / float64(a.font.LineHeight)
	textWidth := a.fontTextWidth(text, scale)
	x := centerX + offset.X
	switch align {
	case "CenterLeft":
		x -= width / 2
	case "CenterRight":
		x += width/2 - textWidth
	default:
		x -= textWidth / 2
	}
	a.drawFont(screen, a.font, text, x, centerY+offset.Y-size/2, scale)
}

func (a *app) drawResultsText(screen *ebiten.Image, ui *formats.UIScreen, menu *resultsMenu, scoreOffset, mainOffset formats.Vec2) {
	normal, ok := a.variables.FloatValue("ENDSCREEN_TEXT_SCALE_NORMAL_VAR")
	if !ok {
		normal = 22
	}
	small, ok := a.variables.FloatValue("ENDSCREEN_TEXT_SCALE_SMALL_VAR")
	if !ok {
		small = 16
	}
	place := func(parent, name string) (x, y, width float64, align string, found bool) {
		component := ui.FindIn(parent, name)
		if component == nil {
			return 0, 0, 0, "", false
		}
		x, y = component.Anchor()
		width, _ = component.Width()
		return x, y, width, component.String("alignment"), true
	}
	if x, y, width, align, ok := place("StoryScore", "Header"); ok {
		a.drawUIText(screen, "Zombies Destroyed!", x, y, width, align, normal, scoreOffset)
	}
	if x, y, width, align, ok := place("StoryScore", "ScoreTag"); ok {
		a.drawUIText(screen, "Score:", x, y, width, align, normal, scoreOffset)
	}
	if x, y, width, align, ok := place("StoryScore", "Score"); ok {
		mult, gold := menu.popEffect(0)
		a.drawUITextFX(screen, fmt.Sprintf("%d", menu.shownScore()), x, y, width, align, normal, scoreOffset, mult, gold) // port addition: count-up (results_count.go)
	}
	rows := []struct {
		title, value, label string
		amount              *int32
		shown               int32
		pop                 int
	}{
		{"StatTop_Title", "StatTop_Value", "Highscore:", menu.Data.Highscore, menu.shownHighscore(), 2},
		{"StatBottom_Title", "StatBottom_Value", "Zombie Kills:", menu.Data.Kills, menu.shownKills(), 1},
	}
	for _, row := range rows {
		if row.amount == nil {
			continue
		}
		if x, y, width, align, ok := place("StatsBox", row.title); ok {
			a.drawUIText(screen, row.label, x, y, width, align, small, mainOffset)
		}
		if x, y, width, align, ok := place("StatsBox", row.value); ok {
			mult, gold := menu.popEffect(row.pop)
			a.drawUITextFX(screen, fmt.Sprintf("%d", row.shown), x, y, width, align, small, mainOffset, mult, gold)
		}
	}
	labels := []struct {
		group, text string
		offset      formats.Vec2
	}{{"MenuBox", "Menu", mainOffset}, {"ReplayBox", "Replay", mainOffset}}
	if !menu.Data.Survival {
		labels[0].text, labels[1].text = "Main Menu", "Continue"
	}
	if menu.Data.Dead {
		labels[1].text = "Retry"
	}
	for index, label := range labels {
		name := []string{"ENDSCREEN_MENU_BOX_INNER_POS_VAR", "ENDSCREEN_REPLAY_BOX_INNER_POS_VAR"}[index]
		a.drawLegacyResultsText(screen, label.text, name, label.offset, false, false)
	}
}

func (a *app) drawResultsMenuUI(screen *ebiten.Image, ui *formats.UIScreen, menu *resultsMenu, gameTime float64) {
	if texture, err := a.Texture("Common0/Textures/portal_menu_SD"); err == nil {
		bounds := texture.Bounds()
		op := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		op.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
		op.GeoM.Scale(864/float64(bounds.Dx()), 864/float64(bounds.Dy()))
		op.GeoM.Rotate(gameTime / 3)
		op.GeoM.Translate(336, 96)
		a.drawImage(screen, texture, op)
	}
	scoreOffset, mainOffset := menu.offset(a.variables, "SCORE"), menu.offset(a.variables, "MAIN")
	boxes, _ := a.Texture("Common0/Textures/TutorialBoxes")
	for _, group := range []struct {
		component, box string
		offset         formats.Vec2
	}{{"StatsBox", "STATS", mainOffset}, {"ReplayBox", "REPLAY", mainOffset}, {"MenuBox", "MENU", mainOffset}, {"ScoreBox", "SCORE", scoreOffset}} {
		if component := ui.Find(group.component); component != nil {
			a.drawResultsUIGroup(screen, boxes, component, group.box, group.offset, gameTime)
		}
	}
	a.drawResultsText(screen, ui, menu, scoreOffset, mainOffset)
}
