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
	ViewportX, ViewportY   float64
	Layers                 map[formats.LayerKind]bool
	Props, Grid, Debug     bool
	dragging               bool
	lastX, lastY           int
	SelectedX, SelectedY   int
	SelectedID             uint32
	SelectedLayer          formats.LayerKind
	inputWidth             int
	inputHeight            int
}

func New(level formats.Level, tileSet formats.TileSet, atlas *ebiten.Image, textures TextureProvider) *Viewer {
	viewer := &Viewer{Level: level, TileSet: tileSet, Atlas: atlas, Textures: textures, Zoom: .5, Layers: map[formats.LayerKind]bool{formats.LayerG: true, formats.LayerHB: true, formats.LayerD: true}, Props: true}
	viewer.inputWidth, viewer.inputHeight = 480, 320
	viewer.fit(480, 320)
	return viewer
}
func (v *Viewer) Back() bool { return inpututil.IsKeyJustPressed(ebiten.KeyEscape) }
func (v *Viewer) SetInputSize(width, height int) {
	if width > 0 {
		v.inputWidth = width
	}
	if height > 0 {
		v.inputHeight = height
	}
}
func (v *Viewer) pointer() (int, int) {
	x, y := ebiten.CursorPosition()
	if v.inputWidth < 1 || v.inputHeight < 1 {
		return x, y
	}
	return x * 480 / v.inputWidth, y * 320 / v.inputHeight
}
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
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
		v.Debug = !v.Debug
	}
	_, wheelY := ebiten.Wheel()
	if wheelY != 0 {
		v.Zoom = clampFloat(v.Zoom*(1+wheelY*.1), .1, 4)
		v.ViewportX, v.ViewportY = 0, 0
		v.clampCamera()
	}
	x, y := v.pointer()
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
		v.ViewportX, v.ViewportY = 0, 0
		if !v.dragging {
			v.lastX, v.lastY = x, y
		}
		v.CameraX -= float64(x-v.lastX) / v.Zoom
		v.CameraY -= float64(y-v.lastY) / v.Zoom
		v.clampCamera()
		v.lastX, v.lastY, v.dragging = x, y, true
	} else {
		v.dragging = false
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		tileSize := v.tileSize()
		worldX := (float64(x)-v.ViewportX)/v.Zoom + v.CameraX
		worldY := (float64(y)-v.ViewportY)/v.Zoom + v.CameraY
		v.SelectedX, v.SelectedY = int(math.Floor(worldX/float64(tileSize))), int(math.Floor(worldY/float64(tileSize)))
		v.SelectedID, v.SelectedLayer = v.tileAt(v.SelectedX, v.SelectedY)
	}
}
func (v *Viewer) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{16, 18, 22, 255})
	tileSize := v.tileSize()
	for _, kind := range formats.BaseRenderLayerKinds {
		if v.Layers[kind] {
			v.drawLayer(screen, kind, tileSize)
		}
	}
	if v.Props {
		v.drawProps(screen)
	}
	if v.Layers[formats.LayerH] {
		v.drawLayer(screen, formats.LayerH, tileSize)
	}
	if v.Layers[formats.LayerC] {
		v.drawCollision(screen, tileSize)
	}
	if v.Grid {
		v.drawGrid(screen, tileSize)
	}
	if v.Debug {
		ebitenutil.DrawRect(screen, 0, 0, 480, 38, color.RGBA{0, 0, 0, 185})
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s %dx%d zoom %.2f tile %d,%d id %d layer %s", v.Level.Info.ID, v.Level.Width, v.Level.Height, v.Zoom, v.SelectedX, v.SelectedY, v.SelectedID, v.SelectedLayer), 4, 4)
		ebitenutil.DebugPrintAt(screen, "1:G 2:D 3:H 4:HB 5:C P:props G:grid R:fit F1:debug", 4, 20)
	}
}
func (v *Viewer) drawLayer(screen *ebiten.Image, kind formats.LayerKind, tileSize int) {
	minX, minY, maxX, maxY := v.visibleBounds(tileSize)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			id := v.Level.Layers[kind][y*v.Level.Width+x]
			if id == math.MaxUint32 {
				continue
			}
			if v.Atlas == nil || !v.drawAtlasTile(screen, id, x, y, tileSize) {
				v.drawFallback(screen, x, y, tileSize, kind)
			}
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
	uvOffset := float32(0)
	filter := ebiten.FilterNearest
	if v.Zoom < 1 {
		uvOffset = float32(v.TileSet.UVOffset)
		filter = ebiten.FilterLinear
	}
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
	destinationX0 += float32(v.ViewportX)
	destinationY0 := float32((float64(y*tileSize)-v.CameraY)*v.Zoom + v.ViewportY)
	destinationX1 := destinationX0 + float32(tileSize)*float32(v.Zoom)
	destinationY1 := destinationY0 + float32(tileSize)*float32(v.Zoom)
	vertices := []ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: sourceX0, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: sourceX1, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: sourceX0, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: sourceX1, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
	screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 3, 2}, v.Atlas, &ebiten.DrawTrianglesOptions{Filter: filter, DisableMipmaps: true})
	return true
}
func (v *Viewer) drawFallback(screen *ebiten.Image, x, y, tileSize int, kind formats.LayerKind) {
	colors := map[formats.LayerKind]color.Color{formats.LayerG: color.RGBA{46, 72, 48, 255}, formats.LayerD: color.RGBA{82, 70, 45, 255}, formats.LayerH: color.RGBA{70, 50, 90, 180}, formats.LayerHB: color.RGBA{50, 85, 100, 180}, formats.LayerC: color.RGBA{130, 45, 45, 180}}
	ebitenutil.DrawRect(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX, (float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY, float64(tileSize)*v.Zoom, float64(tileSize)*v.Zoom, colors[kind])
}
func (v *Viewer) drawCollision(screen *ebiten.Image, tileSize int) {
	minX, minY, maxX, maxY := v.visibleBounds(tileSize)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			id := v.Level.Layers[formats.LayerC][y*v.Level.Width+x]
			if id == math.MaxUint32 {
				continue
			}
			left := (float64(x*tileSize)-v.CameraX)*v.Zoom + v.ViewportX
			top := (float64(y*tileSize)-v.CameraY)*v.Zoom + v.ViewportY
			size := float64(tileSize) * v.Zoom
			marker := collisionColor((id + 1) & 0xffff)
			ebitenutil.DrawRect(screen, left, top, size, size, color.RGBA{marker.R, marker.G, marker.B, 45})
			ebitenutil.DrawLine(screen, left, top, left+size, top, marker)
			ebitenutil.DrawLine(screen, left+size, top, left+size, top+size, marker)
			ebitenutil.DrawLine(screen, left+size, top+size, left, top+size, marker)
			ebitenutil.DrawLine(screen, left, top+size, left, top, marker)
		}
	}
}
func collisionColor(value uint32) color.RGBA {
	switch {
	case value == 1:
		return color.RGBA{230, 45, 45, 220}
	case value == 2:
		return color.RGBA{255, 165, 0, 220}
	case value == 3:
		return color.RGBA{45, 220, 90, 220}
	case value >= 4 && value <= 9:
		return color.RGBA{230, 45, 220, 220}
	case value >= 13 && value <= 16:
		return color.RGBA{45, 210, 240, 220}
	default:
		return color.RGBA{245, 220, 45, 220}
	}
}
func (v *Viewer) drawProps(screen *ebiten.Image) {
	props := append([]formats.Prop(nil), v.Level.Props...)
	sort.SliceStable(props, func(i, j int) bool {
		return props[i].Y+props[i].Height*float64(v.tileSize()) < props[j].Y+props[j].Height*float64(v.tileSize())
	})
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
		worldWidth := scaleX * float64(v.tileSize())
		worldHeight := scaleY * float64(v.tileSize())
		destinationX0 := float32((prop.X-worldWidth/2-v.CameraX)*v.Zoom + v.ViewportX)
		destinationY0 := float32((prop.Y-prop.Height*float64(v.tileSize())-worldHeight/2-v.CameraY)*v.Zoom + v.ViewportY)
		destinationX1 := destinationX0 + float32(worldWidth*v.Zoom)
		destinationY1 := destinationY0 + float32(worldHeight*v.Zoom)
		vertices := []ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: float32(sourceX0), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: float32(sourceX1), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: float32(sourceX0), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: float32(sourceX1), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
		screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest})
	}
}
func (v *Viewer) drawGrid(screen *ebiten.Image, tileSize int) {
	for x := 0; x <= v.Level.Width; x++ {
		ebitenutil.DrawLine(screen, (float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX, -v.CameraY*v.Zoom+v.ViewportY, (float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX, (float64(v.Level.Height*tileSize)-v.CameraY)*v.Zoom+v.ViewportY, color.RGBA{255, 255, 255, 35})
	}
	for y := 0; y <= v.Level.Height; y++ {
		ebitenutil.DrawLine(screen, -v.CameraX*v.Zoom+v.ViewportX, (float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY, (float64(v.Level.Width*tileSize)-v.CameraX)*v.Zoom+v.ViewportX, (float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY, color.RGBA{255, 255, 255, 35})
	}
}
func (v *Viewer) fit(width, height int) {
	v.Zoom = math.Min(float64(width)/float64(v.Level.Width*v.tileSize()), float64(height)/float64(v.Level.Height*v.tileSize()))
	v.CameraX, v.CameraY = 0, 0
	v.ViewportX = (float64(width) - float64(v.Level.Width*v.tileSize())*v.Zoom) / 2
	v.ViewportY = (float64(height) - float64(v.Level.Height*v.tileSize())*v.Zoom) / 2
}
func (v *Viewer) clampCamera() {
	maxX := math.Max(0, float64(v.Level.Width*v.tileSize())-480/v.Zoom)
	maxY := math.Max(0, float64(v.Level.Height*v.tileSize())-320/v.Zoom)
	v.CameraX = clampFloat(v.CameraX, 0, maxX)
	v.CameraY = clampFloat(v.CameraY, 0, maxY)
}
func (v *Viewer) visibleBounds(tileSize int) (int, int, int, int) {
	minX := int(math.Floor(v.CameraX/float64(tileSize))) - 1
	minY := int(math.Floor(v.CameraY/float64(tileSize))) - 1
	maxX := int(math.Ceil((v.CameraX+480/v.Zoom)/float64(tileSize))) + 1
	maxY := int(math.Ceil((v.CameraY+320/v.Zoom)/float64(tileSize))) + 1
	if minX < 0 {
		minX = 0
	}
	if minY < 0 {
		minY = 0
	}
	if maxX > v.Level.Width {
		maxX = v.Level.Width
	}
	if maxY > v.Level.Height {
		maxY = v.Level.Height
	}
	return minX, minY, maxX, maxY
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
		Level     formats.Level              `json:"level"`
		CameraX   float64                    `json:"cameraX"`
		CameraY   float64                    `json:"cameraY"`
		Zoom      float64                    `json:"zoom"`
		ViewportX float64                    `json:"viewportX"`
		ViewportY float64                    `json:"viewportY"`
		Layers    map[formats.LayerKind]bool `json:"layers"`
	}{v.Level, v.CameraX, v.CameraY, v.Zoom, v.ViewportX, v.ViewportY, v.Layers}, "", "  ")
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
