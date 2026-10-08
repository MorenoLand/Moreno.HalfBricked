package weapons

import "strings"

var nativeWeaponSoundSymbols = [...]string{"SFX_HANDGUN", "SFX_SHOTGUN_1", "SFX_SHOTGUN_2", "SFX_FLAMETHROWER", "SFX_RIFLE_1", "SFX_RIFLE_2", "SFX_UZI", "SFX_MINE_PLANT", "SFX_MINE_EXPLODE", "SFX_MINIGUN_SPIN_UP", "SFX_MINIGUN_SPIN_DOWN", "SFX_MINIGUN", "SFX_GRENADE_EXPLODE", "SFX_GRENADE_THROW", "SFX_ROCKET_LAUNCH", "SFX_ROCKET_EXPLODE", "SFX_HANDGUN_DUAL"}
var nativeWeaponSoundFiles = [...]string{"handgun", "shotgun_1", "shotgun_2", "flamethrower", "rifle_01", "rifle_02", "uzi_looping", "mine_plant", "mine_explode", "minigun_spin_up", "minigun_spin_down", "minigun_looping", "grenade_explode", "grenade_throw", "rocket_launch", "rocket_explode", "handgun_dual"}

func nativeWeaponSoundID(symbol string) int {
	if strings.EqualFold(symbol, "SFX_DUAL_PISTOL") { // the Peacemakers' XML name for the native handgun_dual sound
		symbol = "SFX_HANDGUN_DUAL"
	}
	for i, name := range nativeWeaponSoundSymbols {
		if strings.EqualFold(symbol, name) {
			return i
		}
	}
	return -1
}
func NativeWeaponSoundBasename(symbol string) string {
	switch strings.ToUpper(symbol) { // the buzzsaw's clips ship as chainsaw_*.ogg
	case "SFX_BUZZSAW_START":
		return "chainsaw_start"
	case "SFX_BUZZSAW_IDLE":
		return "chainsaw_idle"
	case "SFX_BUZZSAW_DEATH":
		return "chainsaw_stop"
	case "SFX_BUZZSAW_HIT_1":
		return "chainsaw_rev_1"
	case "SFX_BUZZSAW_HIT_2":
		return "chainsaw_rev_2"
	case "SFX_BUZZSAW_HIT_3":
		return "chainsaw_rev_3"
	}
	if id := nativeWeaponSoundID(symbol); id >= 0 {
		return nativeWeaponSoundFiles[id]
	}
	return ""
}
func NativeWeaponSoundLoopSamples(symbol string) int64 {
	if strings.EqualFold(symbol, "SFX_BUZZSAW_IDLE") {
		return 23924 // LOOPSAMPLES comment of chainsaw_idle.ogg (resource loop coordinate)
	}
	switch nativeWeaponSoundID(symbol) {
	case 6:
		return 15456
	case 11:
		return 26904
	}
	return -1
}
func NativeWeaponSoundRange(start, end string, random uint32) string {
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

type WeaponAudioQueue []WeaponAudioEvent

func (q *WeaponAudioQueue) Add(events []WeaponAudioEvent) { *q = append(*q, events...) }
func (q *WeaponAudioQueue) Drain() []WeaponAudioEvent     { events := *q; *q = nil; return events }

type WeaponAudioEvent struct {
	Sound            string
	Stop             bool
	Loop             bool
	LoopPointSamples int64
}
type WeaponAudioBindings struct{ Start, Shoot, End string }
type WeaponAudioState struct {
	Current   string
	Attached  bool
	SpinTimer float32
}

func (s *WeaponAudioState) SpinAttach() { s.Current = ""; s.Attached = true; s.SpinTimer = 0 }
func (s *WeaponAudioState) SpinShot()   { s.SpinTimer = 0 }
func (s *WeaponAudioState) SpinTick(bindings WeaponAudioBindings, firing bool, dt float32, endFinished bool) []WeaponAudioEvent {
	s.SpinTimer += dt
	if !firing {
		if s.SpinTimer <= -.5 {
			if !WeaponAudioValid(s.Current) {
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

func WeaponAudioValid(sound string) bool { return sound != "" && sound != "0" }
func weaponAudioPlay(sound string) []WeaponAudioEvent {
	if !WeaponAudioValid(sound) {
		return nil
	}
	point := NativeWeaponSoundLoopSamples(sound)
	return []WeaponAudioEvent{{Sound: sound, Loop: point >= 0, LoopPointSamples: point}}
}
func weaponAudioShot(bindings WeaponAudioBindings) []WeaponAudioEvent {
	return weaponAudioPlay(bindings.Shoot)
}
func weaponAudioRangeShot(bindings WeaponAudioBindings, selectNativeRange func(string, string) string) []WeaponAudioEvent {
	if selectNativeRange == nil {
		return nil
	}
	return weaponAudioPlay(selectNativeRange(bindings.Start, bindings.End))
}
func (s *WeaponAudioState) Attach(bindings WeaponAudioBindings) []WeaponAudioEvent {
	s.Attached = true
	s.Current = ""
	return weaponAudioPlay(bindings.Start)
}
func (s *WeaponAudioState) ManagedUpdate(bindings WeaponAudioBindings, finished bool) []WeaponAudioEvent {
	if !s.Attached || !finished {
		return nil
	}
	s.Current = bindings.Shoot
	return weaponAudioPlay(bindings.Shoot)
}
func (s *WeaponAudioState) Detach(bindings WeaponAudioBindings) []WeaponAudioEvent {
	var events []WeaponAudioEvent
	if s.Attached {
		events = weaponAudioPlay(bindings.End)
	}
	if WeaponAudioValid(s.Current) {
		events = append(events, WeaponAudioEvent{Sound: s.Current, Stop: true})
	}
	s.Attached = false
	s.Current = ""
	return events
}
func (s *WeaponAudioState) LatchedShot(bindings WeaponAudioBindings) []WeaponAudioEvent {
	if WeaponAudioValid(s.Current) {
		return nil
	}
	s.Current = bindings.Shoot
	return weaponAudioPlay(bindings.Shoot)
}
func (s *WeaponAudioState) ReleaseLatched(bindings WeaponAudioBindings) []WeaponAudioEvent {
	if !WeaponAudioValid(s.Current) {
		return nil
	}
	s.Current = ""
	if !WeaponAudioValid(bindings.Shoot) {
		return nil
	}
	return []WeaponAudioEvent{{Sound: bindings.Shoot, Stop: true}}
}
func (s *WeaponAudioState) SpinUpdate(bindings WeaponAudioBindings, firing bool, nativeTimer float32, endFinished bool) []WeaponAudioEvent {
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
	var events []WeaponAudioEvent
	if WeaponAudioValid(s.Current) {
		events = append(events, WeaponAudioEvent{Sound: s.Current, Stop: true})
	}
	s.Current = wanted
	return append(events, weaponAudioPlay(wanted)...)
}
