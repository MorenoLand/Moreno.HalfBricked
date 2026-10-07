package formats

import (
	"bytes"
	"image/color"
	"testing"
)

func TestDecodeNativeTextureEncodings(t *testing.T) {
	for _, entry := range []struct {
		encoding byte
		payload  []byte
		want     color.NRGBA
	}{{0, []byte{12, 34, 56}, color.NRGBA{12, 34, 56, 255}}, {1, []byte{12, 34, 56, 78}, color.NRGBA{12, 34, 56, 78}}, {2, []byte{12, 34, 56, 78}, color.NRGBA{12, 34, 56, 78}}, {16, []byte{0x34, 0x12}, color.NRGBA{17, 34, 51, 68}}, {16, []byte{0xf9, 0xff}, color.NRGBA{255, 255, 255, 153}}, {16, []byte{0x00, 0x00}, color.NRGBA{0, 0, 0, 0}}} {
		data := make([]byte, 12)
		data[2], data[3] = entry.encoding, 0xff
		img, err := DecodeTexture(bytes.NewReader(append(data, entry.payload...)))
		if err != nil || img.Bounds().Dx() != 1 || img.Bounds().Dy() != 1 {
			t.Fatalf("encoding %d: %v", entry.encoding, err)
		}
		if pixel := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); pixel != entry.want {
			t.Fatalf("encoding %d: got %+v want %+v", entry.encoding, pixel, entry.want)
		}
	}
}
func TestDecodeTextureRejectsBadHeaderAndPayload(t *testing.T) {
	for _, data := range [][]byte{nil, make([]byte, 11), append([]byte{16, 0, 1}, make([]byte, 13)...), append([]byte{0, 0, 99}, make([]byte, 13)...), append([]byte{0, 0, 16}, make([]byte, 10)...), append([]byte{0, 0, 16}, make([]byte, 12)...)} {
		if _, err := DecodeTexture(bytes.NewReader(data)); err == nil {
			t.Fatal("invalid texture accepted")
		}
	}
}
