package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
)

// The 1.2.5 LevelSelectScreen chrome (HD cache only), drawn from LevelSelectScreen.uiscreen:
//   - DarkBacking: Black.tex, 500x73 at (240, 96), alpha 100, behind the card strip;
//   - TopBar/ModeToggle: AOZ_StoreButtons_HD window (0, 0, .5, .4) 200x31 centred at (240, 15)
//     (AnimateIn from y -16, 400 ms); the SURVIVAL tab is the window at texCoordPos y .5. The tab
//     artwork itself reads STORY / SURVIVAL in this cache;
//   - BottomBox (AnimateIn from y 450 to 240, 300 ms): the 354x147 screen with Title (16, cyan
//     59,230,214), Subtitle (12, green) and the LevelDetails list under its Seperator;
//   - ButtonsBox (AnimateIn from x 616 to 416, 300 ms): Play, Co-op and Back (80x32 buttons).
//
// The card strip (LevelSwipie) keeps the port's carousel: its swipe scale and page layout are code,
// not data. LevelSelectScreen.txt (focus graph) is not wired: the arrow keys move the strip.
// Typefaces of the info screen as the original draws them: the title and the best-score rows in
// small capitals, the tagline in plain capitals.
var (
	levelTitleStyle    = nativeTextStyle{smallCaps: true, spacing: .04, em: 1.3}
	levelSubtitleStyle = nativeTextStyle{em: 1}
	levelRowStyle      = nativeTextStyle{smallCaps: true, spacing: .1, em: 1.25}
)

type levelSelectLayout struct {
	band, bottomBox, buttonsBox, mode   *formats.UIComponent
	title, subtitle, details, separator *formats.UIComponent
	play, coop, back                    *formats.UIComponent
	loading, newSurvival, highscores    *formats.UIComponent
}

func (a *app) levelSelectLayout() *levelSelectLayout {
	ui := a.nativeScreen("LevelSelectScreen")
	if ui == nil {
		return nil
	}
	l := &levelSelectLayout{band: ui.Find("DarkBacking"), bottomBox: ui.Find("BottomBox"), buttonsBox: ui.Find("ButtonsBox"), mode: ui.Find("ModeToggle")}
	l.title, l.subtitle = ui.FindIn("BottomBox", "Title"), ui.FindIn("BottomBox", "Subtitle")
	l.details, l.separator = ui.FindIn("BottomBox", "LevelDetails"), ui.FindIn("LevelDetails", "Seperator")
	l.play, l.coop, l.back = ui.FindIn("ButtonsBox", "PlayButton"), ui.FindIn("ButtonsBox", "CoopBtn"), ui.FindIn("ButtonsBox", "Back")
	l.loading, l.newSurvival, l.highscores = ui.Find("LoadingIndicator"), ui.Find("NewSurvival"), ui.Find("Highscores")
	for _, c := range []*formats.UIComponent{l.band, l.bottomBox, l.buttonsBox, l.mode, l.title, l.subtitle, l.details, l.separator, l.play, l.coop, l.back} {
		if c == nil {
			return nil
		}
	}
	return l
}

// levelActionAtNative names the ButtonsBox button under a logical point.
func (a *app) levelActionAtNative(l *levelSelectLayout, x, y int) string {
	fx, fy := float64(x), float64(y)
	in := func(c *formats.UIComponent) bool {
		x0, y0, x1, y1 := nativeRect(c, l.buttonsBox)
		if c == l.play || c == l.coop {
			play, co, _, _ := levelButtonStack(l, a.currentCoopLaunch().available)
			r := play
			if c == l.coop {
				r = co
			}
			x0, y0, x1, y1 = r[0], r[1], r[2], r[3]
		}
		return fx >= x0 && fx < x1 && fy >= y0 && fy < y1
	}
	host, join := a.levelOnlineRects(l)
	inRect := func(r [4]float64) bool { return fx >= r[0] && fx < r[2] && fy >= r[1] && fy < r[3] }
	switch {
	case in(l.play):
		return "play"
	case fx >= levelBackRect[0] && fx < levelBackRect[2] && fy >= levelBackRect[1] && fy < levelBackRect[3]:
		return "back"
	case inRect(host):
		return "host"
	case inRect(join):
		return "join"
	case a.currentCoopLaunch().available && in(l.coop):
		return "coop"
	}
	return ""
}

// levelOnlineRects are the port's HOST and JOIN buttons (x0, y0, x1, y1) in the free space of the
// ButtonsBox between Play and Back: two half-width buttons side by side, above the native Co-op
// slot when local co-op is available (the box's decor sits above and right of that space).
func (a *app) levelOnlineRects(l *levelSelectLayout) (host, join [4]float64) {
	// HD: the buttons are one column, Play, (Co-op), Host, Join, each as wide as Play.
	_, _, host, join = levelButtonStack(l, a.currentCoopLaunch().available)
	return
}

