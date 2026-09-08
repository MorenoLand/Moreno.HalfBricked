package ui

import (
	"bufio"
	"fmt"
	"image"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type Glyph struct{ X, Y, Width, Height, XOffset, YOffset, XAdvance int }
type Font struct {
	Atlas      *ebiten.Image
	LineHeight int
	Glyphs     map[rune]Glyph
}

var fontField = regexp.MustCompile(`(id|x|y|width|height|xoffset|yoffset|xadvance|lineHeight)=(-?\d+)`)

func LoadFont(reader io.Reader, atlas *ebiten.Image) (*Font, error) {
	font := &Font{Atlas: atlas, Glyphs: map[rune]Glyph{}}
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "common ") {
			fields := fields(line)
			font.LineHeight = fields["lineHeight"]
		}
		if !strings.HasPrefix(line, "char ") {
			continue
		}
		fields := fields(line)
		id, ok := fields["id"]
		if !ok {
			continue
		}
		font.Glyphs[rune(id)] = Glyph{X: fields["x"], Y: fields["y"], Width: fields["width"], Height: fields["height"], XOffset: fields["xoffset"], YOffset: fields["yoffset"], XAdvance: fields["xadvance"]}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if font.LineHeight == 0 || len(font.Glyphs) == 0 {
		return nil, fmt.Errorf("invalid bitmap font")
	}
	return font, nil
}
func (f *Font) Draw(screen *ebiten.Image, value string, x, y, scale float64) {
	f.DrawScaled(screen, value, x, y, scale, scale)
}
func (f *Font) DrawScaled(screen *ebiten.Image, value string, x, y, scaleX, scaleY float64) {
	if f == nil || f.Atlas == nil {
		return
	}
	originX := x
	for _, runeValue := range value {
		if runeValue == '\n' {
			x = originX
			y += float64(f.LineHeight) * scaleY
			continue
		}
		glyph, ok := f.Glyphs[runeValue]
		if !ok {
			continue
		}
		if glyph.Width > 0 && glyph.Height > 0 {
			source := f.Atlas.SubImage(image.Rect(glyph.X, glyph.Y, glyph.X+glyph.Width, glyph.Y+glyph.Height)).(*ebiten.Image)
			options := &ebiten.DrawImageOptions{}
			options.Filter = ebiten.FilterNearest
			options.GeoM.Scale(scaleX, scaleY)
			options.GeoM.Translate(x+float64(glyph.XOffset)*scaleX, y+float64(glyph.YOffset)*scaleY)
			screen.DrawImage(source, options)
		}
		x += float64(glyph.XAdvance) * scaleX
	}
}
func fields(line string) map[string]int {
	result := map[string]int{}
	for _, match := range fontField.FindAllStringSubmatch(line, -1) {
		result[match[1]], _ = strconv.Atoi(match[2])
	}
	return result
}
