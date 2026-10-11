package game

import (
	"image"
	"math"

	"github.com/MorenoLand/Moreno.HalfBricked/engine/content"
	"github.com/MorenoLand/Moreno.HalfBricked/engine/weapons"
	"github.com/hajimehoshi/ebiten/v2"
)

// Off-screen markers (Research/native/offscreen-markers-gamepad-2026-10-10.md).
//
// Zombie arrow ("markerzombie", block +0x4 of the zombie class loaded by v7
// FUN_000a1748 / 1.2.5 FUN_001000d8). Every zombie class entity (types 2..15) carries an
// alpha counter (v7 +0x2c4, 1.2.5 +0x2c8 per camera):
//
//	update  (vtable slot 4, v7 FUN_0009fb10 / 1.2.5 FUN_00101a18):
//	        alpha = max(0, uint(alpha - dt * 1000))            (DAT_0009fee8 / DAT_00101dcc)
//	draw registration (v7 FUN_000a1b08 / 1.2.5 FUN_00102278), state != 6 (not dying), when the
//	        zombie is NOT visible (FUN_000a1968 / FUN_000fea68):
//	        health rule: player health > 0 (+0x3dc / +0x38)  -> alpha = int(alpha + dt * 2000),
//	        capped at 0x96 (DAT_000a2094 / DAT_00102408)
//	        boss rule (v7 FUN_000b5310, FUN_000b696c, FUN_000ba8f8; 1.2.5 types 11..14):
//	        -96 < x < (levelWidthTiles + 3) * 32 instead of the health test.
//	draw   (v7 FUN_000a06e8 / 1.2.5 FUN_000ff56c): while alpha > 0 a square of side
//	        sizeZ * 0.35 (DAT_000a09e0 = DAT_000ff8d4 = 0x3eb33333), colour (225, 225, 225,
//	        alpha), at the zombie's screen position clamped to the viewport rectangle,
//	        rotated by atan2(cam.y - z.y, cam.x - z.x).
const (
	markerAlphaRise = 2000.0 // alpha units per second while off screen
	markerAlphaFall = 1000.0 // alpha units per second always (and per resting frame for pickups)
	markerAlphaCap  = 150    // 0x96

	zombieMarkerScale = 0.35 // DAT_000a09e0 = 0x3eb33333 (1.2.5 DAT_000ff8d4)
	zombieMarkerGrey  = 225  // 0xe1 for r, g and b

	// Boss class rule: the x bound is -3 tiles .. width + 3 tiles. The literals are
	// DAT_000b53f4 = -96.0, DAT_000b53f8 = 3.0, DAT_000b53fc = 32.0 (1.2.5 reads the 32 from a global).
	markerBossBoundTiles = 3
	markerBossTile       = 32.0

	// The pickup arrow is "markerweapon" (v7 static block +0x14, loaded by FUN_00092f8c next to
	// "crate", "pickup_indicator" (+0x10) and "Special_Crate" (+0x18)); FUN_000930c8 / 1.2.5
	// FUN_000effdc draw it.
	pickupMarkerTexture   = "Common0/Textures/markerweapon_SD"
	pickupShadowTexture   = "Common0/Textures/pickup_indicator"
	pickupMarkerPulse     = 0.15 // DAT_0009323c = 0x3e19999a
	pickupMarkerHeightDiv = 250.0
	pickupMarkerBase      = 0.5  // DAT_00093238
	pickupMarkerHeightMul = 0.75 // DAT_00093248
	pickupMarkerOffset    = 0.5  // arrow centre = clamped position + direction * size * 0.5
	pickupIconOffset      = 0.6  // DAT_0009324c: the weapon icon sits at 0.6 of the size
	pickupIconScale       = 0.8  // DAT_00093250
	pickupCrateRaise      = 0.375
	pickupShadowAspect    = 0.666 // DAT_00093208 = 0x3f2a7efa
	pickupShadowAlphaMul  = 253.0 // DAT_0009320c
)

