package viewer

import (
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"sort"
	"strconv"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

type TextureProvider interface {
	Texture(name string) (*ebiten.Image, error)
}

// SourceScaleProvider reports the density of a texture's source art (2 for HD
// assets whose coordinates are authored in SD pixels).
type SourceScaleProvider interface {
	TextureSourceScale(name string) float64
}
type Viewer struct {
	Level                      formats.Level
	TileSet                    formats.TileSet
	Atlas                      *ebiten.Image
	Textures                   TextureProvider
	CameraX, CameraY, Zoom     float64
	// ZoomScale is the port's field-of-view multiplier (0 means 1): Zoom is the base zoom the game asks for times
	// ZoomScale, so SetZoom and BaseZoom keep the game's own zoom values separate from the player's FOV setting.
	ZoomScale float64
	ViewportX, ViewportY       float64
	RenderScaleX, RenderScaleY float64
	AnimationTime              float64
	Layers                     map[formats.LayerKind]bool
	Props, Grid, Debug         bool
	dragging                   bool
	lastX, lastY               int
	SelectedX, SelectedY       int
	SelectedID                 uint32
	SelectedLayer              formats.LayerKind
	inputWidth                 int
	drawTrace                  func(label string)
	inputHeight                int
}

func New(level formats.Level, tileSet formats.TileSet, atlas *ebiten.Image, textures TextureProvider) *Viewer {
	viewer := &Viewer{Level: level, TileSet: tileSet, Atlas: atlas, Textures: textures, Zoom: .5, RenderScaleX: 1, RenderScaleY: 1, Layers: map[formats.LayerKind]bool{formats.LayerG: true, formats.LayerHB: true, formats.LayerD: true, formats.LayerH: true}, Props: true}
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
func (v *Viewer) SetRenderScale(scaleX, scaleY float64) {
	if scaleX > 0 {
		v.RenderScaleX = scaleX
	}
	if scaleY > 0 {
		v.RenderScaleY = scaleY
	}
}
func (v *Viewer) SetZoom(zoom float64) {
	if zoom <= 0 {
		return
	}
	v.Zoom = zoom * v.zoomScale()
	v.clampCamera()
}

func (v *Viewer) zoomScale() float64 {
	if v.ZoomScale <= 0 {
		return 1
	}
	return v.ZoomScale
}

// BaseZoom is the zoom without the FOV scale.
func (v *Viewer) BaseZoom() float64 { return v.Zoom / v.zoomScale() }

// SetZoomScale changes the FOV multiplier and rescales the current zoom with it.
func (v *Viewer) SetZoomScale(scale float64) {
	if scale <= 0 {
		return
	}
	base := v.BaseZoom()
	v.ZoomScale = scale
	v.Zoom = base * scale
}
func (v *Viewer) renderScale() (float64, float64) {
	scaleX, scaleY := v.RenderScaleX, v.RenderScaleY
	if scaleX <= 0 {
		scaleX = 1
	}
	if scaleY <= 0 {
		scaleY = 1
	}
	return scaleX, scaleY
}
func (v *Viewer) pointer() (int, int) {
	x, y := ebiten.CursorPosition()
	if v.inputWidth < 1 || v.inputHeight < 1 {
		return x, y
	}
	return x * 480 / v.inputWidth, y * 320 / v.inputHeight
}
func (v *Viewer) Update() {
	v.AnimationTime += 1.0 / 60.0
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
	if inpututil.IsKeyJustPressed(ebiten.Key0) {
		v.Zoom = 1.0
		v.ViewportX, v.ViewportY = 0, 0
		v.clampCamera()
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
	v.DrawWithEntities(screen, nil)
}
// DrawWithEntities draws the level with every entity in the last depth band (the callback is called once, after the
// y-sorted tile rows and props). The game uses DrawWithEntityBands.
func (v *Viewer) DrawWithEntities(screen *ebiten.Image, entities func(*ebiten.Image)) {
	var bands func(*ebiten.Image, EntityBand)
	if entities != nil {
		bands = func(target *ebiten.Image, band EntityBand) {
			if band.Last {
				entities(target)
			}
		}
	}
	v.DrawWithEntityBands(screen, bands)
	if v.Debug {
		scaleX, scaleY := v.renderScale()
		ebitenutil.DrawRect(screen, 0, 0, 480*scaleX, 38*scaleY, color.RGBA{0, 0, 0, 185})
		ebitenutil.DebugPrintAt(screen, fmt.Sprintf("%s %dx%d zoom %.2f tile %d,%d id %d layer %s", v.Level.Info.ID, v.Level.Width, v.Level.Height, v.Zoom, v.SelectedX, v.SelectedY, v.SelectedID, v.SelectedLayer), int(4*scaleX), int(4*scaleY))
		ebitenutil.DebugPrintAt(screen, "1:G 2:D 3:H 4:HB 5:C P:props G:grid R:fit F1:debug", int(4*scaleX), int(20*scaleY))
	}
}
// allRows is the row range of drawLayer for a whole layer.
const allRows = math.MaxInt32

// drawLayer draws the rows [rowFrom, rowTo) of a tile layer that are inside the camera, as one DrawTriangles batch
// (native draws the H layer as one draw object per row, FUN_000c03d0).
func (v *Viewer) drawLayer(screen *ebiten.Image, kind formats.LayerKind, tileSize, rowFrom, rowTo int) {
	minX, minY, maxX, maxY := v.visibleBounds(tileSize)
	if rowFrom > minY {
		minY = rowFrom
	}
	if rowTo < maxY {
		maxY = rowTo
	}
	if maxY <= minY || maxX <= minX {
		return
	}
	vertices := make([]ebiten.Vertex, 0, (maxX-minX)*(maxY-minY)*4)
	indices := make([]uint16, 0, (maxX-minX)*(maxY-minY)*6)
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			id := v.Level.Layers[kind][y*v.Level.Width+x]
			if !rendersTileWord(kind, id) {
				continue
			}
			if v.Atlas == nil {
				v.drawFallback(screen, x, y, tileSize, kind)
				continue
			}
			tile, ok := v.atlasTileVertices(id, x, y, tileSize)
			scaleX, scaleY := v.renderScale()
			if !ok {
				v.drawFallback(screen, x, y, tileSize, kind)
				continue
			}
			for index := range tile {
				tile[index].DstX = float32(scaleX) * tile[index].DstX
				tile[index].DstY = float32(scaleY) * tile[index].DstY
			}
			snapTileToPixels(&tile)
			base := uint16(len(vertices))
			vertices = append(vertices, tile[:]...)
			indices = append(indices, base, base+1, base+2, base+1, base+3, base+2)
		}
	}
	if len(vertices) == 0 {
		return
	}
	screen.DrawTriangles(vertices, indices, v.Atlas, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest, DisableMipmaps: true})
}
// snapTileToPixels rounds a tile's screen-space corners to whole pixels. Each
// tile's edge is computed from its own position, so neighbouring edges that
// should coincide can differ in the last float bits (or fall mid-pixel) and let
// a black hairline show between tiles while the camera moves or zooms. Rounding
// the shared edge the same way for both neighbours closes the gap.
func snapTileToPixels(tile *[4]ebiten.Vertex) {
	for index := range tile {
		tile[index].DstX = float32(math.Round(float64(tile[index].DstX)))
		tile[index].DstY = float32(math.Round(float64(tile[index].DstY)))
	}
}
func rendersTileWord(kind formats.LayerKind, id uint32) bool {
	return kind == formats.LayerG || (int32(id) >= 0 && ((kind != formats.LayerD && kind != formats.LayerHB) || id != 0))
}
func (v *Viewer) atlasTileVertices(id uint32, x, y, tileSize int) ([4]ebiten.Vertex, bool) {
	var vertices [4]ebiten.Vertex
	bounds := v.Atlas.Bounds()
	sourceTileSize := v.sourceTileSize()
	cols := bounds.Dx() / sourceTileSize
	rows := bounds.Dy() / sourceTileSize
	tileID := id & 0xffff
	if cols <= 0 || rows <= 0 {
		return vertices, false
	}
	tileX, tileY := int(tileID)%cols, (int(tileID)/cols)%rows
	sourceX0 := float32(tileX * sourceTileSize)
	sourceY0 := float32(tileY * sourceTileSize)
	sourceX1 := float32((tileX + 1) * sourceTileSize)
	sourceY1 := float32((tileY + 1) * sourceTileSize)
	if id&0x00010000 != 0 {
		sourceX0, sourceX1 = sourceX1, sourceX0
	}
	if id&0x00020000 != 0 {
		sourceY0, sourceY1 = sourceY1, sourceY0
	}
	destinationX0 := float32((float64(x*tileSize) - v.CameraX) * v.Zoom)
	destinationX0 += float32(v.ViewportX)
	destinationY0 := float32((float64(y*tileSize)-v.CameraY)*v.Zoom + v.ViewportY)
	// The far edge is the next tile's near edge, computed the same way, so
	// neighbours share bit-identical coordinates and cannot leave a seam.
	destinationX1 := float32((float64((x+1)*tileSize)-v.CameraX)*v.Zoom) + float32(v.ViewportX)
	destinationY1 := float32((float64((y+1)*tileSize)-v.CameraY)*v.Zoom + v.ViewportY)
	vertices = [4]ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: sourceX0, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: sourceX1, SrcY: sourceY0, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: sourceX0, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: sourceX1, SrcY: sourceY1, ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
	return vertices, true
}
func (v *Viewer) drawFallback(screen *ebiten.Image, x, y, tileSize int, kind formats.LayerKind) {
	colors := map[formats.LayerKind]color.Color{formats.LayerG: color.RGBA{46, 72, 48, 255}, formats.LayerD: color.RGBA{82, 70, 45, 255}, formats.LayerH: color.RGBA{70, 50, 90, 180}, formats.LayerHB: color.RGBA{50, 85, 100, 180}, formats.LayerC: color.RGBA{130, 45, 45, 180}}
	scaleX, scaleY := v.renderScale()
	ebitenutil.DrawRect(screen, ((float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX)*scaleX, ((float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY)*scaleY, float64(tileSize)*v.Zoom*scaleX, float64(tileSize)*v.Zoom*scaleY, colors[kind])
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
			scaleX, scaleY := v.renderScale()
			left, top, sizeX, sizeY := left*scaleX, top*scaleY, size*scaleX, size*scaleY
			marker := collisionColor((id + 1) & 0xffff)
			ebitenutil.DrawRect(screen, left, top, sizeX, sizeY, color.RGBA{marker.R, marker.G, marker.B, 45})
			ebitenutil.DrawLine(screen, left, top, left+sizeX, top, marker)
			ebitenutil.DrawLine(screen, left+sizeX, top, left+sizeX, top+sizeY, marker)
			ebitenutil.DrawLine(screen, left+sizeX, top+sizeY, left, top+sizeY, marker)
			ebitenutil.DrawLine(screen, left, top+sizeY, left, top, marker)
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

// propDraw is one prop (static or animated) ready to draw.
type propDraw struct {
	texture                      string
	x, y, height, scaleX, scaleY float64
	uv1X, uv1Y, uv2X, uv2Y       float64
	xFrames, yFrames             int
	frameTime                    float64
	animated                     bool
}

// Native draw order constants (Research/native/prop-entity-draw-order-2026-10-12.md; v7 FUN_0006ed18, FUN_000bf6d8).
const (
	depthBuckets = 350 // buckets 0..349
	// depthBucketCount is the per-bucket capacity (40): a full bucket spills into b+1. Not modelled, a single
	// 1.3 px bucket never holds 40 objects in the port's levels.
	depthBucketCount = 40
	depthOrigin      = 140.0
	rowKeyBias       = 0.25
	// hbBucket is the first bucket drawn after the hb layer (FUN_000c0580 runs between bucket 9 and bucket 10).
	hbBucket = 10
	// nativeDefaultZoom is the native camera zoom z of every level start (DAT_00085dfc); see game.nativeZoomFromPort.
	nativeDefaultZoom = 0.65
)

// DepthBucket is the native y-bucket of an object at world y: clamp(int((y - camY) / (2 z) + 140), 0, 349), with
// camY the camera CENTRE y in world px and zNative the native camera zoom (0.65 / BaseZoom). float32 maths as in the
// binary, int() truncates toward zero like vcvt.s32.f32.
func DepthBucket(y, camY, zNative float64) int {
	if zNative <= 0 {
		zNative = nativeDefaultZoom
	}
	v := int((float32(y)-float32(camY))/(2*float32(zNative)) + depthOrigin)
	switch {
	case v < 0:
		return 0
	case v > depthBuckets-1:
		return depthBuckets - 1
	}
	return v
}

// hRowKey is the sort key of H-layer row r (native FUN_000bf6d8: (r + 2) * 32 + 0.25).
func hRowKey(row int) float64 { return float64(row+2)*32 + rowKeyBias }

// propLift is the visual lift of a prop in world px: trunc(height) * 32 (FUN_000c2174, vcvt.s32.f32 then * 32).
func propLift(height float64) float64 { return float64(int(height)) * 32 }

// propKeyY is a prop's sort key (and drawn position) y: the position truncated toward zero.
func propKeyY(y float64) float64 { return float64(int(y)) }

// NativeZoom is the native camera zoom z the buckets are computed with (the port zoom is 0.65 / z, FOV scale excluded).
func (v *Viewer) NativeZoom() float64 {
	base := v.BaseZoom()
	if base <= 0 {
		return nativeDefaultZoom
	}
	return nativeDefaultZoom / base
}

// CameraCentreY is the world y at the centre of the 320 px viewport (native G+0x5096C), shake included.
func (v *Viewer) CameraCentreY() float64 {
	zoom := v.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return v.CameraY + (160-v.ViewportY)/zoom
}

// levelProps is every prop of the level in submission order. Native keeps static and animated props in one list in
// XML order (FUN_000c3134); the port's level data keeps the two kinds in separate slices, so XML interleaving is lost
// and animated props follow the static ones (PORT DIVERGENCE, only a tie inside one bucket can see it). Positions are
// truncated like the native constructor.
func (v *Viewer) levelProps() []propDraw {
	props := make([]propDraw, 0, len(v.Level.Props)+len(v.Level.AnimatedProps))
	for _, prop := range v.Level.Props {
		props = append(props, propDraw{texture: prop.Texture, x: propKeyY(prop.X), y: propKeyY(prop.Y), height: prop.Height, scaleX: prop.ScaleX, scaleY: prop.ScaleY, uv1X: prop.UV1X, uv1Y: prop.UV1Y, uv2X: prop.UV2X, uv2Y: prop.UV2Y})
	}
	for _, prop := range v.Level.AnimatedProps {
		props = append(props, propDraw{texture: prop.Texture, x: propKeyY(prop.X), y: propKeyY(prop.Y), height: prop.Height, scaleX: prop.ScaleX, scaleY: prop.ScaleY, xFrames: prop.XFrames, yFrames: prop.YFrames, frameTime: prop.FrameTime, animated: true})
	}
	return props
}

// propOnScreen reports whether the prop's quad touches the viewport (visibility only, the order never depends on it).
func (v *Viewer) propOnScreen(prop propDraw) bool {
	scaleX, scaleY := prop.scaleX, prop.scaleY
	if scaleX == 0 {
		scaleX = 1
	}
	if scaleY == 0 {
		scaleY = 1
	}
	worldWidth, worldHeight := scaleX*float64(v.tileSize()), scaleY*float64(v.tileSize())
	left := (prop.x - worldWidth/2 - v.CameraX) * v.Zoom
	top := (prop.y - propLift(prop.height) - worldHeight/2 - v.CameraY) * v.Zoom
	return left+worldWidth*v.Zoom > -v.ViewportX && left < 480-v.ViewportX && top+worldHeight*v.Zoom > -v.ViewportY && top < 320-v.ViewportY
}

// EntityBand is one slice of the native y-bucket draw list handed to the entity callback of DrawWithEntityBands: the
// entities whose bucket (Bucket of their raw native position y) is in [LoBucket, HiBucket) are drawn now. An entity
// in the same bucket as a tile row or prop is drawn AFTER it. First is the slice before every bucketed object (it
// also draws the ground list: shadows, train, portals); Last is the slice after the last one (bullets, explosions).
type EntityBand struct {
	LoBucket, HiBucket int
	First, Last        bool
	// CamY and Z are the camera centre y and native zoom the buckets are computed with.
	CamY, Z float64
}

// Bucket is the native bucket of an entity at raw position y in this frame.
func (b EntityBand) Bucket(y float64) int { return DepthBucket(y, b.CamY, b.Z) }

// Contains reports whether an entity at raw position y belongs to this band.
func (b EntityBand) Contains(y float64) bool {
	bucket := b.Bucket(y)
	return bucket >= b.LoBucket && bucket < b.HiBucket
}

// trace reports a draw step to the test hook (nil outside tests).
func (v *Viewer) trace(label string) {
	if v.drawTrace != nil {
		v.drawTrace(label)
	}
}

// depthItem is one drawn level object of the y-bucket list.
type depthItem struct {
	bucket int
	label  string // "hb", "row:N", "prop:texture" (draw trace for the tests)
	draw   func()
}

// hRows is the range of H-layer rows submitted this frame: native FUN_000bf52c takes
// [max(0, (camTop >> 5) - 2), min(((camBottom + 10) >> 5) + 3, mapH + 1)). The port uses its own viewport edges
// (the native 480/aspect*z half extent only widens the culled range, rows outside the screen draw nothing).
func (v *Viewer) hRows() (int, int) {
	zoom := v.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	top := v.CameraY - v.ViewportY/zoom
	bottom := top + 320/zoom
	from := int(math.Floor(float64(int(top))/32)) - 2
	to := int(math.Floor(float64(int(bottom)+10)/32)) + 3
	if from < 0 {
		from = 0
	}
	if to > v.Level.Height {
		to = v.Level.Height
	}
	return from, to
}

// hRowHasTiles reports whether H row r has a drawn tile inside the visible columns.
func (v *Viewer) hRowHasTiles(row, minX, maxX int) bool {
	layer := v.Level.Layers[formats.LayerH]
	if row < 0 || row >= v.Level.Height || len(layer) < (row+1)*v.Level.Width {
		return false
	}
	for x := minX; x < maxX && x < v.Level.Width; x++ {
		if x >= 0 && rendersTileWord(formats.LayerH, layer[row*v.Level.Width+x]) {
			return true
		}
	}
	return false
}

// DrawWithEntityBands draws the level in the native order (Research/native/prop-entity-draw-order-2026-10-12.md):
// layers G, D, then the ground slice (the First band), then one y-bucket list that holds the hb layer (before
// bucket 10), the H-layer rows (key (r+2)*32+0.25), the props (trunc(y), XML order) and the entities (raw position
// y, drawn by the callback in slices between the other objects). Within a bucket: hb, rows, props, entities.
func (v *Viewer) DrawWithEntityBands(screen *ebiten.Image, entities func(*ebiten.Image, EntityBand)) {
	screen.Fill(color.Black)
	tileSize := v.tileSize()
	for _, kind := range formats.BaseRenderLayerKinds {
		if v.Layers[kind] {
			v.trace("layer:" + string(kind))
			v.drawLayer(screen, kind, tileSize, 0, allRows)
		}
	}
	camY, z := v.CameraCentreY(), v.NativeZoom()
	var items []depthItem
	if v.Layers[formats.LayerHB] {
		items = append(items, depthItem{bucket: hbBucket, label: "hb", draw: func() { v.drawLayer(screen, formats.LayerHB, tileSize, 0, allRows) }})
	}
	if v.Layers[formats.LayerH] {
		minX, _, maxX, _ := v.visibleBounds(tileSize)
		from, to := v.hRows()
		for row := from; row < to; row++ {
			if !v.hRowHasTiles(row, minX, maxX) {
				continue
			}
			row := row
			items = append(items, depthItem{bucket: DepthBucket(hRowKey(row), camY, z), label: "row:" + strconv.Itoa(row), draw: func() { v.drawLayer(screen, formats.LayerH, tileSize, row, row+1) }})
		}
	}
	if v.Props && v.Textures != nil {
		for _, prop := range v.levelProps() {
			if !v.propOnScreen(prop) {
				continue
			}
			prop := prop
			items = append(items, depthItem{bucket: DepthBucket(propKeyY(prop.y), camY, z), label: "prop:" + prop.texture, draw: func() { v.drawPropList(screen, []propDraw{prop}) }})
		}
	}
	// Stable: hb first, then rows, then props at equal buckets (their append order), as the native flush does.
	sort.SliceStable(items, func(i, j int) bool { return items[i].bucket < items[j].bucket })
	lo, first := math.MinInt32, true
	for _, item := range items {
		if entities != nil && (first || lo < item.bucket) {
			v.trace(fmt.Sprintf("band:%d..%d", lo, item.bucket))
			entities(screen, EntityBand{LoBucket: lo, HiBucket: item.bucket, First: first, CamY: camY, Z: z})
		}
		first = false
		v.trace(item.label)
		item.draw()
		lo = item.bucket
	}
	if entities != nil {
		v.trace(fmt.Sprintf("band:%d..", lo))
		entities(screen, EntityBand{LoBucket: lo, HiBucket: math.MaxInt32, First: first, Last: true, CamY: camY, Z: z})
	}
	if v.Layers[formats.LayerC] {
		v.drawCollision(screen, tileSize)
	}
	if v.Grid {
		v.drawGrid(screen, tileSize)
	}
}

func (v *Viewer) drawPropList(screen *ebiten.Image, props []propDraw) {
	for _, prop := range props {
		if v.Textures == nil {
			continue
		}
		textureName := prop.texture
		texture, err := v.Textures.Texture(textureName)
		if err != nil {
			textureName = prop.texture + "_SD"
			texture, err = v.Textures.Texture(textureName)
		}
		if err != nil {
			continue
		}
		scaleX, scaleY := prop.scaleX, prop.scaleY
		if scaleX == 0 {
			scaleX = 1
		}
		if scaleY == 0 {
			scaleY = 1
		}
		sourceX0, sourceY0 := prop.uv1X, prop.uv1Y
		sourceX1, sourceY1 := prop.uv2X, prop.uv2Y
		if prop.animated {
			xFrames, yFrames := prop.xFrames, prop.yFrames
			if xFrames <= 0 {
				xFrames = 1
			}
			if yFrames <= 0 {
				yFrames = 1
			}
			frame := 0
			if prop.frameTime > 0 {
				frame = int(math.Floor(v.AnimationTime * 1000 / prop.frameTime))
			}
			frame %= xFrames * yFrames
			cellWidth, cellHeight := texture.Bounds().Dx()/xFrames, texture.Bounds().Dy()/yFrames
			if cellWidth <= 0 || cellHeight <= 0 {
				continue
			}
			column, row := frame%xFrames, frame/xFrames
			sourceX0, sourceY0 = float64(column*cellWidth), float64(row*cellHeight)
			sourceX1, sourceY1 = sourceX0+float64(cellWidth), sourceY0+float64(cellHeight)
		}
		if scales, ok := v.Textures.(SourceScaleProvider); ok && !prop.animated {
			if density := scales.TextureSourceScale(textureName); density > 1 {
				sourceX0, sourceY0, sourceX1, sourceY1 = sourceX0*density, sourceY0*density, sourceX1*density, sourceY1*density
			}
		}
		sourceX0, sourceY0, sourceX1, sourceY1 = propSourceBounds(sourceX0, sourceY0, sourceX1, sourceY1, float64(texture.Bounds().Dx()), float64(texture.Bounds().Dy()))
		worldWidth := scaleX * float64(v.tileSize())
		worldHeight := scaleY * float64(v.tileSize())
		destinationX0 := float32((prop.x-worldWidth/2-v.CameraX)*v.Zoom + v.ViewportX)
		destinationY0 := float32((prop.y-propLift(prop.height)-worldHeight/2-v.CameraY)*v.Zoom + v.ViewportY)
		destinationX1 := destinationX0 + float32(worldWidth*v.Zoom)
		destinationY1 := destinationY0 + float32(worldHeight*v.Zoom)
		renderScaleX, renderScaleY := v.renderScale()
		destinationX0 *= float32(renderScaleX)
		destinationX1 *= float32(renderScaleX)
		destinationY0 *= float32(renderScaleY)
		destinationY1 *= float32(renderScaleY)
		vertices := []ebiten.Vertex{{DstX: destinationX0, DstY: destinationY0, SrcX: float32(sourceX0), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY0, SrcX: float32(sourceX1), SrcY: float32(sourceY0), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX0, DstY: destinationY1, SrcX: float32(sourceX0), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}, {DstX: destinationX1, DstY: destinationY1, SrcX: float32(sourceX1), SrcY: float32(sourceY1), ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1}}
		screen.DrawTriangles(vertices, []uint16{0, 1, 2, 1, 3, 2}, texture, &ebiten.DrawTrianglesOptions{Filter: ebiten.FilterNearest})
	}
}
func propSourceBounds(sourceX0, sourceY0, sourceX1, sourceY1, textureWidth, textureHeight float64) (float64, float64, float64, float64) {
	if sourceX1 == sourceX0 || sourceY1 == sourceY0 {
		return 0, 0, textureWidth, textureHeight
	}
	return sourceX0, sourceY0, sourceX1, sourceY1
}
func (v *Viewer) drawGrid(screen *ebiten.Image, tileSize int) {
	for x := 0; x <= v.Level.Width; x++ {
		scaleX, scaleY := v.renderScale()
		x0, y0 := ((float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX)*scaleX, (-v.CameraY*v.Zoom+v.ViewportY)*scaleY
		x1, y1 := ((float64(x*tileSize)-v.CameraX)*v.Zoom+v.ViewportX)*scaleX, ((float64(v.Level.Height*tileSize)-v.CameraY)*v.Zoom+v.ViewportY)*scaleY
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.RGBA{255, 255, 255, 35})
	}
	for y := 0; y <= v.Level.Height; y++ {
		scaleX, scaleY := v.renderScale()
		x0, y0 := (-v.CameraX*v.Zoom+v.ViewportX)*scaleX, ((float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY)*scaleY
		x1, y1 := ((float64(v.Level.Width*tileSize)-v.CameraX)*v.Zoom+v.ViewportX)*scaleX, ((float64(y*tileSize)-v.CameraY)*v.Zoom+v.ViewportY)*scaleY
		ebitenutil.DrawLine(screen, x0, y0, x1, y1, color.RGBA{255, 255, 255, 35})
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
	return 32
}
func (v *Viewer) sourceTileSize() int {
	if v.TileSet.TileSize > 0 {
		return v.TileSet.TileSize
	}
	if v.TileSet.TileShift > 0 {
		return 1 << v.TileSet.TileShift
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
		if !v.Layers[kind] {
			continue
		}
		id := v.Level.Layers[kind][index]
		if id == math.MaxUint32 || (kind != formats.LayerC && int32(id) < 0) {
			continue
		}
		return id, kind
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
