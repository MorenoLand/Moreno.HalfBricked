package game

import (
	"strings"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The 1.2.5 AchievementsScreen (HD cache only): a framed window (Screen, AnimateIn from y -225 to
// 160 in 500 ms) holding the title, the progress line and StatsList, a clipped 326x222 list whose
// rows are the 362x51 SwipiePageTemplate (Icon 40x46 at (27,23), Name 217x14 at (51.7,2), one line,
// Description 274x24 TopLeft at (51.5,18.5), two lines, Seperator), and the BackBox fixture sliding
// in from x 666 to 416. AchievementsScreen.txt: Back is the default and the back node.
type achievementsLayout struct {
	screen, backBox, list, template *formats.UIComponent
	name, description, icon, back   *formats.UIComponent
}

func (a *app) achievementsLayout() *achievementsLayout {
	ui := a.nativeScreen("AchievementsScreen")
	if ui == nil {
		return nil
	}
	l := &achievementsLayout{screen: ui.Find("Screen"), backBox: ui.Find("BackBox"), list: ui.Find("StatsList"), template: ui.Find("SwipiePageTemplate")}
	if l.screen == nil || l.backBox == nil || l.list == nil || l.template == nil {
		return nil
	}
	l.name = ui.FindIn("SwipiePageTemplate", "Name")
	l.description = ui.FindIn("SwipiePageTemplate", "Description")
	l.icon = ui.FindIn("SwipiePageTemplate", "Icon")
	l.back = ui.FindIn("BackBox", "Back")
	if l.name == nil || l.description == nil || l.icon == nil || l.back == nil {
		return nil
	}
	return l
}

// rowPitch is the SwipiePageTemplate height; visibleRows the whole rows the list window shows.
func (l *achievementsLayout) rowPitch() float64 {
	height, _ := l.template.Height()
	return height
}

func (l *achievementsLayout) visibleRows() int {
	height, _ := l.list.Height()
	return max(1, int(height/l.rowPitch()))
}

// achievementProgressCount is the progress line: unlocked/total (the screen's placeholder
// "Unlocked: 3/12" is an editor default; the shipped game shows "0/33").
func achievementProgressCount(unlocked, total int) string {
	return strings.TrimPrefix(achievementProgressText(unlocked, total), "Unlocked: ")
}

func (a *app) updateAchievementsNative(l *achievementsLayout) error {
	if uiKeyJustPressed(ebiten.KeyEscape) || uiKeyJustPressed(ebiten.KeyEnter) {
		return a.closeAchievements()
	}
	maxOffset := max(0, len(a.achievements)-l.visibleRows())
	x, y := a.pointer()
	fx, fy := float64(x), float64(y)
	bx0, by0, bx1, by1 := nativeRect(l.back, l.backBox)
	lx0, ly0, lx1, ly1 := nativeRect(l.list, l.screen)
	n := &a.native
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if fx >= bx0 && fx < bx1 && fy >= by0 && fy < by1 {
			return a.closeAchievements()
		}
		if fx >= lx0 && fx < lx1 && fy >= ly0 && fy < ly1 {
			n.listDrag, n.listDragY, n.listBase = true, fy, a.achievementOffset
		}
	}
	if n.listDrag {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			a.achievementOffset = clamp(n.listBase+int((n.listDragY-fy)/l.rowPitch()), 0, maxOffset)
			return nil
		}
		n.listDrag = false
	}
	delta := 0
	if uiKeyJustPressed(ebiten.KeyDown) {
		delta++
	}
	if uiKeyJustPressed(ebiten.KeyUp) {
		delta--
	}
	if _, wheel := ebiten.Wheel(); wheel < 0 {
		delta++
	} else if wheel > 0 {
		delta--
	}
	a.achievementOffset = clamp(a.achievementOffset+delta, 0, maxOffset)
	return nil
}

