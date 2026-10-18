package game

import (
	"bytes"
	"math"
	"strings"
	"sync"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
)

// The 1.2.5 uiscreens name Fonts/AoZ_Terminal.ttf (a 99-glyph pixel terminal face that the cache
// does not ship). The original front-end captures the owner supplied show a clean humanist sans
// instead, in wide-spaced capitals for headings and small capitals for the level rows, so the native
// screens draw their text with the Go fonts (golang.org/x/image/font/gofont) at the sizes the
// components give, with the letter spacing and capitals those captures show.
type nativeTextStyle struct {
	bold      bool
	smallCaps bool    // lower case letters are drawn as capitals at smallCapsScale of the size
	spacing   float64 // extra advance after each letter, in em
	em        float64 // multiplier from the component's fontSize to the em size (default 1)
}

const smallCapsScale = .78

var (
	nativeFontOnce                  sync.Once
	nativeRegularSrc, nativeBoldSrc *text.GoTextFaceSource
)

func nativeFaceSource(bold bool) *text.GoTextFaceSource {
	nativeFontOnce.Do(func() {
		nativeRegularSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
		nativeBoldSrc, _ = text.NewGoTextFaceSource(bytes.NewReader(gobold.TTF))
	})
	if bold {
		return nativeBoldSrc
	}
	return nativeRegularSrc
}

// nativeGlyphs splits text into the runs a style draws: the glyph string and its size factor.
func nativeGlyphs(s string, st nativeTextStyle) (glyphs []string, factors []float64) {
	for _, r := range s {
		factor := 1.0
		if st.smallCaps && unicode.IsLower(r) {
			r, factor = unicode.ToUpper(r), smallCapsScale
		}
		glyphs, factors = append(glyphs, string(r)), append(factors, factor)
	}
	return
}

// nativeTextAdvance is the logical width of s at the em size (logical units) in the style, unit being the device pixels per logical unit of the font size and sx those of the x axis.
func nativeTextAdvance(s string, em float64, st nativeTextStyle, unit, sx float64) float64 {
	source := nativeFaceSource(st.bold)
	if source == nil || unit <= 0 || sx <= 0 {
		return 0
	}
	glyphs, factors := nativeGlyphs(s, st)
	total := 0.0
	for i, glyph := range glyphs {
		face := &text.GoTextFace{Source: source, Size: em * factors[i] * unit}
		total += text.Advance(glyph, face)/sx + st.spacing*em
	}
	return total - st.spacing*em
}

// drawNativeSans draws s in the box of the given width centred on (cx, cy) (logical units), aligned
// like drawCreditsText ("CenterLeft", "CenterRight", otherwise centred), vertically centring the
// capitals on cy.
func (a *app) drawNativeSans(screen *ebiten.Image, s string, cx, cy, width float64, align string, size float64, r, g, b float64, st nativeTextStyle) {
	source := nativeFaceSource(st.bold)
	if source == nil || s == "" || size <= 0 {
		return
	}
	sx, sy := a.renderScale()
	unit := math.Min(sx, sy)
	if st.em <= 0 {
		st.em = 1
	}
	em := size * st.em
	total := nativeTextAdvance(s, em, st, unit, sx)
	if width > 0 && total > width {
		em *= width / total // a longer text shrinks to its component's width
		total = nativeTextAdvance(s, em, st, unit, sx)
	}
	x := cx - total/2
	switch align {
	case "CenterLeft", "TopLeft":
		x = cx - width/2
	case "CenterRight":
		x = cx + width/2 - total
	}
	glyphs, factors := nativeGlyphs(s, st)
	for i, glyph := range glyphs {
		face := &text.GoTextFace{Source: source, Size: em * factors[i] * unit}
		metrics := face.Metrics()
		capHeight := .72 * face.Size
		options := &text.DrawOptions{}
		// the baseline sits half a cap height below the centre line
		options.GeoM.Translate(x*sx, cy*sy+capHeight/2-metrics.HAscent)
		options.ColorScale.Scale(float32(r/255), float32(g/255), float32(b/255), 1)
		text.Draw(screen, glyph, face, options)
		x += text.Advance(glyph, face)/sx + st.spacing*em
	}
}

// nativeUpper reports whether every letter of s is upper case (headings keep their source case).
func nativeUpper(s string) bool { return strings.ToUpper(s) == s }

// nativeWrap breaks value into lines no wider than maxWidth logical units at the em size.
func (a *app) nativeWrap(value string, em, maxWidth float64, st nativeTextStyle) []string {
	sx, sy := a.renderScale()
	unit := math.Min(sx, sy)
	var lines []string
	line := ""
	for _, word := range strings.Fields(value) {
		candidate := word
		if line != "" {
			candidate = line + " " + word
		}
		if line != "" && nativeTextAdvance(candidate, em, st, unit, sx) > maxWidth {
			lines = append(lines, line)
			line = word
		} else {
			line = candidate
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
