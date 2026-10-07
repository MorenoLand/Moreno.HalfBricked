package main

import "testing"

func TestStatsNativeDefaults(t *testing.T) {
	d := newStatsData()
	if d.ZombiesKilled != 0 || d.BestStoryScore != 0 || d.BestTRexTime != -1 || d.BestBonusTime != -1 {
		t.Fatal(d)
	}
	if d.bossTotal() != 0 {
		t.Fatal("unset boss times contributed")
	}
	d.BestTRexTime, d.BestBonusTime = 12, 24
	if d.bossTotal() != 36 {
		t.Fatal(d.bossTotal())
	}
}
func TestStatsNativeFormatting(t *testing.T) {
	for input, expected := range map[int32]string{-1: "Not Set", 0: "0", 1234567: "1,234,567", -1234: "-1,234"} {
		if got := statsInteger(input); got != expected {
			t.Fatalf("%d: %q", input, got)
		}
	}
	if got := statsTime(3661); got != "                   1:01:01" {
		t.Fatal(got)
	}
	if statsTime(-1) != "Not Set" {
		t.Fatal("sentinel")
	}
}
