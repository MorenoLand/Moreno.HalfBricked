package main

import "math"

type achievementTracking struct {
	previousKills, stationaryBaseline int32
	noKillElapsed                     float32
}
type achievementTrackingFrame struct {
	playerPresent, scriptActive, paused bool
	motionX, motionY, health, dt        float32
	kills                               int32
}
type achievementTrackingSample struct {
	valid                          bool
	stationaryKills, noKillSeconds int32
}

func (t *achievementTracking) Reset() { *t = achievementTracking{} }
func nativeAchievementMotion(x, y float32) (float32, float32) {
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
func (t *achievementTracking) Update(frame achievementTrackingFrame) achievementTrackingSample {
	if !frame.playerPresent {
		return achievementTrackingSample{}
	}
	if frame.motionX != 0 || frame.motionY != 0 || frame.scriptActive || frame.health <= 0 {
		t.stationaryBaseline = frame.kills
	}
	stationaryKills := frame.kills - t.stationaryBaseline
	if t.previousKills != frame.kills || frame.scriptActive || frame.paused || frame.health <= 0 {
		t.noKillElapsed = 0
	}
	t.noKillElapsed += frame.dt
	t.previousKills = frame.kills
	return achievementTrackingSample{valid: true, stationaryKills: stationaryKills, noKillSeconds: int32(t.noKillElapsed)}
}
func achievementTrackingSupported(kind, check, specific string) bool {
	return kind == "SPECIFIC" && check == "ge" && (specific == "no_move" || specific == "no_kills")
}
func (s achievementTrackingSample) Meets(kind, check, specific string, total int) bool {
	if !s.valid || !achievementTrackingSupported(kind, check, specific) {
		return false
	}
	value := s.stationaryKills
	if specific == "no_kills" {
		value = s.noKillSeconds
	}
	return int64(value) >= int64(total)
}
