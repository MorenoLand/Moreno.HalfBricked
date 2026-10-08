package achievements

import "math"

type AchievementTracking struct {
	previousKills, StationaryBaseline int32
	noKillElapsed                     float32
}
type AchievementTrackingFrame struct {
	PlayerPresent, ScriptActive, paused bool
	MotionX, MotionY, Health, Dt        float32
	Kills                               int32
}
type achievementTrackingSample struct {
	valid                          bool
	stationaryKills, noKillSeconds int32
}

func (t *AchievementTracking) Reset() { *t = AchievementTracking{} }
func NativeAchievementMotion(x, y float32) (float32, float32) {
	x, y = x*2, y*2
	length := float32(math.Sqrt(float64(x*x + y*y)))
	if length > 1 {
		x, y = x/length, y/length
	}
	x, y = x*math.Float32frombits(0x3f4f5c29), y*math.Float32frombits(0x3f4f5c29)
	if x*x+y*y <= math.Float32frombits(0x3d4ccccd) {
		return 0, 0
	}
	return x, y
}
func (t *AchievementTracking) Update(frame AchievementTrackingFrame) achievementTrackingSample {
	if !frame.PlayerPresent {
		return achievementTrackingSample{}
	}
	if frame.MotionX != 0 || frame.MotionY != 0 || frame.ScriptActive || frame.Health <= 0 {
		t.StationaryBaseline = frame.Kills
	}
	stationaryKills := frame.Kills - t.StationaryBaseline
	if t.previousKills != frame.Kills || frame.ScriptActive || frame.paused || frame.Health <= 0 {
		t.noKillElapsed = 0
	}
	t.noKillElapsed += frame.Dt
	t.previousKills = frame.Kills
	return achievementTrackingSample{valid: true, stationaryKills: stationaryKills, noKillSeconds: int32(t.noKillElapsed)}
}
func AchievementTrackingSupported(kind, check, specific string) bool {
	return kind == "SPECIFIC" && check == "ge" && (specific == "no_move" || specific == "no_kills")
}
func (s achievementTrackingSample) Meets(kind, check, specific string, total int) bool {
	if !s.valid || !AchievementTrackingSupported(kind, check, specific) {
		return false
	}
	value := s.stationaryKills
	if specific == "no_kills" {
		value = s.noKillSeconds
	}
	return int64(value) >= int64(total)
}
