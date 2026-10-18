package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The 1.2.5 StatsScreen (HD cache only): the Screen window (AnimateIn from y -125 to 125 in 500 ms)
// with the "USER STATISTICS" title, green line and StatsList, a clipped 361x149 ComponentCredits
// that autoscrolls upward (autoScrollSpeed -20, clipsChildren) over 29-unit pages: LeftText (274
// wide, CenterLeft, 14) and RightText (107 wide at x 298, CenterRight, 14). BackBox slides in from
// x 666 to 353. StatsScreen.txt: Back is the default and the back node.
//
// The record list, its kinds and the value formats are the port's existing statistics model
// (stats_menu.go). Pages are laid out from the bottom edge of the window, 29 units apart in
// content space, and the content moves up at 20 units a second from the moment the page opens;
// the model feeds a record every few ticks, which always happens before its page scrolls into
// view (a page needs 1.45 s per slot to arrive, a record is fed every 0.9 s), so the draw does
// not depend on the feed timing. The list wraps through the records like the model does.
const (
	statsScrollSpeed = 20.0 // StatsList autoScrollSpeed
	statsPageHeight  = 29.0 // SwipiePageTemplate height
)

// statsRowStyle is the typeface of the statistics rows.
var (
	statsRowStyle    = nativeTextStyle{em: 1.2}
	statsHeaderStyle = nativeTextStyle{em: 1.7}
)

type statsLayout struct {
	screen, backBox, list, back *formats.UIComponent
	left, right                 *formats.UIComponent
}

func (a *app) statsLayout() *statsLayout {
	ui := a.nativeScreen("StatsScreen")
	if ui == nil {
		return nil
	}
	l := &statsLayout{screen: ui.Find("Screen"), backBox: ui.Find("BackBox"), list: ui.Find("StatsList")}
	l.back = ui.FindIn("BackBox", "Back")
	l.left = ui.FindIn("SwipiePageTemplate", "LeftText")
	l.right = ui.FindIn("SwipiePageTemplate", "RightText")
	if l.screen == nil || l.backBox == nil || l.list == nil || l.back == nil || l.left == nil || l.right == nil {
		return nil
	}
	return l
}

// statsAchievementsRect is the port's shortcut to the achievements page, kept in the free corner
// below-left of the window (it used to sit top right, where the native connector now is).
func statsAchievementsRect() (x0, y0, x1, y1 float64) { return 0, 288, 150, logicalHeight }

func (a *app) updateStatsNative(l *statsLayout) error {
	if uiKeyJustPressed(ebiten.KeyA) {
		a.openAchievements()
		return nil
	}
	x, y := a.pointer()
	fx, fy := float64(x), float64(y)
	back := uiKeyJustPressed(ebiten.KeyEscape) || uiKeyJustPressed(ebiten.KeyEnter)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		bx0, by0, bx1, by1 := nativeRect(l.back, l.backBox)
		ax0, ay0, ax1, ay1 := statsAchievementsRect()
		switch {
		case fx >= bx0 && fx < bx1 && fy >= by0 && fy < by1:
			back = true
		case fx >= ax0 && fx < ax1 && fy >= ay0 && fy < ay1:
			a.openAchievements()
			return nil
		}
	}
	return a.stepStatsMenu(back)
}

func (a *app) drawStatsNative(screen *ebiten.Image, l *statsLayout) {
	menu := a.statsScreen
	a.drawBackdrop(screen)
	progress := a.nativeSlide("stats", nativeAnimateInDuration(l.screen))
	screenOffset := nativeSlideOffset(l.screen, progress)
	backOffset := nativeSlideOffset(l.backBox, progress)
	px, py := a.pointer()
	bx0, by0, bx1, by1 := nativeRect(l.back, l.backBox)
	backHover := a.confirm == nil && float64(px) >= bx0 && float64(px) < bx1 && float64(py) >= by0 && float64(py) < by1
	override := func(c *formats.UIComponent) nativeOverride {
		switch {
		case c == l.list:
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) { a.drawStatsRows(target, l, menu, offset) }}
		case c == l.back && backHover:
			return nativeOverride{brighten: .25}
		case c.Name == "ComponentText" && c.Class == "ComponentText":
			return nativeHeading(c)
		}
		return nativeOverride{}
	}
	a.drawNativeTree(screen, l.screen, screenOffset, a.menuTime, override)
	a.drawNativeTree(screen, l.backBox, backOffset, a.menuTime, override)
	ax0, ay0, _, _ := statsAchievementsRect()
	a.drawNativeSans(screen, "ACHIEVEMENTS", ax0+8+50, ay0+13, 100, "CenterLeft", 9, 255, 255, 255, nativeTextStyle{spacing: .08, em: 1.25})
}

// statsRowTop is the window-relative top of the page of row k after age seconds.
func statsRowTop(k int, windowHeight float64, age float64) float64 {
	return windowHeight + statsPageHeight*float64(k) - statsScrollSpeed*age
}

func (a *app) drawStatsRows(screen *ebiten.Image, l *statsLayout, menu *statsMenu, offset formats.Vec2) {
	if len(menu.Records) == 0 {
		return
	}
	x, y := l.list.Anchor()
	x, y = x+offset.X, y+offset.Y
	width, _ := l.list.Width()
	height, _ := l.list.Height()
	target := a.nativeClip(screen, x, y, x+width, y+height)
	if target == nil {
		return
	}
	age := float64(menu.Age)
	first := max(0, int(math.Floor((statsScrollSpeed*age-height-statsPageHeight)/statsPageHeight)))
	leftSize, _ := l.left.FontSize()
	rightSize, _ := l.right.FontSize()
	leftWidth, _ := l.left.Width()
	rightWidth, _ := l.right.Width()
	rightX, rightY := l.right.Anchor() // centred 107x29 box, relative to the list
	listX, listY := l.list.Anchor()
	for k := first; ; k++ {
		top := statsRowTop(k, height, age)
		if top >= height {
			break
		}
		record := menu.Records[k%len(menu.Records)]
		if record.Kind == 4 || record.Label == "" {
			continue
		}
		r, g, b := 255.0, 255.0, 255.0
		if record.Kind >= 1 && record.Kind <= 3 {
			r, g, b = 0, 255, 0 // the model tints its numeric rows pure green
		}
		style := statsRowStyle
		if record.Kind == 0 {
			r, g, b, style = 16, 173, 0, statsHeaderStyle // section headings: the original's dark green, larger
		}
		leftX, leftY := l.left.Anchor()
		a.drawNativeSans(target, record.Label, x+(leftX-listX)+leftWidth/2, y+top+(leftY-listY)+statsPageHeight/2, leftWidth, "CenterLeft", leftSize, r, g, b, style)
		if record.Kind >= 1 && record.Kind <= 3 {
			a.drawNativeSans(target, menu.valueText(record), x+(rightX-listX), y+top+(rightY-listY), rightWidth, "CenterRight", rightSize, r, g, b, statsRowStyle)
		}
	}
}
