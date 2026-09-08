package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/ui"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type app struct {
	pack          *content.Pack
	levels        []formats.LevelInfo
	variables     formats.FrontendVariables
	page          int
	world         int
	level         int
	mode          int
	debug         bool
	mobile        bool
	titleScreen   bool
	unlocked      map[string]bool
	capture       *engine.Capture
	captureLimit  int
	sound         *engine.SoundSystem
	images        map[string]*ebiten.Image
	sources       map[string]image.Image
	view          *viewer.Viewer
	play          *playState
	font          *ui.Font
	computerFont  *ui.Font
	startupFrames int
	menuTime      float64
	canvas        *ebiten.Image
	splash        *ebiten.Image
	splashLoaded  bool
	outputWidth   int
	outputHeight  int
}

const logicalWidth = 480
const logicalHeight = 320

type bullet struct {
	x, y   float64
	vx, vy float64
	life   float64
	angle  float64
}

type playState struct {
	world                                                *viewer.Viewer
	x, y                                                 float64
	time                                                 float64
	moving                                               bool
	angle                                                int
	flipX                                                bool
	tileSize                                             int
	radius                                               float64
	flash                                                float64
	stick                                                int
	leftBaseX, leftBaseY, leftDeflectX, leftDeflectY     float64
	rightBaseX, rightBaseY, rightDeflectX, rightDeflectY float64
	bullets                                              []bullet
	shootCooldown                                        float64
	paused                                               bool
	shouldQuit                                           bool
	entryScript                                          formats.Script
}

const playerBaseSpeed = 180.0
const playerCollisionRadius = 16.0
const playerCollisionStep = 4.0

func newApp(root string, debug, mobile bool) (*app, error) {
	prepared, err := content.PrepareAssets(root)
	if err != nil {
		return nil, err
	}
	pack, err := content.NewPack(content.NewSource(prepared))
	if err != nil {
		return nil, err
	}
	game := &app{pack: pack, levels: pack.List(), variables: pack.Variables(), debug: debug, mobile: mobile || engine.IsMobileDevice(), titleScreen: true, unlocked: initialUnlocks(pack.List()), sound: engine.NewSoundSystem(pack), images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}, startupFrames: 45}
	game.font, _ = loadFont(pack)
	game.computerFont, _ = loadNamedFont(pack, "Common0/Fonts/ComputerScreen.fnt", "Common0/Fonts/ComputerScreen_0")
	return game, nil
}
func (a *app) Update() error {
	if a.capture != nil && a.captureLimit > 0 && a.capture.Frames() >= uint64(a.captureLimit) {
		return ebiten.Termination
	}
	if a.startupFrames > 0 {
		a.startupFrames--
		return nil
	}
	a.menuTime += 1.0 / 60.0
	if a.titleScreen {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
			a.titleScreen = false
			a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
		}
		return nil
	}
	if a.view != nil {
		if a.view.Back() {
			a.view = nil
			return nil
		}
		a.view.SetInputSize(a.outputWidth, a.outputHeight)
		a.view.Update()
		return nil
	}
	if a.play != nil {
		if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
			if a.play.paused {
				a.play = nil
				return nil
			}
			a.play.paused = true
			return nil
		}
		x, y := a.pointer()
		if a.play.Update(x, y, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), a.mobile) {
			a.sound.Play("audio/sound/sfx/Handgun.ogg", .8)
		}
		if a.play.shouldQuit {
			a.play = nil
			return nil
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if a.page == 2 {
			a.page = 0
		} else if a.page > 0 {
			a.page--
		}
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		a.move(1)
		a.sound.Play("audio/sound/sfx/menu_move.ogg", .7)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		a.move(-1)
		a.sound.Play("audio/sound/sfx/menu_move.ogg", .7)
	}
	if a.page == 2 && inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
		a.move(-1)
		a.sound.Play("audio/sound/sfx/menu_move.ogg", .7)
	}
	if a.page == 2 && inpututil.IsKeyJustPressed(ebiten.KeyRight) {
		a.move(1)
		a.sound.Play("audio/sound/sfx/menu_move.ogg", .7)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
		return a.activate()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		px, py := a.pointer()
		if a.page == 0 {
			if index := a.mainMenuHit(px, py); index >= 0 {
				a.world = mainMenuButtons[index].action
				a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
				return a.activate()
			}
		} else if a.page == 2 {
			if mode := a.levelTabAt(px, py); mode >= 0 {
				a.mode = mode
				a.level = 0
				a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
			} else if index := a.levelHit(px, py); index >= 0 {
				a.level = index
				if a.levelUnlocked(a.filteredLevels()[index]) {
					a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
					return a.activate()
				}
			}
		} else {
			index := (py - 96) / 28
			if index >= 0 && index < len(a.items()) {
				a.setCursor(index)
				a.sound.Play("audio/sound/sfx/menu_select.ogg", .8)
				return a.activate()
			}
		}
	}
	if a.page == 0 {
		px, py := a.pointer()
		if index := a.mainMenuHit(px, py); index >= 0 {
			a.world = mainMenuButtons[index].action
		}
	} else if a.page == 2 {
		px, py := a.pointer()
		if index := a.levelHit(px, py); index >= 0 {
			a.level = index
		}
	} else if a.page < 3 {
		_, y := a.pointer()
		index := (y - 96) / 28
		if index >= 0 && index < len(a.items()) {
			a.setCursor(index)
		}
	}
	return nil
}
func (a *app) mainMenuHit(x, y int) int {
	for i, button := range mainMenuButtons {
		dx, dy := float64(x)-button.cx, float64(y)-button.cy
		cosine, sine := math.Cos(button.angle), math.Sin(button.angle)
		localX, localY := cosine*dx+sine*dy, -sine*dx+cosine*dy
		if math.Abs(localX) <= button.width/2 && math.Abs(localY) <= button.height/2 {
			return i
		}
	}
	return -1
}
func (a *app) Draw(screen *ebiten.Image) {
	// Hide the OS cursor during play so the red reticule crosshair shows instead.
	ebiten.SetCursorMode(ebiten.CursorModeVisible)
	if a.play != nil && !a.mobile {
		ebiten.SetCursorMode(ebiten.CursorModeHidden)
	}
	if a.canvas == nil {
		a.canvas = ebiten.NewImage(logicalWidth, logicalHeight)
	}
	a.canvas.Fill(colorDark)
	if a.startupFrames > 0 {
	} else {
		if a.view != nil {
			a.view.Draw(a.canvas)
		} else if a.play != nil {
			a.drawPlay(a.canvas)
		} else {
			a.drawMenu(a.canvas)
		}
	}
	if a.startupFrames > 0 {
		a.drawStartup(a.canvas)
	} else if a.titleScreen {
		a.drawTitle(a.canvas)
	}
	screen.Fill(colorDark)
	filter := ebiten.FilterNearest
	if a.startupFrames > 0 || a.titleScreen || (a.view == nil && a.play == nil && a.page == 0) {
		filter = ebiten.FilterLinear
	}
	options := &ebiten.DrawImageOptions{Filter: filter}
	options.GeoM.Scale(float64(screen.Bounds().Dx())/logicalWidth, float64(screen.Bounds().Dy())/logicalHeight)
	screen.DrawImage(a.canvas, options)
	if a.capture != nil {
		if err := a.capture.Save(screen, a.captureState()); err != nil {
			log.Printf("capture: %v", err)
			a.capture = nil
		}
	}
}

