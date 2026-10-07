package main

import "strings"

var nativeWeaponSoundSymbols = [...]string{"SFX_HANDGUN", "SFX_SHOTGUN_1", "SFX_SHOTGUN_2", "SFX_FLAMETHROWER", "SFX_RIFLE_1", "SFX_RIFLE_2", "SFX_UZI", "SFX_MINE_PLANT", "SFX_MINE_EXPLODE", "SFX_MINIGUN_SPIN_UP", "SFX_MINIGUN_SPIN_DOWN", "SFX_MINIGUN", "SFX_GRENADE_EXPLODE", "SFX_GRENADE_THROW", "SFX_ROCKET_LAUNCH", "SFX_ROCKET_EXPLODE", "SFX_HANDGUN_DUAL"}
var nativeWeaponSoundFiles = [...]string{"handgun", "shotgun_1", "shotgun_2", "flamethrower", "rifle_01", "rifle_02", "uzi_looping", "mine_plant", "mine_explode", "minigun_spin_up", "minigun_spin_down", "minigun_looping", "grenade_explode", "grenade_throw", "rocket_launch", "rocket_explode", "handgun_dual"}

func nativeWeaponSoundID(symbol string) int {
	for i, name := range nativeWeaponSoundSymbols {
		if strings.EqualFold(symbol, name) {
			return i
		}
	}
	return -1
}
func nativeWeaponSoundBasename(symbol string) string {
	if id := nativeWeaponSoundID(symbol); id >= 0 {
		return nativeWeaponSoundFiles[id]
	}
	return ""
}
func nativeWeaponSoundLoopSamples(symbol string) int64 {
	switch nativeWeaponSoundID(symbol) {
	case 6:
		return 15456
	case 11:
		return 26904
	}
	return -1
}
func nativeWeaponSoundRange(start, end string, random uint32) string {
	first, last := nativeWeaponSoundID(start), nativeWeaponSoundID(end)
	if first < 0 || last < first {
		return ""
	}
	if first == last {
		return nativeWeaponSoundSymbols[first]
	}
	index := first + int(uint64(random)*uint64(last-first)>>32)
	return nativeWeaponSoundSymbols[index]
}

type weaponAudioQueue []weaponAudioEvent

func (q *weaponAudioQueue) Add(events []weaponAudioEvent) { *q = append(*q, events...) }
func (q *weaponAudioQueue) Drain() []weaponAudioEvent     { events := *q; *q = nil; return events }

type weaponAudioEvent struct {
	Sound            string
	Stop             bool
	Loop             bool
	LoopPointSamples int64
}
type weaponAudioBindings struct{ Start, Shoot, End string }
type weaponAudioState struct {
	Current   string
	Attached  bool
	SpinTimer float32
}

func (s *weaponAudioState) SpinAttach() { s.Current = ""; s.Attached = true; s.SpinTimer = 0 }
func (s *weaponAudioState) SpinShot()   { s.SpinTimer = 0 }
func (s *weaponAudioState) SpinTick(bindings weaponAudioBindings, firing bool, dt float32, endFinished bool) []weaponAudioEvent {
	s.SpinTimer += dt
	if !firing {
		if s.SpinTimer <= -.5 {
			if !weaponAudioValid(s.Current) {
				return nil
			}
			if s.Current == bindings.End {
				if endFinished {
					s.Current = ""
				}
				return nil
			}
		} else {
			s.SpinTimer -= dt + dt
			if s.SpinTimer < -.5 {
				s.SpinTimer = -.5
				return nil
			}
			if s.Current == bindings.End {
				return nil
			}
		}
	}
	return s.SpinUpdate(bindings, firing, s.SpinTimer, endFinished)
}

func weaponAudioValid(sound string) bool { return sound != "" && sound != "0" }
func weaponAudioPlay(sound string) []weaponAudioEvent {
	if !weaponAudioValid(sound) {
		return nil
	}
	point := nativeWeaponSoundLoopSamples(sound)
	return []weaponAudioEvent{{Sound: sound, Loop: point >= 0, LoopPointSamples: point}}
}
func weaponAudioShot(bindings weaponAudioBindings) []weaponAudioEvent {
	return weaponAudioPlay(bindings.Shoot)
}
func weaponAudioRangeShot(bindings weaponAudioBindings, selectNativeRange func(string, string) string) []weaponAudioEvent {
	if selectNativeRange == nil {
		return nil
	}
	return weaponAudioPlay(selectNativeRange(bindings.Start, bindings.End))
}
func (s *weaponAudioState) Attach(bindings weaponAudioBindings) []weaponAudioEvent {
	s.Attached = true
	s.Current = ""
	return weaponAudioPlay(bindings.Start)
}
func (s *weaponAudioState) ManagedUpdate(bindings weaponAudioBindings, finished bool) []weaponAudioEvent {
	if !s.Attached || !finished {
		return nil
	}
	s.Current = bindings.Shoot
	return weaponAudioPlay(bindings.Shoot)
}
func (s *weaponAudioState) Detach(bindings weaponAudioBindings) []weaponAudioEvent {
	var events []weaponAudioEvent
	if s.Attached {
		events = weaponAudioPlay(bindings.End)
	}
	if weaponAudioValid(s.Current) {
		events = append(events, weaponAudioEvent{Sound: s.Current, Stop: true})
	}
	s.Attached = false
	s.Current = ""
	return events
}
func (s *weaponAudioState) LatchedShot(bindings weaponAudioBindings) []weaponAudioEvent {
	if weaponAudioValid(s.Current) {
		return nil
	}
	s.Current = bindings.Shoot
	return weaponAudioPlay(bindings.Shoot)
}
func (s *weaponAudioState) ReleaseLatched(bindings weaponAudioBindings) []weaponAudioEvent {
	if !weaponAudioValid(s.Current) {
		return nil
	}
	s.Current = ""
	if !weaponAudioValid(bindings.Shoot) {
		return nil
	}
	return []weaponAudioEvent{{Sound: bindings.Shoot, Stop: true}}
}
func (s *weaponAudioState) SpinUpdate(bindings weaponAudioBindings, firing bool, nativeTimer float32, endFinished bool) []weaponAudioEvent {
	wanted := bindings.End
	if firing {
		wanted = bindings.Shoot
		if nativeTimer < 0 {
			wanted = bindings.Start
		}
	} else if nativeTimer <= -.5 && s.Current == bindings.End {
		if endFinished {
			s.Current = ""
		}
		return nil
	}
	if wanted == s.Current {
		return nil
	}
	var events []weaponAudioEvent
	if weaponAudioValid(s.Current) {
		events = append(events, weaponAudioEvent{Sound: s.Current, Stop: true})
	}
	s.Current = wanted
	return append(events, weaponAudioPlay(wanted)...)
}
