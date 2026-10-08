package achievements

import (
	"math"
	"testing"
)

func TestAchievementTrackingNativeThresholds(t *testing.T) {
	var tracker AchievementTracking
	frame := AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: 19, Dt: .25}
	sample := tracker.Update(frame)
	if sample.Meets("SPECIFIC", "ge", "no_move", 20) {
		t.Fatal("19 stationary kills awarded CAMPER")
	}
	frame.Kills = 20
	sample = tracker.Update(frame)
	if !sample.Meets("SPECIFIC", "ge", "no_move", 20) {
		t.Fatal("20 stationary kills did not award CAMPER")
	}
	tracker.Reset()
	frame.Kills, frame.Dt = 0, 29.75
	sample = tracker.Update(frame)
	if sample.noKillSeconds != 29 || sample.Meets("SPECIFIC", "ge", "no_kills", 30) {
		t.Fatal("fractional seconds were rounded up")
	}
	frame.Dt = .25
	sample = tracker.Update(frame)
	if !sample.Meets("SPECIFIC", "ge", "no_kills", 30) {
		t.Fatal("30 elapsed seconds did not award CARDIO")
	}
}
func TestAchievementTrackingMovementHasNoTolerance(t *testing.T) {
	for _, motion := range []AchievementTrackingFrame{{MotionX: math.SmallestNonzeroFloat32}, {MotionY: -math.SmallestNonzeroFloat32}} {
		var tracker AchievementTracking
		tracker.Update(AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: 19})
		motion.PlayerPresent, motion.Health, motion.Kills = true, 1, 20
		if sample := tracker.Update(motion); sample.stationaryKills != 0 {
			t.Fatal("movement failed to exclude same-frame kills")
		}
		if sample := tracker.Update(AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: 39}); sample.stationaryKills != 19 {
			t.Fatal("stationary baseline was lost")
		}
		if sample := tracker.Update(AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: 40}); !sample.Meets("SPECIFIC", "ge", "no_move", 20) {
			t.Fatal("post-movement stationary streak did not qualify")
		}
	}
}
func TestAchievementTrackingResetAddsCurrentDelta(t *testing.T) {
	for _, reset := range []AchievementTrackingFrame{{Kills: 8, Health: 1}, {Kills: 6, Health: 1}, {Kills: 7, Health: 1, ScriptActive: true}, {Kills: 7, Health: 1, paused: true}, {Kills: 7, Health: 0}, {Kills: 7, Health: -1}} {
		tracker := AchievementTracking{previousKills: 7, StationaryBaseline: 2, noKillElapsed: 29.75}
		reset.PlayerPresent, reset.Dt = true, .25
		sample := tracker.Update(reset)
		if tracker.noKillElapsed != .25 || sample.noKillSeconds != 0 || tracker.previousKills != reset.Kills {
			t.Fatal("reset did not precede dt addition and kill snapshot")
		}
		if reset.ScriptActive || reset.Health <= 0 {
			if sample.stationaryKills != 0 {
				t.Fatal("script/death did not reset stationary baseline")
			}
		} else if sample.stationaryKills != reset.Kills-2 {
			t.Fatal("kill change/pause incorrectly reset stationary baseline")
		}
	}
}
func TestAchievementTrackingMovementDoesNotResetTimer(t *testing.T) {
	tracker := AchievementTracking{previousKills: 5, noKillElapsed: 29.75}
	sample := tracker.Update(AchievementTrackingFrame{PlayerPresent: true, MotionX: 1, Health: 1, Kills: 5, Dt: .25})
	if !sample.Meets("SPECIFIC", "ge", "no_kills", 30) || sample.stationaryKills != 0 {
		t.Fatal("movement affected the wrong condition")
	}
}
func TestAchievementTrackingMissingPlayerPreservesState(t *testing.T) {
	tracker := AchievementTracking{previousKills: 7, StationaryBaseline: 2, noKillElapsed: 29.75}
	before := tracker
	sample := tracker.Update(AchievementTrackingFrame{Kills: 100, Dt: 30, ScriptActive: true, paused: true})
	if tracker != before || sample.valid || sample.Meets("SPECIFIC", "ge", "no_kills", 0) {
		t.Fatal("missing player changed state or emitted an award")
	}
	sample = tracker.Update(AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: 7, Dt: .25})
	if !sample.Meets("SPECIFIC", "ge", "no_kills", 30) {
		t.Fatal("missing player discarded elapsed time")
	}
}
func TestAchievementTrackingFloat32FrameAccumulation(t *testing.T) {
	var tracker AchievementTracking
	frame := AchievementTrackingFrame{PlayerPresent: true, Health: 1, Dt: float32(1.0 / 60.0)}
	var sample achievementTrackingSample
	for i := 0; i < 1800; i++ {
		sample = tracker.Update(frame)
	}
	if sample.noKillSeconds != 29 || sample.Meets("SPECIFIC", "ge", "no_kills", 30) {
		t.Fatal("float32 accumulation was replaced by frame counting")
	}
	sample = tracker.Update(frame)
	if !sample.Meets("SPECIFIC", "ge", "no_kills", 30) {
		t.Fatal("first float32 threshold crossing was missed")
	}
}
func TestAchievementTrackingResetAndSignedDelta(t *testing.T) {
	tracker := AchievementTracking{previousKills: 50, StationaryBaseline: math.MaxInt32, noKillElapsed: 99}
	sample := tracker.Update(AchievementTrackingFrame{PlayerPresent: true, Health: 1, Kills: math.MinInt32})
	if sample.stationaryKills != 1 {
		t.Fatal("32-bit stationary subtraction changed")
	}
	tracker.Reset()
	if tracker != (AchievementTracking{}) {
		t.Fatal("native reset did not clear all three fields")
	}
}
func TestAchievementTrackingRejectsUnrecoveredScopes(t *testing.T) {
	sample := achievementTrackingSample{valid: true, stationaryKills: 100, noKillSeconds: 100}
	for _, args := range [][3]string{{"KILLS", "ge", "no_move"}, {"SPECIFIC", "e", "no_kills"}, {"SPECIFIC", "ge", "pistol_only"}, {"SPECIFIC", "ge", "total"}, {"SPECIFIC", "ge", "wave"}} {
		if AchievementTrackingSupported(args[0], args[1], args[2]) || sample.Meets(args[0], args[1], args[2], 0) {
			t.Fatal("unrecovered scope was evaluated")
		}
	}
}
