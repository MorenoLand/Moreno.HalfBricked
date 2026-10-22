package game

import (
	"bytes"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// The editor is laid out on a finer grid than the game's 480 x 320: 960 x 640 "editor units", so its buttons and text are
// half as chunky, with real TrueType text instead of the game's bitmap font. editorW and editorH map to the window.

const (
	editorW = 960.0
	editorH = 640.0
)

var (
	editorFontOnce sync.Once
	editorRegular  *text.GoTextFaceSource
	editorBold     *text.GoTextFaceSource
)

func editorFace(bold bool, size float64) *text.GoTextFace {
	editorFontOnce.Do(func() {
		editorRegular, _ = text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		editorBold, _ = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	})
	source := editorRegular
	if bold {
		source = editorBold
	}
	return &text.GoTextFace{Source: source, Size: size}
}

// editorScale converts editor units to window pixels.
func (a *app) editorScale() (float64, float64) {
	sx, sy := a.frontendScaleX/2, a.frontendScaleY/2
	if sx <= 0 {
		sx = .5
	}
	if sy <= 0 {
		sy = .5
	}
	return sx, sy
}

// editorPointer is the mouse in editor units.
func (a *app) editorPointer() (float64, float64) {
	x, y := ebiten.CursorPosition()
	if a.outputWidth < 1 || a.outputHeight < 1 {
		return float64(x), float64(y)
	}
	return float64(x) * editorW / float64(a.outputWidth), float64(y) * editorH / float64(a.outputHeight)
}

func (a *app) vRect(screen *ebiten.Image, x, y, w, h float64, clr color.RGBA) {
	sx, sy := a.editorScale()
	vector.FillRect(screen, float32(x*sx), float32(y*sy), float32(w*sx), float32(h*sy), clr, false)
}

func (a *app) vRound(screen *ebiten.Image, x, y, w, h, radius float64, clr color.RGBA) {
	sx, sy := a.editorScale()
	var path vector.Path
	path.MoveTo(float32((x+radius)*sx), float32(y*sy))
	path.LineTo(float32((x+w-radius)*sx), float32(y*sy))
	path.QuadTo(float32((x+w)*sx), float32(y*sy), float32((x+w)*sx), float32((y+radius)*sy))
	path.LineTo(float32((x+w)*sx), float32((y+h-radius)*sy))
	path.QuadTo(float32((x+w)*sx), float32((y+h)*sy), float32((x+w-radius)*sx), float32((y+h)*sy))
	path.LineTo(float32((x+radius)*sx), float32((y+h)*sy))
	path.QuadTo(float32(x*sx), float32((y+h)*sy), float32(x*sx), float32((y+h-radius)*sy))
	path.LineTo(float32(x*sx), float32((y+radius)*sy))
	path.QuadTo(float32(x*sx), float32(y*sy), float32((x+radius)*sx), float32(y*sy))
	path.Close()
	options := &vector.DrawPathOptions{AntiAlias: true}
	options.ColorScale.ScaleWithColor(clr)
	vector.FillPath(screen, &path, &vector.FillOptions{}, options)
}

func (a *app) vOutline(screen *ebiten.Image, x, y, w, h, thickness float64, clr color.RGBA) {
	sx, sy := a.editorScale()
	vector.StrokeRect(screen, float32(x*sx), float32(y*sy), float32(w*sx), float32(h*sy), float32(thickness*sx), clr, false)
}

// vText draws a line of text with its top left at (x, y); size is the pixel height of the font in editor units.
func (a *app) vText(screen *ebiten.Image, value string, x, y, size float64, clr color.RGBA, bold bool) {
	if value == "" {
		return
	}
	sx, sy := a.editorScale()
	face := editorFace(bold, size*sy)
	options := &text.DrawOptions{}
	options.GeoM.Translate(x*sx, y*sy)
	options.ColorScale.ScaleWithColor(clr)
	text.Draw(screen, value, face, options)
}

// vTextWidth is the width of the text in editor units.
func (a *app) vTextWidth(value string, size float64, bold bool) float64 {
	sx, sy := a.editorScale()
	width, _ := text.Measure(value, editorFace(bold, size*sy), 0)
	return width / sx
}

// vWrap breaks text into lines no wider than width editor units.
func (a *app) vWrap(value string, width, size float64) []string {
	var lines []string
	current := ""
	word := ""
	flush := func() {
		if word == "" {
			return
		}
		candidate := current
		if candidate != "" {
			candidate += " "
		}
		candidate += word
		if current != "" && a.vTextWidth(candidate, size, false) > width {
			lines = append(lines, current)
			current = word
		} else {
			current = candidate
		}
		word = ""
	}
	for _, r := range value {
		if r == ' ' {
			flush()
		} else {
			word += string(r)
		}
	}
	flush()
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// wheelStep turns a mouse wheel report into a fixed number of rows (browsers report much larger wheel values than a
// desktop does, which used to jump a list to its end).
func wheelStep(wheel float64, rows int) int {
	switch {
	case wheel > 0:
		return rows
	case wheel < 0:
		return -rows
	}
	return 0
}
