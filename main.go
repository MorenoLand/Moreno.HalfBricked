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
	"github.com/MorenoLand/Moreno.HalfBricked/engine/viewer"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type app struct {
	pack   *content.Pack
	levels []formats.LevelInfo
	page   int
	world  int
	level  int
	images map[string]*ebiten.Image
	view   *viewer.Viewer
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
	return &app{pack: pack, levels: pack.List(), images: map[string]*ebiten.Image{}}, nil
}
func (a *app) Update() error {
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
		switch a.page {
		case 0:
			a.page = 1
			a.world = 0
		case 1:
			a.page = 2
			a.level = 0
		case 2:
			return a.openViewer()
		}
	}
	return nil
}
func (a *app) Draw(screen *ebiten.Image) {
	if a.view != nil {
		a.view.Draw(screen)
		return
	}
	screen.Fill(colorDark)
	ebitenutil.DebugPrintAt(screen, "HALFBRICKED", 32, 24)
	ebitenutil.DebugPrintAt(screen, "MAIN MENU", 32, 44)
	items := a.items()
	for i, item := range items {
		prefix := "  "
		if i == a.cursor() {
			prefix = "> "
		}
		ebitenutil.DebugPrintAt(screen, prefix+item, 48, 96+i*32)
	}
	ebitenutil.DebugPrintAt(screen, "UP/DOWN SELECT   ENTER OPEN   ESC BACK", 24, 292)
}
func (a *app) Layout(_, _ int) (int, int) { return 480, 320 }
func (a *app) items() []string {
	if a.page == 0 {
		return []string{"START / MAP VIEWER", "QUIT"}
	}
	if a.page == 1 {
		var result []string
		for _, world := range a.worlds() {
			result = append(result, fmt.Sprintf("WORLD %d", world+1))
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
		if item.WorldIndex == worlds[a.world] {
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
		a.world = clamp(a.world+delta, 0, 1)
		return
	}
	if a.page == 1 {
		a.world = clamp(a.world+delta, 0, len(a.worlds())-1)
		return
	}
	a.level = clamp(a.level+delta, 0, len(a.filteredLevels())-1)
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
