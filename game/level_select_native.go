package game

import (
	"fmt"
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
		return fx >= x0 && fx < x1 && fy >= y0 && fy < y1
	}
	host, join := a.levelOnlineRects(l)
	inRect := func(r [4]float64) bool { return fx >= r[0] && fx < r[2] && fy >= r[1] && fy < r[3] }
	switch {
	case in(l.play):
		return "play"
	case in(l.back):
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
	px0, _, px1, py1 := nativeRect(l.play, l.buttonsBox)
	_, cy0, _, _ := nativeRect(l.coop, l.buttonsBox)
	_, by0, _, _ := nativeRect(l.back, l.buttonsBox)
	gap := 3.0
	width := (px1 - px0 - gap) / 2
	top, bottom := py1+10, cy0-3 // below the Play row's lights, above Co-op
	if !a.currentCoopLaunch().available {
		bottom = by0 - 6
		top = py1 + 14
	}
	if bottom-top > 26 {
		top = (top+bottom)/2 - 13
		bottom = top + 26
	}
	host = [4]float64{px0, top, px0 + width, bottom}
	join = [4]float64{px1 - width, top, px1, bottom}
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
	a.drawNativeTree(screen, l.band, formats.Vec2{}, a.menuTime, nil)
	levels := a.syncLevelCarousel()
	for index, entry := range levels {
		x := a.levelCardX(index)
		if x < -64 || x > logicalWidth+64 {
			continue
		}
		a.drawLevelCardAt(screen, entry.Info, x, 100, index == a.carouselCore.Selected, a.levelUnlocked(entry.Info))
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
			ov := nativeOverride{}
			switch {
			case !unlocked:
				ov.brighten = -.5
			case hoverAction == "play":
				ov.brighten = .2
			}
			return ov
		case l.coop:
			if !coop {
				return nativeOverride{skip: true}
			}
			ov := nativeOverride{}
			switch {
			case !unlocked:
				ov.brighten = -.5
			case hoverAction == "coop":
				ov.brighten = .2
			}
			return ov
		case l.back:
			if hoverAction == "back" {
				return nativeOverride{brighten: .2}
			}
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
	}{{"Best Score", fmt.Sprintf("%d", a.highscores[info.ID])}}
	for i, row := range rows {
		cy := sepBottom + 10 + float64(i)*20
		a.drawNativeSans(screen, row.label, x0+6+160, cy, 320, "CenterLeft", size, r, g, b, levelRowStyle)
		a.drawNativeSans(screen, row.value, x1-6-50, cy, 100, "CenterRight", size, r, g, b, levelRowStyle)
	}
}
