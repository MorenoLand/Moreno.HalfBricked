package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/color"
	"image/png"
	"log"
	"strings"

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
	images        map[string]*ebiten.Image
	view          *viewer.Viewer
	font          *ui.Font
	startupFrames int
	menuTime      float64
}

func newApp(root string) (*app, error) {
	prepared, err := content.PrepareAssets(root)
	if err != nil {
		return nil, err
	}
	pack, err := content.NewPack(content.NewSource(prepared))
	if err != nil {
		return nil, err
	}
	game := &app{pack: pack, levels: pack.List(), images: map[string]*ebiten.Image{}, startupFrames: 45}
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
		a.view.Update()
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
		_, y := ebiten.CursorPosition()
		index := (y - 96) / 28
		if index >= 0 && index < len(a.items()) {
			a.setCursor(index)
			return a.activate()
		}
	}
	return nil
}
func (a *app) Draw(screen *ebiten.Image) {
	if a.startupFrames > 0 {
		a.drawStartup(screen)
		return
	}
	if a.view != nil {
		a.view.Draw(screen)
		return
	}
	a.drawBackdrop(screen)
	if a.page == 0 {
		a.drawTexture(screen, "Frontend0/Textures/Ageofzombies", 112, 12, .5)
		a.drawTexture(screen, "Frontend0/Textures/Barry", 352, 192, .25)
	} else {
		a.drawTexture(screen, "Frontend0/Textures/Ageofzombies", 24, 10, .25)
	}
	ebitenutil.DrawRect(screen, 24, 18, 432, 42, color.RGBA{8, 12, 20, 155})
	ebitenutil.DrawLine(screen, 24, 60, 456, 60, color.RGBA{115, 165, 195, 220})
	if a.page == 0 {
		a.text(screen, "HALFBRICKED", 38, 30, .5)
	}
	title := "MAIN MENU"
	if a.page == 1 {
		title = "SELECT WORLD"
	}
	if a.page == 2 {
		title = "SELECT LEVEL"
	}
	a.text(screen, title, 320, 34, .5)
	ebitenutil.DrawRect(screen, 24, 78, 276, 190, color.RGBA{8, 12, 20, 205})
	ebitenutil.DrawRect(screen, 312, 78, 144, 190, color.RGBA{8, 12, 20, 180})
	items := a.items()
	for i, item := range items {
		y := 96 + i*28
		if i == a.cursor() {
			ebitenutil.DrawRect(screen, 36, float64(y-3), 252, 22, color.RGBA{57, 78, 119, 255})
		}
		a.text(screen, item, 48, float64(y), .5)
	}
	a.drawDetails(screen)
	a.text(screen, "UP/DOWN SELECT   ENTER OPEN   ESC BACK", 24, 292, .5)
}
func (a *app) Layout(_, _ int) (int, int) { return 480, 320 }
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
		return a.openViewer()
	}
	return nil
}
func (a *app) drawDetails(screen *ebiten.Image) {
	if a.page == 0 {
		a.text(screen, "Explore converted content", 324, 100, .5)
		a.text(screen, "through the map viewer.", 324, 116, .5)
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
		a.text(screen, "ENTER TO VIEW LEVELS", 324, 260, .5)
		return
	}
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return
	}
	item := levels[a.level]
	a.text(screen, item.ID, 324, 104, .5)
	a.text(screen, fmt.Sprintf("WORLD %d", item.WorldIndex+1), 324, 124, .5)
	a.text(screen, strings.ReplaceAll(item.Description, "\n", " / "), 324, 156, .5)
}
func (a *app) drawBackdrop(screen *ebiten.Image) {
	if image, err := a.Texture("Frontend0/Textures/Portal_Menu_SD"); err == nil {
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Translate(float64(-image.Bounds().Dx())/2, float64(-image.Bounds().Dy())/2)
		options.GeoM.Rotate(a.menuTime * .3)
		options.GeoM.Translate(240, 160)
		screen.DrawImage(image, options)
	}
	ebitenutil.DrawRect(screen, 0, 0, 480, 320, color.RGBA{5, 12, 22, 80})
}
func (a *app) drawStartup(screen *ebiten.Image) {
	screen.Fill(colorDark)
	if image, err := a.Texture("Common0/Textures/splashscreen"); err == nil {
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Translate(float64(480-image.Bounds().Dx())/2, float64(320-image.Bounds().Dy())/2)
		screen.DrawImage(image, options)
	} else {
		a.text(screen, "HALFBRICKED", 160, 148, .5)
	}
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
	levels := a.filteredLevels()
	if a.level >= len(levels) {
		return nil
	}
	level, err := a.pack.Load(levels[a.level].ID)
	if err != nil {
		return err
	}
	tileset := a.pack.Manifest().TileSets[strings.ToLower(level.Tileset)]
	atlas, _ := a.Texture(tileset.Texture)
	a.view = viewer.New(level, tileset, atlas, a)
	return nil
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
	a.images[key] = image
	return image, nil
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

var colorDark = color.RGBA{10, 12, 18, 255}

func main() {
	assets := flag.String("assets", "data", "generated cache or reference content directory")
	flag.Parse()
	game, err := newApp(*assets)
	if err != nil {
		log.Fatal(err)
	}
	ebiten.SetWindowSize(960, 640)
	ebiten.SetWindowTitle("HalfBricked")
	if err := ebiten.RunGame(game); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
