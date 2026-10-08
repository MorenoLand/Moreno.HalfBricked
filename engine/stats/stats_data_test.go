package stats

import "testing"

func TestStatsNativeDefaults(t *testing.T) {
	d := NewStatsData()
	if d.ZombiesKilled != 0 || d.BestStoryScore != 0 || d.BestTRexTime != -1 || d.BestBonusTime != -1 {
		t.Fatal(d)
	}
	if d.BossTotal() != 0 {
		t.Fatal("unset boss times contributed")
	}
	d.BestTRexTime, d.BestBonusTime = 12, 24
	if d.BossTotal() != 36 {
		t.Fatal(d.BossTotal())
	}
}
func TestStatsNativeFormatting(t *testing.T) {
	for input, expected := range map[int32]string{-1: "Not Set", 0: "0", 1234567: "1,234,567", -1234: "-1,234"} {
		if got := StatsInteger(input); got != expected {
			t.Fatalf("%d: %q", input, got)
		}
	}
	if got := StatsTime(3661); got != "                   1:01:01" {
		t.Fatal(got)
	}
	if StatsTime(-1) != "Not Set" {
		t.Fatal("sentinel")
	}
}
