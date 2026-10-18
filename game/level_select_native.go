package game

import (
	"fmt"
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
	switch {
	case in(l.play):
		return "play"
	case in(l.back):
		return "back"
	case a.currentCoopLaunch().available && in(l.coop):
		return "coop"
	}
	return ""
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
			return nativeOverride{text: nativeText(levelTextFlat(info.DisplayName, " "))}
		case l.subtitle:
			if selected < 0 {
				return nativeOverride{skip: true}
			}
			return nativeOverride{text: nativeText(levelTextFlat(strings.ReplaceAll(info.Description, "\\n", " "), " "))}
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
	for _, root := range []*formats.UIComponent{l.mode, l.bottomBox, l.buttonsBox} {
		progress := a.nativeSlide("levelselect", nativeAnimateInDuration(root))
		a.drawNativeTree(screen, root, nativeSlideOffset(root, progress), a.menuTime, override)
	}
	a.text(screen, "H  HOST ONLINE", 6, 3, .3)
	a.text(screen, "J  JOIN ONLINE", 6, 15, .3)
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
		a.drawCreditsText(screen, row.label, x0+6+160, cy, 320, "CenterLeft", size, r, g, b)
		a.drawCreditsText(screen, row.value, x1-6-50, cy, 100, "CenterRight", size, r, g, b)
	}
}