func (a *app) captureState() string {
	if a.startupFrames > 0 {
		return "loading"
	}
	if a.titleScreen {
		return "title"
	}
	if a.view != nil {
		return "debug-viewer"
	}
	if a.play != nil {
		return "play"
	}
	switch a.page {
	case 0:
		return "main-menu"
	case 1:
		return "world-select"
	case 2:
		return "level-select"
	default:
		return "frontend"
	}
}

func (a *app) setCaptureState(state string) error {
	a.startupFrames = 0
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "loading":
		a.startupFrames = 45
		a.titleScreen = true
	case "title":
		a.titleScreen = true
	case "main-menu":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 0, 0, 0, 0
	case "world-select":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 1, 0, 0, 0
	case "level-select":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
	case "play":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		return a.openPlay()
	case "debug-viewer":
		a.titleScreen, a.page, a.world, a.mode, a.level = false, 2, 0, 0, 0
		return a.openViewer()
	default:
		return fmt.Errorf("unknown capture state %q", state)
	}
	return nil
}
func (a *app) Layout(outsideWidth, outsideHeight int) (int, int) {
	if outsideWidth < 1 {
		outsideWidth = logicalWidth
	}
	if outsideHeight < 1 {
		outsideHeight = logicalHeight
	}
	a.outputWidth, a.outputHeight = outsideWidth, outsideHeight
	return outsideWidth, outsideHeight
}
func (a *app) pointer() (int, int) {
	x, y := ebiten.CursorPosition()
	if a.outputWidth < 1 || a.outputHeight < 1 {
		return x, y
	}
	return x * logicalWidth / a.outputWidth, y * logicalHeight / a.outputHeight
}
func (a *app) drawMenu(screen *ebiten.Image) {
	if a.page == 0 {
		a.drawBackdrop(screen)
		a.drawMainMenuBanner(screen)
		for _, button := range mainMenuButtons {
			a.drawMarqueeButton(screen, button, button.action == a.world)
		}
	} else if a.page == 1 {
		a.drawWorldSelect(screen)
	} else {
		a.drawLevelSelect(screen)
	}
}

func (a *app) drawWorldSelect(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.drawTexture(screen, "Frontend0/Textures/Ageofzombies", 24, 10, .25)
	for i, item := range a.items() {
		y := 96 + i*28
		a.drawMenuButton(screen, 34, float64(y), i == a.cursor())
		a.text(screen, item, 48, float64(y), .5)
	}
	a.drawWorldDetails(screen)
}

func (a *app) drawWorldDetails(screen *ebiten.Image) {
	worlds := a.worlds()
	if a.world < 0 || a.world >= len(worlds) {
		return
	}
	world := worlds[a.world]
	a.text(screen, fmt.Sprintf("WORLD %d", world+1), 330, 104, .5)
	if world >= 0 && world < 5 {
		a.drawTexture(screen, fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", world+1), 320, 122, 1)
	}
}

func (a *app) drawTitle(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.drawBanner(screen, "SPLASHSCREENS_AOZ_BANNER_POS_VAR", "SPLASHSCREENS_AOZ_BANNER_SIZE_VAR", .4)
	a.drawTitleBarry(screen)
	if image, err := a.Texture("Frontend0/Textures/menu_zombie_2_SD"); err == nil {
		base, ok := a.variables.FloatValue("SPLASHSCREENS_ZOMBIE_BASE_DIST_VAR")
		if !ok {
			return
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Scale(.5, .5)
		options.GeoM.Translate(logicalWidth-base/2-float64(image.Bounds().Dx())*.25, 178-float64(image.Bounds().Dy())*.25)
		screen.DrawImage(image, options)
	}
	a.textCentered(screen, "Touch to Start", 290, .72)
}

type titleBarryPiece struct {
	source      image.Rectangle
	x, y, angle float64
}

var titleBarryPieces = []titleBarryPiece{
	{source: image.Rect(0, 0, 342, 512), x: 110, y: 190},
	{source: image.Rect(342, 152, 512, 512), x: 164, y: 232, angle: math.Pi/2 - .25},
	{source: image.Rect(342, 0, 409, 152), x: 206, y: 222, angle: math.Pi/2 - .25},
	{source: image.Rect(409, 0, 512, 152), x: 85, y: 265, angle: math.Pi/2 - .25},
}

func (a *app) drawTitleBarry(screen *ebiten.Image) {
	texture, err := a.Texture("Frontend0/Textures/Barry")
	if err != nil {
		return
	}
	phase := 2 * math.Pi * (a.menuTime * 28000 / 65536)
	xOffset := math.Sin(phase+2*math.Pi*.5*28000/65536) * 1.5
	yOffset := math.Sin(phase) * 1.5
	for _, piece := range titleBarryPieces {
		part := texture.SubImage(piece.source).(*ebiten.Image)
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Translate(-float64(piece.source.Dx())/2, -float64(piece.source.Dy())/2)
		options.GeoM.Scale(.5, .5)
		options.GeoM.Rotate(piece.angle)
		options.GeoM.Translate(piece.x+xOffset, piece.y+yOffset)
		screen.DrawImage(part, options)
	}
}

func (a *app) drawBanner(screen *ebiten.Image, positionName, scaleName string, angle float64) {
	texture, err := a.Texture("Frontend0/Textures/Ageofzombies")
	if err != nil {
		return
	}
	position, ok := a.variables.Vec2Value(positionName)
	if !ok {
		return
	}
	scale, ok := a.variables.FloatValue(scaleName)
	if !ok {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(texture.Bounds().Dx())/2, -float64(texture.Bounds().Dy())/2)
	options.GeoM.Scale(scale, scale)
	options.GeoM.Rotate(angle)
	options.GeoM.Translate(position.X, position.Y)
	screen.DrawImage(texture, options)
}

func (a *app) drawLevelSelect(screen *ebiten.Image) {
	a.drawBackdrop(screen)
	a.drawLevelTabs(screen)
	levels := a.filteredLevels()
	centres := []float64{90, 240, 390}
	for index, item := range levels {
		if index >= len(centres) {
			break
		}
		a.drawLevelCardAt(screen, item, centres[index], 100, index == a.level, a.levelUnlocked(item))
	}
	if a.level >= 0 && a.level < len(levels) {
		a.drawLevelInfo(screen, levels[a.level])
	}
}

func (a *app) drawLevelTabs(screen *ebiten.Image) {
	texture, err := a.Texture("ShopFront0/Textures/Shop/AOZ_StoreButtons_SD")
	if err != nil {
		return
	}
	position, positionOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_POS_VAR")
	size, sizeOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_SIZE")
	if !positionOK || !sizeOK {
		return
	}
	row := 0
	if a.mode == 1 {
		row = 1
	}
	source := texture.SubImage(image.Rect(0, row*64, 256, row*64+64)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-128, -32)
	options.GeoM.Scale(size.X/256, size.Y/64)
	options.GeoM.Translate(position.X, position.Y)
	screen.DrawImage(source, options)
}

func (a *app) drawLevelCardAt(screen *ebiten.Image, item formats.LevelInfo, x, y float64, selected, unlocked bool) {
	if item.PostcardImage == "" {
		return
	}
	texture, err := a.Texture("ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD")
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-64, -64)
	options.GeoM.Scale(1, 1)
	options.GeoM.Translate(x, y)
	if !selected {
		options.ColorScale.ScaleAlpha(.42)
	}
	if !unlocked {
		options.ColorScale.ScaleAlpha(.58)
	}
	screen.DrawImage(texture, options)
}

func (a *app) drawLevelInfo(screen *ebiten.Image, item formats.LevelInfo) {
	outerPos, ok := a.ndcBox("SHOPFRONT_TEXT_BOX_OUTER_POS_VAR", "SHOPFRONT_TEXT_BOX_OUTER_WIDTH_VAR", "SHOPFRONT_TEXT_BOX_OUTER_HEIGHT_VAR")
	if !ok {
		return
	}
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(screen, texture, image.Rect(0, 0, 64, 64), outerPos)
	}
	innerPos, innerOK := a.ndcBox("SHOPFRONT_TEXT_BOX_INNER_POS_VAR", "SHOPFRONT_TEXT_BOX_INNER_WIDTH_VAR", "SHOPFRONT_TEXT_BOX_INNER_HEIGHT_VAR")
	textX, textY := float64(outerPos.Min.X+24), float64(outerPos.Min.Y+18)
	if innerOK {
		textX, textY = float64(innerPos.Min.X+16), float64(innerPos.Min.Y+14)
	}
	if a.computerFont != nil {
		a.computerFont.Draw(screen, item.DisplayName, textX, textY, .7)
	} else {
		a.text(screen, item.DisplayName, textX, textY, .6)
	}
	if item.Description != "" {
		description := strings.ReplaceAll(item.Description, "\\n", "\n")
		descriptionY := textY + 32
		if innerOK {
			descriptionY = float64(innerPos.Min.Y + 46)
		}
		if a.computerFont != nil {
			a.computerFont.Draw(screen, description, textX, descriptionY, .45)
		} else {
			a.text(screen, description, textX, descriptionY, .45)
		}
	}
	playPosition, playOK := a.variables.Vec2Value("SHOPFRONT_PLAY_ICON_POS_VAR")
	playWidth, playWidthOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_WIDTH_VAR")
	playHeight, playHeightOK := a.variables.FloatValue("SHOPFRONT_PLAY_ICON_HEIGHT_VAR")
	if playOK && playWidthOK && playHeightOK {
		a.drawShopAction(screen, "PLAY", playPosition.X, playPosition.Y, playWidth, playHeight, a.levelUnlocked(item))
	}
	backPosition, backOK := a.variables.Vec2Value("SHOPFRONT_BACK_ICON_NO_GLOBAL_POS_VAR")
	backWidth, backWidthOK := a.variables.FloatValue("SHOPFRONT_BACK_ICON_WIDTH_VAR")
	backHeight, backHeightOK := a.variables.FloatValue("SHOPFRONT_BACK_ICON_HEIGHT_VAR")
	if backOK && backWidthOK && backHeightOK {
		a.drawShopAction(screen, "BACK", backPosition.X, backPosition.Y, backWidth, backHeight, true)
	}
}

