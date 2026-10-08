package game

import (
	"testing"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/formats"
)

func TestAchievementScrollbarThumbRoundTrips(t *testing.T) {
	const total = 33
	top, height := achievementScrollbarThumb(total, 0)
	if top != scrollbarTop || height < 24 {
		t.Fatalf("thumb at start: %v %v", top, height)
	}
	if bottom, h := achievementScrollbarThumb(total, total-achievementRowsVisible); bottom+h != scrollbarBottom {
		t.Fatalf("thumb at end ends at %v", bottom+h)
	}
	for offset := 0; offset <= total-achievementRowsVisible; offset++ {
		top, height := achievementScrollbarThumb(total, offset)
		if got := achievementOffsetForThumb(total, top+height/2); got != offset {
			t.Fatalf("offset %d round-trips to %d", offset, got)
		}
	}
}

func TestAchievementDisplayDescription(t *testing.T) {
	story := formats.Achievement{DescriptionUnlocked: "You finished World 1."}
	if got := story.DisplayDescription(false); got != "You finished World 1." {
		t.Fatalf("locked story entry with only unlocked text: %q", got)
	}
	kills := formats.Achievement{DescriptionLocked: "Kill 50", DescriptionUnlocked: "You killed 50"}
	if kills.DisplayDescription(false) != "Kill 50" || kills.DisplayDescription(true) != "You killed 50" {
		t.Fatal("locked/unlocked text not selected")
	}
}