// levelTabAtNative is 0 for the STORY half of ModeToggle, 1 for SURVIVAL, -1 elsewhere.
func (a *app) levelTabAtNative(l *levelSelectLayout, x, y int) int {
	x0, y0, x1, y1 := nativeRect(l.mode, l.mode)
	fx, fy := float64(x), float64(y)
	if fx < x0 || fx >= x1 || fy < y0 || fy >= y1 {
		return -1
	}
	if fx < (x0+x1)/2 {
		return 0
	}
	return 1
}

func (a *app) drawLevelSelectNative(screen *ebiten.Image, l *levelSelectLayout) {
	a.drawBackdrop(screen)
	// The dark band is thinner than the uiscreen's 73 units (the iPad shows about 48) and lies behind the cards.
	a.drawRect(screen, 0, levelCardY-levelBandHeight/2, logicalWidth, levelBandHeight, color.RGBA{0, 0, 0, 100})
	levels := a.syncLevelCarousel()
	focusX := a.levelCardX(max(0, a.carouselCore.Selected))
	for index, entry := range levels {
		x := a.levelCardX(index)
		if x < -80 || x > logicalWidth+80 {
			continue
		}
		a.drawLevelCardNative(screen, entry.Info, x, levelCardY, math.Abs(x-focusX)/150, a.levelUnlocked(entry.Info))
	}
	selected := -1
	var info formats.LevelInfo
	if index := a.carouselCore.Selected; index >= 0 && index < len(levels) {
		selected, info = index, levels[index].Info
	}
	unlocked := selected >= 0 && a.levelUnlocked(info)
	px, py := a.pointer()
	hoverAction, hoverTab := "", -1
	if a.confirm == nil {
		hoverAction, hoverTab = a.levelActionAtNative(l, px, py), a.levelTabAtNative(l, px, py)
	}
	coop := a.currentCoopLaunch().available
	override := func(c *formats.UIComponent) nativeOverride {
		switch c {
		case l.loading, l.newSurvival, l.highscores:
			return nativeOverride{skip: true}
		}
		if levelPlayLights(c) {
			return nativeOverride{skip: true} // the enlarged Play plate covers the lights row (the iPad box has none)
		}
		switch c {
		case l.mode:
			row := 0.0
			if a.mode == 1 {
				row = .5
			}
			ov := nativeOverride{texPos: &[2]float64{0, row}}
			if hoverTab >= 0 && hoverTab != a.mode {
				ov.brighten = .2
			}
			return ov
		case l.title:
			if selected < 0 {
				return nativeOverride{skip: true}
			}
			return nativeOverride{text: nativeText(levelTextFlat(info.DisplayName, " ")), style: &levelTitleStyle}
		case l.subtitle:
			if selected < 0 {
				return nativeOverride{skip: true}
			}
			return nativeOverride{text: nativeText(levelTextFlat(strings.ReplaceAll(info.Description, "\\n", " "), " ")), style: &levelSubtitleStyle}
		case l.details:
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) {
				a.drawLevelDetails(target, l, info, selected >= 0, offset)
			}}
		case l.play:
			glow := float32(0)
			switch {
			case !unlocked:
				glow = -.5
			case hoverAction == "play":
				glow = .2
			}
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) { a.drawLevelPlay(target, l, offset, glow) }}
		case l.coop:
			if !coop {
				return nativeOverride{skip: true}
			}
			glow := float32(0)
			switch {
			case !unlocked:
				glow = -.5
			case hoverAction == "coop":
				glow = .2
			}
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) { a.drawLevelCoop(target, l, offset, glow) }}
		case l.back:
			return nativeOverride{skip: true} // the corner arrow replaces it
		}
		return nativeOverride{}
	}
	// The STORY / SURVIVAL tabs keep the port's crop of AOZ_StoreButtons: the native ModeToggle window drew both
	// rows of the texture at once, so the tabs are drawn as before (one 256x64 row per mode).
	a.drawLevelTabs(screen)
	for _, root := range []*formats.UIComponent{l.bottomBox, l.buttonsBox} {
		progress := a.nativeSlide("levelselect", nativeAnimateInDuration(root))
		a.drawNativeTree(screen, root, nativeSlideOffset(root, progress), a.menuTime, override)
	}
	a.drawLevelOnlineButtons(screen, l, hoverAction)
	a.drawLevelGlyphs(screen, l, hoverAction, unlocked)
}

