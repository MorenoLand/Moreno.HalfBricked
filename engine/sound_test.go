package engine

import "testing"

func TestMusicLoopBoundsUsesFloatStereoBytes(t *testing.T) {
	intro, loop := musicLoopBounds(532640, 10000000)
	if intro != 4261120 || loop != 5738880 {
		t.Fatalf("music loop bounds = %d,%d, want 4261120,5738880", intro, loop)
	}
}
func TestMusicLoopBoundsFallsBackForInvalidPoint(t *testing.T) {
	intro, loop := musicLoopBounds(100, 50)
	if intro != 0 || loop != 50 {
		t.Fatalf("invalid music loop bounds = %d,%d, want 0,50", intro, loop)
	}
}