// markerBossClass reports whether the entity type uses the boss x-bound rule instead of
// the player-health rule. 1.2.5 slot 7 of type 11 (FUN_00118f40), 12 (0x00117234) and
// 14 (FUN_0011d534) carry the x test; 10 (FUN_0011be70) and 15 (0x0011a0d8) tail-call the
// health rule FUN_00102278.
func markerBossClass(entityType string) bool {
	switch entityType {
	case "boss_gangster", "boss_egyptian", "boss_samurai", "boss_robot":
		return true
	}
	return false
}

// markerAlphaFalls applies the update-slot decay to an integer alpha (uint(float - dt*1000)
// clamped at 0).
func markerAlphaFalls(alpha int, dt float64) int {
	value := float32(alpha) - float32(dt)*float32(markerAlphaFall)
	if value < 0 {
		return 0
	}
	return int(value)
}

// markerAlphaRises applies the draw-registration rise (int(float + dt*2000), capped at 150).
func markerAlphaRises(alpha int, dt float64) int {
	value := int(float32(alpha) + float32(dt)*float32(markerAlphaRise))
	if value > markerAlphaCap {
		return markerAlphaCap
	}
	return value
}

// markerScreen converts a world point to logical screen coordinates (FUN_000a1968 / FUN_00095ad4).
func (p *playState) markerScreen(x, y float64) (float64, float64) {
	w := p.world
	zoom := w.Zoom
	if zoom <= 0 {
		zoom = 1
	}
	return (x-w.CameraX)*zoom + w.ViewportX, (y-w.CameraY)*zoom + w.ViewportY
}

func (p *playState) markerZoom() float64 {
	if p.world == nil || p.world.Zoom <= 0 {
		return 1
	}
	return p.world.Zoom
}

// markerBoxVisible is the visibility test of FUN_000a1968 / FUN_000fea68: the box of half
// extents (w, h)/2 around the screen point against the 480x320 screen and the viewport
// rectangle. The rectangle at scene +0x28..+0x34 was not recovered, so it is taken as the
// whole logical screen (UNRESOLVED), which makes both tests the same.
func markerBoxVisible(screenX, screenY, width, height float64) bool {
	halfW, halfH := width/2, height/2
	return screenX+halfW > 0 && screenX-halfW < logicalWidth && screenY+halfH > 0 && screenY-halfH < logicalHeight
}

// zombieMarkerSize is the +0x30 half-size pair the marker is scaled from: sizeZ for wave
// zombies, the drawn height otherwise.
func zombieMarkerSize(z *zombieState) float64 {
	if z.native.sizeZ > 0 {
		return z.native.sizeZ
	}
	return z.size.Y
}

// zombieMarkerVisible mirrors FUN_000a1968 for one zombie (its box is size.X by size.Y, the sprite
// is raised by the rex leap lift).
func (p *playState) zombieMarkerVisible(z *zombieState) bool {
	if p.world == nil {
		return true
	}
	sx, sy := p.markerScreen(z.x, z.y-p.rexLift(*z))
	zoom := p.markerZoom()
	return markerBoxVisible(sx, sy, z.size.X*zoom, z.size.Y*zoom)
}

// zombieMarkerRises is the per-class condition under which an off-screen zombie raises its alpha.
func (p *playState) zombieMarkerRises(z *zombieState) bool {
	if entity := p.scriptEntities[z.scriptID]; z.scriptID != 0 && entity != nil && markerBossClass(entity.entityType) {
		bound := float64(p.world.Level.Width+markerBossBoundTiles) * markerBossTile
		return z.x > -markerBossBoundTiles*markerBossTile && z.x < bound
	}
	return p.health > 0
}

// stepZombieMarkers runs the update decay and the off-screen rise of every zombie once per tick.
func (p *playState) stepZombieMarkers(dt float64) {
	if p.world == nil {
		return
	}
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.native.markerAlpha > 0 {
			z.native.markerAlpha = markerAlphaFalls(z.native.markerAlpha, dt)
		}
		if z.dying || z.health <= 0 || z.spawnAway {
			continue
		}
		if !p.zombieMarkerVisible(z) && p.zombieMarkerRises(z) {
			z.native.markerAlpha = markerAlphaRises(z.native.markerAlpha, dt)
		}
	}
}