// Port layout of the HD level select, from the iPad build's screens: the Back button is an arrow plate in the top-left
// corner, Play shows a play triangle, and the card strip's band is thinner and sits behind the cards.
var levelBackRect = [4]float64{5, 3, 50, 49}

const (
	levelCardY      = 100.0
	levelBandHeight = 48.0
)

// drawArrowGlyph draws HUD_Arrow (it points left) centred on (cx, cy); mirror points it right.
func (a *app) drawArrowGlyph(screen *ebiten.Image, cx, cy, w, h float64, mirror bool, brighten float32) {
	texture, err := a.Texture(nativeTextureName("Textures/HUD_Arrow.tex"))
	if err != nil {
		return
	}
	bounds := texture.Bounds()
	sx, sy := w/float64(bounds.Dx()), h/float64(bounds.Dy())
	if mirror {
		sx = -sx
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-float64(bounds.Dx())/2, -float64(bounds.Dy())/2)
	options.GeoM.Scale(sx, sy)
	options.GeoM.Translate(cx, cy)
	if brighten != 0 {
		options.ColorScale.Scale(1+brighten, 1+brighten, 1+brighten, 1)
	}
	a.drawImage(screen, texture, options)
}

// drawLevelGlyphs draws the corner Back plate with its arrow and the Play triangle.
func (a *app) drawLevelGlyphs(screen *ebiten.Image, l *levelSelectLayout, hover string, unlocked bool) {
	brighten := float32(0)
	if hover == "back" {
		brighten = .2
	}
	r := levelBackRect
	a.drawNativeNinePatchRect(screen, creditsCleanTexture(l.play.Texture()), r[0], r[1], r[2]-r[0], r[3]-r[1], brighten)
	a.drawArrowGlyph(screen, (r[0]+r[2])/2, (r[1]+r[3])/2, 26, 25, false, brighten)
}

// levelButtonStack lays the buttons box out as one column at rest: Play (larger), Co-op when local co-op exists, then
// Host and Join. Each is as wide as the native Play slot (80).
func levelButtonStack(l *levelSelectLayout, coop bool) (play, co, host, join [4]float64) {
	nx0, _, nx1, _ := nativeRect(l.play, l.buttonsBox)
	x0, x1 := nx0, nx1
	row := func(top, height float64) [4]float64 { return [4]float64{x0, top, x1, top + height} }
	if coop {
		return row(164, 32), row(201, 28), row(234, 28), row(267, 28)
	}
	return row(164, 46), [4]float64{}, row(222, 30), row(258, 30)
}

// levelPlayLights reports whether c is one of the lights or line decor under the Play slot.
func levelPlayLights(c *formats.UIComponent) bool {
	if c.Class != "ComponentAnimatedHudDecor" {
		return false
	}
	x, y := c.Anchor()
	return y >= 192 && y <= 204 && x >= 370 && x <= 470
}

// drawLevelCoop draws the Co-op plate in its stacked slot with the component's own label.
func (a *app) drawLevelCoop(screen *ebiten.Image, l *levelSelectLayout, offset formats.Vec2, glow float32) {
	_, co, _, _ := levelButtonStack(l, true)
	x0, y0, x1, y1 := co[0]+offset.X, co[1]+offset.Y, co[2]+offset.X, co[3]+offset.Y
	a.drawNativeNinePatchRect(screen, creditsCleanTexture(l.coop.Texture()), x0, y0, x1-x0, y1-y0, glow)
	size, _ := l.coop.FontSize()
	r, g, b := nativeTextColour(l.coop, nativeOverride{})
	a.drawNativeSans(screen, l.coop.String("text"), (x0+x1)/2, (y0+y1)/2, x1-x0-4, "", size*.75, r, g, b, nativeTextStyle{bold: true, em: 1.25})
}

// drawLevelPlay draws the Play plate with its triangle.
func (a *app) drawLevelPlay(screen *ebiten.Image, l *levelSelectLayout, offset formats.Vec2, glow float32) {
	play, _, _, _ := levelButtonStack(l, a.currentCoopLaunch().available)
	x0, y0, x1, y1 := play[0]+offset.X, play[1]+offset.Y, play[2]+offset.X, play[3]+offset.Y
	a.drawNativeNinePatchRect(screen, creditsCleanTexture(l.play.Texture()), x0, y0, x1-x0, y1-y0, glow)
	size := math.Min(38, (y1-y0)*.78)
	a.drawArrowGlyph(screen, (x0+x1)/2, (y0+y1)/2, size, size*.95, true, glow)
}

