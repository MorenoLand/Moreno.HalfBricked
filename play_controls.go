package main

import "math"

func (p *playState) secondaryButtonGeometry() (x, y, width, height float64) {
	width, height = p.secondaryControlSize.X, p.secondaryControlSize.Y
	if width <= 0 || height <= 0 {
		width, height = 64, 32
	}
	sx, sy := 1.0, 1.0
	if p.controlWidth > 0 {
		sx = float64(p.controlWidth) / logicalWidth
	}
	if p.controlHeight > 0 {
		sy = float64(p.controlHeight) / logicalHeight
	}
	offset := float64(p.controls.PadRadius) + 20
	if !p.mobileControls {
		width, height, offset = width/sx, height/sy, offset/sy
	}
	x, y = 416, 208
	if p.rightBaseX > 0 && p.rightBaseY > 0 {
		x, y = p.rightBaseX, p.rightBaseY-offset
	}
	x = math.Max(width/2, math.Min(logicalWidth-width/2, x))
	y = math.Max(height/2, math.Min(logicalHeight-height/2, y))
	return
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
	inset := math.Max(float64(previous), 100)
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
