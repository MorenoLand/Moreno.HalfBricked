package main

import (
	"math"
	"testing"
)

func TestNativeMenuZombieOrbitAndRotation(t *testing.T) {
	for _, scale := range []float32{.75, 1} {
		for _, phase := range []float32{0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 8 * math.Pi} {
			m := newNativeMenuZombieMotion(257, 196, 9, scale, phase)
			m.Progress = 1
			m.tick(0)
			if math.Abs(math.Hypot(float64(m.X-257), float64(m.Y-196))-float64(10*scale)) > .00004 || m.Z != 9 {
				t.Fatalf("orbit scale=%v phase=%v: %+v", scale, phase, m)
			}
			want := float64(float32(math.Sin(float64(float32(phase*1.75))))) * float64(float32(.15)) * 180 / math.Pi
			if math.Abs(float64(m.RotationDegrees)-want) > .000002 {
				t.Fatalf("rotation phase=%v: got %v want %v", phase, m.RotationDegrees, want)
			}
			if phase == 0 && (m.X != 257 || m.Y != 196+10*scale) {
				t.Fatalf("phase zero must start below the anchor: %+v", m)
			}
		}
	}
}
func TestNativeMenuZombieEntryAndDelayedTransition(t *testing.T) {
	m := newNativeMenuZombieMotion(81, 192, 0, .75, 2)
	m.setState(nativeMenuZombieEntering)
	if m.Phase != 2 || m.Progress != float32(1.0/60.0)*4 || m.X != 81 || !m.Visible {
		t.Fatalf("entry must retain phase and consume a fixed tick: %+v", m)
	}
	wantScale := .75 * math.Sin(float64(float32(float64(m.Progress)*math.Pi*.5)))
	if math.Abs(float64(m.Scale)-wantScale) > .0000001 {
		t.Fatalf("entry scale got %v want %v", m.Scale, wantScale)
	}
	m.tick(.25)
	if m.Progress != 1 || m.State != nativeMenuZombieEntering || m.Scale != .75 {
		t.Fatalf("arrival transitions on the following call: %+v", m)
	}
	phase := float32(m.Phase + float32(float32(.02)*float32(.35)))
	phase = float32(phase + float32(float32(1.0/60.0)*float32(.35)))
	m.tick(.02)
	if m.State != nativeMenuZombieIdle || m.Phase != phase {
		t.Fatalf("idle transition must consume its nested fixed tick: %+v", m)
	}
}
func TestNativeMenuZombieExitReentryAndClickGate(t *testing.T) {
	m := newNativeMenuZombieMotion(257, 196, 0, 1, 1)
	m.Progress = 1
	m.setState(nativeMenuZombieLeaving)
	if m.Progress != float32(1)-float32(1.0/60.0)*4 || m.State != nativeMenuZombieLeaving {
		t.Fatalf("exit fixed tick: %+v", m)
	}
	progress, phase := m.Progress, m.Phase
	m.setState(nativeMenuZombieEntering)
	if m.Progress < progress || m.Phase <= phase {
		t.Fatalf("reentry must preserve phase and progress: %+v", m)
	}
	m.setState(nativeMenuZombieClicked)
	x, y, rotation, phase := m.X, m.Y, m.RotationDegrees, m.Phase
	m.tick(1)
	if m.X != x || m.Y != y || m.RotationDegrees != rotation || m.Phase != phase {
		t.Fatalf("clicked group must retain its transform: %+v", m)
	}
	m.setState(nativeMenuZombieLeaving)
	m.tick(1)
	if m.Progress != 0 || !m.Visible || m.State != nativeMenuZombieLeaving || m.Scale != 0 {
		t.Fatalf("zero scale precedes hide transition: %+v", m)
	}
	m.tick(.02)
	if m.State != nativeMenuZombieHidden || m.Visible {
		t.Fatalf("hide transition: %+v", m)
	}
}
func TestNativeMenuZombiePhaseIsNotWrappedOrFrameCounted(t *testing.T) {
	m := newNativeMenuZombieMotion(0, 0, 0, 1, 100)
	m.Progress = 1
	for _, dt := range []float32{.01, .027, .004} {
		want := float32(m.Phase + float32(dt*float32(.35)))
		m.tick(dt)
		if m.Phase != want || m.Phase < 100 {
			t.Fatalf("delta and unwrapped phase: %+v", m)
		}
	}
}