// zombieMarkerPlacement is the draw half of FUN_000a06e8: the clamped screen position, the side
// and the rotation. cam is the camera centre in world space.
func zombieMarkerPlacement(screenX, screenY, sizeZ, zombieX, zombieY, camX, camY float64) (x, y, side, angle float64) {
	x = math.Max(0, math.Min(logicalWidth, screenX))
	y = math.Max(0, math.Min(logicalHeight, screenY))
	side = float64(float32(sizeZ) * float32(zombieMarkerScale))
	angle = math.Atan2(camY-zombieY, camX-zombieX)
	return x, y, side, angle
}

// markerDraw draws a left-pointing arrow texture centred at (x, y) with the given side,
// rotation and modulation.
func (a *app) markerDraw(screen *ebiten.Image, texture *ebiten.Image, x, y, side, angle float64, grey, alpha float32) {
	b := texture.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || side <= 0 {
		return
	}
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
	options.GeoM.Scale(side/float64(b.Dx()), side/float64(b.Dy()))
	options.GeoM.Rotate(angle)
	options.GeoM.Translate(x, y)
	options.ColorScale.Scale(grey*alpha, grey*alpha, grey*alpha, alpha)
	a.drawImage(screen, texture, options)
}

// markerCameraCentre is the point the rotation is measured from: 1.2.5 FUN_00095aa8 (centre
// plus shake), v7 the clamped centre.
func (p *playState) markerCameraCentre() (float64, float64) {
	return p.scriptGetCameraX(), p.scriptGetCameraY()
}

// drawZombieMarkers draws the "markerzombie" arrow of every zombie whose alpha is above zero.
func (a *app) drawZombieMarkers(screen *ebiten.Image) {
	p := a.play
	if p == nil || p.world == nil {
		return
	}
	var texture *ebiten.Image
	camX, camY := p.markerCameraCentre()
	v7 := p.waveBuild() == content.WaveBuildV7
	for index := range p.zombies {
		z := &p.zombies[index]
		if z.native.markerAlpha <= 0 {
			continue
		}
		if texture == nil {
			loaded, err := a.Texture("Common0/Textures/markerzombie_SD")
			if err != nil {
				return
			}
			texture = loaded
		}
		lift := 0.0
		if v7 { // v7 FUN_000a1968 stores the lifted y, 1.2.5 FUN_000ff56c converts the plain position
			lift = p.rexLift(*z)
		}
		sx, sy := p.markerScreen(z.x, z.y-lift)
		x, y, side, angle := zombieMarkerPlacement(sx, sy, zombieMarkerSize(z), z.x, z.y, camX, camY)
		alpha := float32(z.native.markerAlpha&0xff) / 255
		a.markerDraw(screen, texture, x, y, side, angle, float32(zombieMarkerGrey)/255, alpha)
	}
}

// pickupMarkerVisible mirrors the box test of FUN_000928f4 / FUN_000ee9c0: the crate box of
// side `size`, raised 0.375 of its size.
func (p *playState) pickupMarkerVisible(e *scriptEntity) bool {
	size := pickupDrawSize(e.texture)
	zoom := p.markerZoom()
	sx, sy := p.markerScreen(e.x, e.y-size*pickupCrateRaise)
	return markerBoxVisible(sx, sy, size*zoom, size*zoom)
}

// stepPickupMarkers applies the pickup alpha rules once per tick (after stepPickupDrops):
//
//	bounce branch (every frame the pickup rests, or lands): alpha = max(0, alpha - dt*1000)
//	                (v7 uint(...) truncation, 1.2.5 float)
//	off screen with the HUD visible: alpha += dt*2000, capped at 150
//	                (v7 FUN_000928f4 tail, 1.2.5 FUN_000f05d0).
func (p *playState) stepPickupMarkers(dt float64) {
	if p.world == nil {
		return
	}
	v7 := p.waveBuild() == content.WaveBuildV7
	for _, e := range p.scriptEntities {
		if e == nil || e.kind != "pickup" {
			continue
		}
		grounded := !e.drop || e.landed
		if grounded && e.markerAlpha > 0 {
			if v7 {
				e.markerAlpha = float64(markerAlphaFalls(int(e.markerAlpha), dt))
			} else {
				e.markerAlpha = math.Max(0, e.markerAlpha-float64(float32(dt)*float32(markerAlphaFall)))
			}
		}
		if p.hudVisible && !p.pickupMarkerVisible(e) {
			if v7 {
				e.markerAlpha = float64(markerAlphaRises(int(e.markerAlpha), dt))
			} else {
				e.markerAlpha = math.Min(markerAlphaCap, e.markerAlpha+float64(float32(dt)*float32(markerAlphaRise)))
			}
		}
	}
}

