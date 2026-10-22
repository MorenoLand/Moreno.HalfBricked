package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// A pop-up list: the file menu, the undo history, the problems list, the stamp library. It opens under a toolbar button, a
// click on a row runs it, a click anywhere else (or Esc) closes it.

type edMenuItem struct {
	label, sub, tip string
	icon            string
	swatch          *color.RGBA
	disabled        bool
	keepOpen        bool // the list stays up after the click (the history)
	armFile         bool // the browser's file chooser opens from a real click on this row
	armPaste        bool // the browser's clipboard is read from a real click on this row
	header          bool // a heading, not clickable
	do              func()
}

type edPopup struct {
	title   string
	x, y, w float64
	rowH    float64
	items   []edMenuItem
	top     int
	maxRows int
	// build, when set, makes the items again each frame (a list that changes while it is open)
	build func() []edMenuItem
}

// openPopup shows a list anchored at x (clamped into the window), just under the toolbar.
func (a *app) openPopup(title string, x, w float64, items []edMenuItem) *edPopup {
	e := a.editor
	e.popup = &edPopup{title: title, x: x, y: editorTopBar + 2, w: w, rowH: 30, items: items, maxRows: 15}
	return e.popup
}

func (p *edPopup) rect() (x, y, w, h float64) {
	rows := min(len(p.items), p.maxRows)
	h = 34 + float64(rows)*p.rowH + 8
	x = p.x
	if x+p.w > editorW-8 {
		x = editorW - p.w - 8
	}
	return x, p.y, p.w, h
}

// updatePopup handles the open list; it is modal.
func (a *app) updatePopup(px, py float64) {
	e := a.editor
	p := e.popup
	if p.build != nil {
		p.items = p.build()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		e.popup = nil
		return
	}
	if _, wheel := ebiten.Wheel(); wheel != 0 {
		p.top = max(0, min(max(0, len(p.items)-p.maxRows), p.top-int(wheel*2)))
	}
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return
	}
	x, y, w, h := p.rect()
	if px >= x && px < x+w && py >= y+30 && py < y+h {
		index := p.top + int((py-y-30)/p.rowH)
		if index >= 0 && index < len(p.items) {
			item := p.items[index]
			if item.disabled || item.header || item.do == nil {
				return
			}
			if !item.keepOpen {
				e.popup = nil
			}
			item.do()
			return
		}
	}
	e.popup = nil
}

// drawPopup draws the open list and returns the hover help of the row under the pointer.
func (a *app) drawPopup(screen *ebiten.Image, px, py float64) string {
	p := a.editor.popup
	if p.build != nil {
		p.items = p.build()
	}
	x, y, w, h := p.rect()
	a.vRound(screen, x-1, y-1, w+2, h+2, 9, color.RGBA{255, 255, 255, 30})
	a.vRound(screen, x, y, w, h, 8, color.RGBA{24, 29, 41, 252})
	a.vText(screen, p.title, x+12, y+8, 14, editorDim, true)
	tip := ""
	for row := 0; row < p.maxRows && p.top+row < len(p.items); row++ {
		item := p.items[p.top+row]
		rowY := y + 30 + float64(row)*p.rowH
		hover := px >= x && px < x+w && py >= rowY && py < rowY+p.rowH
		if hover && !item.disabled && !item.header {
			a.vRound(screen, x+4, rowY, w-8, p.rowH-2, 6, editorBtnHover)
			tip = item.tip
			a.editorArm(armRegion(item.armFile, x, rowY, w, p.rowH), item.armPaste)
		}
		left := x + 14
		clr := editorText
		if item.disabled {
			clr = color.RGBA{96, 104, 124, 255}
		}
		if item.header {
			a.vText(screen, item.label, left, rowY+6, 13, editorDim, true)
			continue
		}
		if item.icon != "" {
			a.drawEditorIcon(screen, item.icon, left+8, rowY+p.rowH/2-1, clr, editorPanel)
			left += 28
		}
		if item.swatch != nil {
			a.vRound(screen, left, rowY+p.rowH/2-7, 14, 14, 3, *item.swatch)
			left += 22
		}
		a.vText(screen, item.label, left, rowY+6, 15, clr, false)
		if item.sub != "" {
			a.vText(screen, item.sub, x+w-14-a.vTextWidth(item.sub, 13, false), rowY+8, 13, editorDim, false)
		}
	}
	if len(p.items) > p.maxRows {
		a.vText(screen, "Scroll for more", x+w-110, y+9, 12, editorDim, false)
	}
	return tip
}

func armRegion(on bool, x, y, w, h float64) [4]float64 {
	if !on {
		return [4]float64{}
	}
	return [4]float64{x, y, x + w, y + h}
}
