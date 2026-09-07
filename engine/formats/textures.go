package formats

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

func DecodeTexture(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 {
		return nil, fmt.Errorf("texture is only %d bytes", len(data))
	}
	width := 1 << data[0]
	height := 1 << data[1]
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid texture dimensions %dx%d", width, height)
	}
	want := 12 + width*height*4
	if len(data) != want {
		return nil, fmt.Errorf("texture dimensions %dx%d require %d bytes, got %d", width, height, want, len(data))
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := 12 + (y*width+x)*4
			img.SetNRGBA(x, y, color.NRGBA{R: data[i], G: data[i+1], B: data[i+2], A: data[i+3]})
		}
	}
	return img, nil
}

func EncodePNG(w io.Writer, img image.Image) error {
	return png.Encode(w, img)
}
