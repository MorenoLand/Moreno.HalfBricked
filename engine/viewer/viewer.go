package viewer

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"sort"

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
	for _, kind := range formats.RenderLayerKinds {
		if v.Layers[kind] {
			v.drawLayer(screen, kind, tileSize)
		}
	}
	if v.Layers[formats.LayerC] {
		v.drawCollision(screen, tileSize)
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
	tileID := id & 0xffff
	if cols <= 0 || rows <= 0 || uint64(tileID) >= uint64(cols*rows) {
		return false
	}
	tileX, tileY := int(tileID)%cols, int(tileID)/cols
	uvOffset := float32(v.TileSet.UVOffset)
	sourceX0 := float32(tileX*tileSize) + uvOffset
	sourceY0 := float32(tileY*tileSize) + uvOffset
	sourceX1 := float32((tileX+1)*tileSize) - uvOffset
	sourceY1 := float32((tileY+1)*tileSize) - uvOffset
	if id&0x00010000 != 0 {
		sourceX0, sourceX1 = sourceX1, sourceX0
	}
	if id&0x00020000 != 0 {
		sourceY0, sourceY1 = sourceY1, sourceY0
	}
	destinationX0 := float32((float64(x*tileSize) - v.CameraX) * v.Zoom)
	destinationY0 := float32((float64(y*tileSize) - v.CameraY) * v.Zoom)
	destinationX1 := destinationX0 + float32(tileSize)*float32(v.Zoom)
	destinationY1 := destinationY0 + float32(tileSize)*float32(v.Zoom)
	vertices := []ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: sourceX0, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: sourceX1, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: sourceX0, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: sourceX1, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
	screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 3, 2}, v.Atlas, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest})
	return true
}
func (v *Viewer) drawFallback(screen *ebiten.Image, x, y, tileSize int, kind formats.LayerKind) {
	colors := map[formats.LayerKind]color.Color{formats.LayerG: color.RGBA{46, 72, 48, 255}, formats.LayerD: color.RGBA{82, 70, 45, 255}, formats.LayerH: color.RGBA{70, 50, 90, 180}, formats.LayerHB: color.RGBA{50, 85, 100, 180}, formats.LayerC: color.RGBA{130, 45, 45, 180}}
	ebitenutil.DrawRect(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom, float64(tileSize)*v.Zoom, float64(tileSize)*v.Zoom, colors[kind])
}
func (v *Viewer) drawCollision(screen *ebiten.Image, tileSize int) {
	for index, id := range v.Level.Layers[formats.LayerC] {
		if id == math.MaxUint32 {
			continue
		}
		x, y := index%v.Level.Width, index/v.Level.Width
		ebitenutil.DrawRect(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom, (float64(y*tileSize)-v.CameraY)*v.Zoom, float64(tileSize)*v.Zoom, float64(tileSize)*v.Zoom, color.RGBA{220, 45, 45, 75})
	}
}
func (v *Viewer) drawProps(screen *ebiten.Image) {
	props := append([]formats.Prop(nil), v.Level.Props...)
	sort.SliceStable(props, func(i, j int) bool { return props[i].Y+props[i].Height < props[j].Y+props[j].Height })
	for _, prop := range props {
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
		sourceX0, sourceY0 := prop.UV1X, prop.UV1Y
		sourceX1, sourceY1 := prop.UV2X, prop.UV2Y
		if sourceX1 <= sourceX0 || sourceY1 <= sourceY0 {
			sourceX0, sourceY0 = 0, 0
			sourceX1, sourceY1 = float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy())
		}
		destinationX0 := float32((prop.X - v.CameraX) * v.Zoom)
		destinationY0 := float32((prop.Y - v.CameraY) * v.Zoom)
		destinationX1 := destinationX0 + float32(scaleX*v.Zoom*float64(v.tileSize()))
		destinationY1 := destinationY0 + float32(scaleY*v.Zoom*float64(v.tileSize()))
		vertices := []ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: float32(sourceX0), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: float32(sourceX1), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: float32(sourceX0), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: float32(sourceX1), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
		screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest})
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
	if v.TileSet.TileShift > 0 {
		return 1 << v.TileSet.TileShift
	}
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
	for _, kind := range []formats.LayerKind{formats.LayerC, formats.LayerH, formats.LayerD, formats.LayerHB, formats.LayerG} {
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
