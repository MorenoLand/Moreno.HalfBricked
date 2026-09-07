package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
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
	page          int
	world         int
	level         int
	mode          int
	debug         bool
	mobile        bool
	images        map[string]*ebiten.Image
	sources       map[string]image.Image
	view          *viewer.Viewer
	play          *playState
	font          *ui.Font
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

type playState struct {
	world                                                *viewer.Viewer
	x, y                                                 float64
	time                                                 float64
	moving                                               bool
	angle                                                int
	tileSize                                             int
	radius                                               float64
	flash                                                float64
	stick                                                int
	leftBaseX, leftBaseY, leftDeflectX, leftDeflectY     float64
	rightBaseX, rightBaseY, rightDeflectX, rightDeflectY float64
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
	game := &app{pack: pack, levels: pack.List(), debug: debug, mobile: mobile || engine.IsMobileDevice(), images: map[string]*ebiten.Image{}, sources: map[string]image.Image{}, startupFrames: 45}
	game.font, _ = loadFont(pack)
	return game, nil
}
func (a *app) Update() error {
	if a.startupFrames > 0 {
		a.startupFrames--
		return nil
	}
	a.menuTime += 1.0 / 60.0
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
			a.play = nil
			return nil
		}
		x, y := a.pointer()
		a.play.Update(x, y, ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft), inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft), a.mobile)
		return nil
	}
	if ebiten.IsKeyPressed(ebiten.KeyEscape) {
		if a.page == 0 {
			return ebiten.Termination
		}
		a.page--
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		a.move(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		a.move(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyKPEnter) {
		return a.activate()
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		_, y := a.pointer()
		index := (y - 96) / 28
		if index >= 0 && index < len(a.items()) {
			a.setCursor(index)
			return a.activate()
		}
	}
	if a.page < 3 {
		_, y := a.pointer()
		index := (y - 96) / 28
		if index >= 0 && index < len(a.items()) {
			a.setCursor(index)
		}
	}
	return nil
}
func (a *app) Draw(screen *ebiten.Image) {
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
	}
	screen.Fill(colorDark)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(float64(screen.Bounds().Dx())/logicalWidth, float64(screen.Bounds().Dy())/logicalHeight)
	screen.DrawImage(a.canvas, options)
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
	a.drawBackdrop(screen)
	if a.page == 0 {
		a.drawTexture(screen, "Frontend0/Textures/Ageofzombies", 112, 12, .5)
		a.drawBarryMenu(screen)
	} else {
		a.drawTexture(screen, "Frontend0/Textures/Ageofzombies", 24, 10, .25)
	}
	items := a.items()
	for i, item := range items {
		y := 96 + i*28
		a.drawMenuButton(screen, 34, float64(y), i == a.cursor())
		a.text(screen, item, 48, float64(y), .5)
	}
	a.drawDetails(screen)
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
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(.25, .25)
	options.GeoM.Translate(x, y)
	screen.DrawImage(source, options)
}
func (a *app) items() []string {
	if a.page == 0 {
		return []string{"PLAY", "SURVIVAL", "LEADERBOARDS", "OPTIONS", "QUIT"}
	}
	if a.page == 1 {
		var result []string
		worldNames := []string{"PREHISTORIC", "1930S CHICAGO", "ANCIENT EGYPT", "FEUDAL JAPAN", "THE FUTURE", "THE WESTERN FRONTIER"}
		for _, world := range a.worlds() {
			label := fmt.Sprintf("WORLD %d", world+1)
			if world >= 0 && world < len(worldNames) {
				label += " / " + worldNames[world]
			}
			result = append(result, label)
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
func (a *app) cursor() int {
	if a.page == 2 {
		return a.level
	}
	return a.world
}
func (a *app) move(delta int) {
	if a.page == 0 {
		a.world = clamp(a.world+delta, 0, len(a.items())-1)
		return
	}
	if a.page == 1 {
		a.world = clamp(a.world+delta, 0, len(a.worlds())-1)
		return
	}
	a.level = clamp(a.level+delta, 0, len(a.filteredLevels())-1)
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
			a.mode, a.page, a.world = 0, 1, 0
		case 1:
			a.mode, a.page, a.world = 1, 1, 0
		case 4:
			return ebiten.Termination
		}
	case 1:
		a.page, a.level = 2, 0
	case 2:
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
		options := &ebiten.DrawImageOptions{}
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
	a.play.world.DrawWithEntities(screen, func(target *ebiten.Image) {
		a.drawBarry(target, screenX, screenY, 1, frame, a.play.angle)
		if a.play.flash > 0 {
			a.drawBarryFlash(target, screenX, screenY, 1, a.play.angle)
		}
	})
	if a.mobile {
		a.drawPlayControls(screen)
	}
}
func (a *app) drawBarry(screen *ebiten.Image, x, y, scale float64, frame, angle int) {
	a.drawBarryPart(screen, "Common0/Textures/Characters/barryidle_SD", x, y, scale, frame, angle)
	a.drawBarryPart(screen, "Common0/Textures/Characters/barrygun_01_SD", x, y, scale, frame, angle)
}
func (a *app) drawBarryPart(screen *ebiten.Image, name string, x, y, scale float64, frame, angle int) {
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	const columns, rows = 8, 4
	cellWidth, cellHeight := texture.Bounds().Dx()/columns, texture.Bounds().Dy()/rows
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	frame = ((frame % rows) + rows) % rows
	angle = ((angle % columns) + columns) % columns
	source := texture.SubImage(image.Rect(angle*cellWidth, frame*cellHeight, (angle+1)*cellWidth, (frame+1)*cellHeight)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(x-float64(cellWidth)*scale/2, y-float64(cellHeight)*scale/2)
	screen.DrawImage(source, options)
}
func (a *app) drawBarryFlash(screen *ebiten.Image, x, y, scale float64, angle int) {
	texture, err := a.Texture("Common0/Textures/Characters/barrygun_01_flash_SD")
	if err != nil {
		return
	}
	const columns = 8
	cellWidth, cellHeight := texture.Bounds().Dx()/columns, texture.Bounds().Dy()
	if cellWidth <= 0 || cellHeight <= 0 {
		return
	}
	angle = ((angle % columns) + columns) % columns
	source := texture.SubImage(image.Rect(angle*cellWidth, 0, (angle+1)*cellWidth, cellHeight)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Scale(scale, scale)
	options.GeoM.Translate(x-float64(cellWidth)*scale/2, y-float64(cellHeight)*scale/2)
	screen.DrawImage(source, options)
}
func (a *app) drawPlayControls(screen *ebiten.Image) {
	if !a.mobile {
		return
	}
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
func loadFont(pack *content.Pack) (*ui.Font, error) {
	manifest := pack.Manifest()
	metadataPath, ok := manifest.Files["Common0/Fonts/font.fnt"]
	if !ok {
		return nil, fmt.Errorf("font metadata not found")
	}
	atlasPath, ok := pack.TexturePath("Common0/Fonts/font_0")
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
	world.Zoom = .5
	tileSize := tileSizeFor(tileset)
	spawnX, spawnY := spawnPosition(level, tileSize)
	a.play = &playState{world: world, x: spawnX, y: spawnY, tileSize: tileSize, radius: playerCollisionRadius}
	a.play.centerCamera()
	return nil
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
func (p *playState) Update(pointerX, pointerY int, pointerDown, pointerJustPressed, mobile bool) {
	tileSize := p.tileSize
	if tileSize <= 0 {
		tileSize = 32
	}
	radius := p.radius
	if radius <= 0 {
		radius = playerCollisionRadius
	}
	p.flash = math.Max(0, p.flash-1.0/60.0)
	if !mobile {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if !pointerDown {
		p.stick = 0
		p.leftDeflectX, p.leftDeflectY, p.rightDeflectX, p.rightDeflectY = 0, 0, 0, 0
	} else if p.stick == 0 && pointerJustPressed {
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
		p.angle = angleFromVector(p.rightDeflectX, p.rightDeflectY)
		if pointerJustPressed {
			p.flash = .1
		}
	}
	if !mobile {
		worldX := (float64(pointerX)-p.world.ViewportX)/p.world.Zoom + p.world.CameraX
		worldY := (float64(pointerY)-p.world.ViewportY)/p.world.Zoom + p.world.CameraY
		if math.Hypot(worldX-p.x, worldY-p.y) > .001 {
			p.angle = angleFromVector(worldX-p.x, worldY-p.y)
		}
		if pointerJustPressed {
			p.flash = .1
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
			p.angle = angleFromVector(dx, dy)
		}
	}
	maxX, maxY := float64(p.world.Level.Width*tileSize), float64(p.world.Level.Height*tileSize)
	p.x = math.Max(float64(tileSize)/2, math.Min(maxX-float64(tileSize)/2, p.x))
	p.y = math.Max(float64(tileSize)/2, math.Min(maxY-float64(tileSize)/2, p.y))
	p.time += 1.0 / 60.0
	p.updateCamera()
}
func stickDeflection(x, y, baseX, baseY float64) (float64, float64) {
	dx, dy := x-baseX, y-baseY
	length := math.Hypot(dx, dy)
	if length > 32 {
		dx, dy = dx/length*32, dy/length*32
	}
	return dx / 32, dy / 32
}
func angleFromVector(x, y float64) int {
	return ((int(math.Round(math.Atan2(-y, x)/(math.Pi/4)))+2)%8 + 8) % 8
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
	p.world.CameraX += (targetX - p.world.CameraX) * 0.15
	p.world.CameraY += (targetY - p.world.CameraY) * 0.15
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

func main() {
	assets := flag.String("assets", "data", "generated cache or reference content directory")
	debug := flag.Bool("debug", false, "enable the diagnostic map viewer and its controls")
	mobile := flag.Bool("mobile", false, "enable the mobile virtual-stick HUD")
	flag.Parse()
	game, err := newApp(*assets, *debug, *mobile)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("HalfBricked")
	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