func (a *app) ndcBox(positionName, widthName, heightName string) (image.Rectangle, bool) {
	position, positionOK := a.variables.Vec2Value(positionName)
	width, widthOK := a.variables.FloatValue(widthName)
	height, heightOK := a.variables.FloatValue(heightName)
	if !positionOK || !widthOK || !heightOK {
		return image.Rectangle{}, false
	}
	centerX, centerY := position.X*logicalWidth, position.Y*logicalHeight
	return image.Rect(int(centerX-width*logicalWidth/2), int(centerY-height*logicalHeight/2), int(centerX+width*logicalWidth/2), int(centerY+height*logicalHeight/2)), true
}

func (a *app) drawShopAction(screen *ebiten.Image, label string, x, y, width, height float64, enabled bool) {
	if texture, err := a.Texture("Common0/Textures/Backing_Square"); err == nil {
		drawNineSlice(screen, texture, image.Rect(0, 0, 64, 64), image.Rect(int(x-width/2), int(y-height/2), int(x+width/2), int(y+height/2)))
	}
	row := 0
	if label == "BACK" {
		row = 9
	}
	texture, err := a.Texture("Common0/Textures/Button_Text_SD")
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	textRect, ok := buttonTextRect(row)
	if !ok {
		return
	}
	options.GeoM.Translate(-float64(textRect.Dx())/2, -float64(textRect.Dy())/2)
	options.GeoM.Scale(width/float64(textRect.Dx()), height/float64(textRect.Dy()))
	options.GeoM.Translate(x, y)
	if !enabled {
		options.ColorScale.ScaleAlpha(.4)
	}
	screen.DrawImage(texture.SubImage(textRect).(*ebiten.Image), options)
}

func buttonTextRect(row int) (image.Rectangle, bool) {
	switch row {
	case 0:
		return image.Rect(0, 0, 128, 32), true
	case 6:
		return image.Rect(0, 96, 128, 112), true
	case 8:
		return image.Rect(0, 128, 128, 148), true
	case 9:
		return image.Rect(0, 144, 128, 176), true
	case 14:
		return image.Rect(0, 224, 128, 240), true
	default:
		return image.Rectangle{}, false
	}
}

func drawNineSlice(screen, texture *ebiten.Image, source, target image.Rectangle) {
	const edge = 8
	sourceParts := []image.Rectangle{
		image.Rect(source.Min.X, source.Min.Y, source.Min.X+edge, source.Min.Y+edge),
		image.Rect(source.Min.X+edge, source.Min.Y, source.Max.X-edge, source.Min.Y+edge),
		image.Rect(source.Max.X-edge, source.Min.Y, source.Max.X, source.Min.Y+edge),
		image.Rect(source.Min.X, source.Min.Y+edge, source.Min.X+edge, source.Max.Y-edge),
		image.Rect(source.Min.X+edge, source.Min.Y+edge, source.Max.X-edge, source.Max.Y-edge),
		image.Rect(source.Max.X-edge, source.Min.Y+edge, source.Max.X, source.Max.Y-edge),
		image.Rect(source.Min.X, source.Max.Y-edge, source.Min.X+edge, source.Max.Y),
		image.Rect(source.Min.X+edge, source.Max.Y-edge, source.Max.X-edge, source.Max.Y),
		image.Rect(source.Max.X-edge, source.Max.Y-edge, source.Max.X, source.Max.Y),
	}
	targetParts := []image.Rectangle{
		image.Rect(target.Min.X, target.Min.Y, target.Min.X+edge, target.Min.Y+edge),
		image.Rect(target.Min.X+edge, target.Min.Y, target.Max.X-edge, target.Min.Y+edge),
		image.Rect(target.Max.X-edge, target.Min.Y, target.Max.X, target.Min.Y+edge),
		image.Rect(target.Min.X, target.Min.Y+edge, target.Min.X+edge, target.Max.Y-edge),
		image.Rect(target.Min.X+edge, target.Min.Y+edge, target.Max.X-edge, target.Max.Y-edge),
		image.Rect(target.Max.X-edge, target.Min.Y+edge, target.Max.X, target.Max.Y-edge),
		image.Rect(target.Min.X, target.Max.Y-edge, target.Min.X+edge, target.Max.Y),
		image.Rect(target.Min.X+edge, target.Max.Y-edge, target.Max.X-edge, target.Max.Y),
		image.Rect(target.Max.X-edge, target.Max.Y-edge, target.Max.X, target.Max.Y),
	}
	for index := range sourceParts {
		if sourceParts[index].Dx() <= 0 || sourceParts[index].Dy() <= 0 || targetParts[index].Dx() <= 0 || targetParts[index].Dy() <= 0 {
			continue
		}
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Scale(float64(targetParts[index].Dx())/float64(sourceParts[index].Dx()), float64(targetParts[index].Dy())/float64(sourceParts[index].Dy()))
		options.GeoM.Translate(float64(targetParts[index].Min.X), float64(targetParts[index].Min.Y))
		screen.DrawImage(texture.SubImage(sourceParts[index]).(*ebiten.Image), options)
	}
}

type menuButton struct {
	zombie, labelRow, action     int
	cx, cy, width, height, angle float64
}

