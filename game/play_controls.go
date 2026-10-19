package game

import "math"

func (p *playState) secondaryButtonGeometry() (x, y, width, height float64) {
	baseX, baseY := p.rightStickAnchor()
	return p.controlButtonGeometry(baseX, baseY, 416, 208, 20)
}

// primaryButtonGeometry places the primary weapon button above the left stick.
func (p *playState) primaryButtonGeometry() (x, y, width, height float64) {
	baseX, baseY := p.leftStickAnchor()
	x, y, width, height = p.controlButtonGeometry(baseX, baseY, 64, 208, primaryButtonGap)
	return
}

// primaryButtonGap places the weapon button above the left stick ring by the same gap the grenade button has above the
// right one (secondaryButtonGeometry), so the two plates sit at the same height over their sticks.
const primaryButtonGap = 20.0

func (p *playState) controlButtonGeometry(baseX, baseY, defaultX, defaultY, gap float64) (x, y, width, height float64) {
	width, height = p.secondaryControlSize.X, p.secondaryControlSize.Y
	if width <= 0 || height <= 0 {
		width, height = 64, 32
	}
	offset := p.padRingHalfHeight() + gap
	x, y = defaultX, defaultY
	if baseX > 0 && baseY > 0 {
		x, y = baseX, baseY-offset
	}
	x = math.Max(width/2, math.Min(logicalWidth-width/2, x))
	y = math.Max(height/2, math.Min(logicalHeight-height/2, y))
	return
}

// padRingHalfHeight is the drawn stick ring's half height in logical units (see
// drawStick). The ring is a circle of PadRadius screen pixels on desktop and a
// circle of PadRadius*W/480 pixels on mobile, so its logical height depends on the
// window size; placing the button by the raw radius left it floating far above the
// ring in a large window.
func (p *playState) padRingHalfHeight() float64 {
	width, height := float64(p.controlWidth), float64(p.controlHeight)
	if width <= 0 {
		width = logicalWidth
	}
	if height <= 0 {
		height = logicalHeight
	}
	sx, sy := width/logicalWidth, height/logicalHeight
	if p.mobileControls {
		return float64(p.controls.PadRadius) * sx / sy
	}
	return float64(p.controls.PadRadius) * p.padScale(height) / sy
}

// padScale is how much larger than its option value the desktop stick ring and nub are drawn: they grow gently with the
// window height (a fixed size looked tiny in a large window, a ring that grows with the width looked huge). 1 up to a
// 720 px high window, at most 2.5. Touch devices keep the option value in logical units (they scale with the window).
func (p *playState) padScale(height float64) float64 {
	if p.mobileControls || height <= 0 {
		return 1
	}
	return math.Max(1, math.Min(2.5, height/720))
}

// leftStickAnchor mirrors rightStickAnchor for the left (move) stick.
func (p *playState) leftStickAnchor() (float64, float64) {
	baseX, baseY := p.leftBaseX, p.leftBaseY
	if p.stick != 1 {
		if p.scriptThumbStickX[0] != 0 {
			baseX = p.scriptThumbStickX[0]
		}
		if p.scriptThumbStickY[0] != 0 {
			baseY = p.scriptThumbStickY[0]
		}
	}
	return baseX, baseY
}

// rightStickAnchor is where the right stick is drawn: the script-placed centre
// (tutorial) while the stick is idle, otherwise its configured base.
func (p *playState) rightStickAnchor() (float64, float64) {
	baseX, baseY := p.rightBaseX, p.rightBaseY
	if p.stick != 2 {
		if p.scriptThumbStickX[1] != 0 {
			baseX = p.scriptThumbStickX[1]
		}
		if p.scriptThumbStickY[1] != 0 {
			baseY = p.scriptThumbStickY[1]
		}
	}
	return baseX, baseY
}

func (p *playState) configureControls(controls optionsControls, width, height int) {
	if width <= 0 {
		width = logicalWidth
	}
	if height <= 0 {
		height = logicalHeight
	}
	previous := p.controls.PadRadius
	if previous == 0 {
		previous = nativeOptionsDefaults(false).PadRadius
	}
	inset := math.Max(float64(previous)*p.padScale(float64(height))*1.15, 100) // keeps the whole ring on screen
	leftX, rightX := inset*logicalWidth/float64(width), (float64(width)-inset)*logicalWidth/float64(width)
	y := (float64(height) - inset) * logicalHeight / float64(height)
	if controls.Normal {
		p.leftBaseX, p.rightBaseX = leftX, rightX
	} else {
		p.leftBaseX, p.rightBaseX = rightX, leftX
	}
	p.leftBaseY, p.rightBaseY, p.controls = y, y, controls
}
func (p *playState) startControlTouch(x, y float64) {
	width, height := p.controlWidth, p.controlHeight
	if width <= 0 {
		width = logicalWidth
	}
	if height <= 0 {
		height = logicalHeight
	}
	sx, sy := float64(width)/logicalWidth, float64(height)/logicalHeight
	radius := float64(p.controls.PadRadius)
	x = math.Max(radius, math.Min(float64(width)-radius, x*sx)) / sx
	y = math.Max(radius, math.Min(float64(height)-radius, y*sy)) / sy
	left := x < logicalWidth/2
	movement := left == p.controls.Normal
	floating := p.controls.RightFloating
	if left {
		floating = p.controls.LeftFloating
	}
	if movement {
		p.stick = 1
		if floating {
			p.leftBaseX, p.leftBaseY = x, y
		}
	} else {
		p.stick = 2
		if floating {
			p.rightBaseX, p.rightBaseY = x, y
		}
	}
}
func (p *playState) updateControlTouch(x, y float64, width, height int) {
	if width <= 0 {
		width = logicalWidth
	}
	if height <= 0 {
		height = logicalHeight
	}
	sx, sy := float64(width)/logicalWidth, float64(height)/logicalHeight
	leftPhysical := (p.stick == 1) == p.controls.Normal
	floating := p.controls.RightFloating
	if leftPhysical {
		floating = p.controls.LeftFloating
	}
	baseX, baseY := &p.rightBaseX, &p.rightBaseY
	deflectX, deflectY := &p.rightDeflectX, &p.rightDeflectY
	if p.stick == 1 {
		baseX, baseY, deflectX, deflectY = &p.leftBaseX, &p.leftBaseY, &p.leftDeflectX, &p.leftDeflectY
	}
	dx, dy, followX, followY := nativeOptionsStickInput(float32((x-*baseX)*sx), float32((y-*baseY)*sy), p.controls.PadRadius, 1.0/60.0, floating)
	*deflectX, *deflectY = float64(dx), float64(dy)
	*baseX += float64(followX) / sx
	*baseY += float64(followY) / sy
}