// pickupMarkerGeometry is the draw half of FUN_000930c8: the pulsing size, the arrow centre and the
// icon centre/size, from the pickup height, its box-centre screen position, the angle (16 bit) from
// the pickup to the player and the pulse cosine.
type pickupMarkerGeometry struct {
	side             float64
	arrowX, arrowY   float64
	iconX, iconY     float64
	iconSide         float64
	rotation         float64
	clampedX, clampY float64
}

func pickupMarkerPlacement(height, boxSize, screenX, screenY float64, angle16 uint16, pulseCos float64) pickupMarkerGeometry {
	side := (1 + pulseCos*pickupMarkerPulse*(1-height/pickupMarkerHeightDiv)) *
		(height*pickupMarkerHeightMul/pickupMarkerHeightDiv + pickupMarkerBase) * boxSize
	x := math.Max(0, math.Min(logicalWidth, screenX))
	y := math.Max(0, math.Min(logicalHeight, screenY))
	dx, dy := shakeCos(angle16)*side, shakeSin(angle16)*side
	return pickupMarkerGeometry{
		side:     side,
		arrowX:   x + dx*pickupMarkerOffset,
		arrowY:   y + dy*pickupMarkerOffset,
		iconX:    x + dx*pickupIconOffset,
		iconY:    y + dy*pickupIconOffset,
		iconSide: side * pickupIconScale,
		rotation: float64(angle16) * 2 * math.Pi / 65536,
		clampedX: x, clampY: y,
	}
}

// pickupMarkerPulse is cos(((phase + 0x4000) & 0x7fff) << 1) with phase the scene's 16 bit pulse phase
// (+0x514f4, written by FUN_000867e4 together with the exploding zombie glow).
func pickupMarkerPulseCos(time float64) float64 {
	phase := uint32(explodingGlowPhase(int(time*60 + .5)))
	return shakeCos(uint16(((phase + 0x4000) & 0x7fff) << 1))
}

// drawPickupMarkers draws the "markerweapon" arrow (and the weapon icon on it) of every pickup whose
// alpha is above zero, pointing from the screen edge toward the pickup.
func (a *app) drawPickupMarkers(screen *ebiten.Image) {
	p := a.play
	if p == nil || p.world == nil || !p.hudVisible {
		return
	}
	var arrow *ebiten.Image
	cosine := pickupMarkerPulseCos(p.time)
	for _, e := range p.scriptEntities {
		if e == nil || e.kind != "pickup" || e.markerAlpha <= 0 {
			continue
		}
		if arrow == nil {
			loaded, err := a.Texture(pickupMarkerTexture)
			if err != nil {
				return
			}
			arrow = loaded
		}
		size := pickupDrawSize(e.texture)
		sx, sy := p.markerScreen(e.x, e.y-size*pickupCrateRaise)
		angle := weapons.NativeWeaponDirection(p.x-e.x, p.y-e.y)
		g := pickupMarkerPlacement(e.lift, size, sx, sy, angle, cosine)
		alpha := int(e.markerAlpha)
		a.markerDraw(screen, arrow, g.arrowX, g.arrowY, g.side, g.rotation, 1, float32(alpha&0xff)/255)
		if icon, rect, ok := a.pickupIconSource(e); ok {
			iconAlpha := alpha * 2
			if iconAlpha > 0xfe {
				iconAlpha = 0xff
			}
			options := &ebiten.DrawImageOptions{Filter: ebiten.FilterNearest}
			options.GeoM.Translate(-float64(rect.Dx())/2, -float64(rect.Dy())/2)
			options.GeoM.Scale(g.iconSide/float64(rect.Dx()), g.iconSide/float64(rect.Dy()))
			options.GeoM.Translate(g.iconX, g.iconY)
			options.ColorScale.ScaleAlpha(float32(iconAlpha) / 255)
			a.drawImage(screen, icon.SubImage(rect).(*ebiten.Image), options)
		}
	}
}