var mainMenuButtons = []menuButton{
	{zombie: 3, labelRow: 8, action: 2, cx: 76, cy: 105, width: 96, height: 48, angle: -0.10},
	{zombie: 2, labelRow: 0, action: 0, cx: 210, cy: 194, width: 96, height: 48, angle: -0.17},
	{zombie: 3, labelRow: 6, action: 4, cx: 76, cy: 274, width: 96, height: 48, angle: -0.16},
	{zombie: 2, labelRow: 14, action: 3, cx: 377, cy: 264, width: 96, height: 48, angle: 0.02},
}

func (a *app) drawMainMenuBanner(screen *ebiten.Image) {
	a.drawBanner(screen, "MAINMENU_AOZ_BANNER_POS_VAR", "SPLASHSCREENS_AOZ_BANNER_SIZE_VAR", .15)
}

func (a *app) drawMarqueeButton(screen *ebiten.Image, button menuButton, selected bool) {
	zombie, err := a.Texture(fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", button.zombie))
	if err == nil {
		const zombieScale = .65
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Scale(zombieScale, zombieScale)
		options.GeoM.Translate(button.cx-float64(zombie.Bounds().Dx())*zombieScale/2, button.cy-float64(zombie.Bounds().Dy())*zombieScale/2-8)
		screen.DrawImage(zombie, options)
	}
	board, err := a.Texture("Common0/Textures/Button_Screen")
	if selected {
		if flash, flashErr := a.Texture("Common0/Textures/Button_Screen_Flash"); flashErr == nil {
			board = flash
			err = nil
			options := marqueeImageOptions(button, .75, 64, 32)
			screen.DrawImage(board.SubImage(image.Rect(0, 128, 128, 192)).(*ebiten.Image), options)
		}
	}
	if err == nil && !selected {
		screen.DrawImage(board.SubImage(image.Rect(0, 0, 128, 64)).(*ebiten.Image), marqueeImageOptions(button, .75, 64, 32))
	}
	labels, err := a.Texture("Common0/Textures/Button_Text_SD")
	textRect, ok := buttonTextRect(button.labelRow)
	if err != nil || !ok {
		return
	}
	screen.DrawImage(labels.SubImage(textRect).(*ebiten.Image), marqueeImageOptions(button, .75, float64(textRect.Dx())/2, float64(textRect.Dy())/2))
}

func marqueeImageOptions(button menuButton, scale, offsetX, offsetY float64) *ebiten.DrawImageOptions {
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-offsetX, -offsetY)
	options.GeoM.Scale(scale, scale)
	options.GeoM.Rotate(button.angle)
	options.GeoM.Translate(button.cx, button.cy)
	return options
}

