package main

import "testing"

func TestWeaponAudioConfirmedSustain(t *testing.T) {
	for symbol, point := range map[string]int64{"SFX_UZI": 15456, "SFX_MINIGUN": 26904} {
		e := weaponAudioPlay(symbol)
		if len(e) != 1 || !e[0].Loop || e[0].LoopPointSamples != point {
			t.Fatal(e)
		}
	}
	for _, symbol := range []string{"SFX_HANDGUN", "SFX_MINIGUN_SPIN_UP", "SFX_MINIGUN_SPIN_DOWN", "unknown"} {
		e := weaponAudioPlay(symbol)
		if len(e) != 1 || e[0].Loop || e[0].LoopPointSamples != -1 {
			t.Fatal(e)
		}
	}
	var s weaponAudioState
	e := s.LatchedShot(weaponAudioBindings{Shoot: "SFX_UZI"})
	if !e[0].Loop || len(s.LatchedShot(weaponAudioBindings{Shoot: "SFX_UZI"})) != 0 {
		t.Fatal("latched sustain")
	}
}

func TestWeaponAudioSpinNativeTimer(t *testing.T) {
	b := weaponAudioBindings{Start: "up", Shoot: "fire", End: "down"}
	var s weaponAudioState
	s.SpinAttach()
	if s.SpinTimer != 0 {
		t.Fatal("attachment timer")
	}
	if e := s.SpinTick(b, true, .1, false); len(e) != 1 || e[0].Sound != b.Shoot {
		t.Fatal(e)
	}
	s.SpinShot()
	if s.SpinTimer != 0 {
		t.Fatal("shot reset")
	}
	if e := s.SpinTick(b, false, .1, false); len(e) != 2 || s.SpinTimer != -.1 {
		t.Fatal(e, s.SpinTimer)
	}
	if e := s.SpinTick(b, true, .025, false); len(e) != 2 || e[1].Sound != b.Start {
		t.Fatal(e)
	}
	s.Current = b.Shoot
	s.SpinTimer = -.49
	if e := s.SpinTick(b, false, .02, false); len(e) != 0 || s.SpinTimer != -.5 || s.Current != b.Shoot {
		t.Fatal("crossing clamp", e, s)
	}
	s.SpinTimer = -.6
	if e := s.SpinTick(b, false, .02, false); len(e) != 2 || e[1].Sound != b.End {
		t.Fatal(e)
	}
	s.SpinTimer = -.6
	if e := s.SpinTick(b, false, .02, true); len(e) != 0 || s.Current != "" {
		t.Fatal(e, s)
	}
}

func TestNativeWeaponSoundAliasesAndRange(t *testing.T) {
	for symbol, want := range map[string]string{"SFX_UZI": "uzi_looping", "SFX_MINIGUN": "minigun_looping", "SFX_RIFLE_1": "rifle_01", "SFX_RIFLE_2": "rifle_02", "SFX_GRENADE_THROW": "grenade_throw"} {
		if got := nativeWeaponSoundBasename(symbol); got != want {
			t.Fatalf("%s: %s", symbol, got)
		}
	}
	if nativeWeaponSoundBasename("missing") != "" {
		t.Fatal("fabricated alias")
	}
	if got := nativeWeaponSoundRange("SFX_SHOTGUN_1", "SFX_SHOTGUN_2", ^uint32(0)); got != "SFX_SHOTGUN_1" {
		t.Fatal(got)
	}
	var q weaponAudioQueue
	q.Add(weaponAudioShot(weaponAudioBindings{Shoot: "SFX_GRENADE_THROW"}))
	if len(q.Drain()) != 1 || len(q.Drain()) != 0 {
		t.Fatal("queue drain")
	}
}

func TestWeaponAudioShotAndRange(t *testing.T) {
	b := weaponAudioBindings{Start: "SHOTGUN_1", Shoot: "0", End: "SHOTGUN_2"}
	if len(weaponAudioShot(b)) != 0 {
		t.Fatal("zero Shoot must stay silent")
	}
	e := weaponAudioRangeShot(b, func(start, end string) string {
		if start != b.Start || end != b.End {
			t.Fatal("range changed")
		}
		return end
	})
	if len(e) != 1 || e[0].Sound != b.End || e[0].Stop {
		t.Fatal(e)
	}
	if weaponAudioRangeShot(b, nil) != nil {
		t.Fatal("must not invent range ordering")
	}
}
func TestWeaponAudioManagedLifecycle(t *testing.T) {
	b := weaponAudioBindings{Start: "start", Shoot: "shoot", End: "end"}
	var s weaponAudioState
	if e := s.Attach(b); len(e) != 1 || e[0].Sound != "start" {
		t.Fatal(e)
	}
	if e := s.ManagedUpdate(b, false); len(e) != 0 {
		t.Fatal(e)
	}
	if e := s.ManagedUpdate(b, true); len(e) != 1 || e[0].Sound != "shoot" {
		t.Fatal(e)
	}
	if e := s.Detach(b); len(e) != 2 || e[0].Sound != "end" || !e[1].Stop {
		t.Fatal(e)
	}
	if len(s.Detach(b)) != 0 || len(s.ManagedUpdate(b, true)) != 0 {
		t.Fatal("detached sounds")
	}
}
func TestWeaponAudioLatchedShot(t *testing.T) {
	b := weaponAudioBindings{Shoot: "uzi"}
	var s weaponAudioState
	if len(s.LatchedShot(b)) != 1 || len(s.LatchedShot(b)) != 0 {
		t.Fatal("duplicate playback")
	}
	if e := s.ReleaseLatched(b); len(e) != 1 || !e[0].Stop {
		t.Fatal(e)
	}
	if len(s.LatchedShot(b)) != 1 {
		t.Fatal("release did not reset")
	}
}
func TestWeaponAudioSpinTransitions(t *testing.T) {
	b := weaponAudioBindings{Start: "spinup", Shoot: "fire", End: "spindown"}
	var s weaponAudioState
	if e := s.SpinUpdate(b, true, -.25, false); len(e) != 1 || e[0].Sound != b.Start {
		t.Fatal(e)
	}
	if len(s.SpinUpdate(b, true, -.1, false)) != 0 {
		t.Fatal("restarted start")
	}
	if e := s.SpinUpdate(b, true, 0, false); len(e) != 2 || !e[0].Stop || e[1].Sound != b.Shoot {
		t.Fatal(e)
	}
	if e := s.SpinUpdate(b, false, -.1, false); len(e) != 2 || e[1].Sound != b.End {
		t.Fatal(e)
	}
	if len(s.SpinUpdate(b, false, -.5, true)) != 0 || s.Current != "" {
		t.Fatal("end completion")
	}
}
