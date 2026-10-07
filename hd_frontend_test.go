package main

import (
	"image"
	"testing"
)

func TestHDSourceCropsPreserveLogicalRegions(t *testing.T) {
	for _, entry := range []struct{ sd, hd image.Rectangle }{{image.Rect(0, 0, 128, 80), image.Rect(0, 0, 256, 160)}, {image.Rect(0, 0, 64, 64), image.Rect(0, 0, 128, 128)}, {image.Rect(0, 92, 128, 114), image.Rect(0, 184, 256, 228)}} {
		if resolutionRect(entry.sd, 1) != entry.sd || resolutionRect(entry.sd, 2) != entry.hd {
			t.Fatal("variant changed logical crop")
		}
	}
}