func (a *app) drawMenuButton(screen *ebiten.Image, x, y float64, selected bool) {
	texture, err := a.Texture("Common0/Textures/Button_Screen")
	if err != nil {
		return
	}
	frame := 0
	if selected {
		frame = 1
	}
	source := texture.SubImage(image.Rect(0, frame*64, 128, (frame+1)*64)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(.25, .25)
	options.GeoM.Translate(x, y)
	screen.DrawImage(source, options)
}
func (a *app) items() []string {
	if a.page == 0 {
		return []string{"OPTIONS", "PLAY", "QUIT", "STATS"}
	}
	if a.page == 1 {
		var result []string
		for _, world := range a.worlds() {
			result = append(result, fmt.Sprintf("WORLD %d / %s", world+1, a.worldName(world)))
		}
		return result
	}
	var result []string
	for _, item := range a.filteredLevels() {
		result = append(result, strings.ReplaceAll(item.DisplayName, "\n", " / "))
	}
	return result
}
func (a *app) worlds() []int {
	seen := map[int]bool{}
	var result []int
	for _, item := range a.levels {
		if !seen[item.WorldIndex] {
			seen[item.WorldIndex] = true
			result = append(result, item.WorldIndex)
		}
	}
	return result
}
func (a *app) worldName(world int) string {
	for _, item := range a.levels {
		if item.WorldIndex == world && !hasLevelFlag(item, "SURVIVAL") {
			name := item.DisplayName
			if index := strings.Index(name, ":"); index >= 0 {
				name = name[:index]
			}
			return strings.ToUpper(strings.TrimSpace(name))
		}
	}
	return fmt.Sprintf("WORLD %d", world+1)
}
func (a *app) filteredLevels() []formats.LevelInfo {
	worlds := a.worlds()
	if a.world >= len(worlds) {
		return nil
	}
	var result []formats.LevelInfo
	for _, item := range a.levels {
		isSurvival := false
		for _, flag := range item.Flags {
			if flag == "SURVIVAL" {
				isSurvival = true
				break
			}
		}
		if item.WorldIndex == worlds[a.world] && ((a.mode == 1) == isSurvival) {
			result = append(result, item)
		}
	}
	return result
}
func (a *app) levelHit(x, y int) int {
	if y < 34 || y > 166 {
		return -1
	}
	levels := a.filteredLevels()
	for index := 0; index < len(levels) && index < 3; index++ {
		if math.Abs(float64(x)-[]float64{90, 240, 390}[index]) <= 70 {
			return index
		}
	}
	return -1
}
func (a *app) levelTabAt(x, y int) int {
	position, positionOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_POS_VAR")
	size, sizeOK := a.variables.Vec2Value("SHOPFRONT_TOPBUTTONS_SIZE")
	if !positionOK || !sizeOK || float64(x) < position.X-size.X/2 || float64(x) > position.X+size.X/2 || float64(y) < position.Y-size.Y/2 || float64(y) > position.Y+size.Y/2 {
		return -1
	}
	if float64(x) < position.X {
		return 0
	}
	return 1
}
func (a *app) levelUnlocked(item formats.LevelInfo) bool { return a.unlocked[item.ID] }
func hasLevelFlag(item formats.LevelInfo, wanted string) bool {
	for _, flag := range item.Flags {
		if flag == wanted {
			return true
		}
	}
	return false
}
func initialUnlocks(levels []formats.LevelInfo) map[string]bool {
	result := map[string]bool{}
	for _, item := range levels {
		if hasLevelFlag(item, "STARTUNLOCKED") {
			result[item.ID] = true
		}
	}
	return result
}
func (a *app) cursor() int {
	if a.page == 0 {
		return a.mainMenuIndex()
	}
	if a.page == 2 {
		return a.level
	}
	return a.world
}
func (a *app) move(delta int) {
	if a.page == 0 {
		index := clamp(a.mainMenuIndex()+delta, 0, len(mainMenuButtons)-1)
		a.world = mainMenuButtons[index].action
		return
	}
	if a.page == 1 {
		a.world = clamp(a.world+delta, 0, len(a.worlds())-1)
		return
	}
	a.level = clamp(a.level+delta, 0, len(a.filteredLevels())-1)
}
func (a *app) mainMenuIndex() int {
	for index, button := range mainMenuButtons {
		if button.action == a.world {
			return index
		}
	}
	return 1
}
func (a *app) setCursor(index int) {
	if a.page == 2 {
		a.level = index
	} else {
		a.world = index
	}
}
func (a *app) activate() error {
	switch a.page {
	case 0:
		switch a.world {
		case 0:
			a.mode, a.page, a.world, a.level = 0, 2, 0, 0
		case 1:
			a.mode, a.page, a.world, a.level = 1, 2, 0, 0
		case 4:
			return ebiten.Termination
		}
	case 1:
		a.page, a.level = 2, 0
	case 2:
		levels := a.filteredLevels()
		if a.level < 0 || a.level >= len(levels) || !a.levelUnlocked(levels[a.level]) {
			return nil
		}
		if a.debug {
			return a.openViewer()
		}
		return a.openPlay()
	}
	return nil
}
func (a *app) drawDetails(screen *ebiten.Image) {
	if a.page == 0 {
		return
	}
	if a.page == 1 {
		worlds := a.worlds()
		if a.world >= len(worlds) {
			return
		}
		a.text(screen, fmt.Sprintf("WORLD %d", worlds[a.world]+1), 330, 104, .5)
		if worlds[a.world] < 5 {
			a.drawTexture(screen, fmt.Sprintf("Frontend0/Textures/menu_zombie_%d_SD", worlds[a.world]+1), 320, 122, 1)
		}
		return
	}
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return
	}
	item := levels[a.level]
	if item.PostcardImage != "" {
		a.drawLevelCard(screen, item)
	}
	a.text(screen, item.ID, 324, 104, .5)
	a.text(screen, fmt.Sprintf("WORLD %d", item.WorldIndex+1), 324, 124, .5)
	a.text(screen, strings.ReplaceAll(item.Description, "\n", " / "), 324, 156, .5)
}
func (a *app) drawLevelCard(screen *ebiten.Image, item formats.LevelInfo) {
	image, err := a.Texture("ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD")
	if err != nil {
		return
	}
	bounds := image.Bounds()
	scale := math.Min(136/float64(bounds.Dx()), 82/float64(bounds.Dy()))
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(316+(136-float64(bounds.Dx())*scale)/2, 180+(82-float64(bounds.Dy())*scale)/2)
	screen.DrawImage(image, options)
}
func (a *app) drawBackdrop(screen *ebiten.Image) {
	if image, err := a.Texture("Frontend0/Textures/Portal_Menu_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
		options.GeoM.Translate(float64(-image.Bounds().Dx())/2, float64(-image.Bounds().Dy())/2)
		options.GeoM.Rotate(a.menuTime * .3)
		options.GeoM.Translate(240, 160)
		screen.DrawImage(image, options)
	}
}
func (a *app) drawStartup(screen *ebiten.Image) {
	screen.Fill(colorDark)
	if !a.splashLoaded {
		a.splashLoaded = true
		if source, err := a.Source("Common0/Textures/splashscreen"); err == nil {
			a.splash = ebiten.NewImageFromImage(cropSplash(source))
		}
	}
	if a.splash != nil {
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Scale(logicalWidth/float64(a.splash.Bounds().Dx()), logicalHeight/float64(a.splash.Bounds().Dy()))
		screen.DrawImage(a.splash, options)
	} else {
		a.text(screen, "HALFBRICKED", 160, 148, .5)
	}
}
func (a *app) drawBarryMenu(screen *ebiten.Image) {
	image, err := a.Texture("Frontend0/Textures/Barry")
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(image.Bounds().Dx())/2, -float64(image.Bounds().Dy())/2)
	options.GeoM.Scale(.25, .25)
	options.GeoM.Rotate(math.Sin(a.menuTime*2.2) * .015)
	options.GeoM.Translate(416+math.Sin(a.menuTime*1.7), 256+math.Sin(a.menuTime*2.2)*1.5)
	screen.DrawImage(image, options)
}
func (a *app) drawPlay(screen *ebiten.Image) {
	screenX := (a.play.x-a.play.world.CameraX)*a.play.world.Zoom + a.play.world.ViewportX
	screenY := (a.play.y-a.play.world.CameraY)*a.play.world.Zoom + a.play.world.ViewportY
	frame := int(a.play.time*8) % 4
	if a.play.moving {
		frame = int(a.play.time*10) % 4
	}
	const scale = 1.0
	a.play.world.DrawWithEntities(screen, func(target *ebiten.Image) {
		a.drawBarryShadow(target, screenX, screenY, scale)
		a.drawBarry(target, screenX, screenY, scale, frame, a.play.angle, a.play.flipX)
		if a.play.flash > 0 {
			a.drawBarryFlash(target, screenX, screenY, scale, a.play.angle, a.play.flipX)
		}
		a.drawBullets(target)
	})
	a.drawPlayControls(screen)
	if !a.mobile {
		a.drawReticule(screen)
	}
	if a.play.paused {
		ebitenutil.DrawRect(screen, 0, 0, logicalWidth, logicalHeight, color.RGBA{0, 0, 0, 160})
		a.text(screen, "PAUSED", 195, 115, 1.0)
		a.text(screen, "RESUME", 212, 160, 0.5)
		a.text(screen, "QUIT TO MENU", 192, 190, 0.5)
	}
}
func (a *app) drawBullets(screen *ebiten.Image) {
	if a.play == nil || len(a.play.bullets) == 0 {
		return
	}
	bulletImg, err := a.Texture("Common0/Textures/bullet_SD")
	if err != nil {
		return
	}
	texW, texH := float64(bulletImg.Bounds().Dx()), float64(bulletImg.Bounds().Dy())
	zoom := a.play.world.Zoom
	for _, b := range a.play.bullets {
		sx := (b.x-a.play.world.CameraX)*zoom + a.play.world.ViewportX
		sy := (b.y-a.play.world.CameraY)*zoom + a.play.world.ViewportY
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Translate(-texW/2, -texH/2)
		options.GeoM.Rotate(b.angle)
		options.GeoM.Scale(zoom, zoom)
		options.GeoM.Translate(sx, sy)
		screen.DrawImage(bulletImg, options)
	}
}
func barryCellRect(col, frame, numCols, numRows, texW, texH int) image.Rectangle {
	x0 := int(math.Round(float64(col) * float64(texW) / float64(numCols)))
	x1 := int(math.Round(float64(col+1) * float64(texW) / float64(numCols)))
	y0 := int(math.Round(float64(frame) * float64(texH) / float64(numRows)))
	y1 := int(math.Round(float64(frame+1) * float64(texH) / float64(numRows)))
	return image.Rect(x0, y0, x1, y1)
}
func (a *app) drawBarry(screen *ebiten.Image, x, y, scale float64, frame, angle int, flipX bool) {
	bodySheet := "Common0/Textures/Characters/barryidle_SD"
	if a.play != nil && a.play.moving {
		bodySheet = "Common0/Textures/Characters/barryrun_SD"
	}
	a.drawBarryPart(screen, bodySheet, x, y, scale, frame, angle, flipX)
	if angle != 8 {
		a.drawBarryPart(screen, "Common0/Textures/Characters/barrygun_01_SD", x, y, scale, frame, angle, flipX)
	}
}
func (a *app) drawBarryPart(screen *ebiten.Image, name string, x, y, scale float64, frame, angle int, flipX bool) {
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	const columns, rows = 9, 4
	if angle < 0 {
		angle = 0
	} else if angle >= columns {
		angle = columns - 1
	}
	frame = ((frame % rows) + rows) % rows
	texW, texH := texture.Bounds().Dx(), texture.Bounds().Dy()
	rect := barryCellRect(angle, frame, columns, rows, texW, texH)
	cellWidth, cellHeight := float64(rect.Dx()), float64(rect.Dy())
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	source := texture.SubImage(rect).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-cellWidth/2, -cellHeight/2)
	if flipX {
		options.GeoM.Scale(-scale, scale)
	} else {
		options.GeoM.Scale(scale, scale)
	}
	options.GeoM.Translate(x, y)
	screen.DrawImage(source, options)
}
func (a *app) drawBarryFlash(screen *ebiten.Image, x, y, scale float64, angle int, flipX bool) {
	if angle == 8 {
		return
	}
	texture, err := a.Texture("Common0/Textures/Characters/barrygun_01_flash_SD")
	if err != nil {
		return
	}
	const columns, rows = 9, 1
	if angle < 0 {
		angle = 0
	} else if angle >= columns {
		angle = columns - 1
	}
	texW, texH := texture.Bounds().Dx(), texture.Bounds().Dy()
	rect := barryCellRect(angle, 0, columns, rows, texW, texH)
	cellWidth, cellHeight := float64(rect.Dx()), float64(rect.Dy())
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	source := texture.SubImage(rect).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-cellWidth/2, -cellHeight/2)
	if flipX {
		options.GeoM.Scale(-scale, scale)
	} else {
		options.GeoM.Scale(scale, scale)
	}
	options.GeoM.Translate(x, y)
	screen.DrawImage(source, options)
}
func (a *app) drawBarryShadow(screen *ebiten.Image, x, y, scale float64) {
	texture, err := a.Texture("Common0/Textures/shadow_SD")
	if err != nil {
		return
	}
	w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	// Draw an elliptical shadow at Barry's feet: scale narrower vertically, slightly below center.
	const shadowScaleX = 0.45
	const shadowScaleY = 0.20
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-w/2, -h/2)
	options.GeoM.Scale(shadowScaleX*scale, shadowScaleY*scale)
	options.GeoM.Translate(x, y+14*scale)
	options.ColorScale.ScaleAlpha(0.55)
	screen.DrawImage(texture, options)
}
func (a *app) drawReticule(screen *ebiten.Image) {
	texture, err := a.Texture("Common0/Textures/Reticule_SD")
	if err != nil {
		return
	}
	px, py := a.pointer()
	w, h := float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(2, 2)
	options.GeoM.Translate(float64(px)-w, float64(py)-h)
	screen.DrawImage(texture, options)
}
func (a *app) drawPlayControls(screen *ebiten.Image) {
	if a.mobile {
		if a.play.stick == 1 {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Move_SD", a.play.leftBaseX, a.play.leftBaseY, a.play.leftDeflectX, a.play.leftDeflectY)
		} else {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Move_SD", 64, 256, 0, 0)
		}
		if a.play.stick == 2 {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Gun_SD", a.play.rightBaseX, a.play.rightBaseY, a.play.rightDeflectX, a.play.rightDeflectY)
		} else {
			a.drawStick(screen, "Common0/Textures/Analog_Nub_Gun_SD", 416, 256, 0, 0)
		}
	}
	if image, err := a.Texture("Common0/Textures/Pause_Large_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5, .5)
		options.GeoM.Translate(448, 8)
		screen.DrawImage(image, options)
	}
}
func (a *app) drawStick(screen *ebiten.Image, name string, baseX, baseY, deflectX, deflectY float64) {
	if image, err := a.Texture("Common0/Textures/Analog_Back_SD"); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5, .5)
		options.GeoM.Translate(baseX-float64(image.Bounds().Dx())*.25, baseY-float64(image.Bounds().Dy())*.25)
		screen.DrawImage(image, options)
	}
	if image, err := a.Texture(name); err == nil {
		options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
		options.GeoM.Scale(.5, .5)
		options.GeoM.Translate(baseX+deflectX*32-float64(image.Bounds().Dx())*.25, baseY+deflectY*32-float64(image.Bounds().Dy())*.25)
		screen.DrawImage(image, options)
	}
}
func cropSplash(source image.Image) image.Image {
	bounds := source.Bounds()
	top, bottom := bounds.Max.Y, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		if splashRowHasContent(source, y, bounds) {
			top = y
			break
		}
	}
	for y := bounds.Max.Y - 1; y >= bounds.Min.Y; y-- {
		if splashRowHasContent(source, y, bounds) {
			bottom = y + 1
			break
		}
	}
	if top >= bottom {
		return source
	}
	cropped := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bottom-top))
	draw.Draw(cropped, cropped.Bounds(), source, image.Point{X: bounds.Min.X, Y: top}, draw.Src)
	return cropped
}
func splashRowHasContent(source image.Image, y int, bounds image.Rectangle) bool {
	for x := bounds.Min.X; x < bounds.Max.X; x += 8 {
		r, g, b, a := source.At(x, y).RGBA()
		if a > 0 && r+g+b > 24*257 {
			return true
		}
	}
	return false
}
func (a *app) text(screen *ebiten.Image, value string, x, y, scale float64) {
	if a.font != nil {
		a.font.Draw(screen, value, x, y, scale)
		return
	}
	ebitenutil.DebugPrintAt(screen, value, int(x), int(y))
}
func (a *app) textCentered(screen *ebiten.Image, value string, y, scale float64) {
	width := 0.0
	if a.font != nil {
		for _, runeValue := range value {
			if glyph, ok := a.font.Glyphs[runeValue]; ok {
				width += float64(glyph.XAdvance) * scale
			}
		}
	}
	a.text(screen, value, (logicalWidth-width)/2, y, scale)
}
func loadFont(pack *content.Pack) (*ui.Font, error) {
	return loadNamedFont(pack, "Common0/Fonts/font.fnt", "Common0/Fonts/font_0")
}
func loadNamedFont(pack *content.Pack, metadataName, textureName string) (*ui.Font, error) {
	metadataPath, ok := pack.SourcePath(metadataName)
	if !ok {
		return nil, fmt.Errorf("font metadata not found")
	}
	atlasPath, ok := pack.TexturePath(textureName)
	if !ok {
		return nil, fmt.Errorf("font atlas not found")
	}
	metadata, err := pack.Open(metadataPath)
	if err != nil {
		return nil, err
	}
	defer metadata.Close()
	atlasReader, err := pack.Open(atlasPath)
	if err != nil {
		return nil, err
	}
	defer atlasReader.Close()
	data, err := ioReadAll(atlasReader)
	if err != nil {
		return nil, err
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return ui.LoadFont(metadata, ebiten.NewImageFromImage(source))
}
func (a *app) drawTexture(screen *ebiten.Image, name string, x, y, scale float64) {
	image, err := a.Texture(name)
	if err != nil {
		return
	}
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(x, y)
	screen.DrawImage(image, options)
}
func (a *app) openViewer() error {
	level, tileset, atlas, err := a.selectedLevel()
	if err != nil {
		return err
	}
	a.view = viewer.New(level, tileset, atlas, a)
	a.view.Debug = true
	return nil
}
func (a *app) openPlay() error {
	level, tileset, atlas, err := a.selectedLevel()
	if err != nil {
		return err
	}
	world := viewer.New(level, tileset, atlas, a)
	world.Zoom = 1.0
	world.ViewportX = 0
	world.ViewportY = 0
	world.Layers[formats.LayerH] = true
	tileSize := tileSizeFor(tileset)
	spawnX, spawnY := spawnPosition(level, tileSize)
	play := &playState{world: world, x: spawnX, y: spawnY, tileSize: tileSize, radius: playerCollisionRadius}
	if a.mode == 0 {
		script, err := a.pack.Script(entryScriptPath(level.Info))
		if err != nil {
			return err
		}
		play.entryScript = script
	}
	a.play = play
	a.play.centerCamera()
	return nil
}

func entryScriptPath(info formats.LevelInfo) string {
	parts := strings.Split(filepath.ToSlash(info.SourceXML), "/")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0] + "/Scripts/" + info.BaseFile + "_entry.script"
}
func (a *app) selectedLevel() (formats.Level, formats.TileSet, *ebiten.Image, error) {
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return formats.Level{}, formats.TileSet{}, nil, fmt.Errorf("selected level is unavailable")
	}
	level, err := a.pack.Load(levels[a.level].ID)
	if err != nil {
		return formats.Level{}, formats.TileSet{}, nil, err
	}
	tileset, ok := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	if !ok {
		return formats.Level{}, formats.TileSet{}, nil, fmt.Errorf("tileset %q not found", level.Tileset)
	}
	atlas, err := a.Texture(tileset.Texture)
	if err != nil {
		return formats.Level{}, formats.TileSet{}, nil, err
	}
	return level, tileset, atlas, nil
}
func tileSizeFor(tileset formats.TileSet) int {
	if tileset.TileShift > 0 {
		return 1 << tileset.TileShift
	}
	if tileset.TileSize > 0 {
		return tileset.TileSize
	}
	return 32
}
func spawnPosition(level formats.Level, tileSize int) (float64, float64) {
	if level.Layers[formats.LayerC] != nil {
		for y := 0; y < level.Height; y++ {
			for x := 0; x < level.Width; x++ {
				if level.Layers[formats.LayerC][y*level.Width+x] == 2 {
					return float64(x*tileSize + tileSize/2), float64(y*tileSize + tileSize/2)
				}
			}
		}
	}
	return float64(level.Width*tileSize) / 2, float64(level.Height*tileSize) / 2
}
func (p *playState) Update(pointerX, pointerY int, pointerDown, pointerJustPressed, mobile bool) bool {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	radius := p.radius
	if radius <= 0 {
		radius = playerCollisionRadius
	}
	inPauseBtn := pointerX >= 440 && pointerX <= 480 && pointerY >= 0 && pointerY <= 48
	if pointerJustPressed && inPauseBtn {
		p.paused = !p.paused
		return false
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		p.paused = !p.paused
		return false
	}
	if p.paused {
		if pointerJustPressed {
			if pointerX >= 180 && pointerX <= 300 && pointerY >= 150 && pointerY <= 175 {
				p.paused = false
				return false
			}
			if pointerX >= 170 && pointerX <= 310 && pointerY >= 180 && pointerY <= 205 {
				p.shouldQuit = true
				return false
			}
		}
		return false
	}
	p.flash = math.Max(0, p.flash-1.0/60.0)
	p.shootCooldown = math.Max(0, p.shootCooldown-1.0/60.0)
	fired := false

	const dt = 1.0 / 60.0
	activeBullets := p.bullets[:0]
	for _, b := range p.bullets {
		b.x += b.vx * dt
		b.y += b.vy * dt
		b.life -= dt
		if b.life > 0 && !p.isSolid(b.x, b.y) {
			activeBullets = append(activeBullets, b)
		}
	}
	p.bullets = activeBullets

	if !mobile {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if !pointerDown {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if p.stick == 0 && pointerJustPressed && !inPauseBtn {
		if pointerX < logicalWidth/2 {
			p.stick = 1
			p.leftBaseX, p.leftBaseY = clampFloat(float64(pointerX), 32, logicalWidth-32), clampFloat(float64(pointerY), 32, logicalHeight-32)
		} else {
			p.stick = 2
			p.rightBaseX, p.rightBaseY = clampFloat(float64(pointerX), 32, logicalWidth-32), clampFloat(float64(pointerY), 32, logicalHeight-32)
		}
	}
	if mobile && pointerDown && p.stick == 1 {
		p.leftDeflectX, p.leftDeflectY = stickDeflection(float64(pointerX), float64(pointerY), p.leftBaseX, p.leftBaseY)
	}
	if mobile && pointerDown && p.stick == 2 {
		p.rightDeflectX, p.rightDeflectY = stickDeflection(float64(pointerX), float64(pointerY), p.rightBaseX, p.rightBaseY)
	}
	dx, dy := 0.0, 0.0
	if ebiten.IsKeyPressed(ebiten.KeyLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
		dy--
	}
	if ebiten.IsKeyPressed(ebiten.KeyDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
		dy++
	}
	if mobile && p.stick == 1 {
		dx, dy = p.leftDeflectX, p.leftDeflectY
	}
	if mobile && p.stick == 2 && math.Hypot(p.rightDeflectX, p.rightDeflectY) > .5 {
		p.angle, p.flipX = barryDirection(p.rightDeflectX, p.rightDeflectY)
		if p.shootCooldown <= 0 {
			fired = p.fire(p.rightDeflectX, p.rightDeflectY)
		}
	}
	if !mobile {
		worldX := (float64(pointerX)-p.world.ViewportX)/p.world.Zoom + p.world.CameraX
		worldY := (float64(pointerY)-p.world.ViewportY)/p.world.Zoom + p.world.CameraY
		aimDX := worldX - p.x
		aimDY := worldY - p.y
		if math.Hypot(aimDX, aimDY) > .001 {
			p.angle, p.flipX = barryDirection(aimDX, aimDY)
		}
		firing := (pointerDown || ebiten.IsKeyPressed(ebiten.KeySpace)) && !inPauseBtn
		if firing && p.shootCooldown <= 0 {
			fired = p.fire(aimDX, aimDY)
		}
	}
	p.moving = dx != 0 || dy != 0
	if p.moving {
		length := math.Sqrt(dx*dx + dy*dy)
		moveX, moveY := dx/length*playerBaseSpeed/60, dy/length*playerBaseSpeed/60
		stepLength := math.Sqrt(moveX*moveX + moveY*moveY)
		steps := int(math.Ceil(stepLength / playerCollisionStep))
		if steps < 1 {
			steps = 1
		}
		for step := 0; step < steps; step++ {
			candidateX, candidateY := p.x+moveX/float64(steps), p.y+moveY/float64(steps)
			pushX, pushY, hit := p.collisionDisplacement(candidateX, candidateY, radius, tileSize)
			if hit {
				candidateX += pushX
				candidateY += pushY
			}
			p.x, p.y = candidateX, candidateY
		}
		if mobile && (p.stick != 2 || math.Hypot(p.rightDeflectX, p.rightDeflectY) <= .5) {
			p.angle, p.flipX = barryDirection(dx, dy)
		}
	}
	maxX, maxY := float64(p.world.Level.Width*tileSize), float64(p.world.Level.Height*tileSize)
	p.x = math.Max(float64(tileSize)/2, math.Min(maxX-float64(tileSize)/2, p.x))
	p.y = math.Max(float64(tileSize)/2, math.Min(maxY-float64(tileSize)/2, p.y))
	p.time += 1.0 / 60.0
	p.updateCamera()
	return fired
}

func (p *playState) fire(dx, dy float64) bool {
	dist := math.Hypot(dx, dy)
	if dist < 0.0001 {
		return false
	}
	dirX, dirY := dx/dist, dy/dist
	const muzzleOffset = 24.0
	const bulletSpeed = 600.0
	const bulletLife = 0.75
	bx := p.x + dirX*muzzleOffset
	by := p.y + dirY*muzzleOffset
	bvx := dirX * bulletSpeed
	bvy := dirY * bulletSpeed
	bAngle := math.Atan2(dirY, dirX) + math.Pi/2
	p.bullets = append(p.bullets, bullet{
		x:     bx,
		y:     by,
		vx:    bvx,
		vy:    bvy,
		life:  bulletLife,
		angle: bAngle,
	})
	p.flash = 0.08
	p.shootCooldown = 0.25
	return true
}

func (p *playState) isSolid(x, y float64) bool {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	tileX := int(math.Floor(x / float64(tileSize)))
	tileY := int(math.Floor(y / float64(tileSize)))
	return p.collisionValue(tileX, tileY) == 1
}
func stickDeflection(x, y, baseX, baseY float64) (float64, float64) {
	dx, dy := x-baseX, y-baseY
	length := math.Hypot(dx, dy)
	if length > 32 {
		dx, dy = dx/length*32, dy/length*32
	}
	return dx / 32, dy / 32
}
func barryDirection(dx, dy float64) (int, bool) {
	flipX := dx < -0.001
	absX := math.Abs(dx)
	if absX < 0.0001 && math.Abs(dy) < 0.0001 {
		return 0, false
	}
	angleRad := math.Atan2(dy, absX)
	t := (math.Pi/2 - angleRad) / math.Pi
	col := int(math.Round(t * 8.0))
	if col < 0 {
		col = 0
	} else if col > 8 {
		col = 8
	}
	return col, flipX
}
func (p *playState) centerCamera() {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	zoom := p.world.Zoom
	worldWidth := float64(p.world.Level.Width * tileSize)
	worldHeight := float64(p.world.Level.Height * tileSize)
	maxX := math.Max(0, worldWidth-float64(logicalWidth)/zoom)
	maxY := math.Max(0, worldHeight-float64(logicalHeight)/zoom)
	p.world.CameraX = math.Max(0, math.Min(maxX, p.x-float64(logicalWidth)/(2*zoom)))
	p.world.CameraY = math.Max(0, math.Min(maxY, p.y-float64(logicalHeight)/(2*zoom)))
	p.world.ViewportX, p.world.ViewportY = 0, 0
}
func (p *playState) updateCamera() {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	zoom := p.world.Zoom
	worldWidth := float64(p.world.Level.Width * tileSize)
	worldHeight := float64(p.world.Level.Height * tileSize)
	maxX := math.Max(0, worldWidth-float64(logicalWidth)/zoom)
	maxY := math.Max(0, worldHeight-float64(logicalHeight)/zoom)
	targetX := math.Max(0, math.Min(maxX, p.x-float64(logicalWidth)/(2*zoom)))
	targetY := math.Max(0, math.Min(maxY, p.y-float64(logicalHeight)/(2*zoom)))
	p.world.CameraX = math.Max(0, math.Min(maxX, p.world.CameraX+(targetX-p.world.CameraX)*0.15))
	p.world.CameraY = math.Max(0, math.Min(maxY, p.world.CameraY+(targetY-p.world.CameraY)*0.15))
	p.world.ViewportX, p.world.ViewportY = 0, 0
}
func (p *playState) collisionDisplacement(x, y, radius float64, tileSize int) (float64, float64, bool) {
	minX := int(math.Floor((x - radius) / float64(tileSize)))
	maxX := int(math.Floor((x + radius) / float64(tileSize)))
	minY := int(math.Floor((y - radius) / float64(tileSize)))
	maxY := int(math.Floor((y + radius) / float64(tileSize)))
	bestX, bestY, bestPen := 0.0, 0.0, math.Inf(1)
	for tileY := minY; tileY <= maxY; tileY++ {
		for tileX := minX; tileX <= maxX; tileX++ {
			if p.collisionValue(tileX, tileY) != 1 {
				continue
			}
			centerX := (float64(tileX) + .5) * float64(tileSize)
			centerY := (float64(tileY) + .5) * float64(tileSize)
			penX := float64(tileSize)/2 + radius - math.Abs(x-centerX)
			penY := float64(tileSize)/2 + radius - math.Abs(y-centerY)
			if penX <= 0 || penY <= 0 {
				continue
			}
			if penX < penY && penX < bestPen {
				bestX, bestY, bestPen = math.Copysign(penX, x-centerX), 0, penX
			} else if penY < bestPen {
				bestX, bestY, bestPen = 0, math.Copysign(penY, y-centerY), penY
			}
		}
	}
	return bestX, bestY, bestPen != math.Inf(1)
}
func (p *playState) collisionValue(tileX, tileY int) uint32 {
	if tileX < 0 || tileX >= p.world.Level.Width || tileY < 0 || tileY >= p.world.Level.Height {
		return 1
	}
	raw := p.world.Level.Layers[formats.LayerC][tileY*p.world.Level.Width+tileX]
	if raw == ^uint32(0) {
		return 0
	}
	return (raw + 1) & 0xffff
}
func (a *app) Texture(name string) (*ebiten.Image, error) {
	key := strings.ToLower(strings.TrimSuffix(name, ".tex"))
	if image, ok := a.images[key]; ok {
		return image, nil
	}
	path, ok := a.pack.TexturePath(name)
	if !ok {
		return nil, fmt.Errorf("texture %q not found", name)
	}
	reader, err := a.pack.Open(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	data, err := ioReadAll(reader)
	if err != nil {
		return nil, err
	}
	source, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	image := ebiten.NewImageFromImage(source)
	a.sources[key] = source
	a.images[key] = image
	return image, nil
}
func (a *app) Source(name string) (image.Image, error) {
	key := strings.ToLower(strings.TrimSuffix(name, ".tex"))
	if source, ok := a.sources[key]; ok {
		return source, nil
	}
	if _, err := a.Texture(name); err != nil {
		return nil, err
	}
	source, ok := a.sources[key]
	if !ok {
		return nil, fmt.Errorf("texture source %q not found", name)
	}
	return source, nil
}
func ioReadAll(reader interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var data bytes.Buffer
	buffer := make([]byte, 32768)
	for {
		count, err := reader.Read(buffer)
		if count > 0 {
			_, _ = data.Write(buffer[:count])
		}
		if err != nil {
			if err.Error() == "EOF" {
				return data.Bytes(), nil
			}
			return nil, err
		}
	}
}
func clamp(value, low, high int) int {
	if high < low {
		return low
	}
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
func clampFloat(value, low, high float64) float64 {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

var colorDark = color.RGBA{10, 12, 18, 255}

//go:embed resources/icon_16.png
var icon16Bytes []byte

//go:embed resources/icon_32.png
var icon32Bytes []byte

//go:embed resources/icon_48.png
var icon48Bytes []byte

//go:embed resources/icon_64.png
var icon64Bytes []byte

//go:embed resources/icon_128.png
var icon128Bytes []byte

//go:embed resources/icon_256.png
var icon256Bytes []byte

func loadAppIcons() []image.Image {
	var icons []image.Image
	for _, b := range [][]byte{icon16Bytes, icon32Bytes, icon48Bytes, icon64Bytes, icon128Bytes, icon256Bytes} {
		if len(b) == 0 {
			continue
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err == nil {
			icons = append(icons, img)
		}
	}
	return icons
}

func main() {
	assets := flag.String("assets", "data", "generated cache, content directory, or APK")
	debug := flag.Bool("debug", false, "enable the diagnostic map viewer and its controls")
	mobile := flag.Bool("mobile", false, "enable the mobile virtual-stick HUD")
	captureDir := flag.String("capture-dir", "", "write rendered state screenshots to this directory")
	captureEvery := flag.Int("capture-every", 0, "capture every N frames; zero captures only state changes")
	captureState := flag.String("capture-state", "", "start a capture probe at loading, title, main-menu, world-select, level-select, play, or debug-viewer")
	captureFrames := flag.Int("capture-frames", 0, "terminate after this many rendered frames when capturing")
	flag.Parse()
	game, err := newApp(*assets, *debug, *mobile)
	if err != nil {
		log.Fatal(err)
	}
	game.capture, err = engine.NewCapture(*captureDir, *captureEvery)
	if err != nil {
		log.Fatal(err)
	}
	game.captureLimit = *captureFrames
	if *captureState != "" {
		if err := game.setCaptureState(*captureState); err != nil {
			log.Fatal(err)
		}
	}
	defer game.sound.Close()
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("HalfBricked")
	if icons := loadAppIcons(); len(icons) > 0 {
		ebiten.SetWindowIcon(icons)
	}
	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
