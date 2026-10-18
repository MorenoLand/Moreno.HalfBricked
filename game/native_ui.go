package game

import (
	"image"
	"math"
	"path/filepath"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// Shared drawing of the 1.2.5 (HD) front-end screens from their compiled
// .uiscreen files (Common0/UserInterface/screens). The SD cache ships none of
// them, so every caller falls back to its previous layout when nativeScreen
// returns nil.
//
// Rules taken from the data (the same ones credits.go and results_screen.go use):
//   - a child's position is relative to its parent's anchor (the centre when the
//     parent is originFromCenter, the top-left corner otherwise);
//   - a component without originFromCenter hangs from its position (top-left) and
//     a rotation turns it about that position; rotation.z is counter-clockwise on
//     the y-down screen (main_menu_groups.go, ARM 0x004108cc negates the angle);
//   - a negative texCoordSize mirrors the image;
//   - AnimateIn is a position track, and its last key is the resting position.

type nativeUIState struct {
	screens    map[string]*formats.UIScreen
	slideKey   string
	slideStart float64
	slideSeen  float64
	listDrag   bool // a pointer drag scrolling a native list (achievements, stats)
	listDragY  float64
	listBase   int
}

// nativeScreen loads (once) a shipped screen by name, nil when the pack has none.
func (a *app) nativeScreen(name string) *formats.UIScreen {
	if a == nil || a.pack == nil {
		return nil
	}
	if screen, ok := a.native.screens[name]; ok {
		return screen
	}
	if a.native.screens == nil {
		a.native.screens = map[string]*formats.UIScreen{}
	}
	var screen *formats.UIScreen
	if path, ok := a.pack.SourcePath("Common0/UserInterface/screens/" + name + ".uiscreen"); ok {
		if reader, err := a.pack.Open(path); err == nil {
			data, readErr := ioReadAll(reader)
			reader.Close()
			if readErr == nil {
				if parsed, parseErr := formats.ParseUIScreen(data); parseErr == nil {
					screen = parsed
				}
			}
		}
	}
	a.native.screens[name] = screen
	return screen
}

// nativeSlide is the 0..1 progress of an AnimateIn that started when the screen
// identified by key was first drawn (or drawn again after a gap).
func (a *app) nativeSlide(key string, duration float64) float64 {
	n := &a.native
	if n.slideKey != key || a.menuTime-n.slideSeen > .1 {
		n.slideKey, n.slideStart = key, a.menuTime
	}
	n.slideSeen = a.menuTime
	if duration <= 0 {
		return 1
	}
	return math.Max(0, math.Min(1, (a.menuTime-n.slideStart)/duration))
}

// nativeAnimateInDuration is the length of a component's AnimateIn position track.
func nativeAnimateInDuration(c *formats.UIComponent) float64 {
	keys := c.AnimationPosition("AnimateIn")
	if len(keys) < 2 {
		return 0
	}
	return keys[len(keys)-1].Time
}

// nativeAnimatePosition is the position the AnimateIn track gives at progress
// (0 = first key, 1 = last key, linear in time between keys).
func nativeAnimatePosition(c *formats.UIComponent, progress float64) (x, y float64, ok bool) {
	keys := c.AnimationPosition("AnimateIn")
	if len(keys) < 2 {
		return 0, 0, false
	}
	t := progress * keys[len(keys)-1].Time
	for i := 0; i+1 < len(keys); i++ {
		a, b := keys[i], keys[i+1]
		if t <= b.Time || i+2 == len(keys) {
			span := b.Time - a.Time
			f := 1.0
			if span > 0 {
				f = math.Max(0, math.Min(1, (t-a.Time)/span))
			}
			return a.X + (b.X-a.X)*f, a.Y + (b.Y-a.Y)*f, true
		}
	}
	return 0, 0, false
}

// nativeSlideOffset is the displacement of an animated root from its stored position at progress.
func nativeSlideOffset(c *formats.UIComponent, progress float64) formats.Vec2 {
	x, y, ok := nativeAnimatePosition(c, progress)
	if !ok {
		return formats.Vec2{}
	}
	baseX, baseY := c.Position()
	return formats.Vec2{X: x - baseX, Y: y - baseY}
}

// nativeRestAnchor is the anchor of a component with its root resting (AnimateIn at its last key).
func nativeRestAnchor(c *formats.UIComponent, root *formats.UIComponent) (float64, float64) {
	x, y := c.Anchor()
	if root != nil {
		off := nativeSlideOffset(root, 1)
		x, y = x+off.X, y+off.Y
	}
	return x, y
}

// nativeOverride lets a screen change how one component is drawn.
type nativeOverride struct {
	skip     bool                              // draw neither the component nor its children
	text     *string                           // replacement text
	rgba     *[4]float64                       // replacement text/tint colour, 0..255
	texPos   *[2]float64                       // replacement texCoordPos
	brighten float32                           // >0 multiplies the colours (focus / hover)
	custom   func(*ebiten.Image, formats.Vec2) // drawn instead of the component and its children
	texture  string                            // replacement texture path (as the screen spells it)
}

func nativeText(s string) *string { return &s }

// drawNativeTree draws node and its descendants in tree order.
func (a *app) drawNativeTree(screen *ebiten.Image, node *formats.UIComponent, offset formats.Vec2, elapsed float64, override func(*formats.UIComponent) nativeOverride) {
	if node == nil || !node.Enabled() || !node.Visible() {
		return
	}
	ov := nativeOverride{}
	if override != nil {
		ov = override(node)
	}
	if ov.skip {
		return
	}
	if ov.custom != nil {
		ov.custom(screen, offset)
		return
	}
	texture := creditsCleanTexture(node.Texture())
	if ov.texture != "" {
		texture = ov.texture
	}
	switch {
	case texture == "" && node.Class == "ComponentAnimatedHudDecor":
		a.drawUIDecor(screen, node, offset, elapsed)
	case texture != "" && strings.Contains(texture, "NinePatch"):
		a.drawNativeNinePatch(screen, node, offset, ov.brighten)
	case texture != "":
		a.drawNativeTexture(screen, node, offset, ov)
	}
	text := node.String("text")
	if ov.text != nil {
		text = *ov.text
	}
	if text != "" {
		a.drawNativeComponentText(screen, node, offset, text, ov)
	}
	for _, child := range node.Children {
		a.drawNativeTree(screen, child, offset, elapsed, override)
	}
}

// nativeTextColour is the colour a component writes its text in.
func nativeTextColour(node *formats.UIComponent, ov nativeOverride) (r, g, b float64) {
	if ov.rgba != nil {
		return ov.rgba[0], ov.rgba[1], ov.rgba[2]
	}
	if cr, cg, cb, _, ok := node.Colour("textColour"); ok {
		return cr, cg, cb
	}
	if cr, cg, cb, _, ok := node.Colour("colour"); ok {
		return cr, cg, cb
	}
	return 255, 255, 255
}

func (a *app) drawNativeComponentText(screen *ebiten.Image, node *formats.UIComponent, offset formats.Vec2, text string, ov nativeOverride) {
	size, ok := node.FontSize()
	if !ok {
		return
	}
	width, _ := node.Width()
	height, _ := node.Height()
	x, y := node.Anchor()
	x, y = x+offset.X, y+offset.Y
	if !node.OriginFromCenter() {
		x, y = x+width/2, y+height/2
	}
	r, g, b := nativeTextColour(node, ov)
	if ov.brighten != 0 {
		r, g, b = math.Min(255, r*float64(1+ov.brighten)), math.Min(255, g*float64(1+ov.brighten)), math.Min(255, b*float64(1+ov.brighten))
	}
	a.drawCreditsText(screen, text, x, y, width, node.String("alignment"), size, r, g, b)
}

// drawNativeTexture draws a ComponentTexture: texture window, mirror, tint, pivot and rotation as above.
func (a *app) drawNativeTexture(screen *ebiten.Image, node *formats.UIComponent, offset formats.Vec2, ov nativeOverride) {
	path := creditsCleanTexture(node.Texture())
	if ov.texture != "" {
		path = ov.texture
	}
	if path == "" {
		return
	}
	texture, err := a.Texture(nativeTextureName(path))
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	tx, ty, tw, th := node.TexCoords()
	if ov.texPos != nil {
		tx, ty = ov.texPos[0], ov.texPos[1]
	}
	rect := image.Rect(bounds.Min.X+int(math.Round(tx*float64(bounds.Dx()))), bounds.Min.Y+int(math.Round(ty*float64(bounds.Dy()))), bounds.Min.X+int(math.Round((tx+tw)*float64(bounds.Dx()))), bounds.Min.Y+int(math.Round((ty+th)*float64(bounds.Dy())))).Intersect(bounds)
	if rect.Empty() {
		return
	}
	width, hasWidth := node.Width()
	height, hasHeight := node.Height()
	switch {
	case !hasWidth && !hasHeight:
		width, height = float64(rect.Dx()), float64(rect.Dy())
	case !hasWidth:
		width = height * float64(rect.Dx()) / float64(rect.Dy())
	case !hasHeight:
		height = width * float64(rect.Dy()) / float64(rect.Dx())
	}
	flipX, flipY := node.TexCoordFlip()
	sx, sy := width/float64(rect.Dx()), height/float64(rect.Dy())
	if flipX {
		sx = -sx
	}
	if flipY {
		sy = -sy
	}
	x, y := node.Anchor()
	x, y = x+offset.X, y+offset.Y
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
	options.GeoM.Scale(sx, sy)
	if !node.OriginFromCenter() {
		options.GeoM.Translate(width/2, height/2)
	}
	options.GeoM.Rotate(-node.Rotation() * math.Pi / 180)
	options.GeoM.Translate(x, y)
	r, g, b, alpha := 1.0, 1.0, 1.0, 1.0
	if cr, cg, cb, ca, ok := node.Colour("colour"); ok {
		r, g, b, alpha = cr/255, cg/255, cb/255, ca/255
	}
	if ov.rgba != nil {
		r, g, b, alpha = ov.rgba[0]/255, ov.rgba[1]/255, ov.rgba[2]/255, ov.rgba[3]/255
	}
	boost := 1.0 + float64(ov.brighten)
	options.ColorScale.Scale(float32(r*alpha*boost), float32(g*alpha*boost), float32(b*alpha*boost), float32(alpha))
	a.drawImage(screen, texture.SubImage(rect).(*ebiten.Image), options)
}

// drawNativeNinePatch is drawCreditsNinePatch with an optional brighten factor.
func (a *app) drawNativeNinePatch(screen *ebiten.Image, node *formats.UIComponent, offset formats.Vec2, brighten float32) {
	path := creditsCleanTexture(node.Texture())
	if path == "" {
		return
	}
	name := nativeTextureName(path)
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	width, okW := node.Width()
	height, okH := node.Height()
	if !okW || !okH {
		return
	}
	resolution := a.pack.TextureSourceScale(name)
	if resolution <= 0 {
		resolution = 1
	}
	if a.creditsMargins == nil {
		a.creditsMargins = map[string]int{}
	}
	margin, cached := a.creditsMargins[name]
	if !cached {
		margin = a.nativeNineMargin(name, texture)
		a.creditsMargins[name] = margin
	}
	x, y := node.Anchor()
	x, y = x+offset.X, y+offset.Y
	if node.OriginFromCenter() {
		x, y = x-width/2, y-height/2
	}
	bounds := texture.Bounds()
	edge := math.Min(float64(margin)/resolution, math.Min(width, height)/2)
	xs := [4]float64{x, x + edge, x + width - edge, x + width}
	ys := [4]float64{y, y + edge, y + height - edge, y + height}
	us := [4]int{bounds.Min.X, bounds.Min.X + margin, bounds.Max.X - margin, bounds.Max.X}
	vs := [4]int{bounds.Min.Y, bounds.Min.Y + margin, bounds.Max.Y - margin, bounds.Max.Y}
	for row := 0; row < 3; row++ {
		for column := 0; column < 3; column++ {
			source := image.Rect(us[column], vs[row], us[column+1], vs[row+1])
			targetWidth, targetHeight := xs[column+1]-xs[column], ys[row+1]-ys[row]
			if source.Empty() || targetWidth <= 0 || targetHeight <= 0 {
				continue
			}
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Scale(targetWidth/float64(source.Dx()), targetHeight/float64(source.Dy()))
			options.GeoM.Translate(xs[column], ys[row])
			if brighten != 0 {
				options.ColorScale.Scale(1+brighten, 1+brighten, 1+brighten, 1)
			}
			a.drawImage(screen, texture.SubImage(source).(*ebiten.Image), options)
		}
	}
}

// nativeClip returns the part of screen a clipsChildren list may draw in (logical rectangle).
func (a *app) nativeClip(screen *ebiten.Image, x0, y0, x1, y1 float64) *ebiten.Image {
	sx, sy := a.renderScale()
	clip := image.Rect(int(math.Floor(x0*sx)), int(math.Floor(y0*sy)), int(math.Ceil(x1*sx)), int(math.Ceil(y1*sy))).Intersect(screen.Bounds())
	if clip.Empty() {
		return nil
	}
	return screen.SubImage(clip).(*ebiten.Image)
}

// nativeRect is the screen rectangle of a component at its resting root position
// (root: the animated ancestor whose AnimateIn track ends at the rest position, may be nil).
func nativeRect(c, root *formats.UIComponent) (x0, y0, x1, y1 float64) {
	width, _ := c.Width()
	height, _ := c.Height()
	x, y := nativeRestAnchor(c, root)
	if c.OriginFromCenter() {
		x, y = x-width/2, y-height/2
	}
	return x, y, x + width, y + height
}

// nativeNineMargin is creditsNineMargin read from the texture's CPU-side source image (reading a
// GPU image back is not possible before the game loop runs, which the tests rely on).
func (a *app) nativeNineMargin(name string, texture *ebiten.Image) int {
	source, err := a.Source(name)
	if err != nil {
		return creditsNineMargin(texture)
	}
	bounds := source.Bounds()
	margin := 1
	for _, horizontal := range []bool{true, false} {
		length := bounds.Dx()
		if !horizontal {
			length = bounds.Dy()
		}
		at := func(i int) [4]uint32 {
			var r, g, b, al uint32
			if horizontal {
				r, g, b, al = source.At(bounds.Min.X+i, bounds.Min.Y+bounds.Dy()/2).RGBA()
			} else {
				r, g, b, al = source.At(bounds.Min.X+bounds.Dx()/2, bounds.Min.Y+i).RGBA()
			}
			return [4]uint32{r, g, b, al}
		}
		middle := at(length / 2)
		start := length / 2
		for start > 0 && at(start-1) == middle {
			start--
		}
		end := length / 2
		for end < length-1 && at(end+1) == middle {
			end++
		}
		margin = max(margin, start, length-1-end) // a lower shadow edge counts as much as the top one
	}
	return min(margin, bounds.Dx()/2-1)
}

// nativeTextureName maps a screen texture path to the pack's texture name: the shop art lives in
// ShopFront0, everything else in Common0.
func nativeTextureName(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	if strings.HasPrefix(path, "Textures/Shop/") {
		return "ShopFront0/" + strings.TrimSuffix(path, filepath.Ext(path))
	}
	return uiTextureName(path)
}
