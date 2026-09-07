package viewer

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type TextureProvider interface {
	Texture(name string) (*ebiten.Image, error)
}
type Viewer struct {
	Level                  formats.Level
	TileSet                formats.TileSet
	Atlas                  *ebiten.Image
	Textures               TextureProvider
	CameraX, CameraY, Zoom float64
	Layers                 map[formats.LayerKind]bool
	Props, Grid            bool
	dragging               bool
	lastX, lastY           int
	SelectedX, SelectedY   int
	SelectedID             uint32
	SelectedLayer          formats.LayerKind
}

func New(level formats.Level, tileSet formats.TileSet, atlas *ebiten.Image, textures TextureProvider) *Viewer {
	return &Viewer{Level: level, TileSet: tileSet, Atlas: atlas, Textures: textures, Zoom: .5, Layers: map[formats.LayerKind]bool{formats.LayerG: true, formats.LayerD: true}, Props: true}
}
func (v *Viewer) Back() bool { return inpututil.IsKeyJustPressed(ebiten.KeyEscape) }
func (v *Viewer) Update() {
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		v.toggle(formats.LayerG)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key2) {
		v.toggle(formats.LayerD)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key3) {
		v.toggle(formats.LayerH)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key4) {
		v.toggle(formats.LayerHB)
	}
	if inpututil.IsKeyJustPressed(ebiten.Key5) {
		v.toggle(formats.LayerC)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		v.Props = !v.Props
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		v.Grid = !v.Grid
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		v.fit(480, 320)
	}
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		v.Zoom = clampFloat(v.Zoom*(1+wheelY*.1), .1, 4)
	}
	x, y := ebiten.CursorPosition()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		if !v.dragging {
			v.lastX, v.lastY = x, y
		}
		v.CameraX -= float64(x-v.lastX) / v.Zoom
		v.CameraY -= float64(y-v.lastY) / v.Zoom
		v.lastX, v.lastY, v.dragging = x, y, true
	} else {
		v.dragging = false
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		tileSize := v.tileSize()
		worldX := int(float64(x)/v.Zoom + v.CameraX)
		worldY := int(float64(y)/v.Zoom + v.CameraY)
		v.SelectedX, v.SelectedY = worldX/tileSize, worldY/tileSize
		v.SelectedID, v.SelectedLayer = v.tileAt(v.SelectedX, v.SelectedY)
	}
}
func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{16, 18, 22, 255})
	tileSize := v.tileSize()
	for _, kind := range formats.LayerKinds {
		if v.Layers[kind] {
			v.drawLayer(screen, kind, tileSize)
		}
	}
	if v.Props {
		v.drawProps(screen)
	}
	if v.Grid {
		v.drawGrid(screen, tileSize)
	}
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s %dx%d zoom %.2f tile %d,%d id %d layer %s", v.Level.Info.ID, v.Level.Width, v.Level.Height, v.Zoom, v.SelectedX, v.SelectedY, v.SelectedID, v.SelectedLayer), 4, 4)
	ebitenutil.DebugPrintAt(screen, "1:G 2:D 3:H 4:HB 5:C P:props G:grid R:fit MMB:pan ESC:back", 4, 20)
}
func (v *Viewer) drawLayer(screen *ebiten.Image, kind formats.LayerKind, tileSize int) {
	for index, id := range v.Level.Layers[kind] {
		if id == math.MaxUint32 {
			continue
		}
		x, y := index%v.Level.Width, index/v.Level.Width
		if v.Atlas == nil || !v.drawAtlasTile(screen, id, x, y, tileSize) {
			v.drawFallback(screen, x, y, tileSize, kind)
		}
	}
}
func (v *Viewer) drawAtlasTile(screen *ebiten.Image, id uint32, x, y, tileSize int) bool {
	bounds := v.Atlas.Bounds()
	cols := bounds.Dx() / tileSize
	rows := bounds.Dy() / tileSize
	if cols <= 0 || rows <= 0 || uint64(id) >= uint64(cols*rows) {
		return false
	}
	tileX, tileY := int(id)%cols, int(id)/cols
	source := v.Atlas.SubImage(image.Rect(tileX*tileSize, tileY*tileSize, tileX*tileSize+tileSize, tileY*tileSize+tileSize)).(*ebiten.Image)
	options := &ebiten.DrawImageOptions{}
	options.GeoM.Scale(v.Zoom, v.Zoom)
	options.GeoM.Translate((float64(x*tileSize)-v.CameraX)*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom)
	screen.DrawImage(source, options)
	return true
}
func (v *Viewer) drawFallback(screen *ebiten.Image, x, y, tileSize int, kind formats.LayerKind) {
	colors := map[formats.LayerKind]color.Color{formats.LayerG: color.RGBA{46, 72, 48, 255}, formats.LayerD: color.RGBA{82, 70, 45, 255}, formats.LayerH: color.RGBA{70, 50, 90, 180}, formats.LayerHB: color.RGBA{50, 85, 100, 180}, formats.LayerC: color.RGBA{130, 45, 45, 180}}
	ebitenutil.DrawRect(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom, float64(tileSize)*v.Zoom, float64(tileSize)*v.Zoom, colors[kind])
}
func (v *Viewer) drawProps(screen *ebiten.Image) {
	for _, prop := range v.Level.Props {
		if v.Textures == nil {
			continue
		}
		texture, err := v.Textures.Texture(prop.Texture)
		if err != nil {
			continue
		}
		scaleX, scaleY := prop.ScaleX, prop.ScaleY
		if scaleX == 0 {
			scaleX = 1
		}
		if scaleY == 0 {
			scaleY = 1
		}
		options := &ebiten.DrawImageOptions{}
		options.GeoM.Scale(v.Zoom*scaleX, v.Zoom*scaleY)
		options.GeoM.Translate((prop.X-v.CameraX)*v.Zoom, (prop.Y-v.CameraY)*v.Zoom)
		screen.DrawImage(texture, options)
	}
}
func (v *Viewer) drawGrid(screen *ebiten.Image, tileSize int) {
	for x := 0; x <= v.Level.Width; x++ {
		ebitenutil.DrawLine(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom, -v.CameraY*v.Zoom, (float64(x*tileSize)-v.CameraX)*v.Zoom, (float64(v.Level.Height*tileSize)-v.CameraY)*v.Zoom, color.RGBA{255, 255, 255, 35})
	}
	for y := 0; y <= v.Level.Height; y++ {
		ebitenutil.DrawLine(screen, -v.CameraX*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom, (float64(v.Level.Width*tileSize)-v.CameraX)*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom, color.RGBA{255, 255, 255, 35})
	}
}
func (v *Viewer) fit(width, height int) {
	v.Zoom = math.Min(float64(width)/float64(v.Level.Width*v.tileSize()), float64(height)/float64(v.Level.Height*v.tileSize()))
	v.CameraX, v.CameraY = 0, 0
}
func (v *Viewer) tileSize() int {
	if v.TileSet.TileSize > 0 {
		return v.TileSet.TileSize
	}
	return 32
}
func (v *Viewer) toggle(kind formats.LayerKind) { v.Layers[kind] = !v.Layers[kind] }
func (v *Viewer) tileAt(x, y int) (uint32, formats.LayerKind) {
	if x < 0 || x >= v.Level.Width || y < 0 || y >= v.Level.Height {
		return math.MaxUint32, ""
	}
	index := y*v.Level.Width + x
	for i := len(formats.LayerKinds) - 1; i >= 0; i-- {
		kind := formats.LayerKinds[i]
		if v.Layers[kind] && v.Level.Layers[kind][index] != math.MaxUint32 {
			return v.Level.Layers[kind][index], kind
		}
	}
	return math.MaxUint32, ""
}
func (v *Viewer) DiagnosticJSON() ([]byte, error) {
	return json.MarshalIndent(struct {
		Level   formats.Level              `json:"level"`
		CameraX float64                    `json:"cameraX"`
		CameraY float64                    `json:"cameraY"`
		Zoom    float64                    `json:"zoom"`
		Layers  map[formats.LayerKind]bool `json:"layers"`
	}{v.Level, v.CameraX, v.CameraY, v.Zoom, v.Layers}, "", "  ")
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
