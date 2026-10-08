package game

import "testing"

func TestHoverAmountEasesInAndOut(t *testing.T) {
	a := &app{}
	var prev float64
	for i := 0; i < 30; i++ {
		v := a.hoverAmount("row", true)
		if v < prev {
			t.Fatal("hover should only grow while hovered")
		}
		prev = v
	}
	if prev != 1 {
		t.Fatalf("a hovered row should settle at 1, got %v", prev)
	}
	for i := 0; i < 40; i++ {
		prev = a.hoverAmount("row", false)
	}
	if prev != 0 {
		t.Fatalf("an unhovered row should settle at 0, got %v", prev)
	}
}