func (a *app) drawAchievementsNative(screen *ebiten.Image, l *achievementsLayout) {
	a.drawBackdrop(screen)
	progress := a.nativeSlide("achievements", nativeAnimateInDuration(l.screen))
	screenOffset := nativeSlideOffset(l.screen, progress)
	backOffset := nativeSlideOffset(l.backBox, progress)
	unlocked := 0
	for _, entry := range a.achievements {
		if a.achievementUnlocks[entry.ID] {
			unlocked++
		}
	}
	px, py := a.pointer()
	bx0, by0, bx1, by1 := nativeRect(l.back, l.backBox)
	backHover := a.confirm == nil && float64(px) >= bx0 && float64(px) < bx1 && float64(py) >= by0 && float64(py) < by1
	override := func(c *formats.UIComponent) nativeOverride {
		switch {
		case c == l.list:
			return nativeOverride{custom: func(target *ebiten.Image, offset formats.Vec2) { a.drawAchievementRows(target, l, offset) }}
		case c.Name == "ProgressText":
			return nativeOverride{text: nativeText(achievementProgressCount(unlocked, len(a.achievements))), style: &nativeTextStyle{spacing: .12, em: 1.75}}
		case c.Name == "ComponentText" && c.Class == "ComponentText":
			return nativeHeading(c)
		case c == l.back && backHover:
			return nativeOverride{brighten: .25}
		}
		return nativeOverride{}
	}
	a.drawNativeTree(screen, l.screen, screenOffset, a.menuTime, override)
	a.drawNativeTree(screen, l.backBox, backOffset, a.menuTime, override)
}

// drawAchievementRows draws the visible rows of StatsList, clipped to the list window.
func (a *app) drawAchievementRows(screen *ebiten.Image, l *achievementsLayout, offset formats.Vec2) {
	x, y := l.list.Anchor()
	x, y = x+offset.X, y+offset.Y
	width, _ := l.list.Width()
	height, _ := l.list.Height()
	target := a.nativeClip(screen, x, y, x+width, y+height)
	if target == nil {
		return
	}
	pitch := l.rowPitch()
	descSize, _ := l.description.FontSize()
	descWidth, _ := l.description.Width()
	descHeight, _ := l.description.Height()
	for row := 0; a.achievementOffset+row < len(a.achievements) && float64(row)*pitch < height; row++ {
		entry := a.achievements[a.achievementOffset+row]
		isUnlocked := a.achievementUnlocks[entry.ID]
		rowOffset := formats.Vec2{X: offset.X, Y: offset.Y + float64(row)*pitch}
		override := func(c *formats.UIComponent) nativeOverride {
			switch c {
			case l.name:
				return nativeOverride{text: nativeText(entry.Name)}
			case l.description:
				return nativeOverride{custom: func(t *ebiten.Image, o formats.Vec2) {
					style := nativeTextStyle{em: 1.25}
					lines := a.nativeWrap(entry.DisplayDescription(isUnlocked), descSize*style.em, descWidth, style)
					cx, cy := l.description.Anchor()
					cx, cy = cx+o.X, cy+o.Y // the box is centred: its two lines fill its height
					r, g, b := nativeTextColour(l.description, nativeOverride{})
					for i, line := range lines[:min(len(lines), 2)] {
						a.drawNativeSans(t, line, cx, cy-descHeight/2+descHeight/2*(float64(i)+.5), descWidth, "CenterLeft", descSize, r, g, b, style)
					}
				}}
			case l.icon:
				return nativeOverride{custom: func(t *ebiten.Image, o formats.Vec2) { a.drawAchievementRowIcon(t, l.icon, entry, isUnlocked, o) }}
			}
			return nativeOverride{}
		}
		a.drawNativeTree(target, l.template, rowOffset, a.menuTime, override)
		if label := a.achievementProgressLabel(entry); label != "" {
			tx, ty := l.template.Anchor()
			w, _ := l.template.Width()
			r, g, b := nativeTextColour(l.name, nativeOverride{})
			a.drawNativeSans(target, label, tx+rowOffset.X+w-24, ty+rowOffset.Y+9, 48, "CenterRight", 8, r, g, b, nativeTextStyle{em: 1.25})
		}
	}
}

// drawAchievementRowIcon draws the lock for a locked achievement and its own art once unlocked,
// stretched over the Icon component's 40x46 box.
func (a *app) drawAchievementRowIcon(screen *ebiten.Image, icon *formats.UIComponent, entry formats.Achievement, unlocked bool, offset formats.Vec2) {
	art := a.achievementIcon("Locked")
	if unlocked {
		art = a.achievementIcon(entry.Texture)
	}
	if art == nil {
		return
	}
	width, _ := icon.Width()
	height, _ := icon.Height()
	cx, cy := uiCenter(icon, offset, width, height)
	a.drawUIImage(screen, art, cx, cy, width, height, 0)
}
