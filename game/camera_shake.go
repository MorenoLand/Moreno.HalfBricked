package game

import (
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
)

// Native camera shake (libmortargame.so v7): setup FUN_000be1fc, per-frame step
// FUN_000be0d0 (called from the scene update FUN_000c0cec while the remaining
// time at scene+0x1ac38 is above zero), camera composition FUN_000bf23c.
//
//	start(pos, duration, ampX, ampY, angleMul):
//	  angle16 = atan2(camY-posY, camX-posX)            (FUN_001ba6e4, 0x10000 = 360 deg)
//	  target.x = cos(angle16) * 9.0 * ampX
//	  target.y = sin(uint16(angle16*angleMul)) * 9.0 * ampY
//	  remaining = total = duration                     (+0x38, +0x3c)
//	step (every frame while remaining > 0):
//	  remaining -= frameTime
//	  if |target-current|^2 < 16:                     (0x000be1e8 = 16.0)
//	      angle16 += 0x6388 + rng.Bounded(0x38e0)      (16-bit wrap; 0x000be1f4/f8)
//	      m = remaining/total * 9.0                    (0x000be1ec = 9.0)
//	      target = (cos(angle16)*m, sin(angle16)*m)
//	  current += (target-current) * 0.2               (0x000be1f0 = 0x3e4ccccd)
//	camera = clampedFollowBase + current              (FUN_000bf23c, +0x5c/+0x60)
//
// Units are world pixels (the same space as the camera position). The current
// offset is not cleared when the shake runs out; the step simply stops being
// called, exactly as natively. FUN_001ba49c is sin and FUN_001ba4b8 is cos of a
// 4096-entry table indexed by angle16>>4.
const (
	cameraShakeUnit      = 9.0
	cameraShakeReachSq   = 16.0
	cameraShakeEase      = 0.2
	cameraShakeTurn      = 0x6388
	cameraShakeTurnRange = 0x38e0
)

type cameraShake struct {
	remaining, total   float64
	angle              uint16
	targetX, targetY   float64
	currentX, currentY float64
}

func shakeSin(angle uint16) float64 { return math.Sin(2 * math.Pi * float64(angle>>4) / 4096) }
func shakeCos(angle uint16) float64 { return math.Cos(2 * math.Pi * float64(angle>>4) / 4096) }

// start mirrors FUN_000be1fc. camX/camY is the camera position including the
// current shake offset (native +0x1ac5c/+0x1ac60).
func (s *cameraShake) start(originX, originY, camX, camY, duration, ampX, ampY, angleMul float64) {
	s.angle = weapons.NativeWeaponDirection(camX-originX, camY-originY)
	s.targetX = shakeCos(s.angle) * cameraShakeUnit * ampX
	scaled := float64(s.angle) * angleMul
	var angleY uint16
	if scaled > 0 {
		angleY = uint16(int(scaled) & 0xffff)
	}
	s.targetY = shakeSin(angleY) * cameraShakeUnit * ampY
	s.remaining, s.total = duration, duration
}

// step mirrors the shake part of FUN_000c0cec + FUN_000be0d0 and returns the
// current camera offset.
func (s *cameraShake) step(dt float64, rng *weapons.NativeRNG) (float64, float64) {
	if s.remaining > 0 && s.total > 0 {
		s.remaining -= dt
		dx, dy := s.targetX-s.currentX, s.targetY-s.currentY
		if dx*dx+dy*dy < cameraShakeReachSq {
			var random uint32
			if rng != nil {
				random = rng.Bounded(cameraShakeTurnRange)
			}
			s.angle = uint16(int(s.angle) + cameraShakeTurn + int(random))
			m := s.remaining / s.total * cameraShakeUnit
			s.targetX, s.targetY = shakeCos(s.angle)*m, shakeSin(s.angle)*m
		}
		s.currentX += (s.targetX - s.currentX) * cameraShakeEase
		s.currentY += (s.targetY - s.currentY) * cameraShakeEase
	}
	return s.currentX, s.currentY
}

// active reports whether the shake timer is still running.
func (s *cameraShake) active() bool { return s.remaining > 0 }

// startCameraShake is the port entry for every native FUN_000be1fc caller.
func (p *playState) startCameraShake(x, y, duration, ampX, ampY, angleMul float64) {
	camX, camY := p.shake.currentX, p.shake.currentY
	if p.world != nil {
		camX += p.world.CameraX
		camY += p.world.CameraY
	}
	p.shake.start(x, y, camX, camY, duration, ampX, ampY, angleMul)
}

// updateShake advances the shake one frame and returns the camera offset.
func (p *playState) updateShake(dt float64) (float64, float64) {
	return p.shake.step(dt, p.rng)
}
