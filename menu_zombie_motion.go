package main

import "math"

const (
	nativeMenuZombieIdle = iota
	nativeMenuZombieClicked
	nativeMenuZombieEntering
	nativeMenuZombieLeaving
	nativeMenuZombieHidden
)

type nativeMenuZombieMotion struct {
	Phase, Progress, BaseX, BaseY, BaseZ, BaseScale float32
	X, Y, Z, Scale, RotationDegrees                 float32
	State                                           int
	Visible                                         bool
}

func newNativeMenuZombieMotion(x, y, z, scale, phase float32) nativeMenuZombieMotion {
	return nativeMenuZombieMotion{Phase: phase, BaseX: x, BaseY: y, BaseZ: z, BaseScale: scale, X: x, Y: y, Z: z, Scale: scale, Visible: true}
}
func (m *nativeMenuZombieMotion) setState(state int) {
	m.State = state
	switch state {
	case nativeMenuZombieIdle, nativeMenuZombieClicked, nativeMenuZombieEntering:
		m.Visible = true
	case nativeMenuZombieHidden:
		m.Visible = false
	}
	m.tick(float32(1.0 / 60.0))
}
func (m *nativeMenuZombieMotion) tick(dt float32) {
	if m.Progress > 0 && m.State != nativeMenuZombieClicked {
		m.Phase = float32(m.Phase + float32(dt*float32(.35)))
		amplitude := float32(m.BaseScale * 10)
		m.X = float32(m.BaseX - float32(amplitude*float32(math.Sin(float64(m.Phase)))))
		m.Y = float32(m.BaseY + float32(amplitude*float32(math.Cos(float64(m.Phase)))))
		m.Z = m.BaseZ
		angle := float32(float32(math.Sin(float64(float32(m.Phase*1.75)))) * float32(.15))
		m.RotationDegrees = float32(float64(float32(angle*180)) / math.Pi)
	}
	switch m.State {
	case nativeMenuZombieEntering:
		if m.Progress >= 1 {
			m.setState(nativeMenuZombieIdle)
			return
		}
		m.Progress = float32(m.Progress + float32(dt*4))
	case nativeMenuZombieLeaving:
		if m.Progress <= 0 {
			m.setState(nativeMenuZombieHidden)
			return
		}
		m.Progress = float32(m.Progress - float32(dt*4))
	default:
		return
	}
	m.Progress = min(float32(1), max(float32(0), m.Progress))
	argument := float32(float64(m.Progress) * math.Pi * .5)
	m.Scale = float32(float32(math.Sin(float64(argument))) * m.BaseScale)
}