// drawLevelCardNative draws one level card opaque, with a soft shadow behind it and a size that falls from the
// selected card (about 1.13 of the 128x80 art) to its neighbours (about 0.81), as on the iPad.
func (a *app) drawLevelCardNative(screen *ebiten.Image, item formats.LevelInfo, x, y, distance float64, unlocked bool) {
	if item.PostcardImage == "" {
		return
	}
	name := "ShopFront0/Textures/Shop/" + item.PostcardImage + "_SD"
	texture, err := a.Texture(name)
	if err != nil {
		return
	}
	resolution := a.pack.TextureSourceScale(name)
	cardRect := resolutionRect(image.Rect(0, 0, 128, 80), resolution)
	if !cardRect.In(texture.Bounds()) {
		return
	}
	scale := 1.13 - .32*math.Min(1, distance)
	w, h := 128*scale, 80*scale
	// Shadow: a few widening translucent layers, a little lower than the card.
	for i := 6; i >= 1; i-- {
		grow := float64(i) * 1.6
		a.drawRect(screen, x-w/2-grow, y-h/2-grow+3, w+2*grow, h+2*grow, color.RGBA{0, 0, 0, 16})
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
	options.GeoM.Translate(-64*resolution, -40*resolution)
	options.GeoM.Scale(scale/resolution, scale/resolution)
	options.GeoM.Translate(x, y)
	a.drawImage(screen, texture.SubImage(cardRect).(*ebiten.Image), options)
	a.drawCatalogLevelBadges(screen, item, x, y, w, h)
}

// drawLevelOnlineButtons draws HOST and JOIN as the panel's grey buttons (HUD_ButtonNinePatch, bold
// black label, brightened under the pointer like Play and Back).
func (a *app) drawLevelOnlineButtons(screen *ebiten.Image, l *levelSelectLayout, hover string) {
	progress := a.nativeSlide("levelselect", nativeAnimateInDuration(l.buttonsBox))
	offset := nativeSlideOffset(l.buttonsBox, progress)
	host, join := a.levelOnlineRects(l)
	path := creditsCleanTexture(l.play.Texture())
	size, _ := l.play.FontSize()
	for _, button := range []struct {
		rect  [4]float64
		label string
		name  string
	}{{host, "Host", "host"}, {join, "Join", "join"}} {
		brighten := float32(0)
		if hover == button.name {
			brighten = .2
		}
		x0, y0, w, h := button.rect[0]+offset.X, button.rect[1]+offset.Y, button.rect[2]-button.rect[0], button.rect[3]-button.rect[1]
		a.drawNativeNinePatchRect(screen, path, x0, y0, w, h, brighten)
		r, g, b := nativeTextColour(l.play, nativeOverride{})
		if brighten != 0 {
			r, g, b = math.Min(255, r*1.2), math.Min(255, g*1.2), math.Min(255, b*1.2)
		}
		a.drawNativeSans(screen, button.label, x0+w/2, y0+h/2, w-4, "", size*.6, r, g, b, nativeTextStyle{bold: true, em: 1.25})
	}
}

// drawLevelDetails draws the LevelDetails separator and its rows. The shipped template holds
// only placeholder text; the game fills the rows, of which the port tracks the best score.
func (a *app) drawLevelDetails(screen *ebiten.Image, l *levelSelectLayout, info formats.LevelInfo, haveLevel bool, offset formats.Vec2) {
	a.drawNativeTexture(screen, l.separator, offset, nativeOverride{})
	if !haveLevel {
		return
	}
	detailsX, _ := l.details.Anchor() // centred, 325 wide
	detailsWidth, _ := l.details.Width()
	_, sepY := l.separator.Anchor() // centred, 7 high
	sepHeight, _ := l.separator.Height()
	x0, x1 := detailsX+offset.X-detailsWidth/2, detailsX+offset.X+detailsWidth/2
	sepBottom := sepY + offset.Y + sepHeight/2
	size := 12.0
	r, g, b := 100.0, 228.0, 100.0
	rows := []struct {
		label string
		value string
	}{
		{"Best Score", fmt.Sprintf("%d", a.highscores[info.ID])},
		{"Best Deaths", fmt.Sprintf("%d", a.levelBests[info.ID].Deaths)},
		{"Best Kills", fmt.Sprintf("%d", a.levelBests[info.ID].Kills)},
	}
	for i, row := range rows {
		cy := sepBottom + 10 + float64(i)*20
		a.drawNativeSans(screen, row.label, x0+6+160, cy, 320, "CenterLeft", size, r, g, b, levelRowStyle)
		a.drawNativeSans(screen, row.value, x1-6-50, cy, 100, "CenterRight", size, r, g, b, levelRowStyle)
	}
}
