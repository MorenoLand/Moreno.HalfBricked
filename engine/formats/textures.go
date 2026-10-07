package formats

import (
	"encoding/binary"
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
	if data[0] >= 16 || data[1] >= 16 {
		return nil, fmt.Errorf("invalid texture dimension exponents %d,%d", data[0], data[1])
	}
	width, height := 1<<data[0], 1<<data[1]
	stride := 0
	switch data[2] {
	case 0:
		stride = 3
	case 1, 2:
		stride = 4
	case 16:
		stride = 2
	default:
		return nil, fmt.Errorf("unsupported texture encoding %d", data[2])
	}
	want := 12 + uint64(width)*uint64(height)*uint64(stride)
	if uint64(len(data)) != want {
		return nil, fmt.Errorf("texture dimensions %dx%d require %d bytes, got %d", width, height, want, len(data))
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			i := 12 + (y*width+x)*stride
			pixel := color.NRGBA{}
			switch data[2] {
			case 0:
				pixel = color.NRGBA{R: data[i], G: data[i+1], B: data[i+2], A: 255}
			case 1, 2:
				pixel = color.NRGBA{R: data[i], G: data[i+1], B: data[i+2], A: data[i+3]}
			case 16:
				value := binary.LittleEndian.Uint16(data[i : i+2])
				pixel = color.NRGBA{R: uint8(value>>12) * 17, G: uint8(value>>8&15) * 17, B: uint8(value>>4&15) * 17, A: uint8(value&15) * 17}
			}
			img.SetNRGBA(x, y, pixel)
		}
	}
	return img, nil
}

func EncodePNG(w io.Writer, img image.Image) error {
	return png.Encode(w, img)
}
