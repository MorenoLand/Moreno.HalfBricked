package engine

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type Capture struct {
	directory string
	every     uint64
	frame     uint64
	lastState string
}

func (c *Capture) Frames() uint64 {
	if c == nil {
		return 0
	}
	return c.frame
}

func NewCapture(directory string, every int) (*Capture, error) {
	if directory == "" {
		return nil, nil
	}
	if every < 0 {
		return nil, fmt.Errorf("capture interval cannot be negative")
	}
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, err
	}
	return &Capture{directory: filepath.Clean(directory), every: uint64(every)}, nil
}

func (c *Capture) Save(screen *ebiten.Image, state string) error {
	if c == nil || screen == nil {
		return nil
	}
	frame := c.frame
	c.frame++
	if state == c.lastState && (c.every == 0 || frame%c.every != 0) {
		return nil
	}
	c.lastState = state
	bounds := screen.Bounds()
	pixels := make([]byte, 4*bounds.Dx()*bounds.Dy())
	screen.ReadPixels(pixels)
	output := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	copy(output.Pix, pixels)
	name := fmt.Sprintf("%06d-%s.png", frame, captureStateName(state))
	path := filepath.Join(c.directory, name)
	temporary := path + ".tmp"
	file, err := os.Create(temporary)
	if err != nil {
		return err
	}
	if err := png.Encode(file, output); err != nil {
		file.Close()
		os.Remove(temporary)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

func captureStateName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "frame"
	}
	var builder strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	return strings.Trim(builder.String(), "-")
}