// pickupIconSource is the weapon icon cell the crate overlay uses (drawScriptEntity); special
// crates draw their own "?" and have no icon.
func (a *app) pickupIconSource(e *scriptEntity) (*ebiten.Image, image.Rectangle, bool) {
	if pickupCrateTexture(e.texture) == "Common0/Textures/Special_Crate" {
		return nil, image.Rectangle{}, false
	}
	path, columns, col := commonSDTexture(e.texture), 1, 0
	if binding, ok := weapons.PickupParityBinding(e.texture); ok {
		path, columns, col = "Common0/Textures/Weapons_Primary_SD", 8, binding.Cell
		if binding.Secondary {
			path = "Common0/Textures/Weapons_Secondary_SD"
		}
	}
	if cell, ok := pickupPrimaryCell(e.texture); ok {
		path, columns, col = "Common0/Textures/Weapons_Primary_SD", 8, cell
	}
	texture, err := a.Texture(path)
	if err != nil {
		return nil, image.Rectangle{}, false
	}
	b := texture.Bounds()
	cellW := b.Dx() / columns
	if cellW <= 0 || b.Dy() <= 0 {
		return nil, image.Rectangle{}, false
	}
	return texture, image.Rect(b.Min.X+col*cellW, b.Min.Y, b.Min.X+(col+1)*cellW, b.Max.Y), true
}

// pickupShadowGeometry is the else branch of v7 FUN_000930c8 (run through the first of the two
// registrations FUN_000928ac makes): the "pickup_indicator" disc under a pickup of drop height h.
// side = size * (h/250 + 1), height = -0.666 * side (a vertical flip), alpha byte =
// (int)(1 + (1 - h/250) * 253). 1.2.5 FUN_000f05d0 replaces this by the generic Game::Shadow list
// (FUN_000f0b64, alpha 0x80 / 2 and a (1 - h/250) * 1.5 parameter), whose rendering was not traced.
func pickupShadowGeometry(size, height float64) (width, tall float64, alpha uint8) {
	width = (height/pickupMarkerHeightDiv + 1) * size
	tall = width * pickupShadowAspect
	value := float32(1) + (float32(1)-float32(height)/pickupMarkerHeightDiv)*float32(pickupShadowAlphaMul)
	if value > 0 {
		alpha = uint8(int(value))
	}
	return width, tall, alpha
}

// pickupShadowDraws is true on the SD (v7) build only.
func (p *playState) pickupShadowDraws() bool {
	return p != nil && p.waveBuild() == content.WaveBuildV7
}

// drawPickupShadow draws the v7 ground disc of one pickup centred on its position.
func (a *app) drawPickupShadow(screen *ebiten.Image, e *scriptEntity, screenX, screenY, zoom float64) {
	if !a.play.pickupShadowDraws() {
		return
	}
	texture, err := a.Texture(pickupShadowTexture)
	if err != nil {
		return
	}
	size := pickupDrawSize(e.texture)
	width, tall, alpha := pickupShadowGeometry(size, e.lift)
	if alpha == 0 {
		return
	}
	b := texture.Bounds()
	options := &ebiten.DrawImageOptions{Filter: ebiten.FilterLinear}
	options.GeoM.Translate(-float64(b.Dx())/2, -float64(b.Dy())/2)
	options.GeoM.Scale(width*zoom/float64(b.Dx()), tall*zoom/float64(b.Dy()))
	options.GeoM.Translate(screenX, screenY)
	options.ColorScale.ScaleAlpha(float32(alpha) / 255)
	a.drawImage(screen, texture, options)
}

// drawOffscreenMarkers is the single entry point from the HUD pass.
func (a *app) drawOffscreenMarkers(screen *ebiten.Image) {
	if a.play == nil || a.play.world == nil {
		return
	}
	a.drawZombieMarkers(screen)
	a.drawPickupMarkers(screen)
}
